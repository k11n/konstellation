package resources

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
)

func GetAppConfig(kclient client.Client, app, target string) (ac *v1alpha1.AppConfig, err error) {
	appConfigList := v1alpha1.AppConfigList{}
	err = kclient.List(context.TODO(), &appConfigList, client.MatchingLabels{
		APP_LABEL:    app,
		TARGET_LABEL: target,
	})
	if err != nil {
		return
	}

	if len(appConfigList.Items) == 0 {
		err = ErrNotFound
		return
	}

	ac = &appConfigList.Items[0]
	return
}

func SaveAppConfig(kclient client.Client, ac *v1alpha1.AppConfig) error {
	existing := v1alpha1.AppConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ac.Namespace,
			Name:      ac.Name,
		},
	}
	_, err := controllerutil.CreateOrUpdate(context.TODO(), kclient, &existing, func() error {
		existing.Labels = ac.Labels
		existing.Annotations = ac.Annotations
		existing.ConfigYaml = ac.ConfigYaml
		return nil
	})
	return err
}
