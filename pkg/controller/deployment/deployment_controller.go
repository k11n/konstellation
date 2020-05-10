package deployment

import (
	"context"

	"github.com/thoas/go-funk"
	istio "istio.io/client-go/pkg/apis/networking/v1beta1"
	autoscalev2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
)

var log = logf.Log.WithName("controller.Deployment")

func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDeployment{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("deployment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	at := &v1alpha1.AppTarget{}
	ownerHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    at,
	}
	err = c.Watch(&source.Kind{Type: at}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource VirtualService, Service, IngressRequest, AppRelease
	secondaryTypes := []runtime.Object{
		&v1alpha1.AppRelease{},
		&v1alpha1.IngressRequest{},
		&corev1.Service{},
		&istio.VirtualService{},
		&autoscalev2beta2.HorizontalPodAutoscaler{},
	}
	for _, t := range secondaryTypes {
		err = c.Watch(&source.Kind{Type: t}, ownerHandler)
		if err != nil {
			return err
		}
	}

	// watch config changes
	err = c.Watch(&source.Kind{Type: &v1alpha1.AppConfig{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(configMapObject handler.MapObject) []reconcile.Request {
			requests := []reconcile.Request{}
			// check which apps
			appConfig := configMapObject.Object.(*v1alpha1.AppConfig)
			targets, err := resources.GetAppTargets(mgr.GetClient(), appConfig.Labels[v1alpha1.AppLabel])
			if err != nil {
				return requests
			}
			desiredTarget := appConfig.Labels[v1alpha1.TargetLabel]

			for _, target := range targets {
				if desiredTarget != "" && desiredTarget != target.Spec.Target {
					// skip if it's a target specific config change
					continue
				}
				requests = append(requests, reconcile.Request{
					types.NamespacedName{
						Namespace: target.Namespace,
						Name:      target.Name,
					},
				})
			}

			return requests
		}),
	}, predicate.Funcs{
		// grab ingress events so that we could update its status
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	})

	return nil
}

// blank assignment to verify that ReconcileDeployment implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileDeployment{}

/**
 * The deployment controller is the primary controller that manages an AppTarget. It's responsible for
 * - Creating AppReleases, one for each of the most recent builds
 * - Creating Service & VirtualService
 * - Creating IngressRequests
 * - Managing # of instances of each release
 * - Managing the deployment process, shifting traffic from one version to the next
 */
type ReconcileDeployment struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileDeployment) Reconcile(request reconcile.Request) (res reconcile.Result, err error) {
	at, err := resources.GetAppTarget(r.client, request.Name)
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

	// copy status to detect changes
	atStatus := at.Status.DeepCopy()

	err = r.ensureNamespaceCreated(at)
	if err != nil {
		return
	}

	// figure out configs
	configMap, err := r.reconcileConfigMap(at)
	if err != nil {
		return
	}

	// create releases and figure out traffic split
	releases, arRes, err := r.reconcileAppReleases(at, configMap)
	if err != nil {
		return
	}
	if arRes != nil {
		if arRes.Requeue {
			res.Requeue = arRes.Requeue
		}
		if arRes.RequeueAfter != 0 {
			res.RequeueAfter = arRes.RequeueAfter
		}
	}

	// see which releases we need to autoscale
	err = r.reconcileAutoScaler(at, releases)
	if err != nil {
		return
	}

	// reconcile Service
	service, err := r.reconcileService(at)
	if err != nil {
		return
	}

	// filter only releases with traffic
	activeReleases := funk.Filter(releases, func(ar *v1alpha1.AppRelease) bool {
		return ar.Spec.TrafficPercentage > 0
	}).([]*v1alpha1.AppRelease)
	err = r.reconcileDestinationRule(at, service, activeReleases)
	if err != nil {
		return
	}

	err = r.reconcileVirtualService(at, service, activeReleases)
	if err != nil {
		return
	}

	err = r.reconcileIngressRequest(at)
	if err != nil {
		return
	}

	// update at status
	if !apiequality.Semantic.DeepEqual(atStatus, at.Status) {
		// reload apptarget and update status
		status := at.Status
		at, err = resources.GetAppTarget(r.client, at.Name)
		if err != nil {
			return
		}
		at.Status = status
		err = r.client.Status().Update(context.TODO(), at)
	}
	return
}

func (r *ReconcileDeployment) ensureNamespaceCreated(at *v1alpha1.AppTarget) error {
	namespace := at.TargetNamespace()
	_, err := resources.GetNamespace(r.client, namespace)
	if err == nil {
		return nil
	}

	// create a new one
	n := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				resources.IstioInjectLabel: "enabled",
				resources.AppLabel:         at.Spec.App,
				resources.TargetLabel:      at.Spec.Target,
				resources.ManagedByLabel:   resources.Konstellation,
			},
		},
	}
	// ensures namespace is cleaned up after app target is
	err = controllerutil.SetControllerReference(at, &n, r.scheme)
	if err != nil {
		return err
	}

	return r.client.Create(context.TODO(), &n)
}

func (r *ReconcileDeployment) reconcileConfigMap(at *v1alpha1.AppTarget) (configMap *corev1.ConfigMap, err error) {
	// grab app release for this app
	ac, err := resources.GetMergedAppConfig(r.client, at.Spec.App, at.Spec.Target)
	if err != nil {
		return
	}

	// check if existing configmap with the hash
	configMap, err = resources.GetConfigMap(r.client, at.ScopedName(), ac.ConfigHash())
	if errors.IsNotFound(err) {
		log.Info("Creating ConfigMap", "app", at.Spec.App, "target", at.Spec.Target)
		// create new
		configMap = ac.ToConfigMap()
		configMap.Namespace = at.ScopedName()
		err = r.client.Create(context.Background(), configMap)
	}
	return
}

func labelsForAppTarget(at *v1alpha1.AppTarget) map[string]string {
	return map[string]string{
		resources.AppLabel:    at.Spec.App,
		resources.TargetLabel: at.Spec.Target,
	}
}
