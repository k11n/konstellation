package cli

import (
	"testing"
)

func TestKubeCtl(t *testing.T) {
	err := KubeCtl()
	if err != nil {
		t.Fatalf("Error running kubectl: %v", err)
	}
}
