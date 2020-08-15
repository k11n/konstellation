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
	"sort"
	"time"

	"github.com/go-logr/logr"
	"github.com/thoas/go-funk"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
)

// AppReleaseReconciler reconciles a AppRelease object
type AppReleaseReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k11n.dev,resources=appreleases;builds;,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps;pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=k11n.dev,resources=appreleases/status,verbs=get;update;patch

func (r *AppReleaseReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	reqLogger := r.Log.WithValues("apprelease", req.NamespacedName)
	res := ctrl.Result{}

	// Fetch the AppRelease instance
	ar := &v1alpha1.AppRelease{}
	err := r.Client.Get(ctx, req.NamespacedName, ar)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return res, err
	}

	// load build & config
	build, err := resources.GetBuildByName(r.Client, ar.Spec.Build)
	if err != nil {
		return res, err
	}

	var cm *corev1.ConfigMap
	if ar.Spec.Config != "" {
		cm, err = resources.GetConfigMap(r.Client, ar.Namespace, ar.Spec.Config)
		if err != nil && !errors.IsNotFound(err) {
			return res, err
		}
	}
	rs, err := r.newReplicaSetForAR(ar, build, cm)
	if err != nil {
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
	}

	shouldUpdate := true
	if ar.Spec.Role == v1alpha1.ReleaseRoleActive && ar.Labels[resources.TargetReleaseLabel] == "1" {
		// when we are reconciling the active release, it means autoscaler is in charge of setting the numDesired field
		// on the replicaset. We don't want to proceed with updates
		key, err := client.ObjectKeyFromObject(rs)
		if err != nil {
			return res, err
		}
		err = r.Client.Get(ctx, key, rs)
		if err == nil && ar.Spec.NumDesired != 0 && *rs.Spec.Replicas != 0 {
			shouldUpdate = false
		}
	}

	if ar.Spec.NumDesired == 0 {
		// delete ReplicaSet
		err = client.IgnoreNotFound(
			r.Client.Delete(ctx, rs),
		)
	} else if shouldUpdate {
		var op controllerutil.OperationResult
		op, err = resources.UpdateResource(r.Client, rs, ar, r.Scheme)
		resources.LogUpdates(reqLogger, op, "Updated ReplicaSet", "numAvailable", rs.Status.AvailableReplicas)
	}
	if err != nil {
		return res, err
	}

	// sync status
	status := v1alpha1.AppReleaseStatus{
		State:        v1alpha1.ReleaseStateNew,
		NumDesired:   *rs.Spec.Replicas, // use replicaset data due to autoscaling
		NumReady:     rs.Status.ReadyReplicas,
		NumAvailable: rs.Status.AvailableReplicas,
	}

	if ar.Spec.Role == v1alpha1.ReleaseRoleActive {
		if status.NumReady == status.NumAvailable && status.NumReady > 0 {
			status.State = v1alpha1.ReleaseStateReleased
		} else {
			status.State = v1alpha1.ReleaseStateReleasing
		}
		if status.NumDesired == 0 {
			// halted
			status.State = v1alpha1.ReleaseStateHalted
		}
	} else if ar.Spec.Role == v1alpha1.ReleaseRoleTarget {
		status.State = v1alpha1.ReleaseStateReleasing
	} else if ar.Spec.Role == v1alpha1.ReleaseRoleBad {
		status.State = v1alpha1.ReleaseStateBad
	} else {
		if ar.Spec.NumDesired == 0 {
			status.State = v1alpha1.ReleaseStateRetired
		} else {
			status.State = v1alpha1.ReleaseStateRetiring
		}
	}
	// keep existing change time
	if ar.CreationTimestamp.IsZero() || status.State != ar.Status.State {
		status.StateChangedAt = metav1.Now()
	} else if status.State == ar.Status.State {
		status.StateChangedAt = ar.Status.StateChangedAt
	}

	// check pod failures and update message/status
	status.PodErrors = nil
	if ar.Spec.NumDesired >= 0 && status.NumAvailable < ar.Spec.NumDesired {
		podList := corev1.PodList{}
		err = r.Client.List(ctx, &podList, client.InNamespace(rs.Namespace),
			client.MatchingLabels(rs.Spec.Template.Labels))
		if err != nil {
			return res, err
		}

		// loop through the pods and see what's going on
		numSuccessful := 0
		var createdAt *time.Time
		for _, pod := range podList.Items {
			if podError := getPodError(pod); podError != nil {
				status.PodErrors = append(status.PodErrors, *podError)
			}

			switch pod.Status.Phase {
			case corev1.PodRunning, corev1.PodSucceeded:
				numSuccessful += 1
			}
			if createdAt == nil || createdAt.After(pod.CreationTimestamp.Time) {
				createdAt = &pod.CreationTimestamp.Time
			}
		}
		if numSuccessful == 0 && createdAt != nil && time.Now().Sub(*createdAt) > ar.Spec.Probes.GetReadinessTimeout() {
			status.State = v1alpha1.ReleaseStateFailed
		}
	}

	if !apiequality.Semantic.DeepEqual(status, ar.Status) {
		//reqLogger.Info("status changed", "old", ar.Status, "new", status)
		ar.Status = status
		err = r.Client.Status().Update(ctx, ar)
		reqLogger.Info("Updated AppRelease status", "numAvailable", status.NumAvailable, "numDesired", status.NumDesired)
		if err != nil {
			return res, err
		}
	}

	return res, err
}

