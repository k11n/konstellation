package resources

import "fmt"

const (
	NODEPOOL_PREFIX  = "kon-nodepool"
	NODEPOOL_LABEL   = "k11n.dev/nodepool"
	NODEPOOL_PRIMARY = "primary"
	APP_LABEL        = "k11n.dev/app"
	TARGET_LABEL     = "k11n.dev/target"
	APPTARGET_LABEL  = "k11n.dev/appTarget"
)

var (
	ErrNotFound = fmt.Errorf("The resource is not found")
)
