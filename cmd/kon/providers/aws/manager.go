package aws

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/cmd/kon/config"
	"github.com/k11n/konstellation/cmd/kon/kube"
	"github.com/k11n/konstellation/cmd/kon/terraform"
	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/cloud"
	kaws "github.com/k11n/konstellation/pkg/cloud/aws"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/pkg/utils/tls"
	"github.com/k11n/konstellation/version"
)

const (
	roleArnAnnotation = "eks.amazonaws.com/role-arn"
)

type AWSManager struct {
	region            string
	stateBucket       string
	stateBucketRegion string
	kubeSvc           *kaws.EKSService
	acmSvc            *kaws.ACMService
}

func NewAWSManager(region string) *AWSManager {
	return &AWSManager{
		region:            region,
		stateBucket:       config.GetConfig().Clouds.AWS.StateS3Bucket,
		stateBucketRegion: config.GetConfig().Clouds.AWS.StateS3BucketRegion,
	}
}

func (a *AWSManager) CheckCreatePermissions() error {
	sess, err := a.awsSession()
	if err != nil {
		return errors.Wrap(err, "Couldn't get aws session")
	}
	iamSvc := iam.New(sess)
	user, err := iamSvc.GetUser(&iam.GetUserInput{})
	if err != nil {
		return errors.Wrapf(err, "Couldn't make authenticated calls using provided credentials")
	}

	p := func(s string) *string { return &s }
	resp, err := iamSvc.SimulatePrincipalPolicy(&iam.SimulatePrincipalPolicyInput{
		ActionNames: []*string{
			p("autoscaling:*"),
			p("ec2:*"),
			p("eks:*"),
			p("iam:*"),
			p("pricing:*"),
			p("s3:*"),
		},
		PolicySourceArn: user.User.Arn,
	})
	if err != nil {
		return errors.Wrapf(err, "Failed to check AWS permissions")
	}
	return checkPermissions(resp.EvaluationResults)
}

func (a *AWSManager) CheckDestroyPermissions() error {
	sess, err := a.awsSession()
	if err != nil {
		return errors.Wrap(err, "Couldn't get aws session")
	}
	iamSvc := iam.New(sess)
	user, err := iamSvc.GetUser(&iam.GetUserInput{})
	if err != nil {
		return errors.Wrapf(err, "Couldn't make authenticated calls using provided credentials")
	}

	p := func(s string) *string { return &s }
	resp, err := iamSvc.SimulatePrincipalPolicy(&iam.SimulatePrincipalPolicyInput{
		ActionNames: []*string{
			p("ec2:*"),
			p("eks:*"),
			p("iam:*"),
			p("s3:*"),
		},
		PolicySourceArn: user.User.Arn,
	})
	if err != nil {
		return errors.Wrapf(err, "Failed to check AWS permissions")
	}
	return checkPermissions(resp.EvaluationResults)
}

