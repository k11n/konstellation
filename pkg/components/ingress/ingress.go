package ingress

import (
	"log"

	"github.com/davidzhao/konstellation/pkg/components"
)

func NewIngressForCluster(cloud, cluster string) components.ComponentInstaller {
	if cloud == "aws" {
		return &AWSALBIngress{Cluster:cluster}
	}
	log.Fatalf("Unsupported cloud: %s", cloud)
	return nil
}
