package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopLevelDomain(t *testing.T) {
	domain := "mydomain.com"
	assert.Equal(t, domain, TopLevelDomain(domain))
	assert.Equal(t, domain, TopLevelDomain("subdomain.mydomain.com"))
	assert.Equal(t, domain, TopLevelDomain("subsub.subdomain.mydomain.com"))
}

func TestCertificateCovers(t *testing.T) {
	assert.True(t, CertificateCovers("mydomain.com", "mydomain.com"))
	assert.True(t, CertificateCovers("sub.mydomain.com", "sub.mydomain.com"))
	assert.True(t, CertificateCovers("*.mydomain.com", "mydomain.com"))
	assert.True(t, CertificateCovers("*.mydomain.com", "sub.mydomain.com"))
	assert.False(t, CertificateCovers("*.mydomain.com", "sub.sub.mydomain.com"))
	assert.False(t, CertificateCovers("mydomain.com", "wrongdomain.com"))
}
