package resources

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetServiceDNS(service *corev1.Service) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", service.Name, service.Namespace)
}

func GetService(kclient client.Client, namespace, name string) (svc *corev1.Service, err error) {
	svc = &corev1.Service{}
	err = kclient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, svc)
	return
}
