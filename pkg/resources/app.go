package resources

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
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
		AppLabel: appName,
	})
	if err != nil {
		return
	}
	targets = appTargetList.Items
	return
}

func GetAppTarget(kclient client.Client, name string) (*v1alpha1.AppTarget, error) {
	at := &v1alpha1.AppTarget{}
	err := kclient.Get(context.TODO(), types.NamespacedName{Name: name}, at)
	if err != nil {
		return nil, err
	}
	return at, nil
}

func GetAppTargetWithLabels(kclient client.Client, app, target string) (*v1alpha1.AppTarget, error) {
	appTargetList := v1alpha1.AppTargetList{}
	err := kclient.List(context.TODO(), &appTargetList, client.MatchingLabels{
		AppLabel:    app,
		TargetLabel: target,
	})
	if err != nil {
		return nil, err
	}
	if len(appTargetList.Items) == 0 {
		return nil, ErrNotFound
	}
	return &appTargetList.Items[0], nil
}
