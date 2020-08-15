package resources

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"github.com/thoas/go-funk"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
)

var (
	releasePattern = regexp.MustCompile(`^([\w-]+)-\d{8}-\d{4}-\w{4}$`)
)

func GetAppReleases(kclient client.Client, app string, target string) ([]*v1alpha1.AppRelease, error) {
	releases := make([]*v1alpha1.AppRelease, 0)
	err := ForEach(kclient, &v1alpha1.AppReleaseList{}, func(item interface{}) error {
		release := item.(v1alpha1.AppRelease)
		releases = append(releases, &release)
		return nil
	}, client.MatchingLabels{
		AppLabel:    app,
		TargetLabel: target,
	}, client.InNamespace(target))
	if err != nil {
		return nil, err
	}

	SortAppReleasesByLatest(releases)
	return releases, nil
}

func GetAppReleaseByName(kclient client.Client, release string, target string) (ar *v1alpha1.AppRelease, err error) {
	matches := releasePattern.FindStringSubmatch(release)
	if len(matches) != 2 {
		err = fmt.Errorf("Not a valid release name: %s", release)
		return
	}
	app := matches[1]
	// figure out app name from this
	if target == "" {
		targets, err := GetAppTargets(kclient, app)
		if err != nil {
			return nil, err
		}
		if len(targets) == 0 {
			return nil, fmt.Errorf("Could not find targets for app %s", app)
		}
		target = targets[0].Spec.Target
	}

	return GetAppRelease(kclient, app, target, release)
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
	}, client.InNamespace(target))
	if err != nil {
		return nil, err
	}
	return ar, nil
}

func GetTargetRelease(kclient client.Client, app, target string) (*v1alpha1.AppRelease, error) {
	var ar, tr *v1alpha1.AppRelease
	err := ForEach(kclient, &v1alpha1.AppReleaseList{}, func(item interface{}) error {
		release := item.(v1alpha1.AppRelease)
		if release.Spec.Role == v1alpha1.ReleaseRoleTarget {
			tr = &release
		} else if release.Spec.Role == v1alpha1.ReleaseRoleActive {
			ar = &release
		}
		return nil
	}, client.MatchingLabels{
		AppLabel:    app,
		TargetLabel: target,
	}, client.InNamespace(target))
	if err != nil {
		return nil, err
	}

	// if no target, then return active
	if tr == nil {
		tr = ar
	}
	return tr, nil
}

func GetAppRelease(kclient client.Client, app, target, name string) (*v1alpha1.AppRelease, error) {
	ar := &v1alpha1.AppRelease{}
	err := kclient.Get(context.TODO(), client.ObjectKey{
		Namespace: target,
		Name:      name,
	}, ar)
	return ar, err
}

func SortAppReleasesByLatest(releases []*v1alpha1.AppRelease) {
	sort.SliceStable(releases, func(i, j int) bool {
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

var statusOrder = []corev1.PodPhase{
	corev1.PodRunning,
	corev1.PodSucceeded,
	corev1.PodFailed,
	corev1.PodUnknown,
	corev1.PodPending,
}

func GetReplicaSetForAppRelease(kclient client.Client, ar *v1alpha1.AppRelease) (rs *appsv1.ReplicaSet, err error) {
	rs = &appsv1.ReplicaSet{}
	key := client.ObjectKey{Namespace: ar.Namespace, Name: ar.Name}
	err = kclient.Get(context.TODO(), key, rs)
	return
}

func GetPodsForAppRelease(kclient client.Client, namespace string, release string) (pods []*corev1.Pod, err error) {
	err = ForEach(kclient, &corev1.PodList{}, func(item interface{}) error {
		pod := item.(corev1.Pod)
		pods = append(pods, &pod)
		return nil
	}, client.MatchingLabels{
		AppReleaseLabel: release,
	}, client.InNamespace(namespace))

	if err != nil {
		return
	}

	sort.SliceStable(pods, func(i, j int) bool {
		return funk.IndexOf(statusOrder, pods[i].Status.Phase) < funk.IndexOf(statusOrder, pods[j].Status.Phase)
	})
	return
}

func GetFirstDeployableRelease(releases []*v1alpha1.AppRelease) *v1alpha1.AppRelease {
	for _, ar := range releases {
		if ar.Spec.Role == v1alpha1.ReleaseRoleBad {
			continue
		}
		return ar
	}
	return nil
}
