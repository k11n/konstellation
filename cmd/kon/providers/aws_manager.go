package providers

import (
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/manifoldco/promptui"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/utils"
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
	iamSvc := kaws.NewIAMService(sess)

	input := eks.CreateClusterInput{}

	prompt := promptui.Prompt{
		Label: "Cluster name",
	}
	result, err := prompt.Run()
	if err != nil {
		return
	}
	input.SetName(result)

	selectedRole, err := a.promptSelectOrCreateServiceRole(iamSvc)
	if err != nil {
		return
	}
	input.SetRoleArn(*selectedRole.Arn)

	// VPC
	ec2Svc := ec2.New(sess)
	vpcResp, err := ec2Svc.DescribeVpcs(&ec2.DescribeVpcsInput{})
	if err != nil {
		return
	}
	vpcs := vpcResp.Vpcs
	vpcNames := make([]string, 0)
	for _, vpc := range vpcs {
		vpcNames = append(vpcNames, fmt.Sprintf("%s - %s", *vpc.VpcId, *vpc.CidrBlock))
	}
	vpcSelect := promptui.Select{
		Label: "VPC (to use for your EKS Cluster resources)",
		Items: vpcNames,
	}
	idx, _, err := vpcSelect.Run()
	if err != nil {
		return
	}
	selectedVpc := vpcs[idx]

	// fetch VPC subnets
	subnets, err := kaws.ListSubnets(ec2Svc, *selectedVpc.VpcId)
	if err != nil {
		return
	}
	subnetIds := []*string{}
	for _, sub := range subnets {
		subnetIds = append(subnetIds, sub.SubnetId)
	}

	// fetch VPC security groups and prompt user to select one
	// EKS will create its own anyways
	securityGroups, err := kaws.ListSecurityGroups(ec2Svc, *selectedVpc.VpcId)
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
	idx, _, err = sgSelect.Run()
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

func (a *AWSManager) promptSelectOrCreateServiceRole(iamSvc *kaws.IAMService) (role *iam.Role, err error) {
	// list all the IAM roles
	roles, err := iamSvc.ListEKSServiceRoles()
	if err != nil {
		return
	}

	if len(roles) == 0 {
		// Create service role
		namePrompt := promptui.Prompt{
			Label:   "Create a new EKS service role",
			Default: "eks-service-role",
		}
		var roleName string
		roleName, err = namePrompt.Run()
		if err != nil {
			return
		}

		role, err = iamSvc.CreateEKSServiceRole(roleName)
		return
	}

	// choose an existing role
	roleNames := make([]string, 0, len(roles))
	for _, role := range roles {
		roleNames = append(roleNames, *role.RoleName)
	}
	sort.Strings(roleNames)
	roleSelect := promptui.Select{
		Label:    "EKS service role name",
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
