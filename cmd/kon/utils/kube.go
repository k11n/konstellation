package utils

import (
	"io"
	"io/ioutil"
	"os"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/k11n/konstellation/pkg/utils/cli"
)

func KubeApplyFile(filename string, context string) error {
	filepath, err := TempfileFromDeployResource(filename)
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
	return cli.KubeCtl(args...)
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

func SaveKubeObject(encoder runtime.Encoder, obj runtime.Object, target string) error {
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()
	return encoder.Encode(obj, file)
}

func LoadKubeObject(decoder runtime.Decoder, objType runtime.Object, target string) (obj runtime.Object, err error) {
	data, err := ioutil.ReadFile(target)
	if err != nil {
		return
	}

	obj, _, err = decoder.Decode(data, nil, objType)
	return
}

//
//type PortForwarder struct {
//	KubeForwarder *portforward.PortForwarder
//	LocalPort     int
//	StopChan      chan struct{}
//	ReadyChan     <-chan struct{}
//}
//
//func NewPortForwarderForService(kclient client.Client, namespace string, service string) (pf *PortForwarder, err error) {
//	// find service and one of the pods
//	kclient.
//	return
//}
