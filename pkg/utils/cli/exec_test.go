package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripComments(t *testing.T) {
	test1 := `no comments
valid text
`
	assert.Equal(t, test1, stripCommentsString(test1))

	testComments := `#comment line
#
valid text`
	assert.Equal(t, "valid text", stripCommentsString(testComments))

	testCommentsInMiddle := `valid line
#comment line
valid line`
	testCommentsInMiddleExpected := `valid line
valid line`
	assert.Equal(t, testCommentsInMiddleExpected, stripCommentsString(testCommentsInMiddle))
}
