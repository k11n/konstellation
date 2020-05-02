package resources

import (
	"context"
	"sort"

	netv1beta1 "k8s.io/api/networking/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
)

const (
	IngressName = "kon-ingress"
	GatewayName = "kon-gateway"
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
func GetIngressRequestForAppTarget(kclient client.Client, app, target string) (ir *v1alpha1.IngressRequest, err error) {
	reqList := &v1alpha1.IngressRequestList{}
	err = kclient.List(context.TODO(), reqList, client.MatchingLabels{
		AppLabel:    app,
		TargetLabel: target,
	})
	if err != nil {
		return
	}

	if len(reqList.Items) == 0 {
		return
	}

	ir = &reqList.Items[0]
	return
}

func GetKonIngress(kclient client.Client) (*netv1beta1.Ingress, error) {
	ingress := netv1beta1.Ingress{}
	err := kclient.Get(context.TODO(), client.ObjectKey{Namespace: IstioNamespace, Name: IngressName}, &ingress)
	if err != nil {
		return nil, err
	}
	return &ingress, nil
}

func MatchesHost(mainHost string, matchingHost string) bool {
	return true
}
