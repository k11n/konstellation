package commands

import (
	"os"
	"path"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kconf "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/davidzhao/konstellation/pkg/apis"
)

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

var (
	// construct a client from local config
	scheme = runtime.NewScheme()
)

func init() {
	// register both our scheme and konstellation scheme
	clientgoscheme.AddToScheme(scheme)
	apis.AddToScheme(scheme)
}

func KubernetesClientWithContext(contextName string) (client.Client, error) {
	conf, err := kconf.GetConfigWithContext(contextName)
	if err != nil {
		return nil, err
	}
	return client.New(conf, client.Options{Scheme: scheme})
}

func GetKubeDecoder() runtime.Decoder {
	return clientgoscheme.Codecs.UniversalDeserializer()
}

func GetKubeEncoder() runtime.Encoder {
	return json.NewSerializerWithOptions(json.DefaultMetaFactory, nil, nil,
		json.SerializerOptions{
			Yaml:   true,
			Pretty: true,
			Strict: false,
		})
}

func kubeConfigPath() (string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return path.Join(homedir, ".kube", "config"), nil
}
