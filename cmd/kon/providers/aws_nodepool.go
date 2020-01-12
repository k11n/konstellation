package providers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/pricing"
	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	kaws "github.com/davidzhao/konstellation/pkg/cloud/aws"
	"github.com/davidzhao/konstellation/pkg/resources"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cast"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a *AWSProvider) ConfigureNodepool(name string) (np *v1alpha1.Nodepool, err error) {
	sess, err := a.awsSession()
	if err != nil {
		return
	}

	nps := v1alpha1.NodepoolSpec{
		AWS: &v1alpha1.NodePoolAWS{},
	}
	eksSvc := kaws.NewEKSService(sess)
	iamSvc := kaws.NewIAMService(sess)
	ec2Svc := ec2.New(sess)

	role, err := a.promptSelectOrCreateNodeRole(iamSvc)
	if err != nil {
		return
	}
	nps.AWS.RoleARN = *role.Arn

	// keypair setup
	kpRes, err := ec2Svc.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{})
	if err != nil {
		return
	}
	keypairs := kpRes.KeyPairs
	keypairNames := make([]string, 0, len(keypairs))
	for _, keypair := range keypairs {
		keypairNames = append(keypairNames, *keypair.KeyName)
	}
	keypairPrompt := promptui.SelectWithAdd{
		Label:    "Keypair (for SSH access into nodes)",
		AddLabel: "Create new keypair",
		Items:    keypairNames,
		Validate: utils.ValidateName,
	}
	idx, keypairName, err := keypairPrompt.Run()
	if err != nil {
		return
	}
	if idx == -1 {
		// create new keypair and save it to ~/.ssh
		nps.AWS.SSHKeypair, err = a.promptCreateKeypair(ec2Svc, keypairName)
		if err != nil {
			return
		}
	} else {
		nps.AWS.SSHKeypair = *keypairs[idx].KeyName
	}

	// load cluster details
	descOut, err := eksSvc.EKS.DescribeCluster(&eks.DescribeClusterInput{
		Name: &name,
	})
	if err != nil {
		return
	}
	nps.AWS.VpcID = *descOut.Cluster.ResourcesVpcConfig.VpcId

	// configure node connection
	connectionPrompt := promptui.Select{
		Label: "Allow connection to nodes from anywhere?",
		Items: []string{"allow", "disallow"},
	}
	idx, _, err = connectionPrompt.Run()
	if err != nil {
		return
	}
	if idx == 0 {
		nps.AWS.ConnectFromAnywhere = true
	} else {
		// list security groups
		var securityGroups []*ec2.SecurityGroup
		securityGroups, err = kaws.ListSecurityGroups(ec2Svc, nps.AWS.VpcID)
		if err != nil {
			return
		}
		sgNames := make([]string, 0, len(securityGroups))
		for _, sg := range securityGroups {
			sgNames = append(sgNames, *sg.GroupName)
		}
		sgPrompt := promptui.Select{
			Label: "Security group for connection",
			Items: sgNames,
		}
		idx, _, err = sgPrompt.Run()
		if err != nil {
			return
		}
		nps.AWS.SecurityGroupId = *securityGroups[idx].GroupId
		nps.AWS.SecurityGroupName = *securityGroups[idx].GroupName
	}

	instanceConfirmed := false
	for !instanceConfirmed {
		// node instance config
		gpuPrompt := promptui.Select{
			Label: "Requires GPU instances",
			Items: []string{"no", "require GPU"},
		}
		idx, _, err = gpuPrompt.Run()
		if err != nil {
			return
		}
		if idx == 1 {
			nps.RequiresGPU = true
		}
		var instance *kaws.EC2InstancePricing
		instance, err = a.promptInstanceType(sess, nps.RequiresGPU)
		if err != nil {
			return
		}
		nps.MachineType = instance.InstanceType

		nps.MinSize, nps.MaxSize, err = a.promptInstanceSizing()
		if err != nil {
			return
		}

		// compute budget and inform
		instanceConfirmed, err = a.promptConfirmBudget(instance, nps.MinSize, nps.MaxSize)
		if err != nil {
			return
		}
	}

	diskPrompt := promptui.Prompt{
		Label:    "Size of root disk (GiB)",
		Default:  "20",
		Validate: utils.ValidateInt,
	}
	sizeStr, err := diskPrompt.Run()
	if err != nil {
		return
	}
	nps.DiskSizeGiB = cast.ToInt(sizeStr)

	autoscalePrompt := promptui.Prompt{
		Label:     "Use autoscaler",
		IsConfirm: true,
		Default:   "y",
	}
	if _, err = autoscalePrompt.Run(); err == nil {
		nps.Autoscale = true
	} else if err != promptui.ErrAbort {
		return
	}

	// fill in GPU details
	if nps.RequiresGPU {
		nps.AWS.AMIType = "AL2_x86_64_GPU"
	} else {
		nps.AWS.AMIType = "AL2_x86_64"
	}

	// confirm creation and execute
	utils.PrintDescStruct(nps)
	createConfirmation := promptui.Prompt{
		Label:     "Create nodegroup",
		IsConfirm: true,
		Default:   "y",
	}
	if _, err = createConfirmation.Run(); err != nil {
		if err == promptui.ErrAbort {
			err = fmt.Errorf("nodepool creation aborted")
		}
		return
	}

	// execute plan & save config
	np = &v1alpha1.Nodepool{
		ObjectMeta: v1.ObjectMeta{
			Name: resources.NodepoolName(),
		},
		Spec: nps,
	}
	return
}

