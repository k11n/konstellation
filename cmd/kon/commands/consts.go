package commands

const (
	CLUSTER_CREATE_HELP = `Creates a new Kubernetes cluster including the necessary roles required by Kubernetes.
The created cluster will use sane defaults for network configurations. If customizations are desired beyond what Konstellation provides, please create the cluster using other tools.`
)

var (
	KUBE_RESOURCES = []string{
		"service_account.yaml",
		"role.yaml",
		"role_binding.yaml",
		"crds/k11n.dev_apps_crd.yaml",
		"crds/k11n.dev_clusterconfigs_crd.yaml",
		"crds/k11n.dev_nodepools_crd.yaml",
		// "operator.yaml",
	}
)
