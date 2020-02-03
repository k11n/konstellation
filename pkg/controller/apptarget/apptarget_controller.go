package apptarget

import (
	"context"
	"fmt"
	"reflect"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/autoscaling/v2beta2"
	autoscalev2beta2 "k8s.io/api/autoscaling/v2beta2"
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
		&autoscalev2beta2.HorizontalPodAutoscaler{},
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

	// Reconcile Deployment
	deployment, updated, err := r.reconcileDeployment(appTarget)
	if err != nil {
		return
	}
	log.Info("Reconciled deployment", "deployment", deployment.Name, "updated", updated)
	if updated {
		res.Requeue = true
	}

	// Reconcile Service
	service, updated, err := r.reconcileService(appTarget, deployment)
	if err != nil {
		return
	}
	log.Info("Reconciled service", "service", service.Name, "updated", updated)
	if updated {
		res.Requeue = true
	}

	return
}

func (r *ReconcileAppTarget) reconcileDeployment(appTarget *v1alpha1.AppTarget) (deployment *appsv1.Deployment, updated bool, err error) {
	namespace := namespaceForAppTarget(appTarget)
	deployment = newDeploymentForAppTarget(appTarget)

	// Set AppTarget instance as the owner and controller
	if err = controllerutil.SetControllerReference(appTarget, deployment, r.scheme); err != nil {
		return
	}

	// see if we already have a deployment for this
	existing := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: deployment.Name, Namespace: namespace}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			// create new
			log.Info("Creating deployment", "appTarget", appTarget.Name, "deployment", deployment.Name, "namespace", namespace)
			err = r.client.Create(context.TODO(), deployment)
			updated = true
			return
		} else {
			return
		}
	} else if !reflect.DeepEqual(existing.Spec, deployment.Spec) {
		// update the deployment
		log.Info("deployment updated, updating")
		updated = true
		existing.Spec = deployment.Spec
		err = r.client.Update(context.TODO(), existing)
		return
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
		err = r.client.Status().Update(context.TODO(), appTarget)
	}

	// TODO: Log DeploymentCondition and carry to apptarget

	return
}

func (r *ReconcileAppTarget) reconcileService(at *v1alpha1.AppTarget, deployment *appsv1.Deployment) (service *corev1.Service, updated bool, err error) {
	namespace := namespaceForAppTarget(at)
	// do we need a service? if no ports defined, we don't
	serviceNeeded := len(at.Spec.Ports) > 0
	service = newServiceForAppTarget(at)
	// Set AppTarget instance as the owner and controller
	if err = controllerutil.SetControllerReference(at, service, r.scheme); err != nil {
		return
	}

	// find existing service obj
	existing := corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: service.GetName()}, &existing)
	if err != nil {
		if errors.IsNotFound(err) {
			if !serviceNeeded {
				// don't need a service and none found
				service = nil
				return
			}
			// need a service but not found, need to create it and reconcile later
			log.Info("Creating service", "appTarget", at.Name, "service", service.Name, "namespace", namespace)
			err = r.client.Create(context.TODO(), service)
			updated = true
			return
		} else {
			// other errors, just return
			return
		}
	}

	// do we still want this service?
	if serviceNeeded {
		// update needed?
		if !reflect.DeepEqual(service.Spec, existing.Spec) {
			updated = true
			existing.Spec = service.Spec
			err = r.client.Update(context.TODO(), &existing)
			return
		}

		// update service hostname
		hostname := fmt.Sprintf("%s.%s.svc.cluster.local", existing.Name, namespace)
		if at.Status.Hostname != hostname {
			at.Status.Hostname = hostname
			updated = true
			err = r.client.Status().Update(context.TODO(), at)
		}
	} else {
		// delete existing service
		updated = true
		err = r.client.Delete(context.TODO(), &existing)
	}

	return
}

