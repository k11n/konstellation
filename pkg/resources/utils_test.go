package resources

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
)

func TestCreateOrUpdate(t *testing.T) {
	obj := &v1alpha1.AppTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testName",
			Namespace: "testNamespace",
		},
	}
	UpdateResource(nil, obj)
}
