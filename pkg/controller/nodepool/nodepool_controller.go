package nodepool

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller.Nodepool")

// Add creates a new Nodepool Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileNodepool{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("nodepool-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Nodepool
	err = c.Watch(&source.Kind{Type: &v1alpha1.Nodepool{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Nodes
	err = c.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestsFromMapFunc{
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
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileNodepool implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileNodepool{}

// ReconcileNodepool reconciles a Nodepool object
type ReconcileNodepool struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// this controller doesn't apply nodepool changes due to permissions and being a potentially destructive operation
// instead the controller will update status of the nodepool based on node resources
func (r *ReconcileNodepool) Reconcile(request reconcile.Request) (res reconcile.Result, err error) {
	// Fetch the Nodepool instance
	np := &v1alpha1.Nodepool{}
	err = r.client.Get(context.TODO(), request.NamespacedName, np)
	if err != nil {
		if errors.IsNotFound(err) {
			return res, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	nodes, err := resources.GetNodesForNodepool(r.client, np.Name)
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
		err = r.client.Status().Update(context.Background(), np)
		if err != nil {
			return
		}
	}

	return reconcile.Result{}, nil
}
