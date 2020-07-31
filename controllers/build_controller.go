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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
)

// BuildReconciler reconciles a Build object
type BuildReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k11n.dev,resources=builds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k11n.dev,resources=builds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k11n.dev,resources=apps,verbs=get;list;watch;update

func (r *BuildReconciler) Reconcile(req ctrl.Request) (res ctrl.Result, err error) {
	ctx := context.Background()
	_ = r.Log.WithValues("build", req.NamespacedName)

	// Fetch the Build
	build := &v1alpha1.Build{}
	err = r.Client.Get(ctx, req.NamespacedName, build)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
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
	err = resources.ForEach(r.Client, &v1alpha1.BuildList{}, func(item interface{}) error {
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
		op, err := resources.UpdateResource(r.Client, b, nil, nil)
		if err != nil {
			return res, err
		}
		resources.LogUpdates(r.Log, op, "Removed latest label from build", "build", b.Name, "newBuild", build.Name)
	}
	return
}

func (r *BuildReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1alpha1.App{}, "spec.image", func(rawObj runtime.Object) []string {
		app := rawObj.(*v1alpha1.App)
		return []string{app.Spec.Image}
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Build{}).
		Complete(r)
}
