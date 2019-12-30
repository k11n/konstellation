package aws

import "fmt"

const (
	TagAutoscalerEnabled = "k8s.io/cluster-autoscaler/enabled"
)

var (
	// cannot be consts because aws api requires pointers
	TagValueOwned = "owned"
	TagValueTrue  = "true"
)

func AutoscalerClusterNameTag(cluster string) string {
	return fmt.Sprintf("k8s.io/cluster-autoscaler/%s", cluster)
}
