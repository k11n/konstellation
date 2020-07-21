package assets

import "testing"

func TestTempfileFromResource(t *testing.T) {
	name := "crds.yaml"
	_, err := TempfileFromDeployResource(name)
	if err != nil {
		t.Fatalf("Could not load tempfile %s", name)
	}
}
