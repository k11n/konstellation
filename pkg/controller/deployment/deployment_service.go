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

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
)

var (
	meshGateway = "mesh"
	konGateway  = fmt.Sprintf("%s/%s", resources.IstioNamespace, resources.GatewayName)
	allGateways = []string{meshGateway, konGateway}
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
		log.Info("Deleting unneeded Service", "appTarget", at.Name)
		// delete existing service
		err = r.client.Delete(context.TODO(), existing)
		return
	}

	// service still needed, update
	op, err := resources.UpdateResourceWithMerge(r.client, svc, at, r.scheme)
	if err != nil {
		return
	}

	resources.LogUpdates(log, op, "Updated service", "appTarget", at.Name)

	// update service hostname
	at.Status.Hostname = svc.Spec.ExternalName
	return
}

func (r *ReconcileDeployment) reconcileDestinationRule(at *v1alpha1.AppTarget, service *corev1.Service, releases []*v1alpha1.AppRelease) error {
	dr := newDestinationRule(at, service, releases)
	op, err := resources.UpdateResource(r.client, dr, at, r.scheme)
	resources.LogUpdates(log, op, "Updated DestinationRule", "appTarget", at.Name)
	return err
}

func (r *ReconcileDeployment) reconcileVirtualService(at *v1alpha1.AppTarget, service *corev1.Service, releases []*v1alpha1.AppRelease) error {
	vs := newVirtualService(at, service, releases)

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
		log.Info("Deleting unneeded virtual service", "appTarget", at.Name)
		return r.client.Delete(context.TODO(), existing)
	}

	op, err := resources.UpdateResource(r.client, vs, at, r.scheme)
	if err != nil {
		return err
	}

	resources.LogUpdates(log, op, "Updated VirtualService", "appTarget", at.Name)

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
				resources.AppReleaseLabel: ar.Name,
			},
		})
	}

	dr := &istio.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: at.TargetNamespace(),
		},
		Spec: istionetworking.DestinationRule{
			Host:    resources.ServiceHostname(service.Namespace, service.Name),
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

	svcHost := resources.ServiceHostname(service.Namespace, service.Name)
	allHosts := []string{
		svcHost,
	}
	if at.Spec.Ingress != nil {
		allHosts = append(allHosts, at.Spec.Ingress.Hosts...)
	}

	releasesByPort := map[int32][]*v1alpha1.AppRelease{}
	for _, ar := range releases {
		for _, port := range ar.Spec.Ports {
			releasesByPort[port.Port] = append(releasesByPort[port.Port], ar)
		}
	}

	var routes []*istionetworking.HTTPRoute

	// create internal routes, map each port
	for port, portReleases := range releasesByPort {
		route := &istionetworking.HTTPRoute{
			Match: []*istionetworking.HTTPMatchRequest{
				{
					Gateways: []string{meshGateway},
					Uri: &istionetworking.StringMatch{
						MatchType: &istionetworking.StringMatch_Prefix{
							Prefix: "/",
						},
					},
					Port: uint32(port),
				},
			},
		}
		for _, ar := range portReleases {
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

	// create external route, map the desired port to 80
	if at.Spec.Ingress != nil {
		var targetPort int32
		for _, port := range at.Spec.Ports {
			if targetPort == 0 {
				targetPort = port.Port
			}
			if port.Name == at.Spec.Ingress.Port {
				targetPort = port.Port
				break
			}
		}
		if targetPort != 0 {
			route := &istionetworking.HTTPRoute{
				Match: []*istionetworking.HTTPMatchRequest{
					{
						Gateways: []string{konGateway},
						Uri: &istionetworking.StringMatch{
							MatchType: &istionetworking.StringMatch_Prefix{
								Prefix: "/",
							},
						},
						Port: 80,
					},
				},
			}
			// should always have a port in order for VS to be defined
			for _, ar := range releases {
				rd := &istionetworking.HTTPRouteDestination{
					Destination: &istionetworking.Destination{
						Host:   svcHost,
						Port:   &istionetworking.PortSelector{Number: uint32(targetPort)},
						Subset: ar.Name,
					},
					Weight: ar.Spec.TrafficPercentage,
				}
				route.Route = append(route.Route, rd)
			}
			routes = append(routes, route)
		}
	}

	vs := &istio.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    ls,
		},
		Spec: istionetworking.VirtualService{
			Gateways: allGateways,
			Hosts:    allHosts,
			Http:     routes,
		},
	}

	return vs
}
