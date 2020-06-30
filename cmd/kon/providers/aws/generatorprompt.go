package aws

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/pricing"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cast"
	"github.com/thoas/go-funk"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k11n/konstellation/cmd/kon/config"
	"github.com/k11n/konstellation/cmd/kon/kube"
	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	kaws "github.com/k11n/konstellation/pkg/cloud/aws"
	"github.com/k11n/konstellation/pkg/cloud/types"
	"github.com/k11n/konstellation/pkg/components/ingress"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/version"
)

// generates AWS cluster & nodepool based on prompts to the user
type PromptConfigGenerator struct {
	region      string
	credentials config.AWSCredentials
	session     *session.Session
}

func NewPromptConfigGenerator(region string, credentials config.AWSCredentials) (*PromptConfigGenerator, error) {
	g := &PromptConfigGenerator{
		region:      region,
		credentials: credentials,
	}
	sess, err := sessionForRegion(region)
	if err != nil {
		return nil, err
	}
	g.session = sess
	return g, nil
}

func (g *PromptConfigGenerator) CreateClusterConfig() (cc *v1alpha1.ClusterConfig, err error) {
	as := &v1alpha1.AWSClusterSpec{}
	cc = &v1alpha1.ClusterConfig{}
	cc.Spec.AWS = as
	cc.Spec.Cloud = "aws"
	cc.Spec.Region = g.region
	cc.Spec.Version = version.Version

	// cluster name
	prompt := promptui.Prompt{
		Label:    "Cluster name",
		Validate: utils.ValidateKubeName,
	}
	cc.Name, err = prompt.Run()
	if err != nil {
		return
	}
	conf := config.GetConfig()
	if conf.Clusters[cc.Name] != nil {
		err = fmt.Errorf("Cluster name already in use")
		return
	}

	// kube version
	versionSelect := utils.NewPromptSelect("Kubernetes version", kaws.EKSAvailableVersions)
	if _, cc.Spec.KubeVersion, err = versionSelect.Run(); err != nil {
		return
	}

	// AWS only component
	comps := append(kube.KubeComponents, &ingress.AWSALBIngress{})
	for _, comp := range comps {
		cc.Spec.Components = append(cc.Spec.Components, v1alpha1.ClusterComponent{
			ComponentSpec: v1alpha1.ComponentSpec{
				Name:    comp.Name(),
				Version: comp.VersionForKube(cc.Spec.KubeVersion),
			},
		})
	}

	// VPC
	as.Vpc, as.VpcCidr, err = promptChooseVPC(g.session)
	if err != nil {
		return
	}

	ec2Svc := ec2.New(g.session)
	if as.Vpc == "" {
		// creating a new VPC
		as.AvailabilityZones, err = promptAZs(ec2Svc)
		if err != nil {
			return
		}
		as.Topology, err = promptTopology()
	} else {
		// derive topology & availability zone info from subnets
		var subnetRes *ec2.DescribeSubnetsOutput
		subnetRes, err = ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("vpc-id"),
					Values: []*string{aws.String(as.Vpc)},
				},
			},
		})
		if err != nil {
			return
		}

		hasPrivateSubnets := false
		for _, subnet := range subnetRes.Subnets {
			for _, tag := range subnet.Tags {
				if *tag.Key == kaws.TagSubnetScope && *tag.Value == kaws.TagValuePrivate {
					hasPrivateSubnets = true
					break
				}
			}
			if !funk.Contains(as.AvailabilityZones, *subnet.AvailabilityZone) {
				as.AvailabilityZones = append(as.AvailabilityZones, *subnet.AvailabilityZone)
			}
		}

		sort.Strings(as.AvailabilityZones)
		as.Topology = v1alpha1.AWSTopologyPublic
		if hasPrivateSubnets {
			as.Topology = v1alpha1.AWSTopologyPublicPrivate
		}
	}
	return
}

