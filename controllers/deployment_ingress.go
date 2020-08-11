package controllers

import (
	"context"

	netv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/components/ingress"
	"github.com/k11n/konstellation/pkg/resources"
)

func (r *DeploymentReconciler) reconcileIngress(ctx context.Context, at *v1alpha1.AppTarget) error {
	in, err := r.ingressForAppTarget(at)
	if err != nil {
		return err
	}
	existing := &netv1beta1.Ingress{}
	// find IR
	key, err := client.ObjectKeyFromObject(in)
	if err != nil {
		return err
	}
	err = r.Client.Get(ctx, key, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			existing = nil
			// don't need an ingress, we are good
			if !at.NeedsIngress() {
				return nil
			}
		} else {
			return err
		}
	}

	// if we don't need it, delete existing one
	if !at.NeedsIngress() {
		r.Log.Info("Deleting unused Ingress", "appTarget", at)
		return r.Client.Delete(ctx, existing)
	}

	// create or update
	op, err := resources.UpdateResource(r.Client, in, at, r.Scheme)
	if err != nil {
		return err
	}
	resources.LogUpdates(r.Log, op, "Updated Ingress", "appTarget", at.Name)

	// use existing record's status
	if existing != nil {
		var address string
		for _, lb := range existing.Status.LoadBalancer.Ingress {
			if lb.Hostname != "" {
				address = lb.Hostname
				break
			}
			if lb.IP != "" {
				address = lb.IP
				break
			}
		}
		if at.Status.Hostname != address {
			r.Log.Info("updating appTarget hostname", "appTarget", at.Name, "hostname", address, "old", at.Status.Hostname)
		}
		at.Status.Hostname = address
	}

	return nil
}

func (r *DeploymentReconciler) ingressForAppTarget(at *v1alpha1.AppTarget) (*netv1beta1.Ingress, error) {
	cc, err := resources.GetClusterConfig(r.Client)
	if err != nil {
		return nil, err
	}
	ingressComponent := ingress.NewIngressForCluster(cc.Spec.Cloud, cc.Name)

	in := netv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: resources.IstioNamespace,
			Name:      at.Name,
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

	if err = ingressComponent.ConfigureIngress(r.Client, &in, at.Spec.Ingress); err != nil {
		return nil, err
	}

	return &in, nil
}
