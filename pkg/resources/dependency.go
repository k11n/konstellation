package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
)

func GetServiceHostEnvForReference(kclient client.Client, ref v1alpha1.AppReference, defaultTarget string) (envs []corev1.EnvVar, err error) {
	deps, err := GetDependencyInfos(kclient, ref, defaultTarget)
	if err != nil {
		return
	}

	for _, dep := range deps {
		envs = append(envs, corev1.EnvVar{
			Name:  dep.HostKey(),
			Value: fmt.Sprintf("%s:%d", ServiceHostname(dep.Namespace, dep.Service), dep.Port),
		})
	}
	return
}

type DependencyInfo struct {
	Namespace string
	Service   string
	Port      int
	PortName  string
}

func (d DependencyInfo) HostKey() string {
	return fmt.Sprintf("%s_%s_HOST", ToEnvVar(d.Service), ToEnvVar(d.PortName))
}

// For when running locally, return dependency data so proxies can be created
func GetDependencyInfos(kclient client.Client, ref v1alpha1.AppReference, defaultTarget string) (deps []DependencyInfo, err error) {
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
		deps = append(deps, DependencyInfo{
			Namespace: at.TargetNamespace(),
			Service:   ref.Name,
			Port:      int(port.Port),
			PortName:  port.Name,
		})
	}
	return
}
