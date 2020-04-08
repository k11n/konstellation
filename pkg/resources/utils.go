package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	dateTimeFormat  = "20060102-1504"
	defaultListSize = 10
)

var (
	ErrNotFound = fmt.Errorf("The resource is not found")
)

func GetPodNames(pods []corev1.Pod) []string {
	podNames := make([]string, 0, len(pods))
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}
