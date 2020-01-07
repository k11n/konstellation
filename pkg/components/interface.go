package copmonents

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Component interface {
	Name() string
	Version() string
	// check if local CLI is installed locally
	IsCLIInstalled() (bool, error)
	// installs CLI locally
	InstallCLI() error
	// installs the component onto the kube cluster
	InstallComponent(client.Client) error
}

type ObjectPatcher interface {
	PatchObject(runtime.Object) (runtime.Object, error)
}
