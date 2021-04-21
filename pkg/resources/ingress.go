package resources

import (
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
)

const (
	GatewayName = "ingressgateway"
)

func GetIngressRequests(kclient client.Client, target string) (irs []*v1alpha1.IngressRequest, err error) {
	err = ForEach(kclient, &v1alpha1.IngressRequestList{}, func(item interface{}) error {
		ir := item.(v1alpha1.IngressRequest)
		irs = append(irs, &ir)
		return nil
	}, client.InNamespace(target))
	if err == nil {
		// sort by creation time
		sort.SliceStable(irs, func(i, j int) bool {
			// by creation time
			return irs[j].CreationTimestamp.After(irs[i].CreationTimestamp.Time)
		})
	}
	return
}

func GetIngressRequestsForAppProtocol(kclient client.Client, target string, protocol string) (irs []*v1alpha1.IngressRequest, err error) {
	err = ForEach(kclient, &v1alpha1.IngressRequestList{}, func(item interface{}) error {
		ir := item.(v1alpha1.IngressRequest)
		if ir.Spec.AppProtocol == protocol {
			irs = append(irs, &ir)
		}
		return nil
	}, client.InNamespace(target))
	if err == nil {
		// sort by creation time
		sort.SliceStable(irs, func(i, j int) bool {
			// by creation time
			return irs[j].CreationTimestamp.After(irs[i].CreationTimestamp.Time)
		})
	}
	return
}
