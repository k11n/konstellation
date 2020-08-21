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

	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/components/ingress"
	"github.com/k11n/konstellation/pkg/resources"

	netv1beta1 "k8s.io/api/networking/v1beta1"
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

// +kubebuilder:rbac:groups=k11n.dev,resources=ingressrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k11n.dev,resources=ingressrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *IngressRequestReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("ingressrequest", req.NamespacedName)

	res := ctrl.Result{}

	// fetch all requests in same namespace to reconcile
	irs, err := resources.GetIngressRequests(r.Client, req.Namespace)
	if err != nil {
		return res, err
	}

	if len(irs) == 0 {
		// kill everything
		r.Client.Delete(ctx, &netv1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: resources.IstioNamespace,
				Name:      req.Namespace,
			},
		})
		return res, nil
	}

	// create ingress, one for all hosts
	in, err := r.ingressForRequests(req.Namespace, irs)
	if err != nil {
		return res, err
	}

	op, err := resources.UpdateResource(r.Client, in, nil, nil)
	if err != nil {
		return res, err
	}
	resources.LogUpdates(log, op, "Updated Ingress")

	// load ingress since it has an updated status
	err = r.Client.Get(ctx, client.ObjectKey{Namespace: in.Namespace, Name: in.Name}, in)
	if err != nil {
		return res, err
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
			log.Info("Updated IngressRequest status", "ingress", ir.Name)
		}
	}
	return res, nil
}

func (r *IngressRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	certWatcher := &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(object handler.MapObject) []ctrl.Request {
			requests := []ctrl.Request{}

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

				// just need to request once. there'll be one ingress with all of the hosts
				for _, ingressReq := range irs {
					requests = append(requests, ctrl.Request{
						NamespacedName: types.NamespacedName{
							Name:      ingressReq.Name,
							Namespace: ingressReq.Namespace,
						},
					})
					break
				}

			}
			return requests
		}),
	}

	ingressWatcher := &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(object handler.MapObject) []ctrl.Request {
			ingress := object.Object.(*netv1beta1.Ingress)

			requests := []ctrl.Request{}

			// ensure that ingress is being managed by Kon before triggering
			if ingress.Labels == nil || ingress.Labels[resources.Konstellation] == "" {
				return requests
			}

			// any changes to a single resource in a target, will cause ingress to reconcile
			reqList, err := resources.GetIngressRequests(mgr.GetClient(), ingress.Name)
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
		Watches(&source.Kind{Type: &netv1beta1.Ingress{}}, ingressWatcher).
		Complete(r)
}

func (r *IngressRequestReconciler) ingressForRequests(target string, requests []*v1alpha1.IngressRequest) (*netv1beta1.Ingress, error) {
	cc, err := resources.GetClusterConfig(r.Client)
	if err != nil {
		return nil, err
	}
	ingressComponent := ingress.NewIngressForCluster(cc.Spec.Cloud, cc.Name)

	in := netv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: resources.IstioNamespace,
			Name:      target,
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
										ServiceName: resources.IngressBackendName,
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

	if err = ingressComponent.ConfigureIngress(r.Client, &in, requests); err != nil {
		return nil, err
	}

	return &in, nil
}