func (g *PromptConfigGenerator) CreateNodepoolConfig(cc *v1alpha1.ClusterConfig) (np *v1alpha1.Nodepool, err error) {
	nps := v1alpha1.NodepoolSpec{
		AWS: &v1alpha1.NodePoolAWS{},
	}

	// See if remote access is needed
	sshPrompt := promptui.Prompt{
		Label:     "Enable SSH access to nodes",
		IsConfirm: true,
		Default:   "N",
	}

	_, err = sshPrompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			// skip sequence
		} else {
			return
		}
	} else {
		if err = g.promptSshAccess(&nps, cc.Spec.AWS.Topology); err != nil {
			return
		}
	}

	instanceConfirmed := false
	for !instanceConfirmed {
		//// node instance config
		//gpuPrompt := utils.NewPromptSelect(
		//	"Requires GPU instances",
		//	[]string{"no", "require GPU"},
		//)
		//idx, _, err = gpuPrompt.Run()
		//if err != nil {
		//	return
		//}
		//if idx == 1 {
		//	nps.RequiresGPU = true
		//}
		var instance *kaws.EC2InstancePricing
		instance, err = promptInstanceType(g.session, g.region, nps.RequiresGPU)
		if err != nil {
			return
		}
		nps.MachineType = instance.InstanceType

		nps.MinSize, nps.MaxSize, err = promptInstanceSizing()
		if err != nil {
			return
		}

		// compute budget and inform
		instanceConfirmed, err = promptConfirmBudget(instance, nps.MinSize, nps.MaxSize)
		if err != nil {
			return
		}
	}

	diskPrompt := promptui.Prompt{
		Label:    "Size of root disk (GiB)",
		Default:  "100",
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

	// execute plan & save config
	np = &v1alpha1.Nodepool{
		ObjectMeta: v1.ObjectMeta{
			Name: resources.NodepoolName(),
		},
		Spec: nps,
	}

	// ignore errors, since it might not be available at the time of config generation
	populateClusterInfo(cc, np)
	return
}

func promptChooseVPC(sess *session.Session) (vpcId string, cidrBlock string, err error) {
	vpcProvider := kaws.NewEC2Service(sess)
	vpcs, err := vpcProvider.ListVPCs(context.TODO())
	if err != nil {
		return
	}

	konVpcs := make([]*types.VPC, 0, len(vpcs))
	vpcItems := make([]string, 0, len(vpcs))
	for _, vpc := range vpcs {
		if vpc.SupportsKonstellation {
			konVpcs = append(konVpcs, vpc)
			vpcItems = append(vpcItems, fmt.Sprintf("%s - %s", vpc.ID, vpc.CIDRBlock))
		}
	}
	vpcSelect := promptui.SelectWithAdd{
		Label:    "Choose a VPC (to use for your EKS Cluster resources)",
		Items:    vpcItems,
		AddLabel: "New VPC (enter CIDR Block, i.e. 10.1.0.0/16)",

		Validate: func(v string) error {
			if strings.HasPrefix(v, "172.17.") {
				return fmt.Errorf("172.17. is reserved for internal EKS communications")
			}

			_, newCidr, err := net.ParseCIDR(v)
			if err != nil {
				return err
			}
			firstIp, lastIp := cidr.AddressRange(newCidr)
			for _, vpc := range vpcs {
				_, vpcCidr, err := net.ParseCIDR(vpc.CIDRBlock)
				if err != nil {
					return err
				}
				if vpcCidr.Contains(firstIp) || vpcCidr.Contains(lastIp) {
					return fmt.Errorf("CIDR block overlaps with an existing one")
				}
			}
			return nil
		},
	}
	idx, cidrBlock, err := vpcSelect.Run()
	if err != nil {
		return
	}
	if idx != -1 {
		cidrBlock = konVpcs[idx].CIDRBlock
		vpcId = konVpcs[idx].ID
	}
	return
}

