package clusterconfig

import (
	"context"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller.ClusterConfig")

func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileClusterConfig{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("clusterconfig-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ClusterConfig
	err = c.Watch(&source.Kind{Type: &v1alpha1.ClusterConfig{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileClusterConfig implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileClusterConfig{}

// ReconcileClusterConfig reconciles a ClusterConfig object
type ReconcileClusterConfig struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a ClusterConfig object and makes changes based on the state read
// and what is in the ClusterConfig.Spec
func (r *ReconcileClusterConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the ClusterConfig instance
	cc := &v1alpha1.ClusterConfig{}
	err := r.client.Get(context.TODO(), request.NamespacedName, cc)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	for _, target := range cc.Spec.Targets {
		if err := r.ensureNamespaceCreated(cc, target); err != nil {
			log.Error(err, "could not create namespace", "namespace", target)
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileClusterConfig) ensureNamespaceCreated(cc *v1alpha1.ClusterConfig, target string) error {
	_, err := resources.GetNamespace(r.client, target)
	if err == nil {
		return nil
	}

	// create a new one
	n := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: target,
			Labels: map[string]string{
				resources.IstioInjectLabel: "enabled",
				resources.TargetLabel:      target,
				resources.ManagedByLabel:   resources.Konstellation,
			},
		},
	}
	// ensures namespace is cleaned up after app target is
	err = controllerutil.SetControllerReference(cc, &n, r.scheme)
	if err != nil {
		return err
	}

	return r.client.Create(context.TODO(), &n)
}
