package istio

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/davidzhao/konstellation/pkg/utils/cli"
	"github.com/davidzhao/konstellation/pkg/utils/files"
)

type IstioInstaller struct {
}

func (i *IstioInstaller) Name() string {
	return "istio"
}

func (i *IstioInstaller) Version() string {
	return "1.5.1"
}

// returns true if CLI is needed and has not yet been installed
func (i *IstioInstaller) NeedsCLI() bool {
	p := i.cliPath()
	if _, err := os.Stat(p); err != nil {
		return true
	}

	// check version
	output, err := cli.RunBufferedCommand(i.cliPath(), "--remote=false", "version")
	if err != nil {
		return true
	}

	return !strings.HasPrefix(string(output), i.Version())
}

// installs CLI locally
func (i *IstioInstaller) InstallCLI() error {
	tmpdir, err := ioutil.TempDir("", "istio")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	// download install file
	installCmd := path.Join(tmpdir, "downloadIstio")
	err = files.DownloadFile(installCmd, "https://istio.io/downloadIstio")
	if err != nil {
		return err
	}
	if err = os.Chmod(installCmd, files.ExecutableFileMode); err != nil {
		return err
	}

	// run download script
	cmd := exec.Command(installCmd)
	cmd.Dir = path.Dir(i.installRoot())
	cmd.Env = []string{
		fmt.Sprintf("ISTIO_VERSION=%s", i.Version()),
	}
	if err = cmd.Run(); err != nil {
		return err
	}

	// should be done
	if err = os.MkdirAll(cli.GetBinDir(), files.DefaultDirectoryMode); err != nil {
		return err
	}

	// now symlink
	return os.Symlink(path.Join(i.installRoot(), "bin", "istioctl"), i.cliPath())
}

// installs the component onto the kube cluster
func (i *IstioInstaller) InstallComponent(client.Client) error {
	output, err := cli.RunBufferedCommand(i.cliPath(), "manifest", "generate",
		"--set", "components.sidecarInjector.enabled=true",
		"--set", "addonComponents.kiali.enabled=true",
		"--set", "addonComponents.grafana.enabled=true")
	if err != nil {
		return err
	}

	return cli.KubeApplyReader(bytes.NewBuffer(output))
}

func (i *IstioInstaller) cliPath() string {
	return path.Join(cli.GetBinDir(), "istioctl")
}

func (i *IstioInstaller) installRoot() string {
	return path.Join(cli.GetRootDir(), fmt.Sprintf("istio-%s", i.Version()))
}
