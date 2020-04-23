package resources

import (
	"context"
	"sort"

	netv1beta1 "k8s.io/api/networking/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
)

const (
	IngressName      = "kon-ingress"
	IngressNamespace = "istio-system"
	GatewayName      = "kon-gateway"
)

func GetIngressRequests(kclient client.Client) (requestList *v1alpha1.IngressRequestList, err error) {
	requestList = &v1alpha1.IngressRequestList{}
	err = kclient.List(context.TODO(), requestList)
	if err == nil {
		requests := requestList.Items
		// sort by creation time
		sort.Slice(requests, func(i, j int) bool {
			// by creation time
			return requests[j].CreationTimestamp.After(requests[i].CreationTimestamp.Time)
		})
	}
	return
}

func GetKonIngress(kclient client.Client) (*netv1beta1.Ingress, error) {
	ingress := netv1beta1.Ingress{}
	err := kclient.Get(context.TODO(), client.ObjectKey{Namespace: IngressNamespace, Name: IngressName}, &ingress)
	if err != nil {
		return nil, err
	}
	return &ingress, nil
}

func MatchesHost(mainHost string, matchingHost string) bool {
	return true
}
