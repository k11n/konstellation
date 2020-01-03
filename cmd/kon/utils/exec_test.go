package utils

import (
	"testing"
)

func TestTempfileFromResource(t *testing.T) {
	name := "crds/konstellation.dev_nodepools_crd.yaml"
	_, err := TempfileFromResource(name)
	if err != nil {
		t.Fatalf("Could not load tempfile %s", name)
	}
}

func TestKubeCtl(t *testing.T) {
	err := KubeCtl()
	if err != nil {
		t.Fatalf("Error running kubectl: %v", err)
	}
}
