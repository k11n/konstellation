package resources

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
)

func NodepoolName() string {
	return fmt.Sprintf("%s-%s", NODEPOOL_PREFIX, time.Now().Format(dateTimeFormat))
}

func GetNodepoolOfType(kclient client.Client, kind string) (np *v1alpha1.Nodepool, err error) {
	items := v1alpha1.NodepoolList{}
	err = kclient.List(context.Background(), &items, client.InNamespace(""), client.MatchingLabels{
		NODEPOOL_LABEL: kind,
	})
	if err != nil {
		return
	}
	if len(items.Items) == 0 {
		err = fmt.Errorf("No nodepools found")
		return
	}
	np = &items.Items[0]
	return
}

func UpdateStatus(kclient client.Client, np *v1alpha1.Nodepool) error {
	// get nodelist
	nodeList := corev1.NodeList{}
	err := kclient.List(context.Background(), &nodeList, client.InNamespace(""))
	if err != nil {
		return err
	}

	numReady := 0
	nodes := []string{}
	for _, node := range nodeList.Items {
		nodes = append(nodes, node.ObjectMeta.Name)
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" {
				if condition.Status == "True" {
					numReady += 1
				}
				break
			}
		}
	}

	if np.Status.NumReady == numReady && apiequality.Semantic.DeepEqual(np.Status.Nodes, nodes) {
		// no need to update
		return nil
	}
	np.Status.NumReady = numReady
	np.Status.Nodes = nodes
	return kclient.Status().Update(context.Background(), np)
}
