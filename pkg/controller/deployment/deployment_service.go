package deployment

import (
	"context"
	"fmt"

	istionetworking "istio.io/api/networking/v1alpha3"
	istio "istio.io/client-go/pkg/apis/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/resources"
)

var (
	allGateways = []string{"mesh", fmt.Sprintf("%s.%s.cluster.local", resources.GatewayName, resources.IngressNamespace)}
)

func (r *ReconcileDeployment) reconcileService(at *v1alpha1.AppTarget) (svc *corev1.Service, err error) {
	// do we need a service? if no ports defined, we don't
	serviceNeeded := at.NeedsService()
	svc = newServiceForAppTarget(at)

	// find existing service obj
	existing := &corev1.Service{}
	key, err := client.ObjectKeyFromObject(svc)
	if err != nil {
		return
	}
	err = r.client.Get(context.TODO(), key, existing)
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
		err = r.client.Delete(context.TODO(), existing)
		return
	}

	// service still needed, update
	op, err := resources.UpdateResource(r.client, svc, at, r.scheme)
	// TODO: test to ensure ports is merged correctly
	//op, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, svc, func() error {
	//	svc.Labels = svcTemplate.Labels
	//	objects.MergeSlice(&svc.Spec.Ports, &svcTemplate.Spec.Ports)
	//	if svc.CreationTimestamp.IsZero() {
	//		svc.Spec.Selector = svcTemplate.Spec.Selector
	//		// Set AppTarget instance as the owner and controller
	//		if err := controllerutil.SetControllerReference(at, svc, r.scheme); err != nil {
	//			return err
	//		}
	//	}
	//	return nil
	//})
	if err != nil {
		return
	}
	log.Info("Updated service", "operation", op)

	// update service hostname
	at.Status.Hostname = svc.Spec.ExternalName
	return
}

func (r *ReconcileDeployment) reconcileDestinationRule(at *v1alpha1.AppTarget, service *corev1.Service, releases []*v1alpha1.AppRelease) error {
	dr := newDestinationRule(at, service, releases)
	_, err := resources.UpdateResource(r.client, dr, at, r.scheme)
	return err
}

func (r *ReconcileDeployment) reconcileVirtualService(at *v1alpha1.AppTarget, service *corev1.Service, releases []*v1alpha1.AppRelease) error {
	vs := newVirtualService(at, service, releases)
	log.Info("Reconciling virtualservice", "appTarget", at.Name, "needsService", at.NeedsService())

	// find existing VS obj
	existing := &istio.VirtualService{}
	key, err := client.ObjectKeyFromObject(vs)
	if err != nil {
		return err
	}
	err = r.client.Get(context.TODO(), key, existing)
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
		log.Info("deleting existing virtual service", "appTarget", at.Name)
		return r.client.Delete(context.TODO(), existing)
	}

	op, err := resources.UpdateResource(r.client, vs, at, r.scheme)
	if err != nil {
		return err
	}

	if op != controllerutil.OperationResultNone {
		log.Info("finished reconciling VS", "operation", op)
	}

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

func newDestinationRule(at *v1alpha1.AppTarget, service *corev1.Service, releases []*v1alpha1.AppRelease) *istio.DestinationRule {
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
			Host:    resources.GetServiceDNS(service),
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

func newVirtualService(at *v1alpha1.AppTarget, service *corev1.Service, releases []*v1alpha1.AppRelease) *istio.VirtualService {
	namespace := at.TargetNamespace()
	ls := labelsForAppTarget(at)
	name := at.Spec.App

	svcHost := resources.GetServiceDNS(service)
	hosts := []string{
		svcHost,
	}
	if at.Spec.Ingress != nil {
		hosts = append(hosts, at.Spec.Ingress.Hosts...)
	}

	releasesByPort := map[int32][]*v1alpha1.AppRelease{}
	for _, ar := range releases {
		for _, port := range ar.Spec.Ports {
			releasesByPort[port.Port] = append(releasesByPort[port.Port], ar)
		}
	}

	var routes []*istionetworking.HTTPRoute
	for port, releases := range releasesByPort {
		route := &istionetworking.HTTPRoute{
			Match: []*istionetworking.HTTPMatchRequest{
				{
					Gateways: allGateways,
					Uri: &istionetworking.StringMatch{
						MatchType: &istionetworking.StringMatch_Prefix{
							Prefix: "/",
						},
					},
					Port: uint32(port),
				},
			},
		}
		for _, ar := range releases {
			rd := &istionetworking.HTTPRouteDestination{
				Destination: &istionetworking.Destination{
					Host:   svcHost,
					Port:   &istionetworking.PortSelector{Number: uint32(port)},
					Subset: ar.Name,
				},
				Weight: ar.Spec.TrafficPercentage,
			}
			route.Route = append(route.Route, rd)
		}
		routes = append(routes, route)
	}

	vs := &istio.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    ls,
		},
		Spec: istionetworking.VirtualService{
			Gateways: allGateways,
			Hosts:    hosts,
			Http:     routes,
		},
	}

	return vs
}
