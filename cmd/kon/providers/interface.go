package providers

import (
	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/cloud"
)

type CloudProvider interface {
	ID() string
	IsSetup() bool
	Setup() error
}

type ClusterManager interface {
	Cloud() string
	Region() string

	CreateCluster() (name string, err error)
	ConfigureNodepool(name string) (np *v1alpha1.Nodepool, err error)

	// utils
	KubernetesProvider() cloud.KubernetesProvider
	CertificateProvider() cloud.CertificateProvider
}