func checkPermissions(resp []*iam.EvaluationResult) error {
	missing := make([]string, 0)
	for _, res := range resp {
		if *res.EvalDecision != iam.PolicyEvaluationDecisionTypeAllowed {
			missing = append(missing, *res.EvalActionName)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("unauthorized: missing %s permissions", strings.Join(missing, ", "))
	}
	return nil
}

func (a *AWSManager) CreateCluster(cc *v1alpha1.ClusterConfig) error {
	awsConf := cc.Spec.AWS
	cc.Spec.Region = a.region
	cc.Spec.Version = version.Version

	// TODO: Validate input
	var inventory []string

	if awsConf.VpcId == "" {
		// create new VPC
		inventory = append(inventory,
			fmt.Sprintf("VPC with CIDR (%s)", awsConf.VpcCidr),
			"Subnets for each availability zone",
			"Internet gateway/NAT gateways",
		)
	}
	inventory = append(inventory, "IAM roles for the cluster:")
	inventory = append(inventory, fmt.Sprintf("  kon-%s-service-role", cc.Name))
	inventory = append(inventory, fmt.Sprintf("  kon-%s-node-role", cc.Name))
	inventory = append(inventory, fmt.Sprintf("  kon-%s-admin-role", cc.Name))
	inventory = append(inventory, fmt.Sprintf("  kon-%s-alb-role", cc.Name))
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
	if err := utils.ExplicitConfirmationPrompt("Do you want to proceed?"); err != nil {
		return err
	}

	awsStatus := &v1alpha1.AWSClusterStatus{}
	cc.Status.AWS = awsStatus

	if awsConf.VpcId == "" {
		// run terraform for VPC
		values := a.tfValues()
		values[TFVPCCidr] = awsConf.VpcCidr
		values[TFTopology] = string(awsConf.Topology)
		values[TFEnableIPv6] = cc.Spec.EnableIpv6
		tfVpc, err := NewVPCTFAction(values, awsConf.AvailabilityZones, terraform.OptionDisplayOutput)
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
		tfOut, err := ParseVPCTFOutput(out)
		if err != nil {
			return err
		}
		awsStatus.VpcId = tfOut.VpcId

		// ignore errors
		tfVpc.RemoveDir()
	} else {
		awsStatus.VpcId = awsConf.VpcId
	}

	err := a.updateStatus(awsStatus)
	if err != nil {
		return err
	}

	// create cluster
	values := a.tfValues()
	values[TFCluster] = cc.Name
	values[TFKubeVersion] = cc.Spec.KubeVersion
	values[TFSecurityGroupIds] = awsStatus.SecurityGroups
	values[TFVPCId] = awsStatus.VpcId
	values[TFAdminGroups] = awsConf.AdminGroups
	clusterTf, err := NewEKSClusterTFAction(values, terraform.OptionDisplayOutput)
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
	// ignore errors
	clusterTf.RemoveDir()

	clusterTfOut, err := ParseClusterTFOutput(out)
	if err != nil {
		fmt.Printf("Failed to create the cluster, this might be an issue with AWS quotas. Refer to %s on current usage and request an increase.\n",
			a.quotaConsoleUrl())
		return err
	}

	awsStatus.AlbRoleArn = clusterTfOut.AlbIngressRoleArn
	awsStatus.NodeRoleArn = clusterTfOut.NodeRoleArn
	awsStatus.AdminRoleArn = clusterTfOut.AdminRoleArn

	// tag subnets
	sess, err := a.awsSession()
	if err != nil {
		return err
	}
	eksSvc := kaws.NewEKSService(sess)
	subnetIds := make([]string, 0)
	for _, sub := range awsStatus.PublicSubnets {
		subnetIds = append(subnetIds, sub.SubnetId)
	}
	for _, sub := range awsStatus.PrivateSubnets {
		subnetIds = append(subnetIds, sub.SubnetId)
	}

	err = eksSvc.TagSubnetsForCluster(context.Background(), cc.Name, subnetIds)
	if err != nil {
		return err
	}

	// at last add thumbprint so the provider we created could work
	err = a.addCAThumbprintToProvider(cc.Name)
	if err != nil {
		return err
	}

	return nil
}

func (a *AWSManager) CreateNodepool(cc *v1alpha1.ClusterConfig, np *v1alpha1.Nodepool) error {
	fmt.Println("Creating nodepool...")

	sess, err := a.awsSession()
	if err != nil {
		return err
	}

	// check aws nodepool status. if it doesn't exist, then create it
	kubeProvider := a.KubernetesProvider()
	ready, err := kubeProvider.IsNodepoolReady(context.Background(), cc.Name, np.Name)

	if err != nil {
		// create it
		err = kubeProvider.CreateNodepool(context.TODO(), cc, np)
		if err != nil {
			fmt.Printf("Failed to create the nodepool, this might be an issue with AWS quotas. Refer to %s on current usage and request an increase.\n",
				a.quotaConsoleUrl())
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
			fmt.Printf("Nodepool creation failed after timeout, please check %s for details.",
				a.eksNodePoolUrl(cc, np))
			return err
		}
	}

	np.Status.AWS = &v1alpha1.AWSNodepoolStatus{}

	// now grab the autoscaling group for this
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
					np.Status.AWS.ASGID = *asg.AutoScalingGroupName
					return false
				}
			}
			return true
		},
	)

	return err
}

func (a *AWSManager) ActivateCluster(cc *v1alpha1.ClusterConfig) error {
	if err := a.addAdminRole(cc.Name, cc.Status.AWS.AdminRoleArn); err != nil {
		return err
	}

	eksSvc := eks.New(session.Must(a.awsSession()))

	res, err := eksSvc.DescribeCluster(&eks.DescribeClusterInput{
		Name: &cc.Name,
	})

	if err != nil {
		return err
	}

	_, err = eksSvc.TagResource(&eks.TagResourceInput{
		ResourceArn: res.Cluster.Arn,
		Tags: map[string]*string{
			kaws.TagClusterActivated: aws.String(kaws.TagValue1),
		},
	})

	return err
}

