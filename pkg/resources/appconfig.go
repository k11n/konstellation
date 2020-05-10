package resources

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
)

func GetAppConfig(kclient client.Client, app, target string) (ac *v1alpha1.AppConfig, err error) {
	appConfigList := v1alpha1.AppConfigList{}
	err = kclient.List(context.TODO(), &appConfigList, client.MatchingLabels{
		AppLabel:    app,
		TargetLabel: target,
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

func GetConfigMap(kclient client.Client, namespace string, name string) (cm *corev1.ConfigMap, err error) {
	cm = &corev1.ConfigMap{}
	err = kclient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, cm)
	return
}

func GetMergedAppConfig(kclient client.Client, app, target string) (ac *v1alpha1.AppConfig, err error) {
	// grab app release for this app
	baseConfig, err := GetAppConfig(kclient, app, "")
	if err == ErrNotFound {
		baseConfig = nil
	} else if err != nil {
		return
	}

	targetConfig, err := GetAppConfig(kclient, app, target)
	if err == ErrNotFound {
		targetConfig = nil
	} else if err != nil {
		return
	}

	if baseConfig == nil {
		baseConfig = targetConfig
		targetConfig = nil
	}

	if baseConfig == nil {
		return
	}

	// merge if needed
	if targetConfig != nil {
		baseConfig.MergeWith(targetConfig)
	}

	return baseConfig, nil
}
