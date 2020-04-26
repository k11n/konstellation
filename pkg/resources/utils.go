package resources

import (
	"context"
	"fmt"
	"reflect"

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
	lookupObj, ok := newObj.(runtime.Object)
	if !ok {
		err = fmt.Errorf("Not an runtime Object")
		return
	}

	newObj.SetNamespace(object.GetNamespace())
	newObj.SetName(object.GetName())

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

func toStructPtr(val reflect.Value) interface{} {
	// Create a new instance of the underlying type
	vp := reflect.New(val.Type())
	vp.Elem().Set(val)
	return vp.Interface()
}
