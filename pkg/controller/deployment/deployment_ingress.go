package deployment

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/resources"
)

func (r *ReconcileDeployment) reconcileIngressRequest(at *v1alpha1.AppTarget) error {
	ir := newIngressRequestForAppTarget(at)
	existing := &v1alpha1.IngressRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: ir.Name,
		},
	}
	// find IR
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: ir.Name}, existing)
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

	// if we don't need it, delete existing one
	if !at.NeedsIngress() {
		return r.client.Delete(context.TODO(), existing)
	}

	// create or update
	_, err = resources.UpdateResource(r.client, ir, at, r.scheme)
	return err
}

func newIngressRequestForAppTarget(at *v1alpha1.AppTarget) *v1alpha1.IngressRequest {
	labels := labelsForAppTarget(at)
	// always created in default namespace
	ir := &v1alpha1.IngressRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      at.ScopedName(),
			Namespace: "istio-system",
			Labels:    labels,
		},
	}
	if at.Spec.Ingress != nil {
		ir.Spec.Hosts = at.Spec.Ingress.Hosts
		ir.Spec.Port = at.Spec.Ingress.Port
		if ir.Spec.Port == 0 {
			// when not specified.. we try to pick the first port
			if len(at.Spec.Ports) > 0 {
				ir.Spec.Port = at.Spec.Ports[0].Port
			}
		}
	}
	return ir
}
