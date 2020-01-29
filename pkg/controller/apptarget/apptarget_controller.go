package apptarget

import (
	"context"
	"fmt"
	"reflect"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	autoscalev1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	netv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_apptarget")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new AppTarget Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileAppTarget{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("apptarget-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource AppTarget
	err = c.Watch(&source.Kind{Type: &v1alpha1.AppTarget{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner App
	secondaryTypes := []runtime.Object{
		&appsv1.Deployment{},
		&corev1.Service{},
		&autoscalev1.HorizontalPodAutoscaler{},
		&netv1beta1.Ingress{},
	}
	for _, t := range secondaryTypes {
		err = c.Watch(&source.Kind{Type: t}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &v1alpha1.AppTarget{},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// blank assignment to verify that ReconcileAppTarget implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileAppTarget{}

// ReconcileAppTarget reconciles a AppTarget object
type ReconcileAppTarget struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileAppTarget) Reconcile(request reconcile.Request) (res reconcile.Result, err error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling AppTarget")

	// Fetch the AppTarget instance
	appTarget := &v1alpha1.AppTarget{}
	err = r.client.Get(context.TODO(), request.NamespacedName, appTarget)
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

	// namespace, ensure created
	namespace := namespaceForAppTarget(appTarget)
	if err = resources.EnsureNamespaceCreated(r.client, namespace); err != nil {
		return
	}

	// Define a new Pod object
	pod := newDeploymentForAppTarget(appTarget)
	// Set AppTarget instance as the owner and controller
	if err = controllerutil.SetControllerReference(appTarget, pod, r.scheme); err != nil {
		return
	}

	// Define a new Deployment object
	deployment, updated, err := r.reconcileDeployment(appTarget)
	if err != nil {
		return
	}
	log.Info("Reconciled deployment", "deployment", deployment.Name)

	if updated {
		res.Requeue = true
	}

	return
}

func (r *ReconcileAppTarget) reconcileDeployment(appTarget *v1alpha1.AppTarget) (deployment *appsv1.Deployment, updated bool, err error) {
	namespace := namespaceForAppTarget(appTarget)
	deployment = newDeploymentForAppTarget(appTarget)

	// Set App instance as the owner and controller
	if err = controllerutil.SetControllerReference(appTarget, deployment, r.scheme); err != nil {
		return
	}

	// see if we already have a deployment for this
	existing := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: deployment.Name, Namespace: namespace}, existing)
	if err != nil && errors.IsNotFound(err) {
		// create new
		log.Info("Creating deployment", "appTarget", appTarget.Name, "deployment", deployment.Name, "namespace", namespace)
		err = r.client.Create(context.TODO(), deployment)
		updated = true
		return
	} else if err != nil {
		return
	}

	if !reflect.DeepEqual(existing.Spec, deployment.Spec) {
		// update the deployment
		log.Info("deployment updated, updating")
		updated = true
		err = r.client.Update(context.TODO(), deployment)
		if err != nil {
			return
		}
	}

	// update status
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(labelsForAppTarget(appTarget)),
	}
	if err = r.client.List(context.TODO(), podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "namespace", namespace)
		return
	}

	podNames := resources.GetPodNames(podList.Items)
	if !reflect.DeepEqual(podNames, appTarget.Status.Pods) {
		appTarget.Status.Pods = podNames
	}

	return
}

func newDeploymentForAppTarget(at *v1alpha1.AppTarget) *appsv1.Deployment {
	namespace := namespaceForAppTarget(at)
	replicas := int32(at.Spec.Scale.Min)
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      at.Spec.App,
			Namespace: namespace,
			Labels:    labelsForAppTarget(at),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}
	return &deployment
}

func namespaceForAppTarget(at *v1alpha1.AppTarget) string {
	return fmt.Sprintf("%s-%s", at.Spec.App, at.Spec.Target)
}

func labelsForAppTarget(appTarget *v1alpha1.AppTarget) map[string]string {
	return map[string]string{
		resources.APPTARGET_LABEL: appTarget.Name,
	}
}