func (r *ReconcileAppTarget) reconcileAutoscaler(at *v1alpha1.AppTarget, deployment *appsv1.Deployment) (hpa *autoscalev2beta2.HorizontalPodAutoscaler, updated bool, err error) {
	namespace := namespaceForAppTarget(at)
	autoscaler := newAutoscalerForAppTarget(at, deployment)
	if err = controllerutil.SetControllerReference(at, autoscaler, r.scheme); err != nil {
		return
	}

	existing := v2beta2.HorizontalPodAutoscaler{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: autoscaler.ObjectMeta.Name}, &existing)
	if err != nil && errors.IsNotFound(err) {
		// create the resource
		err = r.client.Create(context.TODO(), autoscaler)
		updated = true
		// need to reconcile again after creation complete
		return
	} else if err != nil {
		return
	}

	// check if Spec has updated
	if !reflect.DeepEqual(existing.Spec, autoscaler.Spec) {
		existing.Spec = autoscaler.Spec
		err = r.client.Update(context.TODO(), &existing)
		updated = true
		return
	}

	// update status
	updatedStatus := at.Status.DeepCopy()
	updatedStatus.DesiredReplicas = existing.Status.DesiredReplicas
	updatedStatus.CurrentReplicas = existing.Status.CurrentReplicas
	updatedStatus.LastScaleTime = existing.Status.LastScaleTime
	if !reflect.DeepEqual(updatedStatus, at.Status) {
		// update
		err = r.client.Status().Update(context.TODO(), at)
		updated = true
	}
	return
}

func newDeploymentForAppTarget(at *v1alpha1.AppTarget) *appsv1.Deployment {
	namespace := namespaceForAppTarget(at)
	replicas := int32(at.Spec.Scale.Min)
	ls := labelsForAppTarget(at)

	container := corev1.Container{
		Name:      at.Name,
		Image:     "",
		Command:   at.Spec.Command,
		Args:      at.Spec.Args,
		Env:       at.Spec.Env,
		Resources: at.Spec.Resources,
		Ports:     at.Spec.ContainerPorts(),
	}
	if at.Spec.Probes.Liveness != nil {
		container.LivenessProbe = at.Spec.Probes.Liveness.ToCoreProbe()
	}
	if at.Spec.Probes.Readiness != nil {
		container.ReadinessProbe = at.Spec.Probes.Readiness.ToCoreProbe()
	}
	if at.Spec.Probes.Startup != nil {
		container.StartupProbe = at.Spec.Probes.Startup.ToCoreProbe()
	}

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      at.Spec.App,
			Namespace: namespace,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						container,
					},
				},
			},
		},
	}
	return &deployment
}

func newServiceForAppTarget(at *v1alpha1.AppTarget) *corev1.Service {
	namespace := namespaceForAppTarget(at)
	ls := labelsForAppTarget(at)

	ports := []corev1.ServicePort{}
	for _, p := range at.Spec.Ports {
		ports = append(ports, corev1.ServicePort{
			Name:     p.Name,
			Protocol: p.Protocol,
			Port:     p.Port,
		})
	}

	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      at.Spec.App,
			Namespace: namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			Ports:    ports,
			Selector: ls,
		},
	}
	return &svc
}

func newAutoscalerForAppTarget(at *v1alpha1.AppTarget, deployment *appsv1.Deployment) *autoscalev2beta2.HorizontalPodAutoscaler {
	minReplicas := int32(at.Spec.Scale.Min)
	maxReplicas := int32(at.Spec.Scale.Max)
	if minReplicas == 0 {
		minReplicas = 1
	}
	if maxReplicas < minReplicas {
		maxReplicas = minReplicas
	}
	var metrics []autoscalev2beta2.MetricSpec
	if at.Spec.Scale.TargetCPUUtilization > 0 {
		metrics = append(metrics, autoscalev2beta2.MetricSpec{
			Type: autoscalev2beta2.ResourceMetricSourceType,
			Resource: &autoscalev2beta2.ResourceMetricSource{
				Name: "cpu",
				Target: autoscalev2beta2.MetricTarget{
					Type:               autoscalev2beta2.UtilizationMetricType,
					AverageUtilization: &at.Spec.Scale.TargetCPUUtilization,
				},
			},
		})
	}
	autoscaler := autoscalev2beta2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s-scaler", at.Spec.App),
			Labels: labelsForAppTarget(at),
		},
		Spec: autoscalev2beta2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalev2beta2.CrossVersionObjectReference{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			Metrics:     metrics,
		},
	}
	return &autoscaler
}

func namespaceForAppTarget(at *v1alpha1.AppTarget) string {
	return fmt.Sprintf("%s-%s", at.Spec.App, at.Spec.Target)
}

func labelsForAppTarget(appTarget *v1alpha1.AppTarget) map[string]string {
	return map[string]string{
		resources.APPTARGET_LABEL: appTarget.Name,
	}
}
