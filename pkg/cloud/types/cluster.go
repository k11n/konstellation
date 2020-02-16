package types

import (
	"encoding/json"
	"time"
)

type ClusterStatus int

const (
	StatusCreating ClusterStatus = iota
	StatusActive
	StatusDeleting
	StatusFailed
	StatusUpdating
	StatusUnconfigured
)

func (s ClusterStatus) String() string {
	return [...]string{
		"CREATING",
		"ACTIVE",
		"DELETING",
		"FAILED",
		"UPDATING",
		"UNCONFIGURED",
	}[s]
}

type Cluster struct {
	ID              string
	Name            string
	PlatformVersion string
	Status          ClusterStatus
	Tags            map[string]string
	Version         string
	CloudProvider   string
	// detailed fields
	Endpoint                 string
	CertificateAuthorityData []byte
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
