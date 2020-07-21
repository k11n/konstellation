package resources

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
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
		DomainLabel: TopLevelDomain(domain),
	})
	if err != nil {
		return
	}
	for _, item := range certList.Items {
		if item.Spec.Domain == domain {
			cert = &item
			return
		}
	}
	err = ErrNotFound
	return
}

func GetCertificateThatMatchDomain(kclient client.Client, domain string) (cert *v1alpha1.CertificateRef, err error) {
	// TODO: this should be more efficient
	certList := v1alpha1.CertificateRefList{}
	err = kclient.List(context.TODO(), &certList, client.MatchingLabels{
		DomainLabel: TopLevelDomain(domain),
	})
	if err != nil {
		return
	}

	for _, item := range certList.Items {
		if CertificateCovers(item.Spec.Domain, domain) {
			cert = &item
			return
		}
	}
	err = ErrNotFound
	return
}

func TopLevelDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return domain
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// Checks if certificate can cover the domain
func CertificateCovers(certDomain string, domain string) bool {
	// break down by ., and compare parts, allow initial part to be *
	certParts := strings.Split(certDomain, ".")
	domainParts := strings.Split(domain, ".")

	if len(certParts) != len(domainParts) {
		// only if it's a wildcard and covers domain entirely
		return certDomain == "*."+domain
	}

	for i, certPart := range certParts {
		domainPart := domainParts[i]
		if certPart != domainPart {
			// handle wildcard certs
			if i == 0 && certPart == "*" {
				continue
			}
			return false
		}
	}

	return true
}
