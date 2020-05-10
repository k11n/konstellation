package deployment

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
)

func (r *ReconcileDeployment) reconcileIngressRequest(at *v1alpha1.AppTarget) error {
	ir := newIngressRequestForAppTarget(at)
	existing := &v1alpha1.IngressRequest{}
	// find IR
	key, err := client.ObjectKeyFromObject(ir)
	if err != nil {
		return err
	}
	err = r.client.Get(context.TODO(), key, existing)
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
		log.Info("Deleting unused IngressRequest", "appTarget", at)
		return r.client.Delete(context.TODO(), existing)
	}

	// create or update
	op, err := resources.UpdateResource(r.client, ir, at, r.scheme)
	resources.LogUpdates(log, op, "Updated IngressRequest", "appTarget", at.Name, "hosts", ir.Spec.Hosts)
	return err
}

func newIngressRequestForAppTarget(at *v1alpha1.AppTarget) *v1alpha1.IngressRequest {
	labels := labelsForAppTarget(at)
	// always created in default namespace
	ir := &v1alpha1.IngressRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:   at.ScopedName(),
			Labels: labels,
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
