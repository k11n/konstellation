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
	"github.com/davidzhao/konstellation/pkg/resources"
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

func (r *ReconcileDeployment) reconcileDestinationRule(at *v1alpha1.AppTarget, releases []*v1alpha1.AppRelease) error {
	drTemplate := newDestinationRule(at, releases)

	dr := &istio.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      at.Spec.App,
			Namespace: at.ScopedName(),
		},
	}
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, dr, func() error {
		if dr.CreationTimestamp.IsZero() {
			err := controllerutil.SetControllerReference(at, dr, r.scheme)
			if err != nil {
				return err
			}
		}

		// use merge to avoid defaults clearing out
		objects.MergeObject(&dr.Spec, &drTemplate.Spec)
		return nil
	})

	return err
}

func (r *ReconcileDeployment) reconcileVirtualService(at *v1alpha1.AppTarget, service *corev1.Service, releases []*v1alpha1.AppRelease) error {
	// do we need a service? if no ports defined, we don't
	serviceNeeded := len(at.Spec.Ports) > 0
	vsTemplate := newVirtualService(at, releases)
	namespace := at.ScopedName()

	// find existing service obj
	vs := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vsTemplate.Name,
			Namespace: vsTemplate.Namespace,
		},
	}
	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: vs.GetName()}, vs)
	if err != nil {
		if errors.IsNotFound(err) {
			if !serviceNeeded {
				// don't need a service and none found
				return nil
			}
		} else {
			// other errors, just return
			return err
		}
	}

	_, err = controllerutil.CreateOrUpdate(context.TODO(), r.client, vs, func() error {
		if vs.CreationTimestamp.IsZero() {
			err := controllerutil.SetControllerReference(at, vs, r.scheme)
			if err != nil {
				return err
			}
		}
		vs.Labels = vsTemplate.Labels
		objects.MergeObject(&vs.Spec, &vsTemplate.Spec)
		return nil
	})

	return err
}

func newServiceForAppTarget(at *v1alpha1.AppTarget) *corev1.Service {
	namespace := at.ScopedName()
	ls := labelsForAppTarget(at)

	var ports []corev1.ServicePort
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

func newDestinationRule(at *v1alpha1.AppTarget, releases []*v1alpha1.AppRelease) *istio.DestinationRule {
	subsets := make([]*networkingv1beta1.Subset, 0, len(releases))
	name := at.Spec.App

	for _, ar := range releases {
		subsets = append(subsets, &networkingv1beta1.Subset{
			Name: ar.Name,
			Labels: map[string]string{
				resources.APP_RELEASE_LABEL: ar.Name,
			},
		})
	}

	dr := &istio.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: at.ScopedName(),
		},
		Spec: networkingv1beta1.DestinationRule{
			Host:    name,
			Subsets: subsets,
			// TODO: allow other types of connections
			TrafficPolicy: &networkingv1beta1.TrafficPolicy{
				LoadBalancer: &networkingv1beta1.LoadBalancerSettings{
					LbPolicy: &networkingv1beta1.LoadBalancerSettings_Simple{
						Simple: networkingv1beta1.LoadBalancerSettings_ROUND_ROBIN,
					},
				},
			},
		},
	}
	return dr
}

func newVirtualService(at *v1alpha1.AppTarget, releases []*v1alpha1.AppRelease) *istio.VirtualService {
	namespace := at.ScopedName()
	ls := labelsForAppTarget(at)
	name := at.Spec.App
	hosts := []string{name}
	if len(at.Spec.IngressHosts) != 0 {
		hosts = append(hosts, at.Spec.IngressHosts...)
	}
	routeDestinations := make([]*networkingv1beta1.HTTPRouteDestination, 0, len(releases))
	for _, ar := range releases {
		routeDestinations = append(routeDestinations, &networkingv1beta1.HTTPRouteDestination{
			Destination: &networkingv1beta1.Destination{
				Host:   name,
				Subset: ar.Name,
			},
			Weight: ar.Spec.TrafficPercentage,
		})
	}

	vs := &istio.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    ls,
		},
		Spec: networkingv1beta1.VirtualService{
			Hosts: hosts,
			Http: []*networkingv1beta1.HTTPRoute{
				{
					Route: routeDestinations,
				},
			},
		},
	}
	return vs
}
