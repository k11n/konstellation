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
	ir := r.ingressRequestForAppTarget(at)
	existing := &v1alpha1.IngressRequest{}
	// find IR
	key, err := client.ObjectKeyFromObject(ir)
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
		r.Log.Info("Deleting unused IngressRequest", "appTarget", at)
		return r.Client.Delete(ctx, existing)
	}

	// create or update
	op, err := resources.UpdateResource(r.Client, ir, at, r.Scheme)
	if err != nil {
		return err
	}
	resources.LogUpdates(r.Log, op, "Updated IngressRequest", "appTarget", at.Name)

	// use existing record's status
	if existing != nil {
		address := existing.Status.Address
		at.Status.Hostname = address
	}

	return nil
}

func (r *DeploymentReconciler) ingressRequestForAppTarget(at *v1alpha1.AppTarget) *v1alpha1.IngressRequest {
	labels := labelsForAppTarget(at)
	ir := &v1alpha1.IngressRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      at.ScopedName(),
			Namespace: at.TargetNamespace(),
			Labels:    labels,
		},
	}
	if at.Spec.Ingress != nil {
		ir.Spec.Hosts = at.Spec.Ingress.Hosts
		ir.Spec.Paths = at.Spec.Ingress.Paths
		ir.Spec.RequireHTTPS = at.Spec.Ingress.RequireHTTPS
		ir.Spec.Annotations = at.Spec.Ingress.Annotations
	}
	return ir
}
