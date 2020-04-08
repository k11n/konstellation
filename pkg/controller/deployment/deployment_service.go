package deployment

import (
	"context"

	networkingv1beta1 "istio.io/api/networking/v1beta1"
	istio "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/utils/objects"
)

func (r *ReconcileDeployment) reconcileService(at *v1alpha1.AppTarget) (svc *corev1.Service, err error) {
	// do we need a service? if no ports defined, we don't
	serviceNeeded := len(at.Spec.Ports) > 0
	svcTemplate := newServiceForAppTarget(at)
	namespace := at.ScopedName()

	// find existing service obj
	svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcTemplate.Name,
			Namespace: svcTemplate.Namespace,
		},
	}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: svc.GetName()}, svc)
	if err != nil {
		if errors.IsNotFound(err) {
			if !serviceNeeded {
				// don't need a service and none found
				svc = nil
				return
			}
		} else {
			// other errors, just return
			return
		}
	}

	// do we still want this service?
	if !serviceNeeded {
		// delete existing service
		err = r.client.Delete(context.TODO(), svc)
		return
	}

	// service still needed, update
	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, svc, func() error {
		svc.Labels = svcTemplate.Labels
		objects.MergeSlice(&svc.Spec.Ports, &svcTemplate.Spec.Ports)
		if svc.CreationTimestamp.IsZero() {
			svc.Spec.Selector = svcTemplate.Spec.Selector
			// Set AppTarget instance as the owner and controller
			if err := controllerutil.SetControllerReference(at, svc, r.scheme); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return
	}
	log.Info("Updated service", "operation", op)

	// update service hostname
	hostname := svc.Spec.ExternalName
	if at.Status.Hostname != hostname {
		log.Info("app hostname", "existing", at.Status.Hostname, "new", hostname)
		at.Status.Hostname = hostname
		err = r.client.Status().Update(context.TODO(), at)
	}
	return
}

func (r *ReconcileDeployment) reconcileVirtualService(at *v1alpha1.AppTarget, releases []*v1alpha1.AppRelease) error {
	return nil
}

func newServiceForAppTarget(at *v1alpha1.AppTarget) *corev1.Service {
	namespace := at.ScopedName()
	ls := labelsForAppTarget(at)

	ports := []corev1.ServicePort{}
	for _, p := range at.Spec.Ports {
		ports = append(ports, corev1.ServicePort{
			Name:     p.Name,
			Protocol: p.Protocol,
			Port:     p.Port,
		})
	}

	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      at.Spec.App,
			Namespace: namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			Ports:    ports,
			Selector: ls,
		},
	}
	return &svc
}

func newVirtualServiceForAppTarget(at *v1alpha1.AppTarget, releases []*v1alpha1.AppRelease) *istio.VirtualService {
	return &istio.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: at.ScopedName(),
			Name:      at.Name,
		},
		Spec: networkingv1beta1.VirtualService{},
	}
}
