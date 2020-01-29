package resources

import (
	"context"
	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ListApps(kclient client.Client) (apps []v1alpha1.App, err error) {
	appList := v1alpha1.AppList{}
	err = kclient.List(context.TODO(), &appList)
	if err != nil {
		return
	}
	apps = appList.Items
	return
}
