package resources

import (
	"context"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetReplicasetsForDeployment(kclient client.Client, deployment *appsv1.Deployment) (newReplica *appsv1.ReplicaSet, oldReplicas []*appsv1.ReplicaSet, err error) {
	replicaSets := appsv1.ReplicaSetList{}
	err = kclient.List(context.TODO(), &replicaSets,
		client.InNamespace(deployment.Namespace),
		client.MatchingLabels(deployment.Spec.Selector.MatchLabels),
	)
	if err != nil {
		return
	}
	sort.Slice(replicaSets.Items, func(i, j int) bool {
		if replicaSets.Items[i].CreationTimestamp.Equal(&replicaSets.Items[j].CreationTimestamp) {
			return replicaSets.Items[i].Name < replicaSets.Items[j].Name
		}
		return replicaSets.Items[i].CreationTimestamp.Before(&replicaSets.Items[j].CreationTimestamp)
	})
	for _, r := range replicaSets.Items {
		if newReplica == nil && equalIgnoreHash(&r.Spec.Template, &deployment.Spec.Template) {
			newReplica = &r
		} else {
			oldReplicas = append(oldReplicas, &r)
		}
	}
	return
}

func equalIgnoreHash(template1, template2 *corev1.PodTemplateSpec) bool {
	t1Copy := template1.DeepCopy()
	t2Copy := template2.DeepCopy()
	// Remove hash labels from template.Labels before comparing
	delete(t1Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	delete(t2Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	return apiequality.Semantic.DeepEqual(t1Copy, t2Copy)
}
