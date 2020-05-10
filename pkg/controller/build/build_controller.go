package build

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
)

var log = logf.Log.WithName("controller.Build")

// Add creates a new Build Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileBuild{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("build-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Build
	err = c.Watch(&source.Kind{Type: &v1alpha1.Build{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileBuild implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileBuild{}

// ReconcileBuild reconciles a Build object
type ReconcileBuild struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileBuild) Reconcile(request reconcile.Request) (res reconcile.Result, err error) {
	// Fetch the Build
	build := &v1alpha1.Build{}
	err = r.client.Get(context.TODO(), request.NamespacedName, build)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return
	}

	// if not latest, then we can ignore
	if build.Labels[resources.BuildTypeLabel] != resources.BuildTypeLatest {
		return
	}

	var latestBuilds []*v1alpha1.Build
	// find currently active builds, ensure the latest one has flag set
	err = resources.ForEach(r.client, &v1alpha1.BuildList{}, func(item interface{}) error {
		b := item.(v1alpha1.Build)
		latestBuilds = append(latestBuilds, &b)
		return nil
	}, client.MatchingLabels{
		resources.BuildTypeLabel:     resources.BuildTypeLatest,
		resources.BuildRegistryLabel: build.Labels[resources.BuildRegistryLabel],
		resources.BuildImageLabel:    build.Labels[resources.BuildImageLabel],
	})
	if err != nil {
		return
	}

	// if there's another build later than ours, use that one
	for _, b := range latestBuilds {
		if b.CreationTimestamp.After(build.CreationTimestamp.Time) {
			build = b
		}
	}

	for _, b := range latestBuilds {
		if b.Name == build.Name {
			continue
		}

		// unset label from all other builds
		delete(b.Labels, resources.BuildTypeLabel)
		op, err := resources.UpdateResource(r.client, b, nil, nil)
		if err != nil {
			return res, err
		}
		resources.LogUpdates(log, op, "Removed latest label from build", "build", b.Name, "newBuild", build.Name)
	}

	// trigger app reconcile
	err = resources.ForEach(r.client, &v1alpha1.AppTargetList{}, func(item interface{}) error {
		at := item.(v1alpha1.AppTarget)

		at.Spec.Build = build.Name
		op, err := resources.UpdateResource(r.client, &at, nil, nil)
		if err != nil {
			return err
		}
		resources.LogUpdates(log, op, "Updating appTarget build", "appTarget", at.Name, "build", build.Name)
		return nil
	}, client.MatchingLabels{
		resources.BuildRegistryLabel: build.Labels[resources.BuildRegistryLabel],
		resources.BuildImageLabel:    build.Labels[resources.BuildImageLabel],
	})
	return
}
