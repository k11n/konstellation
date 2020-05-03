package resources

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func UpdateResource(kclient client.Client, object, owner metav1.Object, scheme *runtime.Scheme) (result controllerutil.OperationResult, err error) {
	return updateResource(kclient, object, owner, scheme, false)
}

func UpdateResourceWithMerge(kclient client.Client, object, owner metav1.Object, scheme *runtime.Scheme) (result controllerutil.OperationResult, err error) {
	return updateResource(kclient, object, owner, scheme, true)
}

func updateResource(kclient client.Client, object, owner metav1.Object, scheme *runtime.Scheme, merge bool) (result controllerutil.OperationResult, err error) {
	newVal := reflect.New(reflect.TypeOf(object).Elem())
	newObj := newVal.Interface().(metav1.Object)
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
		newSpec := newVal.Elem().FieldByName("Spec")
		existingSpec := reflect.ValueOf(object).Elem().FieldByName("Spec")

		if merge {
			objects.MergeObject(newSpec.Addr().Interface(), existingSpec.Addr().Interface())
		} else {
			newSpec.Set(existingSpec)
		}

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

/**
 * Helper function to iterate through all resources in a list
 */
func ForEach(kclient client.Client, listObj runtime.Object, eachFunc func(item interface{}) error, opts ...client.ListOption) error {
	shouldRun := true
	var contToken string
	ctx := context.Background()
	for shouldRun {
		shouldRun = false

		options := append(opts, client.Limit(20))
		if contToken != "" {
			options = append(options, client.Continue(contToken))
		}
		err := kclient.List(ctx, listObj, opts...)
		if err != nil {
			return err
		}

		listVal := reflect.ValueOf(listObj).Elem()
		itemsField := listVal.FieldByName("Items")
		if itemsField.IsZero() {
			return fmt.Errorf("List object doesn't not contain Items field")
		}

		for i := 0; i < itemsField.Len(); i += 1 {
			item := itemsField.Index(i).Interface()
			if err = eachFunc(item); err != nil {
				return err
			}
		}

		// find contToken
		listMeta := listVal.FieldByName("ListMeta")
		lm := listMeta.Interface().(metav1.ListMeta)
		contToken = lm.Continue
		if contToken != "" {
			shouldRun = true
		}
	}
	return nil
}
