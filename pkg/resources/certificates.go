package resources

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
)

func ListCertificates(kclient client.Client) (certificates []v1alpha1.CertificateRef, err error) {
	certList := v1alpha1.CertificateRefList{}
	err = kclient.List(context.TODO(), &certList)
	if err == nil {
		certificates = certList.Items
	}
	return
}

func GetCertificateForDomain(kclient client.Client, domain string) (cert *v1alpha1.CertificateRef, err error) {
	certList := v1alpha1.CertificateRefList{}
	err = kclient.List(context.TODO(), &certList, client.MatchingLabels{
		DOMAIN_LABEL: domain,
	})
	if err != nil {
		return
	}
	if len(certList.Items) == 0 {
		err = ErrNotFound
		return
	}
	cert = &certList.Items[0]
	return
}
