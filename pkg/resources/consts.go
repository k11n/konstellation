package resources

import "fmt"

const (
	NODEPOOL_PREFIX        = "kon-nodepool"
	NODEPOOL_LABEL         = "k11n.dev/nodepool"
	NODEPOOL_PRIMARY       = "primary"
	APP_LABEL              = "k11n.dev/app"
	TARGET_LABEL           = "k11n.dev/target"
	APPTARGET_LABEL        = "k11n.dev/appTarget"
	RELEASE_REGISTRY_LABEL = "k11n.dev/releaseRegistry"
	RELEASE_IMAGE_LABEL    = "k11n.dev/releaseImage"
	RELEASE_LABEL          = "k11n.dev/release"
	INGRESS_HOST_LABEL     = "k11n.dev/ingressHost"
	DOMAIN_LABEL           = "k11n.dev/domain"
)

var (
	ErrNotFound = fmt.Errorf("The resource is not found")
)
