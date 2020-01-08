package linkerd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/davidzhao/konstellation/pkg/utils/cli"
	"github.com/davidzhao/konstellation/pkg/utils/files"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LinkerdComponent struct {
}

func (c *LinkerdComponent) Name() string {
	return "linkerd"
}

func (c *LinkerdComponent) Version() string {
	return "stable-2.6.1"
}

func (c *LinkerdComponent) NeedsCLI() bool {
	p := c.cliPath()
	if _, err := os.Stat(p); err != nil {
		// not installed
		return true
	}

	// check version
	output, err := c.runBufferedCommand("version")
	if err != nil {
		// should be able to run this but couldn't
		return true
	}

	if strings.HasPrefix(string(output), fmt.Sprintf("Client version: %s", c.Version())) {
		return false
	}

	return true
}

func (c *LinkerdComponent) InstallCLI() error {
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

	// set env before running this
	err = os.Chmod(installCmd, files.ExecutableFileMode)
	if err != nil {
		return err
	}

	cmd := exec.Command(installCmd)
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

func (c *LinkerdComponent) InstallComponent(kclient client.Client) error {
	return nil
}

func (c *LinkerdComponent) installRoot() string {
	return path.Join(cli.GetRootDir(), "linkerd2")
}

func (c *LinkerdComponent) cliPath() string {
	return path.Join(cli.GetBinDir(), "linkerd")
}

func (c *LinkerdComponent) runBufferedCommand(args ...string) ([]byte, error) {
	cmd := exec.Command(c.cliPath(), args...)
	return cmd.CombinedOutput()
}
