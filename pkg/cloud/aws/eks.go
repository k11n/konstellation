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
	"github.com/davidzhao/konstellation/pkg/cloud/types"
)

const (
	// By default STS signs the url for 15 minutes so we are creating a
	// rfc3339 timestamp with expiration in 14 minutes as part of the token, which
	// is used by some clients (client-go) who will refresh the token after 14 mins
	TOKEN_EXPIRATION_MINS = 14
	URL_TIMEOUT_SECONDS   = 60
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
		ID:                       *ec.Arn,
		CloudProvider:            "aws",
		Name:                     *ec.Name,
		PlatformVersion:          *ec.PlatformVersion,
		Status:                   *ec.Status,
		Version:                  *ec.Version,
		Endpoint:                 *ec.Endpoint,
		CertificateAuthorityData: *ec.CertificateAuthority.Data,
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
