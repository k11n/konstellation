package resources

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sort"
	"strings"

	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
)

func GetConfigForType(kclient client.Client, confType v1alpha1.ConfigType, name string, target string) (ac *v1alpha1.AppConfig, err error) {
	labels := client.MatchingLabels{
		TargetLabel: target,
	}
	if confType == v1alpha1.ConfigTypeApp {
		labels[AppLabel] = name
	} else if confType == v1alpha1.ConfigTypeShared {
		labels[v1alpha1.SharedConfigLabel] = name
	}

	appConfigList := v1alpha1.AppConfigList{}
	err = kclient.List(context.TODO(), &appConfigList, labels)
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
		existing.Type = ac.Type
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

func GetMergedConfigForType(kclient client.Client, confType v1alpha1.ConfigType, name, target string) (ac *v1alpha1.AppConfig, err error) {
	// grab app release for this app
	baseConfig, err := GetConfigForType(kclient, confType, name, "")
	if err == ErrNotFound {
		baseConfig = nil
	} else if err != nil {
		return
	}

	targetConfig, err := GetConfigForType(kclient, confType, name, target)
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
		err = nil
		return
	}

	// merge if needed
	if targetConfig != nil {
		baseConfig.MergeWith(targetConfig)
	}

	return baseConfig, nil
}

func CreateConfigMap(appName string, ac *v1alpha1.AppConfig, sharedConfigs []*v1alpha1.AppConfig) *corev1.ConfigMap {
	data := make(map[string]string)
	if ac != nil {
		data = ac.ToEnvMap()
	}

	for _, conf := range sharedConfigs {
		// store these as straight YAML
		name := conf.GetSharedName()
		name = strings.ToUpper(name)
		name = strings.ReplaceAll(name, "-", "_")

		data[name] = string(conf.ConfigYaml)
	}

	// create sha
	keys := funk.Keys(data).([]string)
	sort.Strings(keys)
	h := sha1.New()
	for _, key := range keys {
		h.Write([]byte(fmt.Sprintf("%s=%s", key, data[key])))
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", appName, hash[:6]),
			Labels: map[string]string{
				v1alpha1.ConfigHashLabel: hash,
			},
		},
		Data: data,
	}
}
