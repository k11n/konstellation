package types

import (
	"github.com/k11n/konstellation/api/v1alpha1"
)

type VPC struct {
	ID                    string
	CloudProvider         string
	CIDRBlock             string
	IPv6                  bool
	Topology              v1alpha1.NetworkTopology
	SupportsKonstellation bool
}
