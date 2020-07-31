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
	"github.com/thoas/go-funk"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1alpha1 "github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k11n.dev,resources=apps;apptargets;builds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k11n.dev,resources=clusterconfigs,verbs=get;watch;list
// +kubebuilder:rbac:groups=k11n.dev,resources=apps/status,verbs=get;update;patch

func (r *AppReconciler) Reconcile(req ctrl.Request) (res ctrl.Result, err error) {
	ctx := context.Background()
	reqLogger := r.Log.WithValues("app", req.Name)

	// Fetch the App instance
	app := &v1alpha1.App{}
	err = r.Client.Get(ctx, req.NamespacedName, app)
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
	build, err := r.reconcileBuild(ctx, app)
	if err != nil {
		return
	}

	// figure out what targets we should work with from cluster config
	cc, err := resources.GetClusterConfig(r.Client)
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
		at, err := resources.GetAppTarget(r.Client, target)
		if err != nil {
			if !errors.IsNotFound(err) {
				reqLogger.Error(err, "Could not get AppTarget", "target", target, "app", app.Name)
			}
			continue
		}
		reqLogger.Info("Deleting inactive AppTargets", "target", target)
		err = r.Client.Delete(ctx, at)
	}
	return
}

func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	clusterConfigWatcher := &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(configMapObject handler.MapObject) []ctrl.Request {
			apps, err := resources.ListApps(r.Client)
			var requests []ctrl.Request
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
					requests = append(requests, ctrl.Request{
						NamespacedName: types.NamespacedName{
							Name: app.Name,
						},
					})
				}
			}

			return requests
		}),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.App{}).
		Owns(&v1alpha1.AppTarget{}).
		Watches(&source.Kind{Type: &v1alpha1.ClusterConfig{}}, clusterConfigWatcher).
		Complete(r)
}

func (r *AppReconciler) reconcileBuild(ctx context.Context, app *v1alpha1.App) (*v1alpha1.Build, error) {
	build := v1alpha1.NewBuild(app.Spec.Registry, app.Spec.Image, app.Spec.ImageTag)
	build.Labels = resources.LabelsForBuild(build)

	existing := &v1alpha1.Build{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: build.GetName()}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			build.Labels[resources.BuildTypeLabel] = resources.BuildTypeLatest
			// create this build
			err = r.Client.Create(ctx, build)
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

func (r *AppReconciler) reconcileAppTarget(app *v1alpha1.App, target string, build *v1alpha1.Build) error {
	appTarget := newAppTargetForApp(app, target, build)
	if err := appTarget.UpdateHash(); err != nil {
		return err
	}
	op, err := resources.UpdateResource(r.Client, appTarget, app, r.Scheme)
	if err != nil {
		return err
	}

	resources.LogUpdates(r.Log, op, "reconciled AppTarget", "app", app.Name, "target", target)
	return nil
}

func newAppTargetForApp(app *v1alpha1.App, target string, build *v1alpha1.Build) *v1alpha1.AppTarget {
	ls := labelsForApp(app, target)
	for k, v := range resources.LabelsForBuild(build) {
		ls[k] = v
	}
	at := &v1alpha1.AppTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:   app.GetAppTargetName(target),
			Labels: ls,
		},
		Spec: v1alpha1.AppTargetSpec{
			App:    app.Name,
			Target: target,
			Build:  build.Name,
			AppCommonSpec: v1alpha1.AppCommonSpec{
				Ports:            app.Spec.Ports,
				Command:          app.Spec.Command,
				Args:             app.Spec.Args,
				Dependencies:     app.Spec.Dependencies,
				ImagePullSecrets: app.Spec.ImagePullSecrets,
				ServiceAccount:   app.Spec.ServiceAccount,
				Resources:        *app.Spec.ResourcesForTarget(target),
				Probes:           *app.Spec.ProbesForTarget(target),
			},
			DeployMode: app.Spec.DeployModeForTarget(target),
			Configs:    app.Spec.Configs,
			Scale:      *app.Spec.ScaleSpecForTarget(target),
			Prometheus: app.Spec.Prometheus,
		},
	}

	tc := app.Spec.GetTargetConfig(target)
	// TODO: this should never be nil
	if tc != nil {
		at.Spec.Ingress = tc.Ingress
	}

	return at
}

func labelsForApp(app *v1alpha1.App, target string) map[string]string {
	return map[string]string{
		resources.AppLabel:    app.Name,
		resources.TargetLabel: target,
	}
}
