package v1alpha1

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateAppReleaseName(t *testing.T) {
	at := &AppTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-prod",
			Labels: map[string]string{
				AppTargetHash: "abcdefghijklmn",
			},
		},
		Spec: AppTargetSpec{
			App: "app",
		},
	}

	build := &Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "somebuild",
			CreationTimestamp: metav1.Now(),
		},
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "prod",
			Name:      "conf",
			Labels: map[string]string{
				ConfigHashLabel: "2388537556582",
			},
		},
	}

	// test w/o config
	name := GenerateAppReleaseName(at, build, nil)
	assert.NotEmpty(t, name)
	assert.Len(t, strings.Split(name, "-"), 4)

	// test with config, should be the same
	name = GenerateAppReleaseName(at, build, cm)
	assert.NotEmpty(t, name)
	assert.Len(t, strings.Split(name, "-"), 4)
}
