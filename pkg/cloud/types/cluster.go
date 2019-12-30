package types

import "time"

import "encoding/json"

type Cluster struct {
	ID              string
	Name            string
	PlatformVersion string
	Status          string
	Tags            map[string]string
	Version         string
	CloudProvider   string
	// detailed fields
	Endpoint                 string
	CertificateAuthorityData string
}

type AuthToken struct {
	Kind       string                 `json:"kind"`
	ApiVersion string                 `json:"apiVersion"`
	Spec       map[string]interface{} `json:"spec"`
	Status     struct {
		ExpirationTimestamp RFC3339Time `json:"expirationTimestamp"`
		Token               string      `json:"token"`
	} `json:"status"`
}

type RFC3339Time time.Time

func (t RFC3339Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).Format(time.RFC3339))
}
