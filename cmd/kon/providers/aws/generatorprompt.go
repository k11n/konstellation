package aws

import (
	"fmt"
	"net"
	"sort"
	"strconv"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cast"
	"github.com/thoas/go-funk"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	kaws "github.com/davidzhao/konstellation/pkg/cloud/aws"
)

// generates AWS cluster & nodepool based on prompts to the user
type PromptConfigGenerator struct {
	region      string
	credentials *config.AWSCredentials
	session     *session.Session
}

func NewPromptConfigGenerator(region string, credentials *config.AWSCredentials) (*PromptConfigGenerator, error) {
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

	// VPC
	ec2Svc := ec2.New(g.session)
	as.Vpc, as.VpcCidr, err = promptChooseVPC(ec2Svc)
	if err != nil {
		return
	}

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
	return
}

func promptChooseVPC(ec2Svc *ec2.EC2) (vpcId string, cidrBlock string, err error) {
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

func promptAZs(ec2Svc *ec2.EC2) (zones []string, err error) {
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
