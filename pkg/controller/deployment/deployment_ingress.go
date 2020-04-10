package deployment

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/utils/objects"
)

func (r *ReconcileDeployment) reconcileIngressRequest(at *v1alpha1.AppTarget) error {
	irTemplate := newIngressRequestForAppTarget(at)
	ir := &v1alpha1.IngressRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: irTemplate.Name,
		},
	}
	// find IR
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: ir.Name}, ir)
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
		return r.client.Delete(context.TODO(), ir)
	}

	// create or update
	_, err = controllerutil.CreateOrUpdate(context.TODO(), r.client, ir, func() error {
		if ir.CreationTimestamp.IsZero() {
			err := controllerutil.SetControllerReference(at, ir, r.scheme)
			if err != nil {
				return err
			}
		}
		objects.MergeObject(&ir.Spec, &irTemplate.Spec)
		return nil
	})
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
	}
	return ir
}

//func newGateWayForAppTarget(at *v1alpha1.AppTarget) *istio.Gateway {
//	return &istio.Gateway{
//		ObjectMeta: v1.ObjectMeta{
//			Namespace: at.TargetNamespace(),
//			Name:      fmt.Sprintf("%s-ingress", at.Spec.App),
//		},
//		Spec: v1beta1.Gateway{
//			Selector: map[string]string{
//				"istio": "ingressgateway",
//			},
//			Servers: []*v1beta1.Server{},
//		},
//	}
//}
