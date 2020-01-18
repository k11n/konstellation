package cli

import (
	"io"
	"os"
	"os/exec"
)

func KubeCtl(args ...string) error {
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func KubeApplyReader(reader io.Reader) error {
	args := []string{
		"apply", "-f", "-",
	}
	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = reader
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