func (a *AWSManager) DeleteCluster(cluster string) error {
	// list all nodepools, and delete them
	sess := session.Must(a.awsSession())
	eksSvc := kaws.NewEKSService(sess)

	// find config and untag resources
	if kclient, err := a.kubernetesClient(cluster); err == nil {
		if cc, err := resources.GetClusterConfig(kclient); err == nil {
			var subnetIds []string
			if len(cc.Status.AWS.PublicSubnets) > 0 {
				for _, sub := range cc.Status.AWS.PublicSubnets {
					subnetIds = append(subnetIds, sub.SubnetId)
				}
				for _, sub := range cc.Status.AWS.PrivateSubnets {
					subnetIds = append(subnetIds, sub.SubnetId)
				}
			} else {
				// older version, list subnets in the VPC and gather this way
				res, err := eksSvc.EKS.DescribeCluster(&eks.DescribeClusterInput{Name: &cc.Name})
				if err != nil {
					return err
				}
				ec2Svc := ec2.New(sess)
				subnetInput := &ec2.DescribeSubnetsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{res.Cluster.ResourcesVpcConfig.VpcId},
						},
					},
				}
				err = ec2Svc.DescribeSubnetsPages(subnetInput, func(subnetRes *ec2.DescribeSubnetsOutput, b bool) bool {
					for _, subnet := range subnetRes.Subnets {
						subnetIds = append(subnetIds, *subnet.SubnetId)
					}
					return true
				})
				if err != nil {
					return err
				}
			}
			err = eksSvc.UnTagSubnetsForCluster(context.Background(), cc.Name, subnetIds)
			if err != nil {
				return err
			}
		}
	}

	listRes, err := eksSvc.EKS.ListNodegroups(&eks.ListNodegroupsInput{
		ClusterName: &cluster,
	})
	if err != nil {
		return err
	}

	for _, item := range listRes.Nodegroups {
		if err := a.DeleteNodepool(cluster, *item); err != nil {
			return err
		}
	}

	// done, load cluster config and delete cluster
	values := a.tfValues()
	values[TFCluster] = cluster
	tf, err := NewEKSClusterTFAction(values, terraform.OptionDisplayOutput)
	if err != nil {
		return err
	}

	if err = tf.Destroy(); err != nil {
		return err
	}
	tf.RemoveDir()

	return nil
}

func (a *AWSManager) DeleteNodepool(cluster string, nodepool string) error {
	// list all nodepools, and delete them
	sess := session.Must(a.awsSession())
	eksSvc := kaws.NewEKSService(sess)

	if err := eksSvc.DeleteNodepool(context.TODO(), cluster, nodepool); err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == "ResourceNotFoundException" {
				// ignore notfound errors for nodepools
				return nil
			}
		}
		return err
	}

	// wait for nodepool to be fully deleted, delete any other resources if needed
	// wait for nodegroups to disappear
	fmt.Printf("Waiting for nodepool %s to be deleted, this may take a few minutes\n", nodepool)
	err := utils.WaitUntilComplete(utils.ReallyLongTimeoutSec, utils.LongCheckInterval, func() (finished bool, err error) {
		listRes, err := eksSvc.EKS.ListNodegroups(&eks.ListNodegroupsInput{
			ClusterName: &cluster,
		})
		if err != nil {
			return
		}

		finished = true
		for _, ng := range listRes.Nodegroups {
			if *ng == nodepool {
				finished = false
				break
			}
		}
		if !finished {
			// try to delete the security group and network interface manually
			// for some reasons AWS doesn't clean it up
			err = eksSvc.DeleteNodeGroupNetworkingResources(context.TODO(), nodepool)
		}
		return
	})
	if err != nil {
		return err
	}
	return nil
}

