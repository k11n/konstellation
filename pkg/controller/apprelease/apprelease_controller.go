package apprelease

import (
	"context"
	"sort"
	"time"

	"github.com/thoas/go-funk"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
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

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
)

var log = logf.Log.WithName("controller.AppRelease")

func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileAppRelease{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("apprelease-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource AppRelease
	err = c.Watch(&source.Kind{Type: &v1alpha1.AppRelease{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource ReplicaSets and requeue the owner AppRelease
	err = c.Watch(&source.Kind{Type: &appsv1.ReplicaSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.AppRelease{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileAppRelease implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileAppRelease{}

// ReconcileAppRelease reconciles a AppRelease object
type ReconcileAppRelease struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Creates ReplicaSets matching request, TargetRelease uses
func (r *ReconcileAppRelease) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("release", request.Name)
	res := reconcile.Result{}

	// Fetch the AppRelease instance
	ar := &v1alpha1.AppRelease{}
	err := r.client.Get(context.TODO(), request.NamespacedName, ar)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return res, err
	}

	// load build & config
	build, err := resources.GetBuildByName(r.client, ar.Spec.Build)
	if err != nil {
		return res, err
	}

	var cm *corev1.ConfigMap
	if ar.Spec.Config != "" {
		cm, err = resources.GetConfigMap(r.client, ar.Namespace, ar.Spec.Config)
		if err != nil && !errors.IsNotFound(err) {
			return res, err
		}
	}
	rs, err := r.newReplicaSetForAR(ar, build, cm)
	if err != nil {
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, err
	}

	shouldUpdate := true
	if ar.Spec.Role == v1alpha1.ReleaseRoleActive {
		// when we are reconciling the active release, it means autoscaler is in charge of setting the numDesired field
		// on the replicaset. We don't want to proceed with updates
		key, err := client.ObjectKeyFromObject(rs)
		if err != nil {
			return res, err
		}
		err = r.client.Get(context.TODO(), key, rs)
		if err == nil && ar.Spec.NumDesired != 0 && *rs.Spec.Replicas != 0 {
			shouldUpdate = false
		}
	}

	if ar.Spec.NumDesired == 0 {
		// delete ReplicaSet
		err = client.IgnoreNotFound(
			r.client.Delete(context.TODO(), rs),
		)
	} else if shouldUpdate {
		var op controllerutil.OperationResult
		op, err = resources.UpdateResource(r.client, rs, ar, r.scheme)
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
	if ar.Spec.NumDesired >= 0 && status.NumAvailable < ar.Spec.NumDesired {
		podList := corev1.PodList{}
		err = r.client.List(context.TODO(), &podList, client.InNamespace(rs.Namespace),
			client.MatchingLabels(rs.Spec.Template.Labels))
		if err != nil {
			return res, err
		}

		// loop through the pods and see what's going on
		numSuccessful := 0
		var createdAt *time.Time
		for _, pod := range podList.Items {
			switch pod.Status.Phase {
			case corev1.PodPending, corev1.PodFailed:
				status.Reason = pod.Status.Message
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
		err = r.client.Status().Update(context.TODO(), ar)
		reqLogger.Info("Updated AppRelease status", "numAvailable", status.NumAvailable, "numDesired", status.NumDesired)
		if err != nil {
			return res, err
		}
	}

	return res, err
}

func (r *ReconcileAppRelease) newReplicaSetForAR(ar *v1alpha1.AppRelease, build *v1alpha1.Build, cm *corev1.ConfigMap) (*appsv1.ReplicaSet, error) {
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
		envs, err := resources.GetServiceHostEnvForReference(r.client, ref, ar.Spec.Target)
		if err != nil {
			log.Error(err, "could not resolve dependencies", "app", ar.Spec.App, "target", ar.Spec.Target, "dependency", ref.Name)
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
