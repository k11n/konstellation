package v1alpha1

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildUniqueName(t *testing.T) {
	b := Build{Spec: BuildSpec{
		Image: "image",
		Tag:   "t",
	}}
	name := b.GetUniqueName()
	prefix := "image-t-"
	assert.True(t, strings.HasPrefix(name, "image-t-"))
	assert.Len(t, name, len(prefix)+4)

	// add a registry and expect the name to not match
	b.Spec.Registry = "myregistry.com"
	assert.NotEqual(t, name, b.GetUniqueName())

	// ensure that invalid chars are converted to -
	b.Spec = BuildSpec{
		Image: "inv@l1d_NAME",
		Tag:   "a",
	}
	assert.True(t, strings.HasPrefix(b.GetUniqueName(), "inv-l1d-NAME-a"))
}
