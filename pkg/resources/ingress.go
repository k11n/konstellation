package resources

import (
	"context"
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
