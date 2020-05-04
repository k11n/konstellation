package resources

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
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

func GetActiveRelease(kclient client.Client, app, target string) (*v1alpha1.AppRelease, error) {
	var ar *v1alpha1.AppRelease
	err := ForEach(kclient, &v1alpha1.AppReleaseList{}, func(item interface{}) error {
		release := item.(v1alpha1.AppRelease)
		if release.Spec.Role == v1alpha1.ReleaseRoleActive {
			ar = &release
			return Break
		}
		return nil
	}, client.MatchingLabels{
		AppLabel:    app,
		TargetLabel: target,
	})
	if err != nil {
		return nil, err
	}
	return ar, nil
}

func SortAppReleasesByLatest(releases []*v1alpha1.AppRelease) {
	sort.Slice(releases, func(i, j int) bool {
		if !releases[i].CreationTimestamp.IsZero() && !releases[j].CreationTimestamp.IsZero() {
			return releases[i].CreationTimestamp.After(releases[j].CreationTimestamp.Time)
		}
		if releases[i].CreationTimestamp.IsZero() && !releases[j].CreationTimestamp.IsZero() {
			return true
		} else {
			return false
		}
	})
}

func GetPodsForAppRelease(kclient client.Client, namespace string, release string) (pods []string, err error) {
	err = ForEach(kclient, &corev1.PodList{}, func(item interface{}) error {
		pod := item.(corev1.Pod)
		pods = append(pods, pod.Name)
		return nil
	}, client.MatchingLabels{
		AppReleaseLabel: release,
	}, client.InNamespace(namespace))
	return
}
