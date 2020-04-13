package tls

import (
	"crypto/sha1"
	"crypto/tls"
	"fmt"
	"net/http"
)

func GetIssuerCAThumbprint(url string) (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			Proxy: http.ProxyFromEnvironment,
		},
	}

	response, err := client.Get(url)
	if err != nil {
		return "", err
	}

	if response.TLS != nil {
		if numCerts := len(response.TLS.PeerCertificates); numCerts >= 1 {
			root := response.TLS.PeerCertificates[numCerts-1]
			return fmt.Sprintf("%x", sha1.Sum(root.Raw)), nil
		}
	}
	return "", fmt.Errorf("unable to get issuer's certificate")
}