func (a *AWSManager) DestroyVPC(vpcId string) error {
	vpc, err := a.VPCProvider().GetVPC(context.TODO(), vpcId)
	if err != nil {
		return err
	}

	// destroy security groups
	sess, err := a.awsSession()
	if err != nil {
		return err
	}
	ec2Svc := ec2.New(sess)
	groups, err := kaws.ListSecurityGroups(ec2Svc, vpcId)
	if err != nil {
		return err
	}

	for _, group := range groups {
		if *group.GroupName == "default" {
			continue
		}

		_, err = ec2Svc.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{GroupId: group.GroupId})
		if err != nil {
			return err
		}
	}

	values := a.tfValues()
	values[TFTopology] = vpc.Topology
	values[TFVPCCidr] = vpc.CIDRBlock

	tf, err := NewVPCTFAction(values, nil, terraform.OptionDisplayOutput)
	if err != nil {
		return err
	}

	if err = tf.Destroy(); err != nil {
		return err
	}
	tf.RemoveDir()

	return nil
}

func (a *AWSManager) SyncLinkedServiceAccount(cluster string, lsa *v1alpha1.LinkedServiceAccount) error {
	// get oidc info
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

	var oidcArn string
	providersRes, err := iamSvc.ListOpenIDConnectProviders(&iam.ListOpenIDConnectProvidersInput{})
	if err != nil {
		return err
	}

	// strip protocol
	u, _ := url.Parse(oidcUrl)
	oidcUrl = u.Host + u.Path // strip protocol
	for _, provider := range providersRes.OpenIDConnectProviderList {
		providerRes, err := iamSvc.GetOpenIDConnectProvider(&iam.GetOpenIDConnectProviderInput{
			OpenIDConnectProviderArn: provider.Arn,
		})
		if err != nil {
			return err
		}
		if *providerRes.Url == oidcUrl {
			oidcArn = *provider.Arn
			break
		}
	}

	// run TF task
	values := a.tfValues()
	values[TFCluster] = cluster
	values[TFAccount] = lsa.Name
	values[TFTargets] = lsa.Spec.Targets
	values[TFPolicies] = lsa.Spec.AWS.PolicyARNs
	values[TFOIDCArn] = oidcArn
	values[TFOIDCUrl] = oidcUrl
	tf, err := NewLinkedAccountTFAction(values, terraform.OptionDisplayOutput)
	if err != nil {
		return err
	}

	if err = tf.Apply(); err != nil {
		return err
	}

	output, err := tf.GetOutput()
	if err != nil {
		return err
	}

	roleArn, err := ParseLinkedAccountOutput(output)
	if err != nil {
		return err
	}

	kclient, err := a.kubernetesClient(cluster)
	if err != nil {
		return err
	}
	// annotate service account for each target
	for _, target := range lsa.Spec.Targets {
		// these accounts are already created, time to annotate them
		sa := &corev1.ServiceAccount{}
		err = kclient.Get(context.Background(), client.ObjectKey{Namespace: target, Name: lsa.Name}, sa)
		if err != nil {
			return err
		}

		if sa.Annotations == nil {
			sa.Annotations = map[string]string{}
		}
		sa.Annotations[roleArnAnnotation] = roleArn
		if err = kclient.Update(context.Background(), sa); err != nil {
			return err
		}
	}

	// update synced label and LSA
	lsa.Status.LinkedTargets = lsa.Spec.Targets

	return nil
}

