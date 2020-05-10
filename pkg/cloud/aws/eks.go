package aws

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/sts"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/cloud/types"
)

const (
	// By default STS signs the url for 15 minutes so we are creating a
	// rfc3339 timestamp with expiration in 14 minutes as part of the token, which
	// is used by some clients (client-go) who will refresh the token after 14 mins
	TOKEN_EXPIRATION_MINS = 14
	URL_TIMEOUT_SECONDS   = 60
)

var (
	statusMapping = map[string]types.ClusterStatus{
		"CREATING": types.StatusCreating,
		"ACTIVE":   types.StatusActive,
		"DELETING": types.StatusDeleting,
		"FAILED":   types.StatusFailed,
		"UPDATING": types.StatusUpdating,
	}
)

type EKSService struct {
	session *session.Session
	EKS     *eks.EKS
}

func NewEKSService(s *session.Session) *EKSService {
	return &EKSService{
		session: s,
		EKS:     eks.New(s),
	}
}

func (s *EKSService) ListClusters(ctx context.Context) (clusters []*types.Cluster, err error) {
	max := int64(100)
	output, err := s.EKS.ListClustersWithContext(ctx, &eks.ListClustersInput{
		MaxResults: &max,
	})
	if err != nil {
		return
	}
	// describe each cluster
	for _, clusterName := range output.Clusters {
		descOut, err := s.EKS.DescribeClusterWithContext(ctx, &eks.DescribeClusterInput{
			Name: clusterName,
		})
		if err != nil {
			return nil, err
		}
		if descOut.Cluster.Tags[TagKonstellation] != nil && *descOut.Cluster.Tags[TagKonstellation] == TagValue1 {
			clusters = append(clusters, clusterFromEksCluster(descOut.Cluster))
		}
	}
	return
}

func (s *EKSService) GetCluster(ctx context.Context, name string) (cluster *types.Cluster, err error) {
	descOut, err := s.EKS.DescribeClusterWithContext(ctx, &eks.DescribeClusterInput{
		Name: &name,
	})
	if err != nil {
		return
	}
	cluster = clusterFromEksCluster(descOut.Cluster)
	return
}

func (s *EKSService) GetAuthToken(ctx context.Context, cluster string) (authToken *types.AuthToken, err error) {
	stsSvc := sts.New(s.session)
	req, _ := stsSvc.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
	req.HTTPRequest.Header.Set("x-k8s-aws-id", cluster)
	signedUrl, err := req.Presign(URL_TIMEOUT_SECONDS * time.Second)
	if err != nil {
		return
	}
	// fmt.Printf("signed url: %s\n", signedUrl)
	encoded := strings.TrimRight(
		base64.URLEncoding.EncodeToString([]byte(signedUrl)),
		"=",
	)

	authToken = &types.AuthToken{
		Kind:       "ExecCredential",
		ApiVersion: "client.authentication.k8s.io/v1alpha1",
		Spec:       make(map[string]interface{}),
	}

	expTime := time.Now().UTC().Add(TOKEN_EXPIRATION_MINS * time.Minute)
	authToken.Status.ExpirationTimestamp = types.RFC3339Time(expTime)
	authToken.Status.Token = fmt.Sprintf("k8s-aws-v1.%s", encoded)
	return
}

func (s *EKSService) IsNodepoolReady(ctx context.Context, clusterName string, nodepoolName string) (ready bool, err error) {
	res, err := s.EKS.DescribeNodegroupWithContext(ctx, &eks.DescribeNodegroupInput{
		ClusterName:   &clusterName,
		NodegroupName: &nodepoolName,
	})
	if err != nil {
		return
	}
	// https://github.com/aws/aws-sdk-go/blob/ab52e2140da6138c05220ee782cc2bcd85feecee/models/apis/eks/2017-11-01/api-2.json#L1048
	ready = *res.Nodegroup.Status == "ACTIVE"
	return
}

func (s *EKSService) IsNodepoolDeleted(ctx context.Context, clusterName string, nodepoolName string) (deleted bool, err error) {
	res, err := s.EKS.ListNodegroupsWithContext(ctx, &eks.ListNodegroupsInput{
		ClusterName: &clusterName,
		MaxResults:  aws.Int64(DefaultPageSize),
	})
	if err != nil {
		return
	}

	for _, item := range res.Nodegroups {
		if *item == nodepoolName {
			return false, nil
		}
	}

	return true, nil
}

