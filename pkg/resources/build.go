package resources

import (
	"strings"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
)

func LabelsForBuild(build *v1alpha1.Build) map[string]string {
	image := strings.ReplaceAll(build.Spec.Image, "/", "_")
	return map[string]string{
		BUILD_REGISTRY_LABEL: build.Spec.Registry,
		BUILD_IMAGE_LABEL:    image,
	}
}
