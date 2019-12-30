package providers

import "github.com/davidzhao/konstellation/pkg/cloud"

type CloudProvider interface {
	ID() string
	IsSetup() bool
	Setup() error
	CreateCluster() (name string, err error)
	ConfigureCluster(name string) error

	// utils
	KubernetesProvider() cloud.KubernetesProvider
}
