package utils

import (
	"io"
	"io/ioutil"
	"os"

	rice "github.com/GeertJohan/go.rice"
	"github.com/davidzhao/konstellation/pkg/utils/cli"
)

func KubeApply(filename string) error {
	filepath, err := TempfileFromDeployResource(filename)
	if err != nil {
		return err
	}
	defer os.Remove(filepath)

	return cli.KubeCtl("apply", "-f", filepath)
}

func DeployResourcesBox() *rice.Box {
	return rice.MustFindBox("../../../deploy")
}

func TempfileFromDeployResource(name string) (temppath string, err error) {
	source, err := DeployResourcesBox().Open(name)
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
