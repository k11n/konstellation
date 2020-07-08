package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/gammazero/workerpool"

	"github.com/k11n/konstellation/pkg/cloud/types"
	"github.com/k11n/konstellation/pkg/utils/async"
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

	wp := workerpool.New(10)
	tasks := make([]*async.Task, 0, len(out.CertificateSummaryList))
	for i := range out.CertificateSummaryList {
		summary := out.CertificateSummaryList[i]
		task := async.NewTask(func() (interface{}, error) {
			res, err := a.ACM.DescribeCertificateWithContext(ctx, &acm.DescribeCertificateInput{
				CertificateArn: summary.CertificateArn,
			})
			if err != nil {
				return nil, err
			}
			return certificateFromDetails(res.Certificate), nil
		})
		tasks = append(tasks, task)
		wp.Submit(task.Run)
	}

	wp.StopWait()
	for _, t := range tasks {
		if t.Err != nil {
			return nil, t.Err
		}
		certificates = append(certificates, t.Result.(*types.Certificate))
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
