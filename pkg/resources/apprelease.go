package resources

import (
	"context"
	"sort"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
)

func GetAppReleases(kclient client.Client, app string, target string, count int) ([]*v1alpha1.AppRelease, error) {
	releaseList := v1alpha1.AppReleaseList{}
	err := kclient.List(context.TODO(), &releaseList, client.MatchingLabels{
		AppLabel:    app,
		TargetLabel: target,
	}, client.Limit(count))
	if err != nil {
		return nil, err
	}

	// sort list by build name, newest first
	releases := make([]*v1alpha1.AppRelease, 0, len(releaseList.Items))
	for i := range releaseList.Items {
		r := releaseList.Items[i]
		releases = append(releases, &r)
	}
	SortAppReleasesByLatest(releases)
	return releases, nil
}

func SortAppReleasesByLatest(releases []*v1alpha1.AppRelease) {
	sort.Slice(releases, func(i, j int) bool {
		return strings.Compare(releases[i].Name, releases[j].Name) > 0
	})
}

func FirstAvailableRelease(releases []*v1alpha1.AppRelease) *v1alpha1.AppRelease {
	for _, ar := range releases {
		if ar.Status.NumAvailable > 0 {
			return ar
		}
	}
	return nil
}
