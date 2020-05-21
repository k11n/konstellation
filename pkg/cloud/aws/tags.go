package aws

import "fmt"

const (
	TagSubnetScope       = "k11n.dev/subnetScope"
	TagClusterName       = "k11n.dev/clusterName"
	TagAutoscalerEnabled = "k8s.io/cluster-autoscaler/enabled"
	TagKonstellation     = "Konstellation"
	TagEKSNodeGroupName  = "eks:nodegroup-name"
	TagVPCTopology       = "k11n.dev/topology"
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
