package aws

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cast"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/terraform"
	"github.com/davidzhao/konstellation/cmd/kon/utils"
	kaws "github.com/davidzhao/konstellation/pkg/cloud/aws"
)

func (a *AWSManager) CreateCluster() (name string, err error) {
	sess, err := a.awsSession()
	if err != nil {
		return
	}

	prompt := promptui.Prompt{
		Label:    "Cluster name",
		Validate: utils.ValidateKubeName,
	}
	clusterName, err := prompt.Run()
	if err != nil {
		return
	}

	conf := config.GetConfig()
	if conf.Clusters[name] != nil {
		err = fmt.Errorf("Cluster name already in use")
		return
	}
	creationConf := conf.Clouds.AWS.GetCreationConfig(clusterName)
	if creationConf == nil {
		creationConf = &config.ClusterCreationConfig{
			Region: a.region,
		}
	}

	// VPC
	ec2Svc := ec2.New(sess)
	var vpcId string
	newVPC := false
	vpcId, creationConf.VpcCidr, err = a.promptChooseVPC(ec2Svc)
	if err != nil {
		return
	}

	var tfVpc *terraform.TerraformAction
	if vpcId == "" {
		var zones []string
		// creating a new VPC
		zones, err = a.promptAZs(ec2Svc)
		if err != nil {
			return
		}
		creationConf.NumZones = len(zones)
		creationConf.PrivateSubnets, err = a.promptUsePrivateSubnet()
		newVPC = true

		// explicit confirmation about confirmation, or look at terraform file
		fmt.Println("---------------------------------------")
		fmt.Println(" NOTE: PLEASE READ BEFORE CONTINUING")
		fmt.Println("---------------------------------------")
		fmt.Println()
		fmt.Println("Konstellation uses Terraform to manage IAM roles and VPC resources")
		fmt.Println("It'll create the VPC and other shared resources that it needs for the EKS cluster.")
		fmt.Println("These resources will be tagged Konstellation=1")
		fmt.Println("\nThe following resources will be created:")
		fmt.Printf("* VPC with CIDR (%s)\n", creationConf.VpcCidr)
		fmt.Printf("* %d subnets (one per availability zone)\n", creationConf.NumZones)
		fmt.Println("* an internet gateway")
		fmt.Println("* an IAM role for EKS Service")
		fmt.Println("* an IAM role for EKS Nodes")
		if creationConf.PrivateSubnets {
			fmt.Printf("* %d private subnets\n", creationConf.NumZones)
			fmt.Printf("* %d NAT gateways (one for each subnet)\n", creationConf.NumZones)
			fmt.Printf("* %d Elastic IPs (for use with NAT gateways)\n", creationConf.NumZones)
			fmt.Println("* a routing table for private subnets")
		}

		fmt.Println()

		confirmPrompt := promptui.Prompt{
			Label: "Do you want to proceed? (type yes to continue)",
		}
		var res string
		res, err = confirmPrompt.Run()
		if err != nil {
			return
		}

		if strings.ToLower(res) != "yes" {
			err = fmt.Errorf("User aborted")
			return
		}

		// run terraform
		tfVpc, err = NewNetworkingTFAction(a.region, creationConf.VpcCidr, zones, creationConf.PrivateSubnets)
		if err != nil {
			return
		}

		if err = tfVpc.Apply(); err != nil {
			return
		}
	} else {
		// TODO: set these vars to reflect current config
		//creationConf.NumZones = len(zones)
		//creationConf.PrivateSubnets, err = a.promptUsePrivateSubnet()
		// not going to run create here.
		tfVpc, err = NewNetworkingTFAction(a.region, creationConf.VpcCidr, []string{}, creationConf.PrivateSubnets)
		if err != nil {
			return
		}
	}

	// get output and parse data
	out, err := tfVpc.GetOutput()
	if err != nil {
		return
	}

	tfOut, err := ParseNetworkingTFOutput(out)
	if err != nil {
		return
	}

	// fetch VPC subnets
	subnetIds := []*string{}
	for _, sub := range tfOut.PublicSubnets {
		subnetIds = append(subnetIds, aws.String(sub.Id))
	}
	for _, sub := range tfOut.PrivateSubnets {
		subnetIds = append(subnetIds, aws.String(sub.Id))
	}

	// fetch VPC security groups and prompt user to select one
	// EKS will create its own anyways
	securityGroups, err := kaws.ListSecurityGroups(ec2Svc, tfOut.VpcId)
	if err != nil {
		return
	}
	groups := []string{}
	groupsDisplay := []string{}
	for _, sg := range securityGroups {
		for _, tag := range sg.Tags {
			if strings.HasPrefix(*tag.Key, "kubernetes.io/cluster") {
				// dedicated security group for clusters
				continue
			}
		}
		if strings.HasPrefix(*sg.GroupName, "k8s-elb") {
			continue
		}
		groups = append(groups, *sg.GroupId)
		groupsDisplay = append(groupsDisplay, fmt.Sprintf("%s (%s)", *sg.GroupId, *sg.GroupName))
	}
	sgSelect := utils.NewPromptSelect("Primary security group", groupsDisplay)
	idx, _, err := sgSelect.Run()
	if err != nil {
		return
	}
	sgGroups := []string{groups[idx]}

	// explicit confirmation about confirmation, or look at terraform file
	fmt.Println("---------------------------------------")
	fmt.Println(" NOTE: PLEASE READ BEFORE CONTINUING")
	fmt.Println("---------------------------------------")
	fmt.Println()
	fmt.Printf("Konstellation will now create the EKS cluster %s and other required resources\n", clusterName)
	fmt.Println("These resources will be tagged Konstellation=1")
	fmt.Println("\nThe following resources will be created or updated")
	fmt.Printf("* EKS Cluster %s\n", clusterName)
	fmt.Printf("* an IAM OIDC provider for this cluster\n")
	fmt.Printf("* an IAM role that allows this cluster to manage Application Load Balancers\n")

	confirmPrompt := promptui.Prompt{
		Label: "Do you want to proceed? (type yes to continue)",
	}
	res, err := confirmPrompt.Run()
	if err != nil {
		return
	}
	if strings.ToLower(res) != "yes" {
		err = fmt.Errorf("User aborted")
		return
	}
	clusterTf, err := NewCreateEKSClusterTFAction(a.region, tfOut.VpcId, clusterName, sgGroups, terraform.OptionDisplayOutput)
	if err != nil {
		return
	}

	if err = clusterTf.Apply(); err != nil {
		return
	}
	out, err = clusterTf.GetOutput()
	if err != nil {
		return
	}

	clusterTfOut, err := ParseClusterTFOutput(out)
	if err != nil {
		return
	}
	name = clusterTfOut.ClusterName

	if newVPC {
		conf.Clouds.AWS.SetCreationConfig(name, creationConf)
		err = conf.Persist()
	}

	return
}

