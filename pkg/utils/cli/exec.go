package cli

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

const (
	editComment = "# Please edit the object below. Lines beginning with a '#' will be ignored.\n#\n"
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
	f, err := os.Create(target)
	if err != nil {
		return
	}

	f.WriteString(editComment)
	f.Write(original)
	f.Close()

	// launch editor
	var args []string
	parts := strings.Split(editor, " ")
	if len(parts) > 1 {
		editor = parts[0]
		args = parts[1:]
	}
	args = append(args, target)
	cmd := exec.Command(editor, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return
	}

	content, err := ioutil.ReadFile(target)
	if err != nil {
		return
	}

	edited = stripComments(content)
	return
}

var commentPattern = regexp.MustCompile("#.*?\n")

func stripComments(original []byte) []byte {
	return commentPattern.ReplaceAll(original, nil)
}

func stripCommentsString(original string) string {
	return string(stripComments([]byte(original)))
}
