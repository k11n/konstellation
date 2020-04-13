package linkerd

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

type LinkerdInstaller struct {
}

func (c *LinkerdInstaller) Name() string {
	return "linkerd"
}

func (c *LinkerdInstaller) Version() string {
	return "stable-2.6.1"
}

func (c *LinkerdInstaller) NeedsCLI() bool {
	p := c.cliPath()
	if _, err := os.Stat(p); err != nil {
		// not installed
		return true
	}

	// check version
	output, err := cli.RunBufferedCommand(c.cliPath(), "version")
	if err != nil {
		// should be able to run this but couldn't
		return true
	}

	// TODO: more sophisticated version parsing
	if strings.HasPrefix(string(output), fmt.Sprintf("Client version: %s", c.Version())) {
		return false
	}

	return true
}

func (c *LinkerdInstaller) InstallCLI() error {
	tmpdir, err := ioutil.TempDir("", "linkerd")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	// download install file
	installCmd := path.Join(tmpdir, "install")
	err = files.DownloadFile(installCmd, "https://run.linkerd.io/install")
	if err != nil {
		return err
	}

	err = os.Chmod(installCmd, files.ExecutableFileMode)
	if err != nil {
		return err
	}

	// fmt.Printf("running install cmd: %s\n", installCmd)
	cmd := exec.Command(installCmd)
	// set env before running this
	cmd.Env = []string{
		fmt.Sprintf("LINKERD2_VERSION=%s", c.Version()),
		fmt.Sprintf("INSTALLROOT=%s", c.installRoot()),
	}
	err = cmd.Run()
	if err != nil {
		return err
	}

	err = os.MkdirAll(cli.GetBinDir(), files.DefaultDirectoryMode)
	if err != nil {
		return err
	}

	// now symlink
	return os.Symlink(path.Join(c.installRoot(), "bin", "linkerd"), c.cliPath())
}

func (c *LinkerdInstaller) InstallComponent(kclient client.Client) error {
	output := new(bytes.Buffer)
	cmd := exec.Command(c.cliPath(), "install")
	cmd.Stdout = output
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	return cli.KubeApplyReader(output)
}

func (c *LinkerdInstaller) installRoot() string {
	return path.Join(cli.GetRootDir(), "linkerd2")
}

func (c *LinkerdInstaller) cliPath() string {
	return path.Join(cli.GetBinDir(), "linkerd")
}
