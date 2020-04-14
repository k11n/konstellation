package terraform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/pkg/errors"

	"github.com/davidzhao/konstellation/pkg/utils/files"
)

type TerraformFlags string

const (
	OptionDisplayOutput   TerraformFlags = "display_output"
	OptionRequireApproval TerraformFlags = "require_approval"
)

type TerraformAction struct {
	WorkingDir      string
	vars            map[string]string
	displayOutput   bool
	requireApproval bool

	initialized bool
}

type TerraformOption interface {
	Apply(*TerraformAction)
}

func (f TerraformFlags) Apply(a *TerraformAction) {
	switch f {
	case OptionDisplayOutput:
		a.displayOutput = true
	case OptionRequireApproval:
		a.requireApproval = true
	}
}

type TerraformVars map[string]interface{}

func (v TerraformVars) Apply(a *TerraformAction) {
	if a.vars == nil {
		a.vars = make(map[string]string)
	}
	for key, val := range v {
		// if not a string, encode to json
		if strVal, ok := val.(string); ok {
			a.vars[key] = strVal
		} else {
			data, _ := json.Marshal(val)
			a.vars[key] = string(data)
		}
	}
}

func NewTerraformAction(dir string, opts ...TerraformOption) *TerraformAction {
	a := &TerraformAction{
		WorkingDir: dir,
	}
	for _, o := range opts {
		o.Apply(a)
	}
	return a
}

func (a *TerraformAction) Option(opt TerraformOption) *TerraformAction {
	opt.Apply(a)
	return a
}

func (a *TerraformAction) replaceTemplates() error {
	files, err := ioutil.ReadDir(a.WorkingDir)
	if err != nil {
		return err
	}
	for _, fi := range files {
		if !fi.IsDir() {
			if err = a.replaceTemplate(path.Join(a.WorkingDir, fi.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *TerraformAction) replaceTemplate(filePath string) error {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	s := string(content)
	hasReplacements := false
	for key, val := range a.vars {
		search := fmt.Sprintf("$${%s}", key)
		if strings.Contains(s, search) {
			hasReplacements = true
			s = strings.ReplaceAll(s, search, val)
		}
	}
	if hasReplacements {
		return ioutil.WriteFile(filePath, []byte(s), files.DefaultFileMode)
	}
	return nil
}

func (a *TerraformAction) Apply() error {
	args := []string{
		"apply",
		"-compact-warnings",
	}
	if !a.requireApproval {
		args = append(args, "-auto-approve")
	}

	for key, val := range a.vars {
		args = append(args, "-var")
		args = append(args, fmt.Sprintf("%s=%s", key, val))
	}
	return a.runAction(args...)
}

func (a *TerraformAction) Destroy() error {
	args := []string{
		"destroy",
	}
	if !a.requireApproval {
		args = append(args, "-auto-approve")
	}
	return a.runAction(args...)
}

func (a *TerraformAction) runAction(args ...string) error {
	// first initialize terraform
	if err := a.initIfNeeded(); err != nil {
		return err
	}

	connectStdOut := false
	connectStdIn := false
	if a.displayOutput || a.requireApproval {
		connectStdOut = true
	}
	if a.requireApproval {
		connectStdIn = true
	}

	fmt.Printf("Generated terraform plan: %s\n", a.WorkingDir)
	fmt.Printf("Running: terraform %s\n", strings.Join(args, " "))
	cmd := exec.Command("terraform", args...)
	cmd.Dir = a.WorkingDir
	cmd.Stderr = os.Stderr
	if connectStdIn {
		cmd.Stdin = os.Stdin
	}
	if connectStdOut {
		cmd.Stdout = os.Stdout
	}

	return cmd.Run()
}

func (a *TerraformAction) initIfNeeded() error {
	if a.initialized {
		return nil
	}

	fmt.Println("Initializing terraform...")
	if err := a.replaceTemplates(); err != nil {
		return err
	}
	cmd := exec.Command("terraform", "init")
	cmd.Dir = a.WorkingDir
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "Could not init terraform. Path: %s", a.WorkingDir)
	}

	a.initialized = true
	return nil
}

func (a *TerraformAction) GetOutput() (content []byte, err error) {
	// first initialize terraform
	if err = a.initIfNeeded(); err != nil {
		return
	}

	buf := bytes.NewBuffer(nil)
	cmd := exec.Command("terraform", "output", "-json")
	cmd.Dir = a.WorkingDir
	cmd.Stderr = os.Stderr
	cmd.Stdout = buf

	if err = cmd.Run(); err != nil {
		return
	}
	content = buf.Bytes()
	return
}
