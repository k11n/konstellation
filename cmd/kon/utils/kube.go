package utils

import (
	"io/ioutil"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
)

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
