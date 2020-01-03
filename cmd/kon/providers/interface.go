package providers

import (
	"github.com/davidzhao/konstellation/pkg/apis/konstellation/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/cloud"
)

type CloudProvider interface {
	ID() string
	IsSetup() bool
	Setup() error
	CreateCluster() (name string, err error)
	ConfigureCluster(name string) (*v1alpha1.Nodepool, error)

	// utils
	KubernetesProvider() cloud.KubernetesProvider
}