func (a *AWSProvider) promptSelectOrCreateNodeRole(iamSvc *kaws.IAMService) (role *iam.Role, err error) {
	// list all the IAM roles
	roles, err := iamSvc.ListEKSNodeRoles()
	if err != nil {
		return
	}

	if len(roles) == 0 {
		// Create service role
		namePrompt := promptui.Prompt{
			Label:   "Create a new EKS node role",
			Default: "eks-node-role",
		}
		var roleName string
		roleName, err = namePrompt.Run()
		if err != nil {
			return
		}

		role, err = iamSvc.CreateEKSNodeRole(roleName)
		return
	}

	// choose an existing role
	roleNames := make([]string, 0, len(roles))
	for _, role := range roles {
		roleNames = append(roleNames, *role.RoleName)
	}
	sort.Strings(roleNames)
	roleSelect := promptui.Select{
		Label:    "EKS node role name",
		Items:    roleNames,
		Searcher: utils.SearchFuncFor(roleNames, false),
	}
	idx, _, err := roleSelect.Run()
	if err != nil {
		return
	}
	role = roles[idx]

	return
}

func (a *AWSProvider) promptCreateKeypair(ec2Svc *ec2.EC2, name string) (keyName string, err error) {
	res, err := ec2Svc.CreateKeyPair(&ec2.CreateKeyPairInput{
		KeyName: &name,
	})
	if err != nil {
		return
	}
	keyName = *res.KeyName
	// write raw key to ~/.ssh
	homeDir, err := os.UserHomeDir()
	saved := false
	if err == nil {
		printTargetPath := path.Join("~", ".ssh", name+".pem")
		saveTargetPath := path.Join(homeDir, ".ssh", name+".pem")
		savePrompt := promptui.Prompt{
			IsConfirm: true,
			Default:   "y",
			Label:     fmt.Sprintf("Save new keypair to %s", printTargetPath),
		}
		_, err = savePrompt.Run()
		if err == nil {
			err = ioutil.WriteFile(saveTargetPath, []byte(*res.KeyMaterial), 0600)
			if err != nil {
				fmt.Println("Error while saving key:", err)
			} else {
				fmt.Printf("Keypair %s saved to: %s\n", keyName, printTargetPath)
				saved = true
			}
		}
	}
	if !saved {
		utils.PrintImportant(*res.KeyMaterial, "IMPORTANT: Your new keypair is only displayed once.")
	}

	return
}

