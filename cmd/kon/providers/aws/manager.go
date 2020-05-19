package aws

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/manifoldco/promptui"

	"github.com/k11n/konstellation/cmd/kon/config"
	"github.com/k11n/konstellation/cmd/kon/terraform"
	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/cloud"
	kaws "github.com/k11n/konstellation/pkg/cloud/aws"
	"github.com/k11n/konstellation/pkg/utils/tls"
)

type AWSManager struct {
	region      string
	stateBucket string
	kubeSvc     *kaws.EKSService
	acmSvc      *kaws.ACMService
}

func NewAWSManager(region string) *AWSManager {
	return &AWSManager{
		region:      region,
		stateBucket: config.GetConfig().Clouds.AWS.StateS3Bucket,
	}
}

func (a *AWSManager) CreateCluster(cc *v1alpha1.ClusterConfig) error {
	awsConf := cc.Spec.AWS

	// TODO: Validate input
	var inventory []string

	if awsConf.Vpc == "" {
		// create new VPC
		inventory = append(inventory,
			fmt.Sprintf("VPC with CIDR (%s)", awsConf.VpcCidr),
			"subnets for each availability zone",
			"internet gateway/NAT gateways",
			"IAM roles for EKS",
		)
	}
	inventory = append(inventory, fmt.Sprintf("EKS Cluster %s", cc.Name))

	// explicit confirmation about confirmation, or look at terraform file
	fmt.Println("---------------------------------------")
	fmt.Println(" NOTE: PLEASE READ BEFORE CONTINUING")
	fmt.Println("---------------------------------------")
	fmt.Println()
	fmt.Println("Konstellation will connect to AWS and create your EKS cluster.")
	fmt.Println("It'll also create other required resources such as the VPC network.")
	fmt.Println("Everything that's created will be tagged Konstellation=1")
	fmt.Println("\nThe following resources will be created:")
	for _, item := range inventory {
		fmt.Printf("* %s\n", item)
	}
	fmt.Println()

	// explicit confirmation
	confirmPrompt := promptui.Prompt{
		Label: "Do you want to proceed? (type yes to continue)",
	}
	utils.FixPromptBell(&confirmPrompt)
	res, err := confirmPrompt.Run()
	if err != nil {
		return err
	}
	if res != "yes" {
		return fmt.Errorf("User canceled")
	}

	if awsConf.Vpc == "" {
		// run terraform for VPC
		tfVpc, err := NewNetworkingTFAction(a.stateBucket, a.region, awsConf.VpcCidr, awsConf.AvailabilityZones,
			awsConf.Topology == v1alpha1.AWSTopologyPublicPrivate, terraform.OptionDisplayOutput)
		if err != nil {
			return err
		}

		if err = tfVpc.Apply(); err != nil {
			return err
		}

		// get VPC ID from here
		out, err := tfVpc.GetOutput()
		if err != nil {
			return err
		}
		tfOut, err := ParseNetworkingTFOutput(out)
		if err != nil {
			return err
		}
		awsConf.Vpc = tfOut.VpcId
	}

	err = a.updateVPCInfo(awsConf)
	if err != nil {
		return err
	}

	// create cluster
	clusterTf, err := NewCreateEKSClusterTFAction(a.stateBucket, a.region, awsConf.Vpc, cc.Name,
		awsConf.SecurityGroups, terraform.OptionDisplayOutput)
	if err != nil {
		return err
	}

	if err = clusterTf.Apply(); err != nil {
		return err
	}
	out, err := clusterTf.GetOutput()
	if err != nil {
		return err
	}

	clusterTfOut, err := ParseClusterTFOutput(out)
	if err != nil {
		return err
	}

	awsConf.AlbRoleArn = clusterTfOut.AlbIngressRoleArn

	// at last add thumbprint so the provider we created could work
	return a.addCAThumbprintToProvider(cc.Name)
}

