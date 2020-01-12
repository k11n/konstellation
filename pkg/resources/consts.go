package resources

import "fmt"

const (
	NODEPOOL_PREFIX  = "kon-nodepool"
	NODEPOOL_LABEL   = "k11n.dev/nodepool"
	NODEPOOL_PRIMARY = "primary"
)

var (
	ErrNotFound = fmt.Errorf("The resource is not found")
)
