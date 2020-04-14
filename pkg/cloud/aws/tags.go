package aws

import "fmt"

const (
	TagKonstellation          = "Konstellation"
	TagSubnetScope            = "k11n.dev/subnetScope"
	TagIngressELBRole         = "kubernetes.io/role/elb"
	TagIngressInternalELBRole = "kubernetes.io/role/internal-elb"
	TagAutoscalerEnabled      = "k8s.io/cluster-autoscaler/enabled"

	TagValue1       = "1"
	TagValuePrivate = "private"
	TagValuePublic  = "public"
	TagValueOwned   = "owned"
	TagValueTrue    = "true"
)

func AutoscalerClusterNameTag(cluster string) string {
	return fmt.Sprintf("k8s.io/cluster-autoscaler/%s", cluster)
}
