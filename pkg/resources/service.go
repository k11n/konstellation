package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func GetServiceDNS(service *corev1.Service) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", service.Name, service.Namespace)
}
