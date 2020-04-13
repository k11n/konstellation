package commands

const (
	CLUSTER_CREATE_HELP = `Creates a new Kubernetes cluster including the roles and networking resources required by Konstellation.`
)

var (
	KUBE_RESOURCES = []string{
		"service_account.yaml",
		"role.yaml",
		"role_binding.yaml",
		"crds/k11n.dev_apps_crd.yaml",
		"crds/k11n.dev_apptargets_crd.yaml",
		"crds/k11n.dev_builds_crd.yaml",
		"crds/k11n.dev_certificaterefs_crd.yaml",
		"crds/k11n.dev_clusterconfigs_crd.yaml",
		"crds/k11n.dev_ingressrequests_crd.yaml",
		"crds/k11n.dev_nodepools_crd.yaml",
		// "operator.yaml",
	}
)
