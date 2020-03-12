package resources

import (
	"context"
	"sort"
	"strings"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func LabelsForRelease(build *v1alpha1.Release) map[string]string {
	image := strings.ReplaceAll(build.Spec.Image, "/", "_")
	return map[string]string{
		RELEASE_REGISTRY_LABEL: build.Spec.Registry,
		RELEASE_IMAGE_LABEL:    image,
		RELEASE_LABEL:          build.Name,
	}
}

func GetReleaseByName(kclient client.Client, name string) (*v1alpha1.Release, error) {
	r := v1alpha1.Release{}
	err := kclient.Get(context.TODO(), types.NamespacedName{Name: name}, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func GetReleasesByImage(kclient client.Client, registry, image string) (releases []v1alpha1.Release, err error) {
	releaseList := v1alpha1.ReleaseList{}
	err = kclient.List(context.TODO(), &releaseList, client.MatchingLabels{
		RELEASE_REGISTRY_LABEL: registry,
		RELEASE_IMAGE_LABEL:    image,
	})
	if err != nil {
		return
	}

	releases = releaseList.Items
	// sort by latest at top
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].GetCreationTimestamp().After(releases[j].GetCreationTimestamp().Time)
	})
	return
}
