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
	"github.com/manifoldco/promptui"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/terraform"
	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/cloud"
	kaws "github.com/davidzhao/konstellation/pkg/cloud/aws"
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
	return nil
}

func (a *AWSManager) CreateNodepool(cc *v1alpha1.ClusterConfig, np *v1alpha1.Nodepool) error {
	fmt.Println("Creating nodepool...")

	// check aws nodepool status. if it doesn't exist, then create it
	kubeProvider := a.KubernetesProvider()
	ready, _ := kubeProvider.IsNodepoolReady(context.Background(), cc.Name, np.Name)
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
	return nil
}

func (a *AWSManager) DeleteCluster(cluster string) error {
	// list all nodepools, and delete them
	sess := session.Must(a.awsSession())
	eksSvc := kaws.NewEKSService(sess)

	// remove from selected
	conf := config.GetConfig()
	conf.SelectedCluster = ""
	err := conf.Persist()
	if err != nil {
		return err
	}

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
	subnetRes, err := ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: vpcFilter,
	})
	if err != nil {
		return err
	}

	// iterate through subnets and assign
	for _, subnet := range subnetRes.Subnets {
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

	// get security groups info, pick default for VPC
	sgRes, err := ec2Svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: vpcFilter,
	})
	if err != nil {
		return err
	}

	for _, sg := range sgRes.SecurityGroups {
		if *sg.GroupName == "default" {
			awsConf.SecurityGroups = append(awsConf.SecurityGroups, *sg.GroupId)
		}
	}

	return nil
}

func sessionForRegion(region string) (*session.Session, error) {
	conf := config.GetConfig().Clouds.AWS
	creds, err := conf.GetDefaultCredentials()
	if err != nil {
		return nil, err
	}
	return session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, ""),
	})
}
