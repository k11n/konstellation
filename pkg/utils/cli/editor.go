package cli

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"

	errorshelper "github.com/pkg/errors"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ResourceEditor struct {
	context   context.Context
	kclient   client.Client
	namespace string
	name      string
	obj       runtime.Object
	encoder   runtime.Encoder
	decoder   runtime.Decoder
}

func NewResourceEditor(kclient client.Client, obj runtime.Object, namespace, name string) *ResourceEditor {
	return &ResourceEditor{
		context:   context.Background(),
		kclient:   kclient,
		namespace: namespace,
		name:      name,
		obj:       obj,
		encoder: json.NewSerializerWithOptions(json.DefaultMetaFactory, nil, nil,
			json.SerializerOptions{
				Yaml:   true,
				Pretty: true,
				Strict: false,
			}),
		decoder: clientgoscheme.Codecs.UniversalDeserializer(),
	}
}

func (r *ResourceEditor) EditExisting(ignoreNotFound bool) (res controllerutil.OperationResult, err error) {
	// no exiting file to read from, read current state from Kube
	objExists := true
	err = r.kclient.Get(r.context, client.ObjectKey{Namespace: r.namespace, Name: r.name}, r.obj)
	if err != nil {
		if ignoreNotFound && errors.IsNotFound(err) {
			objExists = false
			err = nil
		} else {
			return
		}
	}

	buf := bytes.NewBuffer(nil)
	if err = r.encoder.Encode(r.obj, buf); err != nil {
		err = errorshelper.Wrapf(err, "could not encode object: %s", r.name)
		return
	}

	// now edit the thing
	data, err := ExecuteUserEditor(buf.Bytes(), fmt.Sprintf("%s.yaml", r.name))
	if err != nil {
		return
	}

	return r.saveObject(data, !objExists)
}

func (r *ResourceEditor) UpdateFromFile(filename string) (res controllerutil.OperationResult, err error) {
	var data []byte
	if filename == "-" {
		// read from stdin
		data, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			return res, errorshelper.Wrap(err, "could not read from stdin")
		}
	} else {
		// read from file
		data, err = ioutil.ReadFile(filename)
		if err != nil {
			return res, errorshelper.Wrapf(err, "could not read resource from file %s", filename)
		}
	}

	objExists := true
	err = r.kclient.Get(r.context, client.ObjectKey{Namespace: r.namespace, Name: r.name}, r.obj)
	if err != nil {
		if errors.IsNotFound(err) {
			objExists = false
			err = nil
		} else {
			return
		}
	}

	return r.saveObject(data, !objExists)
}

func (r *ResourceEditor) saveObject(data []byte, createNew bool) (res controllerutil.OperationResult, err error) {
	obj, _, err := r.decoder.Decode(data, nil, r.obj.DeepCopyObject())
	if err != nil {
		return
	}

	if apiequality.Semantic.DeepEqual(obj, r.obj) {
		return controllerutil.OperationResultNone, nil
	} else if !createNew {
		res = controllerutil.OperationResultUpdated
		err = r.kclient.Update(r.context, obj)
	} else {
		res = controllerutil.OperationResultCreated
		err = r.kclient.Create(r.context, obj)
	}
	return
}
