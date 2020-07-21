package resources

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
)

func GetClusterConfig(kclient client.Client) (cc *v1alpha1.ClusterConfig, err error) {
	ccList := v1alpha1.ClusterConfigList{}
	err = kclient.List(context.Background(), &ccList)
	if err != nil {
		return
	}

	if len(ccList.Items) == 0 {
		err = ErrNotFound
		return
	}

	cc = &ccList.Items[0]
	return
}
