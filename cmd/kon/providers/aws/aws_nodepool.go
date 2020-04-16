package aws

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/pricing"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cast"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	kaws "github.com/davidzhao/konstellation/pkg/cloud/aws"
	"github.com/davidzhao/konstellation/pkg/resources"
)

func (a *AWSManager) ConfigureNodepool(cc *v1alpha1.ClusterConfig) (np *v1alpha1.Nodepool, err error) {
	sess, err := a.awsSession()
	if err != nil {
		return
	}

	awsConf := cc.Spec.AWSConfig
	if awsConf == nil {
		err = fmt.Errorf("Invalid condition, couldn't find AWS config")
		return
	}
	nps := v1alpha1.NodepoolSpec{
		AWS: &v1alpha1.NodePoolAWS{},
	}
	iamSvc := kaws.NewIAMService(sess)
	ec2Svc := ec2.New(sess)

	roleRes, err := iamSvc.IAM.GetRole(&iam.GetRoleInput{
		RoleName: aws.String(kaws.EKSNodeRole),
	})
	if err != nil {
		return
	}
	nps.AWS.RoleARN = *roleRes.Role.Arn

	// subnet ids, with public/private networks, allocate into private
	var targetSubnets []*v1alpha1.AWSSubnet
	if len(awsConf.PrivateSubnets) > 0 {
		targetSubnets = awsConf.PrivateSubnets
	} else {
		targetSubnets = awsConf.PublicSubnets
	}
	for _, subnet := range targetSubnets {
		nps.AWS.SubnetIds = append(nps.AWS.SubnetIds, subnet.SubnetId)
	}

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
		Validate: utils.ValidateKubeName,
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

	// configure node connection
	if len(cc.Spec.AWSConfig.PrivateSubnets) == 0 {
		// remote access is only possible when VPC is public-only
		connectionPrompt := utils.NewPromptSelect(
			"Allow remote access to nodes from the internet?",
			[]string{"allow", "disallow"},
		)
		idx, _, err = connectionPrompt.Run()
		if err != nil {
			return
		}
		nps.AWS.ConnectFromAnywhere = idx == 0
	}

	if !nps.AWS.ConnectFromAnywhere {
		// list security groups
		var securityGroups []*ec2.SecurityGroup
		securityGroups, err = kaws.ListSecurityGroups(ec2Svc, awsConf.Vpc)
		if err != nil {
			return
		}
		sgNames := make([]string, 0, len(securityGroups))
		for _, sg := range securityGroups {
			sgNames = append(sgNames, *sg.GroupName)
		}
		sgPrompt := utils.NewPromptSelect(
			"Security group for connection",
			sgNames,
		)
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
		gpuPrompt := utils.NewPromptSelect(
			"Requires GPU instances",
			[]string{"no", "require GPU"},
		)
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

func (a *AWSManager) promptCreateKeypair(ec2Svc *ec2.EC2, name string) (keyName string, err error) {
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

func (a *AWSManager) promptInstanceType(session *session.Session, gpu bool) (instance *kaws.EC2InstancePricing, err error) {
	// find all ec2 instances and create listing for price
	pricingSvc := pricing.New(session, aws.NewConfig().WithRegion("us-east-1"))
	instances, err := kaws.ListEC2Instances(pricingSvc, a.region, true)
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

	instancePrompt := utils.NewPromptSelect(
		"Instance type to use for nodes",
		instanceLabels,
	)
	instancePrompt.Size = 15
	instancePrompt.Searcher = utils.SearchFuncFor(instanceLabels, true)

	idx, _, err := instancePrompt.Run()
	if err != nil {
		return
	}

	instance = filteredInstances[idx]
	return
}

func (a *AWSManager) promptInstanceSizing() (minNodes int64, maxNodes int64, err error) {
	sizePrompt := promptui.Prompt{
		Label:    "Minimum/initial number of nodes (recommend 3 minimum for a production setup)",
		Validate: utils.ValidateIntWithLimits(2, -1),
	}
	sizeStr, err := sizePrompt.Run()
	if err != nil {
		return
	}
	minNodes = cast.ToInt64(sizeStr)
	sizePrompt = promptui.Prompt{
		Label:    "Maximum number of nodes with autoscale",
		Validate: utils.ValidateIntWithLimits(int(minNodes), -1),
	}
	sizeStr, err = sizePrompt.Run()
	if err != nil {
		return
	}
	maxNodes = cast.ToInt64(sizeStr)
	return
}

func (a *AWSManager) promptConfirmBudget(instance *kaws.EC2InstancePricing, minNodes, maxNodes int64) (bool, error) {
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
