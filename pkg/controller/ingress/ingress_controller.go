package ingress

import (
	"context"

	istionetworking "istio.io/api/networking/v1alpha3"
	istio "istio.io/client-go/pkg/apis/networking/v1alpha3"
	netv1beta1 "k8s.io/api/networking/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/components/ingress"
	"github.com/k11n/konstellation/pkg/resources"
)

var log = logf.Log.WithName("controller.Ingress")

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
	c, err := controller.New("ingress-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource IngressRequest
	err = c.Watch(&source.Kind{Type: &v1alpha1.IngressRequest{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch changes to certificates, it may require the domain to be reconciled
	err = c.Watch(&source.Kind{Type: &v1alpha1.CertificateRef{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(configMapObject handler.MapObject) []reconcile.Request {
			requests := []reconcile.Request{}

			// load all ingress requests, and trigger
			reqList, err := resources.GetIngressRequests(mgr.GetClient())
			if err != nil {
				return requests
			}

			// just need to request once. there'll be one ingress with all of the hosts
			for _, ingressReq := range reqList.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: ingressReq.Name,
					},
				})
				break
			}
			return requests
		}),
	})
	if err != nil {
		return err
	}

	// Watch ingress for changes, if it's deleted, we'd need to recreate it
	err = c.Watch(&source.Kind{Type: &netv1beta1.Ingress{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(configMapObject handler.MapObject) []reconcile.Request {
			requests := []reconcile.Request{}
			// any changes to one resource in the domain, will require us
			// to load all requests for that domain to get merged
			reqList, err := resources.GetIngressRequests(mgr.GetClient())
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
	}, predicate.Funcs{
		// grab ingress events so that we could update its status
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return false },
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
	logger := log.WithValues("ingress", request.Name)

	res := reconcile.Result{}

	// fetch all requests to reconcile
	requestList, err := resources.GetIngressRequests(r.client)
	if err != nil {
		return res, err
	}

	// create gateway, shared across all domains
	gw := gatewayForRequests(requestList.Items)

	if len(requestList.Items) == 0 {
		// kill everything
		r.client.Delete(context.TODO(), gw)
		r.client.Delete(context.TODO(), &netv1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: resources.IstioNamespace,
				Name:      resources.IngressName,
			},
		})
		return reconcile.Result{}, nil
	}

	_, err = resources.UpdateResource(r.client, gw, nil, nil)
	if err != nil {
		return res, err
	}

	// create ingress, one for all hosts
	ingress, err := r.ingressForRequests(requestList.Items)
	if err != nil {
		return res, err
	}

	op, err := resources.UpdateResource(r.client, ingress, nil, nil)
	if err != nil {
		return res, err
	}
	resources.LogUpdates(logger, op, "Updated Ingress")

	var address string
	for _, lb := range ingress.Status.LoadBalancer.Ingress {
		if lb.Hostname != "" {
			address = lb.Hostname
			break
		}
		if lb.IP != "" {
			address = lb.IP
			break
		}
	}

	if address != "" {
		// update ingress url
		for _, ir := range requestList.Items {
			statusCopy := ir.Status.DeepCopy()
			ir.Status.Address = address

			if !apiequality.Semantic.DeepEqual(statusCopy, &ir.Status) {
				err = r.client.Status().Update(context.TODO(), &ir)
				if err != nil {
					break
				}
				logger.Info("Updated IngressRequest status", "ingress", ir.Name)
			}
		}
	}

	return res, err
}

func gatewayForRequests(requests []v1alpha1.IngressRequest) *istio.Gateway {
	var hosts []string
	for _, r := range requests {
		for _, h := range r.Spec.Hosts {
			hosts = append(hosts, h)
		}
	}
	gw := &istio.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: resources.IstioNamespace,
			Name:      resources.GatewayName,
		},
		Spec: istionetworking.Gateway{
			Selector: map[string]string{
				"istio": "ingressgateway",
			},
			Servers: []*istionetworking.Server{
				{
					Hosts: hosts,
					Port: &istionetworking.Port{
						Number:   80,
						Protocol: "HTTP",
						Name:     "kon-http",
					},
				},
			},
		},
	}
	return gw
}

func (r *ReconcileIngressRequest) ingressForRequests(requests []v1alpha1.IngressRequest) (*netv1beta1.Ingress, error) {
	cc, err := resources.GetClusterConfig(r.client)
	if err != nil {
		return nil, err
	}
	ingressComponent := ingress.NewIngressForCluster(cc.Spec.Cloud, cc.Name)

	ingress := netv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: resources.IstioNamespace,
			Name:      resources.IngressName,
		},
		Spec: netv1beta1.IngressSpec{
			Rules: []netv1beta1.IngressRule{},
			Backend: &netv1beta1.IngressBackend{
				ServiceName: "istio-ingressgateway",
				ServicePort: intstr.FromInt(80),
			},
		},
	}

	hostsUsed := map[string]bool{}
	for _, req := range requests {
		for _, host := range req.Spec.Hosts {
			if hostsUsed[host] {
				continue
			}

			rule := netv1beta1.IngressRule{
				Host: host,
				IngressRuleValue: netv1beta1.IngressRuleValue{
					HTTP: &netv1beta1.HTTPIngressRuleValue{
						Paths: []netv1beta1.HTTPIngressPath{
							{
								Path: "/*",
								Backend: netv1beta1.IngressBackend{
									ServiceName: "istio-ingressgateway",
									ServicePort: intstr.FromInt(80),
								},
							},
						},
					},
				},
			}
			ingress.Spec.Rules = append(ingress.Spec.Rules, rule)
			hostsUsed[host] = true
		}
	}

	var tlsHosts []string
	for host := range hostsUsed {
		_, err := resources.GetCertificateThatMatchDomain(r.client, host)
		if err == nil {
			tlsHosts = append(tlsHosts, host)
		}
	}
	annotations, err := ingressComponent.GetIngressAnnotations(r.client, tlsHosts)
	if err != nil {
		return nil, err
	}
	// https://medium.com/@cy.chiang/how-to-integrate-aws-alb-with-istio-v1-0-b17e07cae156
	ingress.Annotations = annotations

	if len(tlsHosts) != 0 {
		ingress.Spec.TLS = []netv1beta1.IngressTLS{
			{
				Hosts: tlsHosts,
			},
		}
	}
	return &ingress, nil
}