func promptAZs(ec2Svc *ec2.EC2) (zones []string, err error) {
	// query availability zones and ask users how many to use
	zoneRes, err := ec2Svc.DescribeAvailabilityZones(&ec2.DescribeAvailabilityZonesInput{})
	if err != nil {
		return
	}

	totalZones := len(zoneRes.AvailabilityZones)
	if totalZones < 2 {
		err = fmt.Errorf("Konstellation requires at least 2 availability zones, the current region contains only %d", totalZones)
		return
	}

	zoneItems := make([]string, 0, totalZones)
	for i := 2; i <= totalZones; i++ {
		zoneItems = append(zoneItems, strconv.Itoa(i))
	}
	zonePrompt := utils.NewPromptSelect("How many availability zones would you use", zoneItems)
	_, res, err := zonePrompt.Run()
	if err != nil {
		return
	}

	numZones := cast.ToInt(res)
	zones = make([]string, 0, numZones)
	for i := 0; i < numZones; i++ {
		z := zoneRes.AvailabilityZones[i]
		// TODO: maybe check availability
		zones = append(zones, *z.ZoneName)
	}
	return
}

func promptTopology() (topology v1alpha1.AWSTopology, err error) {
	fmt.Println(topologyMessage)
	prompt := utils.NewPromptSelect(
		"What type of network topology?",
		[]string{
			"Public subnets",
			"Public + Private subnets",
		},
	)

	topologies := []v1alpha1.AWSTopology{
		v1alpha1.AWSTopologyPublic,
		v1alpha1.AWSTopologyPublicPrivate,
	}
	idx, _, err := prompt.Run()
	if err != nil {
		return
	}
	return topologies[idx], nil
}

func (g *PromptConfigGenerator) promptSshAccess(nps *v1alpha1.NodepoolSpec, topology v1alpha1.AWSTopology) error {
	ec2Svc := kaws.NewEC2Service(g.session)
	// keypairs for access
	kpRes, err := ec2Svc.EC2.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{})
	if err != nil {
		return err
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
		return err
	}
	if idx == -1 {
		// create new keypair and save it to ~/.ssh
		nps.AWS.SSHKeypair, err = promptCreateKeypair(ec2Svc.EC2, keypairName)
		if err != nil {
			return err
		}
	} else {
		nps.AWS.SSHKeypair = *keypairs[idx].KeyName
	}

	// configure node connection
	if topology == v1alpha1.AWSTopologyPublic {
		// remote access is only possible when VPC is public-only
		connectionPrompt := utils.NewPromptSelect(
			"Allow remote access to nodes from the internet?",
			[]string{"allow", "disallow"},
		)
		idx, _, err = connectionPrompt.Run()
		if err != nil {
			return err
		}
		nps.AWS.ConnectFromAnywhere = idx == 0
	}
	return nil
}

func promptCreateKeypair(ec2Svc *ec2.EC2, name string) (keyName string, err error) {
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
		savePrompt := promptui.Prompt{
			Label:     "Path to save this keypair",
			AllowEdit: true,
			Default:   path.Join(homeDir, ".ssh", name+".pem"),
		}
		utils.FixPromptBell(&savePrompt)
		if saveTargetPath, err := savePrompt.Run(); err == nil {
			err = ioutil.WriteFile(saveTargetPath, []byte(*res.KeyMaterial), 0600)
			if err != nil {
				fmt.Println("Error while saving key:", err)
			} else {
				fmt.Printf("Keypair %s saved to: %s\n", keyName, saveTargetPath)
				saved = true
			}
		}
	}
	if !saved {
		utils.PrintImportant(*res.KeyMaterial, "IMPORTANT: Your new keypair is only displayed once.")
	}

	return
}

func promptInstanceType(session *session.Session, region string, gpu bool) (instance *kaws.EC2InstancePricing, err error) {
	// find all ec2 instances and create listing for price
	pricingSvc := pricing.New(session, aws.NewConfig().WithRegion("us-east-1"))
	instances, err := kaws.ListEC2Instances(pricingSvc, region, true)
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

func promptInstanceSizing() (minNodes int64, maxNodes int64, err error) {
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

func promptConfirmBudget(instance *kaws.EC2InstancePricing, minNodes, maxNodes int64) (bool, error) {
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
