package resources

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetNamespace(kclient client.Client, namespace string) (*corev1.Namespace, error) {
	n := corev1.Namespace{}
	err := kclient.Get(context.TODO(), types.NamespacedName{Name: namespace}, &n)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func EnsureNamespaceCreated(kclient client.Client, namespace string) error {
	_, err := GetNamespace(kclient, namespace)
	if err == nil {
		return nil
	}

	// create a new one
	n := corev1.Namespace{}
	n.Name = namespace
	return kclient.Create(context.TODO(), &n)
}
