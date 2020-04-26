package resources

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/davidzhao/konstellation/pkg/utils/objects"
)

const (
	dateTimeFormat  = "20060102-1504"
	defaultListSize = 10
)

var (
	ErrNotFound = fmt.Errorf("The resource is not found")
)

func UpdateResource(kclient client.Client, object, owner v1.Object, scheme *runtime.Scheme) (result controllerutil.OperationResult, err error) {
	newVal := reflect.New(reflect.TypeOf(object).Elem())
	newObj := newVal.Interface().(v1.Object)
	newObj.SetNamespace(object.GetNamespace())
	newObj.SetName(object.GetName())
	lookupObj, ok := newObj.(runtime.Object)
	if !ok {
		err = fmt.Errorf("Not an runtime Object")
		return
	}

	result, err = controllerutil.CreateOrUpdate(context.TODO(), kclient, lookupObj, func() error {
		if owner != nil {
			if err := controllerutil.SetControllerReference(owner, newObj, scheme); err != nil {
				return err
			}
		}

		// update labels/annotations
		newObj.SetAnnotations(object.GetAnnotations())
		newObj.SetLabels(object.GetLabels())

		// update spec
		newSpec := newVal.Elem().FieldByName("Spec").Addr().Interface()
		existingSpec := reflect.ValueOf(object).Elem().FieldByName("Spec").Addr().Interface()
		objects.MergeObject(newSpec, existingSpec)

		return nil
	})

	reflect.ValueOf(object).Elem().Set(newVal.Elem())

	return
}

func LogUpdates(log logr.Logger, op controllerutil.OperationResult, message string, keysAndValues ...interface{}) {
	if op == controllerutil.OperationResultNone {
		return
	}
	keysAndValues = append(keysAndValues, "op", op)
	log.Info(message, keysAndValues...)
}
