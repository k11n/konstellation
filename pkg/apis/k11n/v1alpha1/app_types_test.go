package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestAppTargetResources(t *testing.T) {
	cpu1 := resource.MustParse("1000m")
	cpu2 := resource.MustParse("2000m")
	app := &App{
		Spec: AppSpec{
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: cpu1,
				},
			},
			Targets: []TargetConfig{
				{
					Name: "test",
				},
			},
		},
	}

	// ensure test target contains cpu requirement
	resources := app.Spec.ResourcesForTarget("test")
	assert.NotNil(t, resources)
	assert.NotNil(t, resources.Requests.Cpu())
	assert.Equal(t, &cpu1, resources.Requests.Cpu())

	// test when only setting memory, and limits
	memory1 := resource.MustParse("100Mi")
	memory2 := resource.MustParse("200Mi")
	app.Spec.Targets[0].Resources = corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: memory1,
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: memory2,
		},
	}

	// ensure test target contains cpu requirement
	resources = app.Spec.ResourcesForTarget("test")
	assert.Equal(t, &cpu1, resources.Requests.Cpu())
	assert.Equal(t, &memory1, resources.Requests.Memory())
	assert.Equal(t, &memory2, resources.Limits.Memory())

	// test when overriding cpu info
	app.Spec.Targets[0].Resources = corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU: cpu2,
		},
	}
	resources = app.Spec.ResourcesForTarget("test")
	assert.Equal(t, &cpu2, resources.Requests.Cpu())
}
