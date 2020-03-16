package ingressrequest

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	netv1beta1 "k8s.io/api/networking/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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

var log = logf.Log.WithName("controller_ingressrequest")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new IngressRequest Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileIngressRequest{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("ingressrequest-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource IngressRequest
	err = c.Watch(&source.Kind{Type: &v1alpha1.IngressRequest{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch cluster config changes, as it may make it eligible to deploy a new target
	err = c.Watch(&source.Kind{Type: &netv1beta1.Ingress{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(configMapObject handler.MapObject) []reconcile.Request {
			requests := []reconcile.Request{}
			if err != nil {
				return requests
			}

			ingress := configMapObject.Object.(*netv1beta1.Ingress)
			// find all IngressRequests of the same domain
			host := ingress.Labels[resources.INGRESS_HOST_LABEL]
			if host == "" {
				return requests
			}

			if configMapObject.Meta.GetDeletionTimestamp() == nil {
				return requests
			}

			log.Info("Ingress deleted, requesting reconcile", "ingress", ingress.Name,
				"host", host)
			// only thing is if it gets deleted.. we'll need to reconcile
			// since the reconcile loops are a bit diff here..
			// any changes to one resource in the domain, will require us
			// to load all requests for that domain to get merged
			reqList, err := resources.GetIngressRequestsForHost(mgr.GetClient(), host)
			if err != nil || len(reqList.Items) == 0 {
				return requests
			}
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: reqList.Items[0].Name,
				},
			})
			return requests
		}),
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileIngressRequest implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileIngressRequest{}

// ReconcileIngressRequest reconciles a IngressRequest object
type ReconcileIngressRequest struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a IngressRequest object and makes changes based on the state read
// and what is in the IngressRequest.Spec
func (r *ReconcileIngressRequest) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling IngressRequest")

	res := reconcile.Result{}

	// Fetch the IngressRequest instance
	instance := &v1alpha1.IngressRequest{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return res, nil
		}
		// Error reading the object - requeue the request.
		return res, err
	}

	host := instance.Labels[resources.INGRESS_HOST_LABEL]

	// fetch all requests to reconcile
	requestList, err := resources.GetIngressRequestsForHost(r.client, host)
	if err != nil {
		return res, err
	}

	// make a copy so we can compare Status changes
	requestListCopy := requestList.DeepCopy()
	ingress := r.createIngress(host, requestList.Items)

	existing := netv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: ingress.GetName(),
		},
	}
	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, &existing, func() error {
		objects.MergeObject(existing.Spec, ingress.Spec)
		existing.Labels = ingress.Labels
		return nil
	})
	if err != nil {
		return res, err
	}
	reqLogger.Info("reconciled Ingress", "host", host, "op", op)

	updated := false
	for i, req := range requestList.Items {
		rCopy := requestListCopy.Items[i]
		if !apiequality.Semantic.DeepEqual(&req.Status, &rCopy.Status) {
			if err := r.client.Status().Update(context.TODO(), &req); err != nil {
				return res, err
			}
			updated = true
		}
	}

	if updated {
		res.Requeue = true
	}
	return res, nil
}

func (r *ReconcileIngressRequest) createIngress(host string, requests []v1alpha1.IngressRequest) *netv1beta1.Ingress {
	pathUsed := map[string]bool{}
	rule := netv1beta1.IngressRule{
		Host: host,
		IngressRuleValue: netv1beta1.IngressRuleValue{
			HTTP: &netv1beta1.HTTPIngressRuleValue{},
		},
	}
	ingress := netv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: host,
			Labels: map[string]string{
				resources.INGRESS_HOST_LABEL: host,
			},
			Annotations: map[string]string{},
		},
		Spec: netv1beta1.IngressSpec{
			Rules: []netv1beta1.IngressRule{
				rule,
			},
		},
	}

	for _, r := range requests {
		for _, p := range r.Spec.Ports {
			if p.IngressPath == "" || p.Protocol != corev1.ProtocolTCP {
				continue
			}
			pathUsed[p.IngressPath] = true
			rulePath := netv1beta1.HTTPIngressPath{
				Path: p.IngressPath,
				Backend: netv1beta1.IngressBackend{
					ServiceName: r.Spec.Service,
					ServicePort: intstr.FromInt(int(p.Port)),
				},
			}
			rule.IngressRuleValue.HTTP.Paths = append(rule.IngressRuleValue.HTTP.Paths, rulePath)
		}
	}
	return &ingress
}
