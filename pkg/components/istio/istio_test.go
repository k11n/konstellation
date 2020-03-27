package istio

import (
	"os"
	"testing"

	"github.com/davidzhao/konstellation/pkg/utils/cli"
)

func TestInstallIstio(t *testing.T) {
	// set env properly
	rootDir := cli.TestSetRootTempdir()
	defer os.RemoveAll(rootDir)

	l := IstioInstaller{}
	if !l.NeedsCLI() {
		// this should be true.. nothing's installed yet
		t.Fatalf("cli is not installed, but reporting uneeded")
	}

	err := l.InstallCLI()
	if err != nil {
		t.Fatalf("installCLI returned error: %v", err)
	}

	if l.NeedsCLI() {
		t.Fatalf("still needs CLI after successful install")
	}
}
