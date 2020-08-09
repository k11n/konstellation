package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseImageInfo(t *testing.T) {
	ai := appInfo{}
	parseImageInfo("image-with-no-tag", &ai)
	assert.Equal(t, "image-with-no-tag", ai.DockerImage)
	assert.Empty(t, ai.DockerTag)
	assert.Empty(t, ai.Registry)

	// with tag
	ai = appInfo{}
	parseImageInfo("repo/image:1", &ai)
	assert.Equal(t, "repo/image", ai.DockerImage)
	assert.Equal(t, "1", ai.DockerTag)
	assert.Empty(t, ai.Registry)

	// with registry
	ai = appInfo{}
	parseImageInfo("ecr.registry.com/repo/image:1", &ai)
	assert.Equal(t, "repo/image", ai.DockerImage)
	assert.Equal(t, "1", ai.DockerTag)
	assert.Equal(t, "ecr.registry.com", ai.Registry)
}
