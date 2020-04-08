package resources

import (
	"context"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
)

const (
	DefaultStepDelay = 1 * time.Minute
)

func GetAppReleases(kclient client.Client, app string, target string, count int) ([]*v1alpha1.AppRelease, error) {
	releaseList := v1alpha1.AppReleaseList{}
	err := kclient.List(context.TODO(), &releaseList, client.MatchingLabels{
		APP_LABEL:    app,
		TARGET_LABEL: target,
	}, client.Limit(count))
	if err != nil {
		return nil, err
	}

	// sort list by build name, newest first
	releases := make([]*v1alpha1.AppRelease, 0, len(releaseList.Items))
	for _, r := range releaseList.Items {
		releases = append(releases, &r)
	}
	SortAppReleasesByBuild(releases)
	return releases, nil
}

func SortAppReleasesByBuild(releases []*v1alpha1.AppRelease) {
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