func getPodError(pod corev1.Pod) *v1alpha1.PodStatus {
	for _, condition := range pod.Status.Conditions {
		if condition.Reason == "Unschedulable" {
			return &v1alpha1.PodStatus{
				Pod:     pod.Name,
				Reason:  condition.Reason,
				Message: condition.Message,
			}
		}
	}

	var podError *v1alpha1.PodStatus
	for _, status := range pod.Status.ContainerStatuses {
		terminated := status.LastTerminationState.Terminated
		if terminated != nil {
			podError = &v1alpha1.PodStatus{
				Pod:     pod.Name,
				Reason:  terminated.Reason,
				Message: terminated.Message,
			}
			// There might be a better message somewhere else
			if terminated.Reason != "Error" {
				return podError
			}
		}
	}

	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			return &v1alpha1.PodStatus{
				Pod:     pod.Name,
				Reason:  status.State.Waiting.Reason,
				Message: status.State.Waiting.Message,
			}
		}
	}

	return podError
}

func (r *AppReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.AppRelease{}).
		Owns(&appsv1.ReplicaSet{}).
		Complete(r)
}

func (r *AppReleaseReconciler) newReplicaSetForAR(ar *v1alpha1.AppRelease, build *v1alpha1.Build, cm *corev1.ConfigMap) (*appsv1.ReplicaSet, error) {
	labels := labelsForAppRelease(ar)
	labels[resources.BuildLabel] = build.Name
	labels[resources.KubeAppLabel] = ar.Spec.App

	container := corev1.Container{
		Name:      ar.Spec.App,
		Image:     build.FullImageWithTag(),
		Command:   ar.Spec.Command,
		Args:      ar.Spec.Args,
		Resources: ar.Spec.Resources,
		Ports:     ar.Spec.ContainerPorts(),
	}
	if ar.Spec.Probes.Liveness != nil {
		container.LivenessProbe = ar.Spec.Probes.Liveness.ToCoreProbe()
	}
	if ar.Spec.Probes.Readiness != nil {
		container.ReadinessProbe = ar.Spec.Probes.Readiness.ToCoreProbe()
	}
	if ar.Spec.Probes.Startup != nil {
		container.StartupProbe = ar.Spec.Probes.Startup.ToCoreProbe()
	}
	if cm != nil && len(cm.Data) > 0 {
		// set env
		keys := funk.Keys(cm.Data).([]string)
		sort.Strings(keys)
		for _, key := range keys {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  key,
				Value: cm.Data[key],
			})
		}
	}

	// check app dependencies and make urls available
	for _, ref := range ar.Spec.Dependencies {
		envs, err := resources.GetServiceHostEnvForReference(r.Client, ref, ar.Spec.Target)
		if err != nil {
			r.Log.Error(err, "could not resolve dependencies", "app", ar.Spec.App, "target", ar.Spec.Target, "dependency", ref.Name)
			return nil, err
		}
		for _, e := range envs {
			container.Env = append(container.Env, e)
		}
	}

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{
			container,
		},
	}

	if ar.Spec.ServiceAccount != "" {
		podSpec.ServiceAccountName = ar.Spec.ServiceAccount
	}
	if ar.Spec.ImagePullSecrets != nil {
		podSpec.ImagePullSecrets = nil
		for _, s := range ar.Spec.ImagePullSecrets {
			podSpec.ImagePullSecrets = append(podSpec.ImagePullSecrets, corev1.LocalObjectReference{Name: s})
		}
	}

	// release name would use build creation timestamp
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ar.Namespace,
			Name:      ar.Name,
			Labels:    labels,
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &ar.Spec.NumDesired,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: podSpec,
			},
		},
	}
	return rs, nil
}

func labelsForAppRelease(ar *v1alpha1.AppRelease) map[string]string {
	return map[string]string{
		resources.AppLabel:             ar.Spec.App,
		resources.TargetLabel:          ar.Spec.Target,
		resources.AppReleaseLabel:      ar.Name,
		resources.KubeManagedByLabel:   resources.Konstellation,
		resources.KubeAppInstanceLabel: ar.Name,
		resources.KubeAppVersionLabel:  ar.Spec.Build,
	}
}