func (a *AWSManager) CreateNodepool(cc *v1alpha1.ClusterConfig, np *v1alpha1.Nodepool) error {
	fmt.Println("Creating nodepool...")

	// set nodepool config from VPC
	nps := &np.Spec
	awsConf := cc.Spec.AWS
	if len(awsConf.SecurityGroups) == 0 {
		return fmt.Errorf("Could not find security groups")
	}
	nps.AWS.SecurityGroupId = awsConf.SecurityGroups[0]
	var subnetSrc []*v1alpha1.AWSSubnet
	if awsConf.Topology == v1alpha1.AWSTopologyPublicPrivate {
		subnetSrc = awsConf.PrivateSubnets
	} else {
		subnetSrc = awsConf.PublicSubnets
	}
	for _, subnet := range subnetSrc {
		nps.AWS.SubnetIds = append(nps.AWS.SubnetIds, subnet.SubnetId)
	}

	// check aws nodepool status. if it doesn't exist, then create it
	kubeProvider := a.KubernetesProvider()
	ready, err := kubeProvider.IsNodepoolReady(context.Background(), cc.Name, np.Name)

	if err != nil {
		// create it
		err = kubeProvider.CreateNodepool(context.TODO(), cc.Name, np)
		if err != nil {
			return err
		}
	}

	// wait for completion
	if !ready {
		fmt.Printf("Waiting for nodepool become ready, this may take a few minutes\n")
		err := utils.WaitUntilComplete(utils.LongTimeoutSec, utils.LongCheckInterval, func() (bool, error) {
			return kubeProvider.IsNodepoolReady(context.Background(), cc.Name, np.Name)
		})
		if err != nil {
			return err
		}
	}

	// now grab the autoscaling group for this
	sess, err := a.awsSession()
	if err != nil {
		return err
	}
	asSvc := autoscaling.New(sess)
	err = asSvc.DescribeAutoScalingGroupsPagesWithContext(context.Background(), &autoscaling.DescribeAutoScalingGroupsInput{},
		func(res *autoscaling.DescribeAutoScalingGroupsOutput, lastPage bool) bool {
			for _, asg := range res.AutoScalingGroups {
				var cluster, nodeGroup string
				for _, tag := range asg.Tags {
					switch *tag.Key {
					case "eks:cluster-name":
						cluster = *tag.Value
					case "eks:nodegroup-name":
						nodeGroup = *tag.Value
					}
				}
				if cluster == cc.Name && nodeGroup == np.Name {
					// found the cluster, update nodegroup
					np.Spec.AWS.ASGID = *asg.AutoScalingGroupName
					return false
				}
			}
			return true
		},
	)

	return err
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
	err = utils.WaitUntilComplete(utils.ReallyLongTimeoutSec, utils.LongCheckInterval, func() (finished bool, err error) {
		listRes, err := eksSvc.EKS.ListNodegroups(&eks.ListNodegroupsInput{
			ClusterName: &cluster,
		})
		if err != nil {
			return
		}
		finished = len(listRes.Nodegroups) == 0
		if !finished {
			// try to delete the security group and network interface manually
			// for some reasons AWS doesn't clean it up
			for _, item := range listRes.Nodegroups {
				eksSvc.DeleteNodeGroupNetworkingResources(context.TODO(), *item)
			}
		}
		return
	})
	if err != nil {
		return err
	}

	// done, load cluster config and delete cluster
	tf, err := NewDestroyEKSClusterTFAction(a.stateBucket, a.region, cluster, terraform.OptionDisplayOutput)

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

func (a *AWSManager) awsSession() (*session.Session, error) {
	return sessionForRegion(a.region)
}

func (a *AWSManager) updateVPCInfo(awsConf *v1alpha1.AWSClusterSpec) error {
	ec2Svc := ec2.New(session.Must(a.awsSession()))
	vpcFilter := []*ec2.Filter{
		{
			Name:   aws.String("vpc-id"),
			Values: []*string{&awsConf.Vpc},
		},
	}

	// get subnet info
	err := ec2Svc.DescribeSubnetsPages(&ec2.DescribeSubnetsInput{
		Filters: vpcFilter,
	}, func(output *ec2.DescribeSubnetsOutput, last bool) bool {
		// iterate through subnets and assign
		for _, subnet := range output.Subnets {
			awsSubnet := &v1alpha1.AWSSubnet{
				SubnetId:         *subnet.SubnetId,
				Ipv4Cidr:         *subnet.CidrBlock,
				AvailabilityZone: *subnet.AvailabilityZone,
			}
			for _, tag := range subnet.Tags {
				if *tag.Key == kaws.TagSubnetScope {
					// this is our subnet
					if *tag.Value == kaws.TagValuePublic {
						awsSubnet.IsPublic = true
						awsConf.PublicSubnets = append(awsConf.PublicSubnets, awsSubnet)
					} else if *tag.Value == kaws.TagValuePrivate {
						awsSubnet.IsPublic = false
						awsConf.PublicSubnets = append(awsConf.PublicSubnets, awsSubnet)
					}
					break
				}
			}
		}
		return true
	})
	if err != nil {
		return err
	}

	// get security groups info, pick default for VPC
	err = ec2Svc.DescribeSecurityGroupsPages(&ec2.DescribeSecurityGroupsInput{
		Filters: vpcFilter,
	}, func(output *ec2.DescribeSecurityGroupsOutput, last bool) bool {
		for _, sg := range output.SecurityGroups {
			if *sg.GroupName == "default" {
				awsConf.SecurityGroups = append(awsConf.SecurityGroups, *sg.GroupId)
			}
		}
		return true
	})
	if err != nil {
		return err
	}

	return nil
}

func (a *AWSManager) addCAThumbprintToProvider(cluster string) error {
	sess := session.Must(a.awsSession())
	iamSvc := iam.New(sess)
	eksSvc := eks.New(sess)
	// get current cluster and its oidc url
	clusterRes, err := eksSvc.DescribeCluster(&eks.DescribeClusterInput{
		Name: &cluster,
	})
	if err != nil {
		return err
	}
	oidcUrl := *clusterRes.Cluster.Identity.Oidc.Issuer

	thumbprint, err := tls.GetIssuerCAThumbprint(oidcUrl)
	if err != nil {
		return err
	}

	var providerArn string
	thumbprintExists := false
	providersRes, err := iamSvc.ListOpenIDConnectProviders(&iam.ListOpenIDConnectProvidersInput{})
	if err != nil {
		return err
	}

	// strip protocol
	u, _ := url.Parse(oidcUrl)
	existingUrl := u.Host + u.Path
	for _, provider := range providersRes.OpenIDConnectProviderList {
		providerRes, err := iamSvc.GetOpenIDConnectProvider(&iam.GetOpenIDConnectProviderInput{
			OpenIDConnectProviderArn: provider.Arn,
		})
		if err != nil {
			return err
		}
		if *providerRes.Url == existingUrl {
			providerArn = *provider.Arn
			for _, thumb := range providerRes.ThumbprintList {
				if *thumb == thumbprint {
					thumbprintExists = true
					continue
				}
			}
			break
		}
	}

	if providerArn == "" {
		return fmt.Errorf("Could not find OIDC provider")
	}

	if !thumbprintExists {
		_, err := iamSvc.UpdateOpenIDConnectProviderThumbprint(&iam.UpdateOpenIDConnectProviderThumbprintInput{
			OpenIDConnectProviderArn: &providerArn,
			ThumbprintList:           []*string{&thumbprint},
		})
		return err
	}

	return nil
}

func sessionForRegion(region string) (*session.Session, error) {
	conf := config.GetConfig().Clouds.AWS
	creds := conf.GetDefaultCredentials()
	return session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, ""),
	})
}
