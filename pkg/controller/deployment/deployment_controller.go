package deployment

import (
	"context"

	"github.com/thoas/go-funk"
	istio "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
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
)

var log = logf.Log.WithName("controller_deployment")

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
	}
	for _, t := range secondaryTypes {
		err = c.Watch(&source.Kind{Type: t}, ownerHandler)
		if err != nil {
			return err
		}
	}

	// TODO: watch new builds and reconcile apps

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
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("DeploymentController Reconciling")

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

	err = r.ensureNamespaceCreated(at)
	if err != nil {
		return
	}

	// create releases and figure out traffic split
	releases, arRes, err := r.reconcileAppReleases(at)
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

	// reconcile Service
	service, err := r.reconcileService(at)
	if err != nil {
		return
	}

	// filter only releases with traffic
	activeReleases := funk.Filter(releases, func(ar *v1alpha1.AppRelease) bool {
		return ar.Spec.TrafficPercentage > 0
	}).([]*v1alpha1.AppRelease)
	err = r.reconcileDestinationRule(at, activeReleases)
	if err != nil {
		return
	}

	err = r.reconcileVirtualService(at, service, activeReleases)
	if err != nil {
		return
	}

	// cleanup older resources
	return
}

func (r *ReconcileDeployment) ensureNamespaceCreated(at *v1alpha1.AppTarget) error {
	namespace := at.ScopedName()
	_, err := resources.GetNamespace(r.client, namespace)
	if err == nil {
		return nil
	}

	// create a new one
	n := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Annotations: map[string]string{
				resources.ISTIO_INJECT_LABEL: "enabled",
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

func labelsForAppTarget(at *v1alpha1.AppTarget) map[string]string {
	return map[string]string{
		resources.APP_LABEL:    at.Spec.App,
		resources.TARGET_LABEL: at.Spec.Target,
	}
}
