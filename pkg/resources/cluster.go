package resources

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/utils/objects"
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

func SaveClusterConfig(kclient client.Client, cc *v1alpha1.ClusterConfig) error {
	tmpl := &v1alpha1.ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: cc.Name,
		},
	}

	_, err := controllerutil.CreateOrUpdate(context.TODO(), kclient, tmpl, func() error {
		objects.MergeObject(&tmpl.Spec, &cc.Spec)
		return nil
	})
	return err
}
