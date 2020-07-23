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

	"github.com/go-logr/logr"
	netv1beta1 "k8s.io/api/networking/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/components/ingress"
	"github.com/k11n/konstellation/pkg/resources"
)

// IngressRequestReconciler reconciles a IngressRequest object
type IngressRequestReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k11n.dev,resources=ingressrequests;certificaterefs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k11n.dev,resources=ingressrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *IngressRequestReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("ingressrequest", req.NamespacedName)

	res := ctrl.Result{}

	// fetch all requests to reconcile
	requestList, err := resources.GetIngressRequests(r.Client)
	if err != nil {
		return res, err
	}

	if len(requestList.Items) == 0 {
		// kill everything
		r.Client.Delete(ctx, &netv1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: resources.IstioNamespace,
				Name:      resources.IngressName,
			},
		})
		return ctrl.Result{}, nil
	}

	// create ingress, one for all hosts
	ingress, err := r.ingressForRequests(requestList.Items)
	if err != nil {
		return res, err
	}

	op, err := resources.UpdateResource(r.Client, ingress, nil, nil)
	if err != nil {
		return res, err
	}
	resources.LogUpdates(log, op, "Updated Ingress")

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
				err = r.Client.Status().Update(context.TODO(), &ir)
				if err != nil {
					break
				}
				log.Info("Updated IngressRequest status", "ingress", ir.Name)
			}
		}
	}

	return res, err
}

func (r *IngressRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	certWatcher := &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(object handler.MapObject) []ctrl.Request {
			requests := []ctrl.Request{}

			// load all ingress requests, and trigger
			reqList, err := resources.GetIngressRequests(mgr.GetClient())
			if err != nil {
				return requests
			}

			// just need to request once. there'll be one ingress with all of the hosts
			for _, ingressReq := range reqList.Items {
				requests = append(requests, ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name: ingressReq.Name,
					},
				})
				break
			}
			return requests
		}),
	}

	ingressWatcher := &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(object handler.MapObject) []ctrl.Request {
			requests := []ctrl.Request{}
			// TODO: ensure that ingress is being managed by Kon before triggering

			// any changes to one resource in the domain, will require us
			// to load all requests for that domain to get merged
			reqList, err := resources.GetIngressRequests(mgr.GetClient())
			if err != nil || len(reqList.Items) == 0 {
				return requests
			}
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: reqList.Items[0].Name,
				},
			})
			return requests
		}),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.IngressRequest{}).
		Watches(&source.Kind{Type: &v1alpha1.CertificateRef{}}, certWatcher).
		Watches(&source.Kind{Type: &netv1beta1.Ingress{}}, ingressWatcher, builder.WithPredicates(predicate.Funcs{
			// grab ingress events so that we could update its status
			DeleteFunc:  func(e event.DeleteEvent) bool { return true },
			CreateFunc:  func(e event.CreateEvent) bool { return true },
			UpdateFunc:  func(e event.UpdateEvent) bool { return true },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		})).
		Complete(r)
}

func (r *IngressRequestReconciler) ingressForRequests(requests []v1alpha1.IngressRequest) (*netv1beta1.Ingress, error) {
	cc, err := resources.GetClusterConfig(r.Client)
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
			Rules: []netv1beta1.IngressRule{
				{
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
				},
			},
			Backend: &netv1beta1.IngressBackend{
				ServiceName: resources.IngressBackendName,
				ServicePort: intstr.FromInt(80),
			},
		},
	}

	hostsUsed := map[string]bool{}
	for _, req := range requests {
		for _, host := range req.Spec.Hosts {
			hostsUsed[host] = true
		}
	}

	var tlsHosts []string
	for host := range hostsUsed {
		_, err := resources.GetCertificateThatMatchDomain(r.Client, host)
		if err == nil {
			tlsHosts = append(tlsHosts, host)
		}
	}
	// sort to ensure stable ordering
	sort.Strings(tlsHosts)
	annotations, err := ingressComponent.GetIngressAnnotations(r.Client, tlsHosts)
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
