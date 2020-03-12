package resources

import (
	"context"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
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

func GetAppByName(kclient client.Client, name string) (app *v1alpha1.App, err error) {
	app = &v1alpha1.App{}
	err = kclient.Get(context.TODO(), types.NamespacedName{Name: name}, app)
	return
}

func GetAppTargets(kclient client.Client, appName string) (targets []v1alpha1.AppTarget, err error) {
	appTargetList := v1alpha1.AppTargetList{}
	err = kclient.List(context.TODO(), &appTargetList, client.MatchingLabels{
		APP_LABEL: appName,
	})
	if err != nil {
		return
	}
	targets = appTargetList.Items
	return
}