func (a *AWSManager) promptAZs(ec2Svc *ec2.EC2) (zones []string, err error) {
	// query availability zones and ask users how many to use
	zoneRes, err := ec2Svc.DescribeAvailabilityZones(&ec2.DescribeAvailabilityZonesInput{})
	if err != nil {
		return
	}
	zonePrompt := promptui.SelectWithAdd{
		Label:    "How many availability zones would you use?",
		Items:    []string{fmt.Sprintf("All %d zones", len(zoneRes.AvailabilityZones))},
		AddLabel: "Custom (at least two)",
		Validate: func(s string) error {
			num, err := strconv.Atoi(s)
			if err != nil {
				return err
			}
			if num < 2 || num > len(zoneRes.AvailabilityZones) {
				return fmt.Errorf("invalid number")
			}
			return nil
		},
	}
	idx, res, err := zonePrompt.Run()
	if err != nil {
		return
	}

	var numZones int
	if idx != -1 {
		numZones = len(zoneRes.AvailabilityZones)
	} else {
		numZones = cast.ToInt(res)
	}
	zones = make([]string, 0, numZones)
	for i := 0; i < numZones; i++ {
		z := zoneRes.AvailabilityZones[i]
		// TODO: maybe check availability
		zones = append(zones, *z.ZoneName)
	}
	return
}

func (a *AWSManager) promptChooseVPC(ec2Svc *ec2.EC2) (vpcId string, cidrBlock string, err error) {
	vpcResp, err := ec2Svc.DescribeVpcs(&ec2.DescribeVpcsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:Konstellation"),
				Values: []*string{aws.String("1")},
			},
		},
	})
	if err != nil {
		return
	}
	vpcs := vpcResp.Vpcs
	vpcItems := make([]string, 0)
	for _, vpc := range vpcs {
		vpcItems = append(vpcItems, fmt.Sprintf("%s - %s", *vpc.VpcId, *vpc.CidrBlock))
	}
	vpcSelect := promptui.SelectWithAdd{
		Label:    "Choose a VPC (to use for your EKS Cluster resources)",
		Items:    vpcItems,
		AddLabel: "New VPC (enter CIDR Block, i.e. 10.0.0.0/16)",

		Validate: func(v string) error {
			_, newCidr, err := net.ParseCIDR(v)
			if err != nil {
				return err
			}
			firstIp, lastIp := cidr.AddressRange(newCidr)
			for _, vpc := range vpcs {
				_, vpcCidr, err := net.ParseCIDR(*vpc.CidrBlock)
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
		cidrBlock = *vpcs[idx].CidrBlock
		vpcId = *vpcs[idx].VpcId
	}
	return
}

func (a *AWSManager) promptUsePrivateSubnet() (bool, error) {
	fmt.Println(topologyMessage)
	prompt := utils.NewPromptSelect(
		"How should the network be set up?",
		[]string{
			"Public subnets",
			"Public + Private subnets",
		},
	)

	idx, _, err := prompt.Run()
	if err != nil {
		return false, err
	}
	return idx == 1, nil
}
