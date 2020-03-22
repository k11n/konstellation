package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acm"

	"github.com/davidzhao/konstellation/pkg/cloud/types"
)

type ACMService struct {
	session *session.Session
	ACM     *acm.ACM
}

func NewACMService(s *session.Session) *ACMService {
	return &ACMService{
		session: s,
		ACM:     acm.New(s),
	}
}

func (a *ACMService) ListCertificates(ctx context.Context) (certificates []*types.Certificate, err error) {
	out, err := a.ACM.ListCertificatesWithContext(ctx, &acm.ListCertificatesInput{})
	if err != nil {
		return
	}

	for _, summary := range out.CertificateSummaryList {
		var res *acm.DescribeCertificateOutput
		res, err = a.ACM.DescribeCertificateWithContext(ctx, &acm.DescribeCertificateInput{
			CertificateArn: summary.CertificateArn,
		})
		if err != nil {
			return
		}
		certificates = append(certificates, certificateFromDetails(res.Certificate))
	}
	return
}

func (a *ACMService) ImportCertificate(ctx context.Context, cert []byte, pkey []byte, chain []byte, existingID string) (certificate *types.Certificate, err error) {
	input := &acm.ImportCertificateInput{
		Certificate:      cert,
		PrivateKey:       pkey,
		CertificateChain: chain,
	}
	if existingID != "" {
		input.SetCertificateArn(existingID)
	}
	out, err := a.ACM.ImportCertificateWithContext(ctx, input)
	if err != nil {
		return
	}

	res, err := a.ACM.DescribeCertificateWithContext(ctx, &acm.DescribeCertificateInput{
		CertificateArn: out.CertificateArn,
	})
	if err != nil {
		return
	}
	certificate = certificateFromDetails(res.Certificate)
	return
}

func certificateFromDetails(acmCert *acm.CertificateDetail) *types.Certificate {
	cert := &types.Certificate{
		Domain:        *acmCert.DomainName,
		ProviderID:    *acmCert.CertificateArn,
		CloudProvider: "aws",
		ExpiresAt:     acmCert.NotAfter,
	}

	// extract ID from provider ID
	parts := strings.Split(cert.ProviderID, "/")
	cert.ID = parts[1]

	if acmCert.Status == nil {
		cert.Status = types.CertificateStatusNotReady
		return cert
	}
	switch *acmCert.Status {
	case "PENDING_VALIDATION", "INACTIVE":
		cert.Status = types.CertificateStatusNotReady
	case "ISSUED":
		cert.Status = types.CertificateStatusReady
	case "EXPIRED":
		cert.Status = types.CertificateStatusExpired
	case "VALIDATION_TIMED_OUT", "REVOKED", "FAILED":
		cert.Status = types.CertificateStatusError
	}

	if acmCert.Issuer != nil {
		cert.Issuer = *acmCert.Issuer
	}
	if acmCert.KeyAlgorithm != nil {
		cert.KeyAlgorithm = *acmCert.KeyAlgorithm
	}
	if acmCert.SignatureAlgorithm != nil {
		cert.SignatureAlgorithm = *acmCert.SignatureAlgorithm
	}

	return cert
}
