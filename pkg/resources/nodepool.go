package resources

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/utils/objects"
)

func NodepoolName() string {
	return fmt.Sprintf("%s-%s", NodepoolPrefix, time.Now().Format(dateTimeFormat))
}

func GetNodepoolOfType(kclient client.Client, kind string) (np *v1alpha1.Nodepool, err error) {
	items := v1alpha1.NodepoolList{}
	err = kclient.List(context.Background(), &items, client.InNamespace(""), client.MatchingLabels{
		NodepoolLabel: kind,
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

func SaveNodepool(kclient client.Client, nodepool *v1alpha1.Nodepool) error {
	existing := &v1alpha1.Nodepool{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodepool.Name,
		},
	}
	_, err := controllerutil.CreateOrUpdate(context.TODO(), kclient, existing, func() error {
		existing.Labels = nodepool.Labels
		objects.MergeObject(&existing.Spec, &nodepool.Spec)
		return nil
	})
	return err
}
