package resources

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
)

func LabelsForBuild(build *v1alpha1.Build) map[string]string {
	image := strings.ReplaceAll(build.Spec.Image, "/", "_")
	return map[string]string{
		BuildRegistryLabel: build.Spec.Registry,
		BuildImageLabel:    image,
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
