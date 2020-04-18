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

type ClusterConfigGenerator interface {
	CreateClusterConfig() (cc *v1alpha1.ClusterConfig, err error)
	CreateNodepoolConfig(cc *v1alpha1.ClusterConfig) (np *v1alpha1.Nodepool, err error)
}

type ClusterManager interface {
	Cloud() string
	Region() string

	CreateCluster(cc *v1alpha1.ClusterConfig) error
	CreateNodepool(cc *v1alpha1.ClusterConfig, np *v1alpha1.Nodepool) error
	DeleteCluster(name string) error
	UpdateClusterSettings(*v1alpha1.ClusterConfig) error

	// utils
	KubernetesProvider() cloud.KubernetesProvider
	CertificateProvider() cloud.CertificateProvider
}
