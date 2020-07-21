package controllers

import (
	"context"
	"fmt"
	"sort"

	istionetworking "istio.io/api/networking/v1beta1"
	istio "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
)

var (
	meshGateway    = "mesh"
	ingressGateway = fmt.Sprintf("%s/%s", resources.IstioNamespace, resources.GatewayName)
	allGateways    = []string{meshGateway, ingressGateway}
)

func (r *DeploymentReconciler) reconcileService(ctx context.Context, at *v1alpha1.AppTarget) (svc *corev1.Service, err error) {
	// do we need a service? if no ports defined, we don't
	serviceNeeded := at.NeedsService()
	svc = newServiceForAppTarget(at)

	// find existing service obj
	existing := &corev1.Service{}
	key, err := client.ObjectKeyFromObject(svc)
	if err != nil {
		return
	}
	err = r.Client.Get(ctx, key, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			if !serviceNeeded {
				// don't need a service and none found
				return nil, nil
			}
		} else {
			// other errors, just return
			return
		}
	}

	// do we still want this service?
	if !serviceNeeded {
		r.Log.Info("Deleting unneeded Service", "appTarget", at.Name)
		// delete existing service
		err = r.Client.Delete(context.TODO(), existing)
		return
	}

	// service still needed, update
	op, err := resources.UpdateResourceWithMerge(r.Client, svc, at, r.Scheme)
	if err != nil {
		return
	}

	resources.LogUpdates(r.Log, op, "Updated service", "appTarget", at.Name)

	// update service hostname
	at.Status.Hostname = svc.Spec.ExternalName
	return
}

func (r *DeploymentReconciler) reconcileDestinationRule(ctx context.Context, at *v1alpha1.AppTarget, service *corev1.Service, releases []*v1alpha1.AppRelease) error {
	serviceNeeded := service != nil
	dr := newDestinationRule(at, service, releases)

	existing := &istio.DestinationRule{}
	key, err := client.ObjectKeyFromObject(dr)
	if err != nil {
		return err
	}
	err = r.Client.Get(ctx, key, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			if !serviceNeeded {
				return nil
			}
		} else {
			return err
		}
	}

	op, err := resources.UpdateResource(r.Client, dr, at, r.Scheme)
	if err != nil {
		return err
	}
	resources.LogUpdates(r.Log, op, "Updated DestinationRule", "appTarget", at.Name)
	return nil
}

func (r *DeploymentReconciler) reconcileVirtualService(ctx context.Context, at *v1alpha1.AppTarget, service *corev1.Service, releases []*v1alpha1.AppRelease) error {
	serviceNeeded := service != nil
	vs := newVirtualService(at, service, releases)

	// find existing VS obj
	existing := &istio.VirtualService{}
	key, err := client.ObjectKeyFromObject(vs)
	if err != nil {
		return err
	}
	err = r.Client.Get(ctx, key, existing)
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

	// found existing service, but not needed anymore
	if !serviceNeeded {
		// delete existing service
		r.Log.Info("Deleting unneeded virtual service", "appTarget", at.Name)
		return r.Client.Delete(ctx, existing)
	}

	op, err := resources.UpdateResource(r.Client, vs, at, r.Scheme)
	if err != nil {
		return err
	}

	resources.LogUpdates(r.Log, op, "Updated VirtualService", "appTarget", at.Name)

	return nil
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
			Selector: selectorsForAppTarget(at),
		},
	}
	return &svc
}

func newDestinationRule(at *v1alpha1.AppTarget, service *corev1.Service, releases []*v1alpha1.AppRelease) *istio.DestinationRule {
	// service could be nil, if this resource doesn't require a service
	// still go ahead with creation of the rule, but will be used for deletion instead
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
	if service != nil {
		dr.Spec.Host = resources.ServiceHostname(service.Namespace, service.Name)
	}
	return dr
}

func newVirtualService(at *v1alpha1.AppTarget, service *corev1.Service, releases []*v1alpha1.AppRelease) *istio.VirtualService {
	// service could be nil, when a virtual service isn't needed
	namespace := at.TargetNamespace()
	ls := labelsForAppTarget(at)
	name := at.Spec.App

	allHosts := make([]string, 0)
	svcHost := ""
	if service != nil {
		svcHost = resources.ServiceHostname(service.Namespace, service.Name)
		allHosts = append(allHosts, svcHost)
	}
	if at.Spec.Ingress != nil {
		allHosts = append(allHosts, at.Spec.Ingress.Hosts...)
	}

	releasesByPort := map[int32][]*v1alpha1.AppRelease{}
	ports := make([]int32, 0)
	for _, ar := range releases {
		for _, port := range ar.Spec.Ports {
			releasesByPort[port.Port] = append(releasesByPort[port.Port], ar)
			ports = append(ports, port.Port)
		}
	}
	sort.Slice(ports, func(i, j int) bool {
		return ports[i] < ports[j]
	})

	var routes []*istionetworking.HTTPRoute

	// create internal routes, map each port
	for _, port := range ports {
		portReleases := releasesByPort[port]
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
						Gateways: []string{ingressGateway},
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
