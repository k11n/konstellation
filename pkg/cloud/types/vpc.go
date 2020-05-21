package types

type VPC struct {
	ID                    string
	CloudProvider         string
	CIDRBlock             string
	Topology              string
	SupportsKonstellation bool
}
