package resources

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	dateTimeFormat = "20060102-1504"
)

func NodepoolName() string {
	return fmt.Sprintf("%s-%s", NODEPOOL_PREFIX, time.Now().Format(dateTimeFormat))
}

func GetPodNames(pods []corev1.Pod) []string {
	podNames := make([]string, 0, len(pods))
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}
