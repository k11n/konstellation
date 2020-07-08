package aws

import (
	"os"
	"path"

	"github.com/spf13/cast"

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
	linkedAccountFiles = []string{
		"aws/linkedaccount/iam.tf",
		"aws/linkedaccount/main.tf",
		"aws/linkedaccount/tags.tf",
		"aws/linkedaccount/vars.tf",
	}

	TFStateBucket       = terraform.Var{Name: "state_bucket", TemplateOnly: true}
	TFStateBucketRegion = terraform.Var{Name: "state_bucket_region", TemplateOnly: true}
	TFRegion            = terraform.Var{Name: "region"}

	// vpc & cluster
	TFVPCCidr          = terraform.Var{Name: "vpc_cidr"}
	TFEnableIPv6       = terraform.Var{Name: "enable_ipv6", CreationOnly: true}
	TFAZSuffixes       = terraform.Var{Name: "az_suffixes", CreationOnly: true}
	TFTopology         = terraform.Var{Name: "topology"}
	TFCluster          = terraform.Var{Name: "cluster"}
	TFKubeVersion      = terraform.Var{Name: "kube_version", CreationOnly: true}
	TFSecurityGroupIds = terraform.Var{Name: "security_group_ids", CreationOnly: true}
	TFVPCId            = terraform.Var{Name: "vpc_id", CreationOnly: true}
	TFAdminGroups      = terraform.Var{Name: "admin_groups", CreationOnly: true}

	// linked accounts
	TFAccount  = terraform.Var{Name: "account"}
	TFTargets  = terraform.Var{Name: "targets", CreationOnly: true}
	TFPolicies = terraform.Var{Name: "policies", CreationOnly: true}
	TFOIDCUrl  = terraform.Var{Name: "oidc_url", CreationOnly: true}
	TFOIDCArn  = terraform.Var{Name: "oidc_arn", CreationOnly: true}
)

type ObjContainer struct {
	Type  string
	Value interface{}
}

type TFVPCOutput struct {
	VpcId              string      `json:"vpc_id"`
	Ipv6Cidr           string      `json:"ipv6_cidr"`
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
	AdminRoleArn      string
}

func NewVPCTFAction(values terraform.Values, zones []string, opts ...terraform.Option) (a *terraform.Action, err error) {
	vars := []terraform.Var{
		TFStateBucket,
		TFStateBucketRegion,
		TFRegion,
		TFTopology,
		TFVPCCidr,

		// creation only
		TFAZSuffixes,
		TFEnableIPv6,
	}

	targetDir := path.Join(config.TerraformDir(), "aws", "vpc")
	tfFiles := make([]string, 0, len(vpcFiles))
	tfFiles = append(tfFiles, vpcFiles...)
	if values[TFTopology].(string) == "public_private" {
		tfFiles = append(tfFiles, "aws/vpc/vpc_private_subnet.tf")
	}
	err = utils.ExtractBoxFiles(utils.TFResourceBox(), targetDir, tfFiles...)
	if err != nil {
		return
	}

	if len(zones) > 0 {
		var zoneSuffixes []string
		regionLen := len(cast.ToString(values[TFRegion]))
		for _, zone := range zones {
			zoneSuffixes = append(zoneSuffixes, zone[regionLen:])
		}
		values[TFAZSuffixes] = zoneSuffixes
	}

	opts = append(opts,
		values,
		getAWSCredentials(),
	)
	a = terraform.NewTerraformAction(targetDir, vars, opts...)
	return
}

type eksClusterInput struct {
	bucket           string
	bucketRegion     string
	region           string
	vpcId            string
	name             string
	kubeVersion      string
	securityGroupIds []string
}

func NewEKSClusterTFAction(values terraform.Values, opts ...terraform.Option) (a *terraform.Action, err error) {
	vars := []terraform.Var{
		TFStateBucket,
		TFStateBucketRegion,
		TFRegion,
		TFCluster,

		// creation only
		TFKubeVersion,
		TFSecurityGroupIds,
		TFVPCId,
		TFAdminGroups,
	}

	targetDir := path.Join(config.TerraformDir(), "aws", "cluster", values[TFCluster].(string))
	err = utils.ExtractBoxFiles(utils.TFResourceBox(), targetDir, clusterFiles...)
	if err != nil {
		return
	}

	opts = append(opts,
		values,
		getAWSCredentials(),
	)
	a = terraform.NewTerraformAction(targetDir, vars, opts...)
	return
}

func ParseVPCTFOutput(data []byte) (tf *TFVPCOutput, err error) {
	oc, err := terraform.ParseOutput(data)
	if err != nil {
		return
	}

	tf = &TFVPCOutput{
		VpcId:          oc.GetString("vpc_id"),
		Ipv6Cidr:       oc.GetString("ipv6_cidr"),
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
		AdminRoleArn:      oc.GetString("cluster_admin_role_arn"),
	}
	return
}

func NewLinkedAccountTFAction(values terraform.Values, opts ...terraform.Option) (a *terraform.Action, err error) {
	vars := []terraform.Var{
		TFStateBucket,
		TFStateBucketRegion,
		TFRegion,
		TFCluster,
		TFAccount,

		// create only
		TFTargets,
		TFPolicies,
		TFOIDCUrl,
		TFOIDCArn,
	}
	targetDir := path.Join(config.TerraformDir(), "aws", "cluster", values[TFCluster].(string), values[TFAccount].(string))
	err = utils.ExtractBoxFiles(utils.TFResourceBox(), targetDir, linkedAccountFiles...)
	if err != nil {
		return
	}

	opts = append(opts,
		values,
		getAWSCredentials(),
	)
	a = terraform.NewTerraformAction(targetDir, vars, opts...)
	return
}

func ParseLinkedAccountOutput(data []byte) (roleArn string, err error) {
	oc, err := terraform.ParseOutput(data)
	if err != nil {
		return
	}
	return oc.GetString("role_arn"), nil
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
