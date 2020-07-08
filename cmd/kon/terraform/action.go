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

	"github.com/k11n/konstellation/pkg/utils/files"
)

type Flags string

const (
	OptionDisplayOutput   Flags = "display_output"
	OptionRequireApproval Flags = "require_approval"
)

type Action struct {
	WorkingDir      string
	Vars            []Var
	values          map[string]string
	env             map[string]string
	displayOutput   bool
	requireApproval bool

	initialized bool
}

type Var struct {
	Name         string
	CreationOnly bool
	TemplateOnly bool
}

type Values map[Var]interface{}

type Option interface {
	Apply(*Action)
}

func (f Flags) Apply(a *Action) {
	switch f {
	case OptionDisplayOutput:
		a.displayOutput = true
	case OptionRequireApproval:
		a.requireApproval = true
	}
}

func (v Values) Apply(a *Action) {
	if a.values == nil {
		a.values = make(map[string]string)
	}
	for key, val := range v {
		// if not a string, encode to json
		if strVal, ok := val.(string); ok {
			a.values[key.Name] = strVal
		} else {
			data, _ := json.Marshal(val)
			a.values[key.Name] = string(data)
		}
	}
}

type EnvVar map[string]string

func (v EnvVar) Apply(a *Action) {
	if a.env == nil {
		a.env = make(map[string]string)
	}
	for key, val := range v {
		a.env[key] = val
	}
}

func NewTerraformAction(dir string, vars []Var, opts ...Option) *Action {
	a := &Action{
		WorkingDir: dir,
		Vars:       vars,
	}
	for _, o := range opts {
		o.Apply(a)
	}
	return a
}

func (a *Action) Option(opt Option) *Action {
	opt.Apply(a)
	return a
}

func (a *Action) replaceTemplates() error {
	files, err := ioutil.ReadDir(a.WorkingDir)
	if err != nil {
		return err
	}
	for _, fi := range files {
		if strings.HasPrefix(fi.Name(), ".") {
			continue
		} else if !fi.IsDir() {
			if err = a.replaceTemplate(path.Join(a.WorkingDir, fi.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *Action) replaceTemplate(filePath string) error {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	s := string(content)
	hasReplacements := false
	for key, val := range a.values {
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

func (a *Action) Apply() error {
	if err := a.checkRequiredVars(true); err != nil {
		return err
	}

	args := []string{
		"apply",
		"-compact-warnings",
	}
	if !a.requireApproval {
		args = append(args, "-auto-approve")
	}
	for _, v := range a.Vars {
		if v.TemplateOnly {
			continue
		}
		args = append(args, "-var")
		args = append(args, fmt.Sprintf("%s=%s", v.Name, a.values[v.Name]))
	}
	if err := a.runAction(args...); err != nil {
		return errors.Wrap(err, "error with terraform apply")
	}
	return nil
}

func (a *Action) GetOutput() (content []byte, err error) {
	// first initialize terraform
	if err = a.initIfNeeded(); err != nil {
		return
	}

	buf := bytes.NewBuffer(nil)
	cmd := exec.Command("terraform", "output", "-json")
	cmd.Dir = a.WorkingDir
	cmd.Stderr = os.Stderr
	cmd.Stdout = buf
	cmd.Env = a.getEnvVars()

	if err = cmd.Run(); err != nil {
		return
	}
	content = buf.Bytes()
	return
}

func (a *Action) Destroy() error {
	if err := a.checkRequiredVars(false); err != nil {
		return err
	}

	args := []string{
		"destroy",
	}
	if !a.requireApproval {
		args = append(args, "-auto-approve")
	}
	for _, v := range a.Vars {
		if v.CreationOnly || v.TemplateOnly {
			continue
		}
		args = append(args, "-var")
		args = append(args, fmt.Sprintf("%s=%s", v.Name, a.values[v.Name]))
	}
	return a.runAction(args...)
}

func (a *Action) RemoveDir() error {
	return os.RemoveAll(a.WorkingDir)
}

func (a *Action) checkRequiredVars(creation bool) error {
	for _, v := range a.Vars {
		if v.CreationOnly && !creation {
			// creation vars don't have to be passed in during destroys
			continue
		}
		if _, ok := a.values[v.Name]; !ok {
			return fmt.Errorf("value not found for required var: %s", v.Name)
		}
	}
	return nil
}

func (a *Action) runAction(args ...string) error {
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
	cmd.Env = a.getEnvVars()
	if connectStdIn {
		cmd.Stdin = os.Stdin
	}
	if connectStdOut {
		cmd.Stdout = os.Stdout
	}

	return cmd.Run()
}

func (a *Action) initIfNeeded() error {
	if a.initialized {
		return nil
	}

	fmt.Println("Preparing terraform...")
	if err := a.replaceTemplates(); err != nil {
		return err
	}
	cmd := exec.Command("terraform", "init")
	cmd.Dir = a.WorkingDir
	cmd.Env = a.getEnvVars()
	//cmd.Stderr = os.Stderr
	//cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "Could not init terraform. Path: %s", a.WorkingDir)
	}

	a.initialized = true
	return nil
}

func (a *Action) getEnvVars() []string {
	envVars := make([]string, 0, len(a.env))
	for key, val := range a.env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, val))
	}
	return envVars
}
