package aws

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cast"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/terraform"
	"github.com/davidzhao/konstellation/pkg/cloud"
	kaws "github.com/davidzhao/konstellation/pkg/cloud/aws"
)

type AWSManager struct {
	region  string
	kubeSvc cloud.KubernetesProvider
	acmSvc  cloud.CertificateProvider
}

func NewAWSManager(region string) *AWSManager {
	return &AWSManager{region: region}
}

func (a *AWSManager) CreateCluster() (name string, err error) {
	sess, err := a.awsSession()
	if err != nil {
		return
	}
	eksSvc := kaws.NewEKSService(sess)
	input := eks.CreateClusterInput{}

	prompt := promptui.Prompt{
		Label: "Cluster name",
	}
	result, err := prompt.Run()
	if err != nil {
		return
	}
	input.SetName(result)

	// VPC
	ec2Svc := ec2.New(sess)
	vpcCidr, err := a.promptChooseVPC(ec2Svc)
	if err != nil {
		return
	}

	zones, err := a.promptAZs(ec2Svc)
	if err != nil {
		return
	}
	numZones := len(zones)

	usePrivate, err := a.promptUsePrivateSubnet()

	// explicit confirmation about confirmation, or look at terraform file
	fmt.Println("---------------------------------------")
	fmt.Println(" NOTE: PLEASE READ BEFORE CONTINUING")
	fmt.Println("---------------------------------------")
	fmt.Println()
	fmt.Println("Konstellation uses Terraform to manage IAM roles and VPC resources")
	fmt.Println("If Konstellation managed resources cannot be found, it'll attempt to create them.")
	fmt.Println("These resources will be tagged Konstellation=1")
	fmt.Println("\nThe following resources will be created or updated")
	fmt.Printf("* VPC with CIDR (%s)\n", vpcCidr)
	fmt.Printf("* %d subnets (one per availability zone)\n", numZones)
	fmt.Println("* an internet gateway")
	fmt.Println("* an IAM role for EKS Service")
	fmt.Println("* an IAM role for EKS Nodes")
	if usePrivate {
		fmt.Printf("* %d private subnets\n", numZones)
		fmt.Printf("* %d NAT gateways (one for each subnet)\n", numZones)
		fmt.Printf("* %d Elastic IPs (for use with NAT gateways)\n", numZones)
		fmt.Println("* a routing table for private subnets")
	}

	fmt.Println()

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

	// run terraform
	tf, err := NewNetworkingTFAction(a.region, vpcCidr, zones, usePrivate, terraform.OptionDisplayOutput)
	if err != nil {
		return
	}

	if err = tf.Run(); err != nil {
		return
	}

	// get output and parse data
	out, err := tf.GetOutput()
	if err != nil {
		return
	}

	tfOut, err := ParseTerraformOutput(out)
	if err != nil {
		return
	}

	input.SetRoleArn(EKSServiceRole)

	// fetch VPC subnets
	subnets := tfOut.PublicSubnets
	if usePrivate {
		subnets = tfOut.PrivateSubnets
	}
	subnetIds := []*string{}
	for _, sub := range subnets {
		subnetIds = append(subnetIds, aws.String(sub.Id))
	}

	// fetch VPC security groups and prompt user to select one
	// EKS will create its own anyways
	securityGroups, err := kaws.ListSecurityGroups(ec2Svc, tfOut.VpcId)
	if err != nil {
		return
	}
	groups := []string{}
	for _, sg := range securityGroups {
		groups = append(groups, *sg.GroupId)
	}
	sgSelect := promptui.Select{
		Label: "Primary security group",
		Items: groups,
	}
	idx, _, err := sgSelect.Run()
	if err != nil {
		return
	}

	resConf := &eks.VpcConfigRequest{
		SubnetIds:        subnetIds,
		SecurityGroupIds: []*string{&groups[idx]},
	}
	resConf.SetEndpointPrivateAccess(true)
	resConf.SetEndpointPublicAccess(true)
	// create Vpc config request
	input.SetResourcesVpcConfig(resConf)

	// create EKS Cluster
	eksResult, err := eksSvc.EKS.CreateCluster(&input)
	if err != nil {
		return
	}

	name = *eksResult.Cluster.Name
	return
}

func (a *AWSManager) promptAZs(ec2Svc *ec2.EC2) (zones []string, err error) {
	// query availability zones and ask users how many to use
	zoneRes, err := ec2Svc.DescribeAvailabilityZones(&ec2.DescribeAvailabilityZonesInput{})
	if err != nil {
		return
	}
	zonePrompt := promptui.SelectWithAdd{
		Label:    "How many availability zones for this cluster?",
		Items:    []string{fmt.Sprintf("All %d zones", len(zoneRes.AvailabilityZones))},
		AddLabel: "Custom",
		Validate: func(s string) error {
			num, err := strconv.Atoi(s)
			if err != nil {
				return err
			}
			if num < 1 || num > len(zoneRes.AvailabilityZones) {
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

func (a *AWSManager) promptChooseVPC(ec2Svc *ec2.EC2) (cidrBlock string, err error) {
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
		Label:    "VPC (to use for your EKS Cluster resources)",
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
	}
	return
}

func (a *AWSManager) promptUsePrivateSubnet() (bool, error) {
	fmt.Println(subnetMessage)
	prompt := promptui.Select{
		Label: "Create additional private subnets?",
		Items: []string{
			"No",
			"Yes",
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return false, err
	}
	return idx == 1, nil
}

func (a *AWSManager) Region() string {
	return a.region
}

func (a *AWSManager) Cloud() string {
	return "aws"
}

func (a *AWSManager) String() string {
	return fmt.Sprintf("AWS (%s)", a.region)
}

func (a *AWSManager) awsSession() (*session.Session, error) {
	return sessionForRegion(a.region)
}

func (a *AWSManager) KubernetesProvider() cloud.KubernetesProvider {
	if a.kubeSvc == nil {
		session := session.Must(a.awsSession())
		a.kubeSvc = kaws.NewEKSService(session)
	}
	return a.kubeSvc
}

func (a *AWSManager) CertificateProvider() cloud.CertificateProvider {
	if a.acmSvc == nil {
		session := session.Must(a.awsSession())
		a.acmSvc = kaws.NewACMService(session)
	}
	return a.acmSvc
}

func sessionForRegion(region string) (*session.Session, error) {
	conf := config.GetConfig().Clouds.AWS
	if !conf.IsSetup() {
		return nil, fmt.Errorf("AWS has not been setup, run `%s setup`", config.ExecutableName)
	}

	creds, err := conf.GetDefaultCredentials()
	if err != nil {
		return nil, err
	}
	return session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, ""),
	})
}
