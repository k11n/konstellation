package terraform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/pkg/errors"
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
	initialized     bool
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

func (a *TerraformAction) Run() error {
	// first initialize terraform
	if err := a.initIfNeeded(); err != nil {
		return err
	}

	args := []string{
		"apply",
		//"plan",
		"-compact-warnings",
	}
	connectStdOut := false
	connectStdIn := false
	if a.displayOutput || a.requireApproval {
		connectStdOut = true
	}
	if a.requireApproval {
		connectStdIn = true
	} else {
		args = append(args, "-auto-approve")
	}

	for key, val := range a.vars {
		args = append(args, "-var")
		args = append(args, fmt.Sprintf("%s=%s", key, val))
	}

	fmt.Printf("Running: %v\n", args)
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
