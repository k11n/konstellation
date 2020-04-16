package ingress

import (
	"log"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/components"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IngressComponent interface {
	components.ComponentInstaller
	GetIngressAnnotations(kclient client.Client, requests []v1alpha1.IngressRequest) (map[string]string, error)
}

func NewIngressForCluster(cloud, cluster string) IngressComponent {
	if cloud == "aws" {
		return &AWSALBIngress{Cluster: cluster}
	}
	log.Fatalf("Unsupported cloud: %s", cloud)
	return nil
}
