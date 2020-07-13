package resources

const (
	NodepoolPrefix     = "kon-nodepool"
	NodepoolLabel      = "k11n.dev/nodepool"
	AppLabel           = "k11n.dev/app"
	TargetLabel        = "k11n.dev/target"
	BuildRegistryLabel = "k11n.dev/buildRegistry"
	BuildImageLabel    = "k11n.dev/buildImage"
	BuildLabel         = "k11n.dev/build"
	BuildTypeLabel     = "k11n.dev/buildType"
	AppReleaseLabel    = "k11n.dev/appRelease"
	DomainLabel        = "k11n.dev/domain"
	TargetReleaseLabel = "k11n.dev/targetRelease"

	KubeManagedByLabel   = "app.kubernetes.io/managed-by"
	KubeAppLabel         = "app"
	KubeAppVersionLabel  = "app.kubernetes.io/version"
	KubeAppInstanceLabel = "app.kubernetes.io/instance"

	IstioInjectLabel = "istio-injection"

	Konstellation   = "konstellation"
	BuildTypeLatest = "latest"
)
