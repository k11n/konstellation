package resources

import (
	"context"
	"strings"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func LabelsForRelease(build *v1alpha1.Release) map[string]string {
	image := strings.ReplaceAll(build.Spec.Image, "/", "_")
	return map[string]string{
		BUILD_REGISTRY_LABEL: build.Spec.Registry,
		BUILD_IMAGE_LABEL:    image,
		RELEASE_LABEL:        "",
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
