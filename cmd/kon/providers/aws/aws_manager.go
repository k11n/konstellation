package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"

	"github.com/davidzhao/konstellation/cmd/kon/config"
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
	fmt.Printf("AWS config %+v", awsConfig)
	cc.Spec.AWSConfig = &awsConfig
	return nil
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
				fmt.Printf("tag: %s: %s\n", *tag.Key, *tag.Value)
				if *tag.Key == kaws.TagClusterName && *tag.Value == cluster {
					return role, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("Could not find ALB role for cluster")
}

//
//func (a *AWSManager) createALBRServiceRole(clusterName string, oidcIssuer string) (role string, err error) {
//	// TODO: move to cloud/aws
//	iamSvc := iam.New(session.Must(a.awsSession()))
//
//	oidcArn, err := a.enableOIDCProvider(iamSvc, oidcIssuer)
//	if err != nil {
//		return
//	}
//	fmt.Println("oidcArn", oidcArn)
//	return
//}
//
//func (a *AWSManager) enableOIDCProvider(iamSvc *iam.IAM, oidcIssuer string) (oidcArn string, err error) {
//	// TODO: move to cloud/aws
//	// find existing oidc provider
//	listRes, err := iamSvc.ListOpenIDConnectProviders(&iam.ListOpenIDConnectProvidersInput{})
//	for _, provider := range listRes.OpenIDConnectProviderList {
//		oidcRes, err := iamSvc.GetOpenIDConnectProvider(&iam.GetOpenIDConnectProviderInput{
//			OpenIDConnectProviderArn: provider.Arn,
//		})
//		if err != nil {
//			return "", err
//		}
//		if oidcIssuer == *oidcRes.Url {
//			oidcArn = *provider.Arn
//			return oidcArn, nil
//		}
//	}
//
//	// create new
//	thumbprint, err := tlsutil.GetIssuerCAThumbprint(oidcIssuer)
//	if err != nil {
//		err = errors.Wrapf(err, "Could not get issuer thumbprint for %s", oidcIssuer)
//		return
//	}
//	oidcRes, err := iamSvc.CreateOpenIDConnectProvider(&iam.CreateOpenIDConnectProviderInput{
//		Url: &oidcIssuer,
//		ClientIDList: []*string{
//			aws.String("sts.amazonaws.com"),
//		},
//		ThumbprintList: []*string{
//			&thumbprint,
//		},
//	})
//	if err != nil {
//		err = errors.Wrap(err, "Could not create OIDC provider")
//		return
//	}
//	oidcArn = *oidcRes.OpenIDConnectProviderArn
//	return
//}

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
