package utils

import "testing"

func TestTempfileFromResource(t *testing.T) {
	name := "crds/k11n.dev_nodepools_crd.yaml"
	_, err := TempfileFromDeployResource(name)
	if err != nil {
		t.Fatalf("Could not load tempfile %s", name)
	}
}
