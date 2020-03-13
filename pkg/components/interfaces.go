package components

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ComponentInstaller interface {
	Name() string
	Version() string
	// returns true if CLI is needed and has not yet been installed
	NeedsCLI() bool
	// installs CLI locally
	InstallCLI() error
	// installs the component onto the kube cluster
	InstallComponent(client.Client) error
}
