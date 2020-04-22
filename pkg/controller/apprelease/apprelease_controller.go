package apprelease

import (
	"context"
	"time"

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

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/resources"
	"github.com/davidzhao/konstellation/pkg/utils/objects"
)

var log = logf.Log.WithName("controller_apprelease")

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
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling AppRelease")
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

	// load build
	build, err := resources.GetBuildByName(r.client, ar.Spec.Build)
	if err != nil {
		return res, err
	}
	rsTemplate := newReplicaSetForAR(ar, build)

	rs := appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: rsTemplate.Namespace,
			Name:      rsTemplate.Name,
		},
	}
	if ar.Spec.NumDesired == 0 {
		// delete replicaset
		err = client.IgnoreNotFound(
			r.client.Delete(context.TODO(), &rs),
		)
	} else {
		_, err = controllerutil.CreateOrUpdate(context.TODO(), r.client, &rs, func() error {
			rs.ObjectMeta.Labels = rsTemplate.ObjectMeta.Labels
			if rs.CreationTimestamp.IsZero() {
				if err := controllerutil.SetControllerReference(ar, &rs, r.scheme); err != nil {
					return err
				}
			}
			objects.MergeObject(&rs.Spec, &rsTemplate.Spec)
			return nil
		})
	}
	if err != nil {
		return res, err
	}

	// sync status
	status := v1alpha1.AppReleaseStatus{
		State:        v1alpha1.ReleaseStateNew,
		NumDesired:   rs.Status.Replicas,
		NumReady:     rs.Status.ReadyReplicas,
		NumAvailable: rs.Status.AvailableReplicas,
	}
	if ar.Spec.Role == v1alpha1.ReleaseRoleActive {
		status.State = v1alpha1.ReleaseStateReleased
	} else if ar.Spec.Role == v1alpha1.ReleaseRoleTarget {
		status.State = v1alpha1.ReleaseStateReleasing
	} else {
		if ar.Spec.NumDesired == 0 {
			status.State = v1alpha1.ReleaseStateRetired
		}
	}

	// check pod failures and update message/status
	if ar.Spec.NumDesired >= 0 {
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
		ar.Status = status
		err = r.client.Status().Update(context.TODO(), ar)
	}

	return res, err
}

func newReplicaSetForAR(ar *v1alpha1.AppRelease, build *v1alpha1.Build) *appsv1.ReplicaSet {
	labels := labelsForAppRelease(ar)
	labels[resources.BUILD_LABEL] = build.Name

	container := corev1.Container{
		Name:      "app",
		Image:     build.FullImageWithTag(),
		Command:   ar.Spec.Command,
		Args:      ar.Spec.Args,
		Env:       ar.Spec.Env,
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

	// release name would use build creation timestamp
	return &appsv1.ReplicaSet{
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
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						container,
					},
				},
			},
		},
	}
}

func labelsForAppRelease(ar *v1alpha1.AppRelease) map[string]string {
	return map[string]string{
		resources.APP_LABEL:         ar.Spec.App,
		resources.TARGET_LABEL:      ar.Spec.Target,
		resources.APP_RELEASE_LABEL: ar.Name,
	}
}
