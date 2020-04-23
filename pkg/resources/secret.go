package resources

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/davidzhao/konstellation/cmd/kon/utils"
)

const (
	SERVICE_ACCOUNT_KON_ADMIN = "kon-admin"
)

func GetSecretForAccount(kclient client.Client, name string) (secret *v1.Secret, err error) {
	account := v1.ServiceAccount{}
	err = kclient.Get(context.TODO(), client.ObjectKey{
		Namespace: "kube-system",
		Name:      name,
	}, &account)
	if err != nil {
		return
	}
	if len(account.Secrets) == 0 {
		err = fmt.Errorf("ServiceAccount has no secrets")
		return
	}
	secret = &v1.Secret{}
	utils.PrintJSON(account.Secrets[0])
	err = kclient.Get(context.TODO(), client.ObjectKey{
		Namespace: account.Namespace,
		Name:      account.Secrets[0].Name,
	}, secret)
	if err != nil {
		return
	}
	return
}
