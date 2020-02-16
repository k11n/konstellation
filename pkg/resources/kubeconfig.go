package resources

import (
	"fmt"

	cliv1 "k8s.io/client-go/tools/clientcmd/api/v1"
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

func GenerateKubeConfig(cliPath string, clusters []*KubeClusterConfig, selectedIndex int) *cliv1.Config {
	config := &cliv1.Config{
		Kind:       "Config",
		APIVersion: "v1",
	}

	for i, cluster := range clusters {
		if i == selectedIndex {
			config.CurrentContext = cluster.Name()
		}
		namedCluster := cliv1.NamedCluster{
			Name: cluster.Name(),
			Cluster: cliv1.Cluster{
				Server:                   cluster.EndpointUrl,
				CertificateAuthorityData: cluster.CAData,
			},
		}
		config.Clusters = append(config.Clusters, namedCluster)
		authInfo := cliv1.NamedAuthInfo{
			Name: cluster.User(),
			AuthInfo: cliv1.AuthInfo{
				Exec: &cliv1.ExecConfig{
					APIVersion: "client.authentication.k8s.io/v1alpha1",
					Command:    cliPath,
					Args: []string{
						"cluster",
						"get-token",
						"--cloud",
						cluster.Cloud,
						"--cluster",
						cluster.Cluster,
					},
				},
			},
		}
		config.AuthInfos = append(config.AuthInfos, authInfo)
		config.Contexts = append(config.Contexts, cliv1.NamedContext{
			Name: cluster.Name(),
			Context: cliv1.Context{
				Cluster:  cluster.Name(),
				AuthInfo: cluster.User(),
			},
		})
	}
	return config
}

func ContextNameForCluster(cloud, cluster string) string {
	return fmt.Sprintf("%s-%s", cloud, cluster)
}
