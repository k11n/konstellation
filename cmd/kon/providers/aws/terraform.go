package aws

import (
	"fmt"
	"os"
	"path"

	"github.com/k11n/konstellation/cmd/kon/config"
	"github.com/k11n/konstellation/cmd/kon/terraform"
	"github.com/k11n/konstellation/cmd/kon/utils"
)

var (
	vpcFiles = []string{
		"aws/vpc/main.tf",
		"aws/vpc/tags.tf",
		"aws/vpc/vars.tf",
		"aws/vpc/vpc.tf",
	}
	clusterFiles = []string{
		"aws/cluster/cluster.tf",
		"aws/cluster/data.tf",
		"aws/cluster/iam.tf",
		"aws/cluster/main.tf",
		"aws/cluster/tags.tf",
		"aws/cluster/vars.tf",
	}
)

type ObjContainer struct {
	Type  string
	Value interface{}
}

type TFVPCOutput struct {
	VpcId              string      `json:"vpc_id"`
	MainRouteTable     string      `json:"main_route_table"`
	PublicSubnets      []*TFSubnet `json:"public_subnets"`
	PublicGateway      string      `json:"public_gateway"`
	PrivateSubnets     []*TFSubnet `json:"private_subnets"`
	PrivateRouteTables []string    `json:"private_route_tables"`
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

type TFClusterOutput struct {
	ClusterName       string
	AlbIngressRoleArn string
	NodeRoleArn       string
}

func NewCreateVPCTFAction(bucket, bucketRegion, region, vpcCidr string, zones []string, topology string, opts ...terraform.TerraformOption) (a *terraform.TerraformAction, err error) {
	targetDir := path.Join(config.TerraformDir(), "aws", "vpc")
	tfFiles := make([]string, 0, len(vpcFiles))
	tfFiles = append(tfFiles, vpcFiles...)
	if topology == "public_private" {
		tfFiles = append(tfFiles, "aws/vpc/vpc_private_subnet.tf")
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

	opts = append(opts,
		terraform.TerraformVars{
			"region":      region,
			"vpc_cidr":    vpcCidr,
			"az_suffixes": zoneSuffixes,
			"topology":    topology,
		},
		terraform.TerraformTemplateVars{
			"state_bucket":        bucket,
			"state_bucket_region": bucketRegion,
		},
		getAWSCredentials(),
	)
	a = terraform.NewTerraformAction(targetDir, opts...)
	return
}

func NewDestroyVPCTFAction(bucket, bucketRegion, region, vpcCidr string, topology string, opts ...terraform.TerraformOption) (a *terraform.TerraformAction, err error) {
	targetDir := path.Join(config.TerraformDir(), "aws", "vpc")
	tfFiles := make([]string, 0, len(vpcFiles))
	tfFiles = append(tfFiles, vpcFiles...)
	if topology == "public_private" {
		tfFiles = append(tfFiles, "aws/vpc/vpc_private_subnet.tf")
	}
	err = utils.ExtractBoxFiles(utils.TFResourceBox(), targetDir, tfFiles...)
	if err != nil {
		return
	}
	opts = append(opts,
		terraform.TerraformVars{
			"region":   region,
			"vpc_cidr": vpcCidr,
			"topology": topology,
		},
		terraform.TerraformTemplateVars{
			"state_bucket":        bucket,
			"state_bucket_region": bucketRegion,
		},
		getAWSCredentials(),
	)
	a = terraform.NewTerraformAction(targetDir, opts...)
	return
}

func NewCreateEKSClusterTFAction(bucket, bucketRegion, region, vpcId string, name string, securityGroupIds []string, opts ...terraform.TerraformOption) (a *terraform.TerraformAction, err error) {
	targetDir := path.Join(config.TerraformDir(), "aws", "cluster", name)
	err = utils.ExtractBoxFiles(utils.TFResourceBox(), targetDir, clusterFiles...)
	if err != nil {
		return
	}

	opts = append(opts,
		terraform.TerraformVars{
			"region":             region,
			"vpc_id":             vpcId,
			"cluster":            name,
			"security_group_ids": securityGroupIds,
		},
		terraform.TerraformTemplateVars{
			"state_bucket":        bucket,
			"state_bucket_region": bucketRegion,
		},
		getAWSCredentials(),
	)
	a = terraform.NewTerraformAction(targetDir, opts...)
	return
}

func NewDestroyEKSClusterTFAction(bucket, bucketRegion, region string, cluster string, opts ...terraform.TerraformOption) (a *terraform.TerraformAction, err error) {
	targetDir := path.Join(config.TerraformDir(), "aws", "cluster", fmt.Sprintf("%s_destroy", cluster))
	err = utils.ExtractBoxFiles(utils.TFResourceBox(), targetDir, "aws/cluster/main.tf")
	if err != nil {
		return
	}

	opts = append(opts,
		terraform.TerraformVars{
			"region":  region,
			"cluster": cluster,
		},
		terraform.TerraformTemplateVars{
			"state_bucket":        bucket,
			"state_bucket_region": bucketRegion,
		},
		getAWSCredentials(),
	)
	a = terraform.NewTerraformAction(targetDir, opts...)
	return
}

func ParseNetworkingTFOutput(data []byte) (tf *TFVPCOutput, err error) {
	oc, err := terraform.ParseOutput(data)
	if err != nil {
		return
	}

	tf = &TFVPCOutput{
		VpcId:          oc.GetString("vpc_id"),
		MainRouteTable: oc.GetString("main_route_table"),
		PublicGateway:  oc.GetString("public_gateway"),
	}
	oc.ParseField("public_subnets", &tf.PublicSubnets)
	oc.ParseField("private_subnets", &tf.PrivateSubnets)
	oc.ParseField("private_route_tables", &tf.PrivateRouteTables)

	return
}

func ParseClusterTFOutput(data []byte) (tf *TFClusterOutput, err error) {
	oc, err := terraform.ParseOutput(data)
	if err != nil {
		return
	}

	tf = &TFClusterOutput{
		ClusterName:       oc.GetString("cluster_name"),
		AlbIngressRoleArn: oc.GetString("cluster_alb_role_arn"),
		NodeRoleArn:       oc.GetString("cluster_node_role_arn"),
	}
	return
}

func getAWSCredentials() terraform.EnvVar {
	creds := config.GetConfig().Clouds.AWS.Credentials
	home, _ := os.UserHomeDir()
	return terraform.EnvVar{
		"AWS_ACCESS_KEY_ID":     creds.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY": creds.SecretAccessKey,
		// needed by terraform: https://github.com/hashicorp/terraform/issues/24520
		"HOME": home,
	}
}