func (s *EKSService) DeleteNodeGroupNetworkingResources(ctx context.Context, nodegroup string) error {
	ec2Svc := ec2.New(s.session)

	sgs, err := ec2Svc.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", TagEKSNodeGroupName)),
				Values: []*string{&nodegroup},
			},
		}})
	if err != nil {
		return err
	}

	var groupIds []*string
	for _, sg := range sgs.SecurityGroups {
		groupIds = append(groupIds, sg.GroupId)
	}

	if len(groupIds) == 0 {
		return nil
	}

	// find all network interfaces and delete
	niRes, err := ec2Svc.DescribeNetworkInterfacesWithContext(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("group-id"),
				Values: groupIds,
			},
		},
	})
	if err != nil {
		return err
	}

	isSuccess := true
	for _, ni := range niRes.NetworkInterfaces {
		// ignore errors
		_, err := ec2Svc.DeleteNetworkInterfaceWithContext(ctx, &ec2.DeleteNetworkInterfaceInput{
			NetworkInterfaceId: ni.NetworkInterfaceId,
		})
		if err != nil {
			isSuccess = false
		}
	}

	if isSuccess {
		// delete security groups
		for _, groupId := range groupIds {
			ec2Svc.DeleteSecurityGroupWithContext(ctx, &ec2.DeleteSecurityGroupInput{
				GroupId: groupId,
			})
		}
	}
	return nil
}

func (s *EKSService) CreateNodepool(ctx context.Context, clusterName string, np *v1alpha1.Nodepool) error {
	// tag VPC subnets if needed
	ec2Svc := ec2.New(s.session)
	subnetIds := []*string{}
	for _, sId := range np.Spec.AWS.SubnetIds {
		subnetIds = append(subnetIds, aws.String(sId))
	}
	subnetRes, err := ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{SubnetIds: subnetIds})
	if err != nil {
		return err
	}

	clusterTag := "kubernetes.io/cluster/" + clusterName
	resourcesToTag := []*string{}
	for _, subnet := range subnetRes.Subnets {
		tag := GetEC2Tag(subnet.Tags, clusterTag)
		if tag == nil {
			resourcesToTag = append(resourcesToTag, subnet.SubnetId)
		}
	}

	if len(resourcesToTag) > 0 {
		_, err = ec2Svc.CreateTags(&ec2.CreateTagsInput{
			Resources: resourcesToTag,
			Tags: []*ec2.Tag{
				{
					Key:   &clusterTag,
					Value: aws.String(TagValueShared),
				},
			},
		})
		if err != nil {
			return err
		}
	}

	createInput := nodepoolSpecToCreateInput(clusterName, np)
	_, err = s.EKS.CreateNodegroupWithContext(ctx, createInput)
	return err
}

func (s *EKSService) DeleteNodepool(ctx context.Context, clusterName string, nodePool string) error {
	_, err := s.EKS.DeleteNodegroupWithContext(ctx, &eks.DeleteNodegroupInput{
		ClusterName:   &clusterName,
		NodegroupName: &nodePool,
	})
	return err
}

func nodepoolSpecToCreateInput(cluster string, np *v1alpha1.Nodepool) *eks.CreateNodegroupInput {
	nps := np.Spec
	cni := eks.CreateNodegroupInput{}
	cni.SetClusterName(cluster)
	cni.SetAmiType(nps.AWS.AMIType)
	cni.SetDiskSize(int64(nps.DiskSizeGiB))
	cni.SetInstanceTypes([]*string{&nps.MachineType})
	cni.SetNodeRole(nps.AWS.RoleARN)
	cni.SetNodegroupName(np.ObjectMeta.Name)
	rac := eks.RemoteAccessConfig{
		Ec2SshKey: &nps.AWS.SSHKeypair,
	}
	if !nps.AWS.ConnectFromAnywhere && nps.AWS.SecurityGroupId != "" {
		rac.SetSourceSecurityGroups([]*string{&nps.AWS.SecurityGroupId})
	}
	cni.SetRemoteAccess(&rac)
	cni.SetScalingConfig(&eks.NodegroupScalingConfig{
		MinSize:     &nps.MinSize,
		MaxSize:     &nps.MaxSize,
		DesiredSize: &nps.MinSize,
	})
	for _, subnetId := range nps.AWS.SubnetIds {
		cni.Subnets = append(cni.Subnets, aws.String(subnetId))
	}

	tags := make(map[string]*string)
	if nps.Autoscale {
		tags[AutoscalerClusterNameTag(cluster)] = aws.String(TagValueOwned)
		tags[TagAutoscalerEnabled] = aws.String(TagValueTrue)
	}
	cni.SetTags(tags)

	return &cni
}

func clusterFromEksCluster(ec *eks.Cluster) *types.Cluster {
	cluster := &types.Cluster{
		ID:              *ec.Arn,
		CloudProvider:   "aws",
		Name:            *ec.Name,
		PlatformVersion: *ec.PlatformVersion,
		Status:          statusMapping[*ec.Status],
		Version:         *ec.Version,
	}
	if ec.Endpoint != nil {
		cluster.Endpoint = *ec.Endpoint
	}
	if ec.CertificateAuthority != nil && ec.CertificateAuthority.Data != nil {
		var decoded []byte
		decoded, err := base64.StdEncoding.DecodeString(*ec.CertificateAuthority.Data)
		if err == nil {
			cluster.CertificateAuthorityData = decoded
		}
	}
	return cluster
}

func GetEC2Tag(tags []*ec2.Tag, name string) *ec2.Tag {
	for _, tag := range tags {
		if *tag.Key == name {
			return tag
		}
	}
	return nil
}