func (a *AWSProvider) promptInstanceType(session *session.Session, gpu bool) (instance *kaws.EC2InstancePricing, err error) {
	// find all ec2 instances and create listing for price
	pricingSvc := pricing.New(session, aws.NewConfig().WithRegion("us-east-1"))
	conf := config.GetConfig().Clouds.AWS
	instances, err := kaws.ListEC2Instances(pricingSvc, conf.Region, true)
	if err != nil {
		return
	}

	// ask if requires GPU
	filteredInstances := make([]*kaws.EC2InstancePricing, 0, len(instances))
	for _, inst := range instances {
		hasGPU := inst.GPUs > 0
		if hasGPU != gpu {
			continue
		}
		if !kaws.EKSAllowedInstanceSeries[inst.InstanceSeries] {
			continue
		}
		if !kaws.EKSAllowedInstanceSizes[inst.InstanceSize] {
			continue
		}
		filteredInstances = append(filteredInstances, inst)
	}

	instanceLabels := make([]string, 0, len(filteredInstances))
	for _, inst := range filteredInstances {
		var label string
		if gpu {
			// instance type, VCPUs, GPUs, memory, network, price
			label = fmt.Sprintf("%-14v %2v vCPUs    %2v GPUs    Memory: %-7v    Network: %-17v    $%0.2f/hr ($%.0f/mo)",
				inst.InstanceType, inst.VCPUs, inst.GPUs, inst.Memory, inst.NetworkPerformance, inst.OnDemandPriceUSD,
				inst.OnDemandPriceUSD*24*30)
		} else {
			// instance type, VCPUs, memory, network, price
			label = fmt.Sprintf("%-14v %2v vCPUs    Memory: %-7v    Network: %-17v    $%0.2f/hr ($%.0f/mo)",
				inst.InstanceType, inst.VCPUs, inst.Memory, inst.NetworkPerformance, inst.OnDemandPriceUSD,
				inst.OnDemandPriceUSD*24*30)
		}
		instanceLabels = append(instanceLabels, label)
	}

	instancePrompt := promptui.Select{
		Label:    "Instance type to use for nodes",
		Items:    instanceLabels,
		Size:     15,
		Searcher: utils.SearchFuncFor(instanceLabels, true),
	}
	idx, _, err := instancePrompt.Run()
	if err != nil {
		return
	}

	instance = filteredInstances[idx]
	return
}

func (a *AWSProvider) promptInstanceSizing() (minNodes int64, maxNodes int64, err error) {
	sizePrompt := promptui.Prompt{
		Label:    "Minimum/initial number of nodes (recommend 3 minimum for a production setup)",
		Validate: utils.ValidateInt,
	}
	sizeStr, err := sizePrompt.Run()
	if err != nil {
		return
	}
	minNodes = cast.ToInt64(sizeStr)
	sizePrompt.Label = "Maximum number of nodes with autoscale"
	sizeStr, err = sizePrompt.Run()
	if err != nil {
		return
	}
	maxNodes = cast.ToInt64(sizeStr)
	return
}

func (a *AWSProvider) promptConfirmBudget(instance *kaws.EC2InstancePricing, minNodes, maxNodes int64) (bool, error) {
	instanceMonthlyCost := instance.OnDemandPriceUSD * 24 * 30
	minCost := instanceMonthlyCost * float32(minNodes)
	maxCost := instanceMonthlyCost * float32(maxNodes)
	fmt.Printf("Using %d-%d %s nodes will cost between $%0.2f to $%0.2f a month\n",
		minNodes, maxNodes, instance.InstanceType, minCost, maxCost)
	confirmPrompt := promptui.Prompt{
		Label:     "OK to continue",
		IsConfirm: true,
		Default:   "y",
	}
	_, err := confirmPrompt.Run()
	if err == promptui.ErrAbort {
		return false, nil
	} else if err != nil {
		return false, err
	} else {
		return true, nil
	}
}
