package cloud

import (
	"context"

	"github.com/davidzhao/konstellation/pkg/apis/konstellation/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/cloud/types"
)

type KubernetesProvider interface {
	ListClusters(context.Context) ([]*types.Cluster, error)
	GetCluster(context.Context, string) (*types.Cluster, error)
	GetAuthToken(ctx context.Context, cluster string) (*types.AuthToken, error)
	IsNodepoolReady(ctx context.Context, clusterName string, nodepoolName string) (bool, error)
	CreateNodepool(ctx context.Context, clusterName string, np *v1alpha1.Nodepool, purpose string) error
}
