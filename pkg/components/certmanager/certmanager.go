package certmanager

import (
	"fmt"

	"github.com/davidzhao/konstellation/pkg/utils/cli"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CertManagerInstaller struct {
}

func (i *CertManagerInstaller) Name() string {
	return "cert-manager"
}

func (d *CertManagerInstaller) Version() string {
	return "0.14.1"
}

func (d *CertManagerInstaller) NeedsCLI() bool {
	return false
}

func (d *CertManagerInstaller) InstallCLI() error {
	return nil
}

func (d *CertManagerInstaller) InstallComponent(kclient client.Client) error {
	url := fmt.Sprintf("https://github.com/jetstack/cert-manager/releases/download/v%s/cert-manager.yaml",
		d.Version())
	return cli.KubeApply(url)
}
