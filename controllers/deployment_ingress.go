package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
)

func (r *DeploymentReconciler) reconcileIngressRequest(ctx context.Context, at *v1alpha1.AppTarget) error {
	ir := newIngressRequestForAppTarget(at)
	existing := &v1alpha1.IngressRequest{}
	// find IR
	key, err := client.ObjectKeyFromObject(ir)
	if err != nil {
		return err
	}
	err = r.Client.Get(ctx, key, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			// don't need an ingress, we are good
			if !at.NeedsIngress() {
				return nil
			}
		} else {
			return err
		}
	}
	//log.Info("existing IR", "ingressRequest", existing)

	// if we don't need it, delete existing one
	if !at.NeedsIngress() {
		r.Log.Info("Deleting unused IngressRequest", "appTarget", at)
		return r.Client.Delete(ctx, existing)
	}

	// create or update
	op, err := resources.UpdateResource(r.Client, ir, at, r.Scheme)
	resources.LogUpdates(r.Log, op, "Updated IngressRequest", "appTarget", at.Name, "hosts", ir.Spec.Hosts)

	at.Status.Hostname = ir.Status.Address
	return err
}

func newIngressRequestForAppTarget(at *v1alpha1.AppTarget) *v1alpha1.IngressRequest {
	labels := labelsForAppTarget(at)
	// always created in default namespace
	ir := &v1alpha1.IngressRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      at.ScopedName(),
			Namespace: at.TargetNamespace(),
			Labels:    labels,
		},
	}
	if at.Spec.Ingress != nil {
		ir.Spec.Hosts = at.Spec.Ingress.Hosts
	}
	return ir
}
