package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/terraform"
	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/cloud"
	kaws "github.com/davidzhao/konstellation/pkg/cloud/aws"
)

type AWSManager struct {
	region  string
	kubeSvc *kaws.EKSService
	acmSvc  *kaws.ACMService
}

func NewAWSManager(region string) *AWSManager {
	return &AWSManager{region: region}
}

// Perform cluster cloud specific setup, including tagging subnets, etc
func (a *AWSManager) UpdateClusterSettings(cc *v1alpha1.ClusterConfig) error {
	fmt.Println("updating cluster settings")
	awsConfig := v1alpha1.AWSCloudConfig{
		Region: a.region,
	}
	// ensure it's initialized
	sess := session.Must(a.awsSession())
	eksSvc := eks.New(sess)
	ec2Svc := ec2.New(sess)

	res, err := eksSvc.DescribeCluster(&eks.DescribeClusterInput{
		Name: aws.String(cc.Name),
	})
	if err != nil {
		return err
	}
	//oidcIssuer := *res.Cluster.Identity.Oidc.Issuer
	vpcConf := res.Cluster.ResourcesVpcConfig
	awsConfig.Vpc = *vpcConf.VpcId
	for _, sg := range vpcConf.SecurityGroupIds {
		awsConfig.SecurityGroups = append(awsConfig.SecurityGroups, *sg)
	}

	// get subnet info
	subnetRes, err := ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
		SubnetIds: vpcConf.SubnetIds,
	})
	if err != nil {
		return err
	}

	for _, sub := range subnetRes.Subnets {
		isPublic := false
		for _, tag := range sub.Tags {
			if *tag.Key == kaws.TagSubnetScope {
				isPublic = *tag.Value == kaws.TagValuePublic
				break
			}
		}
		subConf := v1alpha1.AWSSubnet{
			SubnetId:         *sub.SubnetId,
			Ipv4Cidr:         *sub.CidrBlock,
			IsPublic:         isPublic,
			AvailabilityZone: *sub.AvailabilityZone,
		}
		if isPublic {
			awsConfig.PublicSubnets = append(awsConfig.PublicSubnets, &subConf)
		} else {
			awsConfig.PrivateSubnets = append(awsConfig.PrivateSubnets, &subConf)
		}
	}

	albRole, err := a.getAlbRole(cc.Name)
	if err != nil {
		return err
	}
	awsConfig.AlbRoleArn = *albRole.Arn
	cc.Spec.AWSConfig = &awsConfig
	return nil
}

func (a *AWSManager) DeleteCluster(cluster string) error {
	// list all nodepools, and delete them
	sess := session.Must(a.awsSession())
	eksSvc := kaws.NewEKSService(sess)

	listRes, err := eksSvc.EKS.ListNodegroups(&eks.ListNodegroupsInput{
		ClusterName: &cluster,
	})
	if err != nil {
		return err
	}

	for _, item := range listRes.Nodegroups {
		// TODO: this might involve deleting the remote access groups
		if err = eksSvc.DeleteNodepool(context.TODO(), cluster, *item); err != nil {
			return err
		}
	}

	// wait for nodegroups to disappear
	fmt.Printf("Waiting for nodepools to be deleted, this may take a few minutes\n")
	err = utils.WaitUntilComplete(utils.LongTimeoutSec, utils.LongCheckInterval, func() (finished bool, err error) {
		listRes, err := eksSvc.EKS.ListNodegroups(&eks.ListNodegroupsInput{
			ClusterName: &cluster,
		})
		if err != nil {
			return
		}
		finished = len(listRes.Nodegroups) == 0
		return
	})
	if err != nil {
		return err
	}

	// done, load cluster config and delete cluster
	tf, err := NewDestroyEKSClusterTFAction(a.region, cluster, terraform.OptionDisplayOutput)

	return tf.Destroy()
}

func (a *AWSManager) getAlbRole(cluster string) (*iam.Role, error) {
	sess := session.Must(a.awsSession())
	iamSvc := kaws.NewIAMService(sess)
	roles, err := iamSvc.ListRoles()
	if err != nil {
		return nil, err
	}

	for _, role := range roles {
		if strings.HasPrefix(*role.RoleName, "kon-alb-role-") {
			// get role to include tags
			roleOut, err := iamSvc.IAM.GetRole(&iam.GetRoleInput{RoleName: role.RoleName})
			if err != nil {
				return nil, err
			}
			role = roleOut.Role
			for _, tag := range role.Tags {
				if *tag.Key == kaws.TagClusterName && *tag.Value == cluster {
					return role, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("Could not find ALB role for cluster")
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
