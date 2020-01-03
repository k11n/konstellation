package providers

import (
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/cloud"
	kaws "github.com/davidzhao/konstellation/pkg/cloud/aws"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
)

const (
	KUBE_NODEPOOL_NAME = "kon-nodepool-0"
)

type AWSProvider struct {
	id          string
	DisplayName string
	kubeSvc     cloud.KubernetesProvider
}

func NewAWSProvider() *AWSProvider {
	provider := AWSProvider{
		id:          "aws",
		DisplayName: "AWS",
	}

	return &provider
}

func (a *AWSProvider) ID() string {
	return a.id
}

func (a *AWSProvider) String() string {
	return a.DisplayName
}

func (a *AWSProvider) IsSetup() bool {
	return config.GetConfig().Clouds.AWS.IsSetup()
}

func (a *AWSProvider) Setup() error {
	conf := config.GetConfig()
	awsConf := &conf.Clouds.AWS

	prompt := promptui.Prompt{
		Label:   "AWS Access Key",
		Default: awsConf.AccessKey,
	}
	result, err := prompt.Run()
	if err != nil {
		return err
	}
	awsConf.AccessKey = result

	prompt = promptui.Prompt{
		Label:   "AWS Secret Key",
		Default: awsConf.SecretKey,
		Mask:    '*',
	}
	result, err = prompt.Run()
	if err != nil {
		return err
	}
	if result != "" {
		awsConf.SecretKey = result
	}

	resolver := endpoints.DefaultResolver()
	partitions := resolver.(endpoints.EnumPartitions).Partitions()
	regions := []string{}

	// TODO: handle other AWS partitions than standard, should handle at top level
	for _, p := range partitions {
		if p.ID() == "aws" {
			for id, _ := range p.Regions() {
				regions = append(regions, id)
			}
		}
	}
	sort.Strings(regions)
	cursorPos := 0
	regionSelect := promptui.Select{
		Label:             "Region",
		Items:             regions,
		Searcher:          utils.SearchFuncFor(regions, false),
		StartInSearchMode: true,
	}
	_, result, err = regionSelect.RunCursorAt(cursorPos, 0)
	if err != nil {
		return err
	}
	awsConf.Region = result

	// check if region is valid
	// EKS hasn't made it into the metadata update in the current go ver. approximate with ECS
	_, err = resolver.EndpointFor("ecs", awsConf.Region, endpoints.StrictMatchingOption)
	if err != nil {
		return errors.Wrapf(err, "EKS service is not available in %s", awsConf.Region)
	}

	// check if key works
	session, err := a.awsSession()
	if err != nil {
		return errors.Wrapf(err, "AWS credentials are not valid")
	}

	iamSvc := iam.New(session)
	_, err = iamSvc.GetUser(&iam.GetUserInput{})
	if err != nil {
		return errors.Wrapf(err, "Couldn't make authenticated calls using provided credentials")
	}

	return conf.Persist()
}

func (a *AWSProvider) CreateCluster() (name string, err error) {
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

	// fetch VPC security groups
	securityGroups, err := kaws.ListSecurityGroups(ec2Svc, *selectedVpc.VpcId)
	if err != nil {
		return
	}
	sgIds := []*string{}
	for _, sg := range securityGroups {
		sgIds = append(sgIds, sg.GroupId)
	}

	// create Vpc config request
	input.SetResourcesVpcConfig(&eks.VpcConfigRequest{
		SubnetIds:        subnetIds,
		SecurityGroupIds: sgIds,
	})

	// create EKS Cluster
	eksResult, err := eksSvc.EKS.CreateCluster(&input)
	if err != nil {
		return
	}

	name = *eksResult.Cluster.Name
	return
}

func (a *AWSProvider) promptSelectOrCreateServiceRole(iamSvc *kaws.IAMService) (role *iam.Role, err error) {
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

func (a *AWSProvider) KubernetesProvider() cloud.KubernetesProvider {
	if a.kubeSvc == nil {
		if a.IsSetup() {
			session := session.Must(a.awsSession())
			a.kubeSvc = kaws.NewEKSService(session)
		}
	}
	return a.kubeSvc
}

func (a *AWSProvider) awsSession() (*session.Session, error) {
	conf := config.GetConfig().Clouds.AWS
	if !conf.IsSetup() {
		return nil, fmt.Errorf("AWS has not been setup, run `%s setup`", config.ExecutableName)
	}
	return session.NewSession(&aws.Config{
		Region:      aws.String(conf.Region),
		Credentials: credentials.NewStaticCredentials(conf.AccessKey, conf.SecretKey, ""),
	})
}
