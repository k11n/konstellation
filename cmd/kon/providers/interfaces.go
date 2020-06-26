package providers

import (
	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/cloud"
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

	CheckCreatePermissions() error
	CheckDestroyPermissions() error

	// Cluster
	CreateCluster(cc *v1alpha1.ClusterConfig) error
	CreateNodepool(cc *v1alpha1.ClusterConfig, np *v1alpha1.Nodepool) error
	DeleteCluster(name string) error

	// VPC
	DestroyVPC(vpcId string) error

	// LinkedServiceAccount
	SyncLinkedServiceAccount(cluster string, lsa *v1alpha1.LinkedServiceAccount) error
	DeleteLinkedServiceAccount(cluster string, lsa *v1alpha1.LinkedServiceAccount) error

	// utils
	KubernetesProvider() cloud.KubernetesProvider
	CertificateProvider() cloud.CertificateProvider
	VPCProvider() cloud.VPCProvider
}
