package resources

import (
	"context"
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
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

func MatchesHost(mainHost string, matchingHost string) bool {
	return true
}
