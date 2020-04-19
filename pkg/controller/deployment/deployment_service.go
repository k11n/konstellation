package deployment

import (
	"context"

	istionetworking "istio.io/api/networking/v1alpha3"
	istio "istio.io/client-go/pkg/apis/networking/v1alpha3"
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
	serviceNeeded := at.NeedsService()
	svcTemplate := newServiceForAppTarget(at)
	namespace := at.TargetNamespace()

	// find existing service obj
	svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcTemplate.Name,
			Namespace: svcTemplate.Namespace,
		},
	}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: svc.Name}, svc)
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
	at.Status.Hostname = svc.Spec.ExternalName
	return
}

func (r *ReconcileDeployment) reconcileDestinationRule(at *v1alpha1.AppTarget, releases []*v1alpha1.AppRelease) error {
	drTemplate := newDestinationRule(at, releases)

	dr := &istio.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      at.Spec.App,
			Namespace: at.TargetNamespace(),
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
	vsTemplate := newVirtualService(at, releases)
	namespace := at.TargetNamespace()

	// find existing VS obj
	vs := &istio.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vsTemplate.Name,
			Namespace: vsTemplate.Namespace,
		},
	}
	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: vs.GetName()}, vs)
	if err != nil {
		if errors.IsNotFound(err) {
			if !at.NeedsService() {
				// don't need a service and none found
				return nil
			}
		} else {
			// other errors, just return
			return err
		}
	}

	// found existing service, but not needed anymore
	if !at.NeedsService() {
		// delete existing service
		return r.client.Delete(context.TODO(), vs)
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
	namespace := at.TargetNamespace()
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
	subsets := make([]*istionetworking.Subset, 0, len(releases))
	name := at.Spec.App

	for _, ar := range releases {
		subsets = append(subsets, &istionetworking.Subset{
			Name: ar.Name,
			Labels: map[string]string{
				resources.APP_RELEASE_LABEL: ar.Name,
			},
		})
	}

	dr := &istio.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: at.TargetNamespace(),
		},
		Spec: istionetworking.DestinationRule{
			Host:    name,
			Subsets: subsets,
			// TODO: allow other types of connections
			TrafficPolicy: &istionetworking.TrafficPolicy{
				LoadBalancer: &istionetworking.LoadBalancerSettings{
					LbPolicy: &istionetworking.LoadBalancerSettings_Simple{
						Simple: istionetworking.LoadBalancerSettings_ROUND_ROBIN,
					},
				},
			},
		},
	}
	return dr
}

func newVirtualService(at *v1alpha1.AppTarget, releases []*v1alpha1.AppRelease) *istio.VirtualService {
	namespace := at.TargetNamespace()
	ls := labelsForAppTarget(at)
	name := at.Spec.App
	hosts := []string{name}
	if at.Spec.Ingress != nil {
		hosts = append(hosts, at.Spec.Ingress.Hosts...)
	}
	routeDestinations := make([]*istionetworking.HTTPRouteDestination, 0, len(releases))
	for _, ar := range releases {
		routeDestinations = append(routeDestinations, &istionetworking.HTTPRouteDestination{
			Destination: &istionetworking.Destination{
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
		Spec: istionetworking.VirtualService{
			Hosts: hosts,
			Http: []*istionetworking.HTTPRoute{
				{
					Route: routeDestinations,
				},
			},
		},
	}
	return vs
}
