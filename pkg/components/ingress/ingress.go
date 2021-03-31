package ingress

import (
	"log"

	netv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/components"
)

type IngressComponent interface {
	components.ComponentInstaller
	ConfigureIngress(kclient client.Client, ingress *netv1.Ingress, irs []*v1alpha1.IngressRequest) error
}

func NewIngressForCluster(cloud, cluster string) IngressComponent {
	if cloud == "aws" {
		return &AWSALBIngress{}
	}
	log.Fatalf("Unsupported cloud: %s", cloud)
	return nil
}
