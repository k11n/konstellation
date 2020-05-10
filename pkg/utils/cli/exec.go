package cli

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/k11n/konstellation/pkg/utils/files"
)

func RunBufferedCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

func RunCommandWithStd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ExecuteUserEditor(original []byte, name string) (edited []byte, err error) {
	// check editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	dir, err := ioutil.TempDir("", "konstellation")
	if err != nil {
		return
	}
	defer os.RemoveAll(dir)

	target := path.Join(dir, name)
	if original == nil {
		original = make([]byte, 0)
	}
	err = ioutil.WriteFile(target, original, files.DefaultFileMode)
	if err != nil {
		return
	}

	// launch editor
	var args []string
	parts := strings.Split(editor, " ")
	if len(parts) > 1 {
		editor = parts[0]
		args = parts[1:]
	}
	args = append(args, target)
	cmd := exec.Command(editor, args...)
	err = cmd.Run()
	if err != nil {
		return
	}

	return ioutil.ReadFile(target)
}
