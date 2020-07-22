package kube

import (
	istioapi "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	metrics "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kconf "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/components/autoscaler"
	"github.com/k11n/konstellation/pkg/components/grafana"
	"github.com/k11n/konstellation/pkg/components/istio"
	"github.com/k11n/konstellation/pkg/components/konstellation"
	"github.com/k11n/konstellation/pkg/components/kubedash"
	"github.com/k11n/konstellation/pkg/components/metricsserver"
	"github.com/k11n/konstellation/pkg/components/prometheus"
)

var (
	KUBE_RESOURCES = []string{
		"admin_account.yaml",
		"crds.yaml",
	}

	KubeComponents = []components.ComponentInstaller{
		// TODO: this might not be required on some installs
		&metricsserver.MetricsServer{},
		&kubedash.KubeDash{},
		&autoscaler.ClusterAutoScaler{},
		&istio.IstioInstaller{},
		&prometheus.KubePrometheus{},
		&grafana.GrafanaOperator{},
		&konstellation.Konstellation{},
	}
)

var (
	// construct a client from local config
	scheme = runtime.NewScheme()
)

func init() {
	// register both our scheme and konstellation scheme
	v1alpha1.AddToScheme(scheme)
	clientgoscheme.AddToScheme(scheme)
	metrics.AddToScheme(scheme)
	istioapi.AddToScheme(scheme)
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
