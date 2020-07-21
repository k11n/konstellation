/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
)

// NodepoolReconciler reconciles a Build object
type NodepoolReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k11n.dev,resources=nodepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k11n.dev,resources=nodepools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;create;update;patch;delete

func (r *NodepoolReconciler) Reconcile(req ctrl.Request) (res ctrl.Result, err error) {
	ctx := context.Background()
	log := r.Log.WithValues("nodepool", req.Name)

	// Fetch the Nodepool instance
	np := &v1alpha1.Nodepool{}
	err = r.Client.Get(ctx, req.NamespacedName, np)
	if err != nil {
		if errors.IsNotFound(err) {
			return res, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	nodes, err := resources.GetNodesForNodepool(r.Client, np.Name)
	if err != nil {
		return
	}

	existingStatus := np.Status.DeepCopy()
	np.Status.Nodes = make([]string, 0, len(nodes))
	np.Status.NumReady = 0
	for _, node := range nodes {
		isReady := false
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				isReady = true
				break
			}
		}

		if isReady {
			np.Status.NumReady += 1
			np.Status.Nodes = append(np.Status.Nodes, node.Name)
		}
	}

	if !apiequality.Semantic.DeepEqual(existingStatus, &np.Status) {
		log.Info("Updating Nodepool status", "nodepool", np.Name, "numReady", np.Status.NumReady)
		err = r.Client.Status().Update(context.Background(), np)
		if err != nil {
			return
		}
	}

	return reconcile.Result{}, nil
}

func (r *NodepoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	nodeWatcher := &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(mapObj handler.MapObject) []reconcile.Request {
			var reqs []reconcile.Request
			nodeObj := mapObj.Object.(*corev1.Node)

			// get nodepool from node
			npName := resources.NodepoolNameFromNode(nodeObj)
			if npName != "" {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: npName,
					},
				})
			}

			return reqs
		}),
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Nodepool{}).
		Watches(&source.Kind{Type: &corev1.Node{}}, nodeWatcher).
		Complete(r)
}
