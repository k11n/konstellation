package cli

import (
	"io"
	"os"
	"os/exec"

	"github.com/k11n/konstellation/pkg/utils/assets"
)

var KubeDisplayOutput = false

func KubeCtl(args ...string) error {
	cmd := exec.Command("kubectl", args...)
	if KubeDisplayOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func KubeApplyReader(reader io.Reader) error {
	args := []string{
		"apply", "-f", "-",
	}
	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = reader
	if KubeDisplayOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func KubeApplyFromBox(filename string, context string) error {
	filepath, err := assets.TempfileFromDeployResource(filename)
	if err != nil {
		return err
	}
	defer os.Remove(filepath)

	args := []string{
		"apply", "-f", filepath,
	}
	if context != "" {
		args = append(args, "--context", context)
	}
	return KubeCtl(args...)
}

func KubeApply(url string) error {
	args := []string{
		"apply", "-f", url,
	}
	cmd := exec.Command("kubectl", args...)
	if KubeDisplayOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}
