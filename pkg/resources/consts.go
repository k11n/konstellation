package resources

import "fmt"

const (
	NODEPOOL_PREFIX      = "kon-nodepool"
	NODEPOOL_LABEL       = "k11n.dev/nodepool"
	NODEPOOL_PRIMARY     = "primary"
	APP_LABEL            = "k11n.dev/app"
	TARGET_LABEL         = "k11n.dev/target"
	APPTARGET_LABEL      = "k11n.dev/appTarget"
	BUILD_REGISTRY_LABEL = "k11n.dev/buildRegistry"
	BUILD_IMAGE_LABEL    = "k11n.dev/buildImage"
)

var (
	ErrNotFound = fmt.Errorf("The resource is not found")
)
