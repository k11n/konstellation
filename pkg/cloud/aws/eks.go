package aws

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/sts"
	resources "github.com/davidzhao/konstellation/pkg/apis/konstellation/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/cloud/types"
)

const (
	// By default STS signs the url for 15 minutes so we are creating a
	// rfc3339 timestamp with expiration in 14 minutes as part of the token, which
	// is used by some clients (client-go) who will refresh the token after 14 mins
	TOKEN_EXPIRATION_MINS = 14
	URL_TIMEOUT_SECONDS   = 60

	NODEGROUP_NAME = "konstellation-0"
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
		cluster, err := s.GetCluster(ctx, *clusterName)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, cluster)
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
	ec := descOut.Cluster
	cluster = &types.Cluster{
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
	if ec.CertificateAuthority != nil {
		cluster.CertificateAuthorityData = *ec.CertificateAuthority.Data
	}
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
	ready = (*res.Nodegroup.Status == "ACTIVE")
	return
}

func NodepoolSpecToCreateInput(cluster string, np *resources.NodepoolSpec) *eks.CreateNodegroupInput {
	cni := eks.CreateNodegroupInput{}
	cni.SetClusterName(cluster)
	cni.SetAmiType(np.AWS.AMIType)
	cni.SetDiskSize(int64(np.DiskSizeGiB))
	cni.SetInstanceTypes([]*string{&np.MachineType})
	cni.SetNodeRole(np.AWS.RoleARN)
	cni.SetNodegroupName(NODEGROUP_NAME)
	rac := eks.RemoteAccessConfig{
		Ec2SshKey: &np.AWS.SSHKeypair,
	}
	if !np.AWS.ConnectFromAnywhere && np.AWS.SecurityGroupId != "" {
		rac.SetSourceSecurityGroups([]*string{&np.AWS.SecurityGroupId})
	}
	cni.SetRemoteAccess(&rac)
	cni.SetScalingConfig(&eks.NodegroupScalingConfig{
		MinSize:     &np.MinSize,
		MaxSize:     &np.MaxSize,
		DesiredSize: &np.MinSize,
	})

	tags := make(map[string]*string)
	if np.Autoscale {
		tags[AutoscalerClusterNameTag(cluster)] = &TagValueOwned
		tags[TagAutoscalerEnabled] = &TagValueTrue
	}
	cni.SetTags(tags)
	return &cni
}
