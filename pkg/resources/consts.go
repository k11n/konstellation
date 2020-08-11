package resources

const (
	IstioNamespace      = "istio-system"
	KubeSystemNamespace = "kube-system"
	KonSystemNamespace  = "kon-system"
	GrafanaNamespace    = "grafana"

	IngressBackendName = "istio-ingressgateway"
	IngressHealthPath  = "/healthz/ready"
	IngressGatewayName = "ingressgateway"
	MeshGatewayName    = "mesh"
)
