package resources

import (
	"fmt"
	"strings"

	cliv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

const (
	konPrefix = "kon-"
)

type KubeClusterConfig struct {
	Cloud       string
	Cluster     string
	EndpointUrl string
	CAData      []byte
}

func (c *KubeClusterConfig) Name() string {
	return ContextNameForCluster(c.Cloud, c.Cluster)
}

func (c *KubeClusterConfig) User() string {
	return fmt.Sprintf("%s-user", c.Name())
}

func NewKubeConfig() *cliv1.Config {
	return &cliv1.Config{
		Kind:       "Config",
		APIVersion: "v1",
	}
}

func UpdateKubeConfig(kconf *cliv1.Config, cliPath string, clusters []*KubeClusterConfig, selectedIndex int) {
	// first remove all context and clusters with Kon in them, then append our own
	newClusters := make([]cliv1.NamedCluster, 0, len(kconf.Clusters))
	newUsers := make([]cliv1.NamedAuthInfo, 0, len(kconf.AuthInfos))
	newContexts := make([]cliv1.NamedContext, 0, len(kconf.Contexts))
	for _, cluster := range kconf.Clusters {
		if !strings.HasPrefix(cluster.Name, konPrefix) {
			newClusters = append(newClusters, cluster)
		}
	}
	for _, user := range kconf.AuthInfos {
		if !strings.HasPrefix(user.Name, konPrefix) {
			newUsers = append(newUsers, user)
		}
	}
	for _, ctx := range kconf.Contexts {
		if !strings.HasPrefix(ctx.Name, konPrefix) {
			newContexts = append(newContexts, ctx)
		}
	}
	kconf.Clusters = newClusters
	kconf.Contexts = newContexts
	kconf.AuthInfos = newUsers

	if strings.HasPrefix(kconf.CurrentContext, konPrefix) {
		kconf.CurrentContext = ""
	}

	// append our contexts and select active
	for i, cluster := range clusters {
		if i == selectedIndex {
			kconf.CurrentContext = cluster.Name()
		}
		namedCluster := cliv1.NamedCluster{
			Name: cluster.Name(),
			Cluster: cliv1.Cluster{
				Server:                   cluster.EndpointUrl,
				CertificateAuthorityData: cluster.CAData,
			},
		}
		kconf.Clusters = append(kconf.Clusters, namedCluster)
		authInfo := cliv1.NamedAuthInfo{
			Name: cluster.User(),
			AuthInfo: cliv1.AuthInfo{
				Exec: &cliv1.ExecConfig{
					APIVersion: "client.authentication.k8s.io/v1alpha1",
					Command:    cliPath,
					Args: []string{
						"cluster",
						"get-token",
						"--cluster",
						cluster.Cluster,
					},
				},
			},
		}
		kconf.AuthInfos = append(kconf.AuthInfos, authInfo)
		kconf.Contexts = append(kconf.Contexts, cliv1.NamedContext{
			Name: cluster.Name(),
			Context: cliv1.Context{
				Cluster:  cluster.Name(),
				AuthInfo: cluster.User(),
			},
		})
	}
}

func KubeConfigContainsClusters(kconf *cliv1.Config, cloud string, clusters []string) bool {
	containsAll := true

	for _, cluster := range clusters {
		containsCluster := false
		contextName := ContextNameForCluster(cloud, cluster)
		for _, ctx := range kconf.Contexts {
			if ctx.Name == contextName {
				containsCluster = true
				break
			}
		}
		if !containsCluster {
			containsAll = false
			break
		}
	}
	return containsAll
}

func ContextNameForCluster(cloud, cluster string) string {
	return fmt.Sprintf("%s%s-%s", konPrefix, cloud, cluster)
}
