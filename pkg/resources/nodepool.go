package resources

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
)

func NodepoolName() string {
	return fmt.Sprintf("%s-%s", NodepoolPrefix, time.Now().Format(dateTimeFormat))
}

func GetNodepools(kclient client.Client) (pools []*v1alpha1.Nodepool, err error) {
	npList := v1alpha1.NodepoolList{}
	err = kclient.List(context.Background(), &npList)
	if err != nil {
		return
	}

	for i := range npList.Items {
		pool := npList.Items[i]
		pools = append(pools, &pool)
	}
	return
}

var (
	nodepoolLabels = []string{
		"eks.amazonaws.com/nodegroup",
	}
)

func NodepoolNameFromNode(node *corev1.Node) string {
	if node.Labels == nil {
		return ""
	}
	for key, val := range node.Labels {
		for _, l := range nodepoolLabels {
			if l == key {
				return val
			}
		}
	}
	return ""
}

func GetNodesForNodepool(kclient client.Client, npName string) (nodes []*corev1.Node, err error) {
	err = ForEach(kclient, &corev1.NodeList{}, func(obj interface{}) error {
		node := obj.(corev1.Node)
		if NodepoolNameFromNode(&node) == npName {
			nodes = append(nodes, &node)
		}
		return nil
	})
	if err != nil {
		return
	}

	sort.Slice(nodes, func(i, j int) bool {
		return strings.Compare(nodes[i].Name, nodes[j].Name) < 0
	})
	return
}
