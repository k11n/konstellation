package cloud

import (
	"context"

	"github.com/davidzhao/konstellation/pkg/cloud/types"
)

type KubernetesProvider interface {
	ListClusters(context.Context) ([]*types.Cluster, error)
	GetCluster(context.Context, string) (*types.Cluster, error)
	GetAuthToken(ctx context.Context, cluster string) (*types.AuthToken, error)
}
