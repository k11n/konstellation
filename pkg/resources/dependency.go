package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
)

func GetServiceHostEnvForReference(kclient client.Client, ref v1alpha1.AppReference, defaultTarget string) (envs []corev1.EnvVar, err error) {
	target := ref.Target
	if target == "" {
		target = defaultTarget
	}

	at, err := GetAppTargetWithLabels(kclient, ref.Name, target)
	if err != nil {
		return
	}

	for _, port := range at.Spec.Ports {
		if ref.Port != "" && ref.Port != port.Name {
			continue
		}
		name := fmt.Sprintf("%s_%s_HOST", ToEnvVar(ref.Name), ToEnvVar(port.Name))
		envs = append(envs, corev1.EnvVar{
			Name:  name,
			Value: fmt.Sprintf("%s:%d", at.ServiceHostName(), port.Port),
		})
	}
	return
}