func (a *AWSManager) DeleteLinkedServiceAccount(cluster string, lsa *v1alpha1.LinkedServiceAccount) error {
	// removes all resources for the linked account
	// run TF task
	values := a.tfValues()
	values[TFCluster] = cluster
	values[TFAccount] = lsa.Name
	tf, err := NewLinkedAccountTFAction(values, terraform.OptionDisplayOutput)
	if err != nil {
		return err
	}
	if err = tf.Destroy(); err != nil {
		return err
	}

	kclient, err := a.kubernetesClient(cluster)
	if err != nil {
		return err
	}
	// annotate service account for each target
	for _, target := range lsa.Spec.Targets {
		// these accounts are already created, time to annotate them
		sa := &corev1.ServiceAccount{}
		err = kclient.Get(context.Background(), client.ObjectKey{Namespace: target, Name: lsa.Name}, sa)
		if err != nil {
			if kerrors.IsNotFound(err) {
				continue
			}
			return err
		}

		if sa.Annotations != nil {
			delete(sa.Annotations, roleArnAnnotation)
			sa.Annotations = map[string]string{}
		}
		if err = kclient.Update(context.Background(), sa); err != nil {
			return err
		}
	}
	lsa.Status.LinkedTargets = []string{}

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
		if strings.HasSuffix(*role.RoleName, "-alb-role") {
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

func (a *AWSManager) VPCProvider() cloud.VPCProvider {
	return kaws.NewEC2Service(session.Must(a.awsSession()))
}

func (a *AWSManager) awsSession() (*session.Session, error) {
	return sessionForRegion(a.region)
}

func (a *AWSManager) updateStatus(awsStatus *v1alpha1.AWSClusterStatus) error {
	ec2Svc := ec2.New(session.Must(a.awsSession()))
	vpcFilter := []*ec2.Filter{
		{
			Name:   aws.String("vpc-id"),
			Values: []*string{&awsStatus.VpcId},
		},
	}

	// get subnet info
	awsStatus.PublicSubnets = nil
	awsStatus.PrivateSubnets = nil
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
			for _, as := range subnet.Ipv6CidrBlockAssociationSet {
				if as.Ipv6CidrBlock != nil {
					awsSubnet.Ipv6Cidr = *as.Ipv6CidrBlock
				}
			}
			for _, tag := range subnet.Tags {
				if *tag.Key == kaws.TagSubnetScope {
					// this is our subnet
					if *tag.Value == kaws.TagValuePublic {
						awsSubnet.IsPublic = true
						awsStatus.PublicSubnets = append(awsStatus.PublicSubnets, awsSubnet)
					} else if *tag.Value == kaws.TagValuePrivate {
						awsSubnet.IsPublic = false
						awsStatus.PrivateSubnets = append(awsStatus.PrivateSubnets, awsSubnet)
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
	awsStatus.SecurityGroups = nil
	err = ec2Svc.DescribeSecurityGroupsPages(&ec2.DescribeSecurityGroupsInput{
		Filters: vpcFilter,
	}, func(output *ec2.DescribeSecurityGroupsOutput, last bool) bool {
		for _, sg := range output.SecurityGroups {
			if *sg.GroupName == "default" {
				awsStatus.SecurityGroups = append(awsStatus.SecurityGroups, *sg.GroupId)
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

func (a *AWSManager) addAdminRole(cluster, roleArn string) error {
	kclient, err := a.kubernetesClient(cluster)
	if err != nil {
		return err
	}

	// load the configmap
	confMap, err := resources.GetConfigMap(kclient, resources.KubeSystemNamespace, "aws-auth")
	if err != nil {
		return err
	}

	mappingList, err := ParseRoleMappingList(confMap.Data["mapRoles"])
	if err != nil {
		return err
	}

	r := mappingList.GetRole(roleArn)
	if r == nil {
		r = &IAMRoleMapping{RoleArn: roleArn}
		mappingList = append(mappingList, r)
	}

	r.Groups = []string{"system:masters"}
	r.Username = "user:{{SessionName}}"

	// serialize back and save
	data, err := yaml.Marshal(mappingList)
	if err != nil {
		return err
	}

	confMap.Data["mapRoles"] = string(data)

	return kclient.Update(context.Background(), confMap)
}

func (a *AWSManager) quotaConsoleUrl() string {
	return fmt.Sprintf("https://%s.console.aws.amazon.com/servicequotas/home", a.Region())
}

func (a *AWSManager) eksNodePoolUrl(cc *v1alpha1.ClusterConfig, np *v1alpha1.Nodepool) string {
	return fmt.Sprintf("https://%s.console.aws.amazon.com/eks/home?region=%s#/clusters/%s/nodegroups/%s",
		a.Region(), a.Region(), cc.Name, np.Name)
}

func (a *AWSManager) tfValues() terraform.Values {
	return terraform.Values{
		TFStateBucket:       a.stateBucket,
		TFStateBucketRegion: a.stateBucketRegion,
		TFRegion:            a.region,
	}
}

func (a *AWSManager) kubernetesClient(cluster string) (client.Client, error) {
	contextName := resources.ContextNameForCluster(a.Cloud(), cluster)
	return kube.KubernetesClientWithContext(contextName)
}

func sessionForRegion(region string) (*session.Session, error) {
	conf := config.GetConfig().Clouds.AWS
	creds := conf.GetDefaultCredentials()
	return session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, ""),
	})
}
