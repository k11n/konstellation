package utils

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	rice "github.com/GeertJohan/go.rice"
)

func KubeApply(filename string) error {
	filepath, err := TempfileFromResource(filename)
	if err != nil {
		return err
	}
	defer os.Remove(filepath)

	return KubeCtl("apply", "-f", filepath)
}

func KubeCtl(args ...string) error {
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ResourcesBox() *rice.Box {
	return rice.MustFindBox("../../../deploy")
}

func TempfileFromResource(name string) (temppath string, err error) {
	source, err := ResourcesBox().Open(name)
	if err != nil {
		return
	}
	defer source.Close()
	temp, err := ioutil.TempFile("", "kbox")
	if err != nil {
		return
	}
	defer temp.Close()
	_, err = io.Copy(temp, source)
	if err != nil {
		return
	}

	temppath = temp.Name()
	temp = nil
	return
}
