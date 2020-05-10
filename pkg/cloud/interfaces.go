package cloud

import (
	"context"
	"io"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/cloud/types"
)

type KubernetesProvider interface {
	ListClusters(context.Context) ([]*types.Cluster, error)
	GetCluster(context.Context, string) (*types.Cluster, error)
	GetAuthToken(ctx context.Context, cluster string) (*types.AuthToken, error)
	IsNodepoolReady(ctx context.Context, clusterName string, nodepoolName string) (bool, error)
	CreateNodepool(ctx context.Context, clusterName string, np *v1alpha1.Nodepool) error
}

type CertificateProvider interface {
	ListCertificates(context.Context) ([]*types.Certificate, error)
	ImportCertificate(ctx context.Context, cert []byte, pkey []byte, chain []byte, existingID string) (*types.Certificate, error)
}

type StorageProvider interface {
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	PutObject(ctx context.Context, key string, obj io.ReadCloser) error
}
