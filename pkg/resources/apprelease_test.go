package resources

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k11n/konstellation/api/v1alpha1"
)

func TestSortAppReleasesByLatest(t *testing.T) {
	releases := []*v1alpha1.AppRelease{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "third",
				CreationTimestamp: metav1.Time{Time: time.Now().Add(time.Minute * -1)},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "second",
				CreationTimestamp: metav1.Time{Time: time.Now()},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "first",
			},
		},
	}

	SortAppReleasesByLatest(releases)

	assert.Len(t, releases, 3)
	assert.Equal(t, "first", releases[0].Name)
	assert.Equal(t, "second", releases[1].Name)
	assert.Equal(t, "third", releases[2].Name)
}

func TestReleasePattern(t *testing.T) {
	release := "app2048-20200423-1531-c495"
	assert.True(t, releasePattern.MatchString(release))
	matches := releasePattern.FindStringSubmatch(release)
	assert.Len(t, matches, 2)
	assert.Equal(t, "app2048", matches[1])

	release2 := "app-with-dashes-20200423-1531-c495"
	assert.True(t, releasePattern.MatchString(release2))
	matches = releasePattern.FindStringSubmatch(release2)
	assert.Len(t, matches, 2)
	assert.Equal(t, "app-with-dashes", matches[1])
}
