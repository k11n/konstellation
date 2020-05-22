package ingress

import (
	"log"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/components"
)

type IngressComponent interface {
	components.ComponentInstaller
	GetIngressAnnotations(kclient client.Client, tlsHosts []string) (map[string]string, error)
}

func NewIngressForCluster(cloud, cluster string) IngressComponent {
	if cloud == "aws" {
		return &AWSALBIngress{}
	}
	log.Fatalf("Unsupported cloud: %s", cloud)
	return nil
}
