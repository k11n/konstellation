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
	"fmt"

	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/components/ingress"
	"github.com/k11n/konstellation/pkg/resources"

	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
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

	res := ctrl.Result{}

	// find the requested ingress, since we need to get its app protocol
	reconcileAll := false
	ir := &v1alpha1.IngressRequest{}
	err := r.Client.Get(context.Background(), req.NamespacedName, ir)
	if err != nil {
		if errors.IsNotFound(err) {
			reconcileAll = true
		} else {
			return res, err
		}
	}

	// fetch all requests in same namespace to reconcile
	irs, err := resources.GetIngressRequests(r.Client, req.Namespace)
	if err != nil {
		return res, err
	}

	// aggregate items by app protocol => []*ingressRequest
	itemsToReconcile := make(map[string][]*v1alpha1.IngressRequest)
	for _, r := range irs {
		p := r.Spec.AppProtocol
		if reconcileAll || p == ir.Spec.AppProtocol {
			itemsToReconcile[p] = append(itemsToReconcile[p], r)
		}
	}

	for protocol, irs := range itemsToReconcile {
		err = r.reconcileForAppProtocol(req.Namespace, irs, protocol)
		if err != nil {
			return res, err
		}
	}

	// remove ingresses where name doesn't match
	ingressList := &netv1.IngressList{}
	err = r.Client.List(ctx, ingressList)
	if err != nil {
		return res, err
	}

	// when in full reconcile mode, delete ingresses that don't match
	if reconcileAll {
		for _, in := range ingressList.Items {
			protocol := in.Labels[resources.AppProtocolLabel]
			if itemsToReconcile[protocol] == nil {
				// no longer valid, delete
				if err := r.Client.Delete(ctx, &in); err != nil {
					return res, err
				}
			}
		}

	}

	return res, nil
}

func (r *IngressRequestReconciler) reconcileForAppProtocol(namespace string, irs []*v1alpha1.IngressRequest, protocol string) error {
	log := r.Log.WithValues("ingressrequest", namespace, "protocol", protocol)
	ctx := context.Background()

	// create ingress, one for all hosts sharing the same protocol
	in, err := r.ingressForRequests(namespace, irs, protocol)
	if err != nil {
		return err
	}

	op, err := resources.UpdateResource(r.Client, in, nil, nil)
	if err != nil {
		return err
	}
	resources.LogUpdates(log, op, "Updated Ingress")

	// reload ingress to grab updated status
	err = r.Client.Get(ctx, client.ObjectKey{Namespace: in.Namespace, Name: in.Name}, in)
	if err != nil {
		return err
	}

	var address string
	for _, lb := range in.Status.LoadBalancer.Ingress {
		if lb.Hostname != "" {
			address = lb.Hostname
			break
		}
		if lb.IP != "" {
			address = lb.IP
			break
		}
	}

	// update ingress url
	for _, ir := range irs {
		statusCopy := ir.Status.DeepCopy()
		ir.Status.Address = address

		if !apiequality.Semantic.DeepEqual(statusCopy, &ir.Status) {
			err = r.Client.Status().Update(context.TODO(), ir)
			if err != nil {
				break
			}
			log.Info("Updated IngressRequest status",
				"ingress", ir.Name,
				"address", address)
		}
	}
	return nil
}

func (r *IngressRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// trigger when certificates change
	certWatcher := &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(object handler.MapObject) []ctrl.Request {
			cert := object.Object.(*v1alpha1.CertificateRef)
			var requests []ctrl.Request

			// trigger a single request per namespace
			cc, err := resources.GetClusterConfig(mgr.GetClient())
			if err != nil {
				return requests
			}

			for _, target := range cc.Spec.Targets {
				irs, err := resources.GetIngressRequests(mgr.GetClient(), target)
				if err != nil {
					return requests
				}

				// queue hosts matching certs
				for _, ingressReq := range irs {
					for _, h := range ingressReq.Spec.Hosts {
						if resources.CertificateCovers(cert.Spec.Domain, h) {
							requests = append(requests, ctrl.Request{
								NamespacedName: types.NamespacedName{
									Name:      ingressReq.Name,
									Namespace: ingressReq.Namespace,
								},
							})
						}
					}
					break
				}

			}
			return requests
		}),
	}

	ingressWatcher := &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(object handler.MapObject) []ctrl.Request {
			in := object.Object.(*netv1.Ingress)

			var requests []ctrl.Request

			// ensure that ingress is being managed by Kon before triggering
			if in.Labels == nil || in.Labels[resources.Konstellation] == "" {
				return requests
			}

			protocol := in.Labels[resources.AppProtocolLabel]
			target := in.Labels[resources.TargetLabel]

			// any changes to a single resource in a target, will cause ingress to reconcile
			reqList, err := resources.GetIngressRequestsForAppProtocol(mgr.GetClient(), target, protocol)
			if err != nil || len(reqList) == 0 {
				return requests
			}
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      reqList[0].Name,
					Namespace: reqList[0].Namespace,
				},
			})
			return requests
		}),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.IngressRequest{}).
		Watches(&source.Kind{Type: &v1alpha1.CertificateRef{}}, certWatcher).
		Watches(&source.Kind{Type: &netv1.Ingress{}}, ingressWatcher).
		Complete(r)
}

func (r *IngressRequestReconciler) ingressForRequests(target string, requests []*v1alpha1.IngressRequest, protocol string) (*netv1.Ingress, error) {
	cc, err := resources.GetClusterConfig(r.Client)
	if err != nil {
		return nil, err
	}
	ingressComponent := ingress.NewIngressForCluster(cc.Spec.Cloud, cc.Name)

	backend := netv1.IngressBackend{
		Service: &netv1.IngressServiceBackend{
			Name: resources.IngressBackendName,
			Port: netv1.ServiceBackendPort{
				Number: 80,
			},
		},
	}

	ingressName := target
	if protocol != "" {
		ingressName = fmt.Sprintf("%s-%s", target, protocol)
	}
	pathType := netv1.PathTypePrefix
	in := netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: resources.IstioNamespace,
			Name:      ingressName,
			Labels: map[string]string{
				resources.Konstellation:    "1",
				resources.AppProtocolLabel: protocol,
				resources.TargetLabel:      target,
			},
		},
		Spec: netv1.IngressSpec{
			Rules: []netv1.IngressRule{
				{
					IngressRuleValue: netv1.IngressRuleValue{
						HTTP: &netv1.HTTPIngressRuleValue{
							Paths: []netv1.HTTPIngressPath{
								{
									Path:     "/",
									Backend:  backend,
									PathType: &pathType,
								},
							},
						},
					},
				},
			},
			DefaultBackend: &backend,
		},
	}

	hostsUsed := map[string]bool{}
	for _, req := range requests {
		for _, host := range req.Spec.Hosts {
			hostsUsed[host] = true
		}
	}

	if err = ingressComponent.ConfigureIngress(r.Client, &in, requests); err != nil {
		return nil, err
	}

	return &in, nil
}
