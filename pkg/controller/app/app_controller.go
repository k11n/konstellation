package app

import (
	"context"

	"github.com/thoas/go-funk"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

var log = logf.Log.WithName("controller.App")

/**
* App Controller is the top level handler. It generates Build(s) and AppTarget(s)
* after figuring out the target environment
**/

// Add creates a new App Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileApp{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("app-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource App
	err = c.Watch(&source.Kind{Type: &v1alpha1.App{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource app targets
	secondaryTypes := []runtime.Object{
		&v1alpha1.AppTarget{},
	}
	for _, t := range secondaryTypes {
		err = c.Watch(&source.Kind{Type: t}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &v1alpha1.App{},
		})
		if err != nil {
			return err
		}
	}

	// Watch cluster config changes, as it may make it eligible to deploy a new target
	err = c.Watch(&source.Kind{Type: &v1alpha1.ClusterConfig{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(configMapObject handler.MapObject) []reconcile.Request {
			apps, err := resources.ListApps(mgr.GetClient())
			requests := []reconcile.Request{}
			if err != nil {
				return requests
			}

			newTargets := map[string]bool{}
			clusterConfig := configMapObject.Object.(*v1alpha1.ClusterConfig)
			for _, target := range clusterConfig.Spec.Targets {
				newTargets[target] = true
			}

			// reconcile all apps that the cluster supports
			for _, app := range apps {
				needsReconcile := false
				for _, target := range app.Spec.Targets {
					if newTargets[target.Name] {
						if !funk.Contains(app.Status.ActiveTargets, target.Name) {
							needsReconcile = true
							break
						}
					} else {
						// this target has been removed, reconcile also
						needsReconcile = true
					}
				}
				if needsReconcile {
					// not yet active, reconcile this app
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name: app.Name,
						},
					})
				}
			}

			return requests
		}),
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileApp implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileApp{}

// ReconcileApp reconciles a App object
type ReconcileApp struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileApp) Reconcile(request reconcile.Request) (res reconcile.Result, err error) {
	reqLogger := log.WithValues("app", request.Name)

	// Fetch the App instance
	app := &v1alpha1.App{}
	err = r.client.Get(context.TODO(), request.NamespacedName, app)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			err = nil
			return
		}
		// Error reading the object - requeue the request.
		return
	}

	// see if we need to store the build
	build, err := r.reconcileBuild(app)
	if err != nil {
		return
	}

	// figure out what targets we should work with from cluster config
	cc, err := resources.GetClusterConfig(r.client)
	if err != nil {
		return
	}

	clusterTargets := map[string]bool{}
	for _, target := range cc.Spec.Targets {
		clusterTargets[target] = true
	}

	var invalidTargets []string
	// deploy the intersection of app and cluster targets
	for _, target := range app.Spec.Targets {
		if !clusterTargets[target.Name] {
			// skip reconcile, since cluster doesn't support it
			invalidTargets = append(invalidTargets, target.Name)
			continue
		}
		err = r.reconcileAppTarget(app, target.Name, build)
		if err != nil {
			return
		}
	}

	// remove old targets that aren't valid
	for _, target := range invalidTargets {
		at, err := resources.GetAppTarget(r.client, target)
		if err != nil {
			if !errors.IsNotFound(err) {
				reqLogger.Error(err, "Could not get AppTarget", "target", target, "app", app.Name)
			}
			continue
		}
		reqLogger.Info("Deleting inactive AppTargets", "target", target)
		err = r.client.Delete(context.TODO(), at)
	}

	return
}

func (r *ReconcileApp) reconcileBuild(app *v1alpha1.App) (*v1alpha1.Build, error) {
	build := v1alpha1.NewBuild(app.Spec.Registry, app.Spec.Image, app.Spec.ImageTag)
	build.Labels = resources.LabelsForBuild(build)

	existing := &v1alpha1.Build{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: build.GetName()}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			build.Labels[resources.BuildTypeLabel] = resources.BuildTypeLatest
			// create this build
			err = r.client.Create(context.TODO(), build)
			if err != nil {
				return nil, err
			}
			return build, nil
		} else {
			return nil, err
		}
	}

	return existing, nil
}

func (r *ReconcileApp) reconcileAppTarget(app *v1alpha1.App, target string, build *v1alpha1.Build) error {
	appTarget := newAppTargetForApp(app, target, build)
	if err := appTarget.UpdateHash(); err != nil {
		return err
	}
	op, err := resources.UpdateResource(r.client, appTarget, app, r.scheme)
	if err != nil {
		return err
	}

	resources.LogUpdates(log, op, "reconciled AppTarget", "app", app.Name, "target", target)
	return nil
}

func newAppTargetForApp(app *v1alpha1.App, target string, build *v1alpha1.Build) *v1alpha1.AppTarget {
	ls := labelsForAppTarget(app, target)
	for k, v := range resources.LabelsForBuild(build) {
		ls[k] = v
	}
	at := &v1alpha1.AppTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:   app.GetAppTargetName(target),
			Labels: ls,
		},
		Spec: v1alpha1.AppTargetSpec{
			App:          app.Name,
			Target:       target,
			Build:        build.Name,
			Ports:        app.Spec.Ports,
			Command:      app.Spec.Command,
			Args:         app.Spec.Args,
			Configs:      app.Spec.Configs,
			Dependencies: app.Spec.Dependencies,
			Resources:    *app.Spec.ResourcesForTarget(target),
			Scale:        *app.Spec.ScaleSpecForTarget(target),
			Probes:       *app.Spec.ProbesForTarget(target),
		},
	}

	tc := app.Spec.GetTargetConfig(target)
	// TODO: this should never be nil
	if tc != nil {
		at.Spec.Ingress = tc.Ingress
	}

	return at
}

func labelsForAppTarget(app *v1alpha1.App, target string) map[string]string {
	return map[string]string{
		resources.AppLabel:    app.Name,
		resources.TargetLabel: target,
	}
}
