package ingress

import (
	"log"

	netv1beta1 "k8s.io/api/networking/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/components"
)

type IngressComponent interface {
	components.ComponentInstaller
	ConfigureIngress(kclient client.Client, ingress *netv1beta1.Ingress, config *v1alpha1.IngressConfig) error
}

func NewIngressForCluster(cloud, cluster string) IngressComponent {
	if cloud == "aws" {
		return &AWSALBIngress{}
	}
	log.Fatalf("Unsupported cloud: %s", cloud)
	return nil
}
