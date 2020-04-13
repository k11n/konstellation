package aws

import (
	"path"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/terraform"
	"github.com/davidzhao/konstellation/cmd/kon/utils"
)

type ObjContainer struct {
	Type  string
	Value interface{}
}

type TFVPCOutput struct {
	VpcId             string      `json:"vpc_id"`
	MainRouteTable    string      `json:"main_route_table"`
	PublicSubnets     []*TFSubnet `json:"public_subnets"`
	PublicGateway     string      `json:"public_gateway"`
	PrivateSubnets    []*TFSubnet `json:"private_subnets"`
	PrivateRouteTable string      `json:"private_route_table"`
}

type TFSubnet struct {
	Id                         string `json:"id"`
	Arn                        string `json:"arn"`
	AssignIpv6OnCreation       bool   `json:"assign_ipv6_address_on_creation"`
	AvailabilityZone           string `json:"availability_zone"`
	AvailabilityZoneId         string `json:"availability_zone_id"`
	CidrBlock                  string `json:"cidr_block"`
	Ipv6CidrBlock              string `json:"ipv6_cidr_block"`
	Ipv6CidrBlockAssociationId string `json:"ipv6_cidr_block_association_id"`
	MapPublicIpOnLaunch        bool   `json:"map_public_ip_on_launch"`
	VpcId                      string `json:"vpc_id"`
}

var (
	networkingFiles = []string{
		"aws/config.tf",
		"aws/networking.tf",
		"aws/networking_vars.tf",
		"aws/roles.tf",
		"aws/security.tf",
		"aws/tags.tf",
	}
)

func NewNetworkingTFAction(region string, vpcCidr string, zones []string, usePrivateSubnet bool, opts ...terraform.TerraformOption) (a *terraform.TerraformAction, err error) {
	targetDir := path.Join(config.GetConfig().TFDir(), "aws", "networking")
	tfFiles := make([]string, 0, len(networkingFiles))
	tfFiles = append(tfFiles, networkingFiles...)
	if usePrivateSubnet {
		tfFiles = append(tfFiles, "aws/private_subnet.tf")
	}
	err = utils.ExtractBoxFiles(utils.TFResourceBox(), targetDir, tfFiles...)
	if err != nil {
		return
	}

	var zoneSuffixes []string
	regionLen := len(region)
	for _, zone := range zones {
		zoneSuffixes = append(zoneSuffixes, zone[regionLen:])
	}

	a = terraform.NewTerraformAction(targetDir, terraform.TerraformVars{
		"region":      region,
		"vpc_cidr":    vpcCidr,
		"az_suffixes": zoneSuffixes,
	})
	for _, o := range opts {
		a.Option(o)
	}
	return
}

func ParseTerraformOutput(data []byte) (tf *TFVPCOutput, err error) {
	oc, err := terraform.ParseOutput(data)
	if err != nil {
		return
	}

	tf = &TFVPCOutput{
		VpcId:             oc.GetString("vpc_id"),
		MainRouteTable:    oc.GetString("main_route_table"),
		PublicGateway:     oc.GetString("public_gateway"),
		PrivateRouteTable: oc.GetString("private_route_table"),
	}
	oc.ParseField("public_subnets", &tf.PublicSubnets)
	oc.ParseField("private_subnets", &tf.PrivateSubnets)

	return
}
