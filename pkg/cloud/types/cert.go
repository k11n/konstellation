package types

import "time"

type CertificateStatus int

const (
	CertificateStatusNotReady CertificateStatus = iota
	CertificateStatusReady
	CertificateStatusExpired
	CertificateStatusError
)

func (s CertificateStatus) String() string {
	return [...]string{
		"NOT_READY",
		"READY",
		"EXPIRED",
		"ERROR",
	}[s]
}

type Certificate struct {
	ID                 string
	ProviderID         string
	Domain             string
	CloudProvider      string
	Issuer             string
	ExpiresAt          *time.Time
	KeyAlgorithm       string
	SignatureAlgorithm string
	Status             CertificateStatus
}
