package aws

import "fmt"

const (
	TagSubnetScope       = "k11n.dev/subnetScope"
	TagClusterName       = "k11n.dev/clusterName"
	TagClusterActivated  = "k11n.dev/clusterActivated"
	TagAutoscalerEnabled = "k8s.io/cluster-autoscaler/enabled"
	TagKonstellation     = "Konstellation"
	TagEKSNodeGroupName  = "eks:nodegroup-name"
	TagVPCTopology       = "k11n.dev/topology"
	TagKubeClusterPrefix = "kubernetes.io/cluster/"
	//TagIngressELBRole         = "kubernetes.io/role/elb"
	//TagIngressInternalELBRole = "kubernetes.io/role/internal-elb"

	TagValue1       = "1"
	TagValuePrivate = "private"
	TagValuePublic  = "public"
	TagValueOwned   = "owned"
	TagValueShared  = "shared"
	TagValueTrue    = "true"
)

func AutoscalerClusterNameTag(cluster string) string {
	return fmt.Sprintf("k8s.io/cluster-autoscaler/%s", cluster)
}

func KubeClusterTag(cluster string) string {
	return fmt.Sprintf("%s%s", TagKubeClusterPrefix, cluster)
}
