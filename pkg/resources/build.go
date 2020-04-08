package resources

import (
	"context"
	"sort"
	"strings"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func LabelsForBuild(build *v1alpha1.Build) map[string]string {
	image := strings.ReplaceAll(build.Spec.Image, "/", "_")
	return map[string]string{
		BUILD_REGISTRY_LABEL: build.Spec.Registry,
		BUILD_IMAGE_LABEL:    image,
		BUILD_LABEL:          build.Name,
	}
}

func GetBuildByName(kclient client.Client, name string) (*v1alpha1.Build, error) {
	r := v1alpha1.Build{}
	err := kclient.Get(context.TODO(), types.NamespacedName{Name: name}, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func GetBuildsByImage(kclient client.Client, registry, image string, count int) (builds []v1alpha1.Build, err error) {
	buildList := v1alpha1.BuildList{}
	if count == 0 {
		count = defaultListSize
	}
	err = kclient.List(context.TODO(), &buildList, client.MatchingLabels{
		BUILD_REGISTRY_LABEL: registry,
		BUILD_IMAGE_LABEL:    image,
	}, client.Limit(count))
	if err != nil {
		return
	}

	builds = buildList.Items
	// sort by latest at top
	sort.Slice(builds, func(i, j int) bool {
		return builds[i].GetCreationTimestamp().After(builds[j].GetCreationTimestamp().Time)
	})
	return
}
