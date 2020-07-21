package assets

import (
	"io"
	"io/ioutil"
	"os"
	"path"

	rice "github.com/GeertJohan/go.rice"
	"github.com/pkg/errors"

	"github.com/k11n/konstellation/pkg/utils/files"
)

func DeployResourcesBox() *rice.Box {
	return rice.MustFindBox("../../../deploy")
}

func TFResourceBox() *rice.Box {
	return rice.MustFindBox("../../../components/terraform")
}

func ExtractBoxFiles(box *rice.Box, target string, items ...string) error {
	if _, err := os.Stat(target); err == nil {
		// delete entire folder
		if err = os.RemoveAll(target); err != nil {
			return err
		}
	}

	err := os.MkdirAll(target, files.DefaultDirectoryMode)
	if err != nil {
		return err
	}

	// extract files over
	for _, item := range items {
		source, err := box.Open(item)
		if err != nil {
			return errors.Wrapf(err, "Could not find file %s", item)
		}
		defer source.Close()
		dest, err := os.Create(path.Join(target, path.Base(item)))
		if err != nil {
			return err
		}
		defer dest.Close()

		if _, err = io.Copy(dest, source); err != nil {
			return err
		}
	}
	return nil
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
