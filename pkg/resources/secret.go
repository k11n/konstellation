package resources

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	SERVICE_ACCOUNT_KON_ADMIN = "kon-admin"
)

func GetSecretForAccount(kclient client.Client, namespace, name string) (secret *v1.Secret, err error) {
	account := v1.ServiceAccount{}
	err = kclient.Get(context.TODO(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &account)
	if err != nil {
		return
	}
	if len(account.Secrets) == 0 {
		err = fmt.Errorf("ServiceAccount has no secrets")
		return
	}
	return GetSecret(kclient, account.Namespace, account.Secrets[0].Name)
}

func GetSecret(kclient client.Client, namespace, name string) (secret *v1.Secret, err error) {
	secret = &v1.Secret{}
	err = kclient.Get(context.TODO(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, secret)
	return
}
