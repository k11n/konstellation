package resources

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/davidzhao/konstellation/pkg/utils/objects"
)

const (
	dateTimeFormat  = "20060102-1504"
	defaultListSize = 10
)

var (
	ErrNotFound = fmt.Errorf("The resource is not found")
	log         = logf.Log.WithName("resources")
)

func UpdateResource(kclient client.Client, object, owner metav1.Object, scheme *runtime.Scheme) (result controllerutil.OperationResult, err error) {
	return updateResource(kclient, object, owner, scheme, false)
}

func UpdateResourceWithMerge(kclient client.Client, object, owner metav1.Object, scheme *runtime.Scheme) (result controllerutil.OperationResult, err error) {
	return updateResource(kclient, object, owner, scheme, true)
}

// Create or update the resource
// only handles updates to Annotations, Labels, and Spec
func updateResource(kclient client.Client, object, owner metav1.Object, scheme *runtime.Scheme, merge bool) (controllerutil.OperationResult, error) {
	existingVal := reflect.New(reflect.TypeOf(object).Elem())
	existingObj := existingVal.Interface().(metav1.Object)
	existingObj.SetNamespace(object.GetNamespace())
	existingObj.SetName(object.GetName())
	existingRuntimeObj, ok := existingObj.(runtime.Object)
	if !ok {
		return controllerutil.OperationResultNone, fmt.Errorf("Not a runtime Object")
	}

	key, err := client.ObjectKeyFromObject(existingRuntimeObj)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	// create new if existing is not found
	if err := kclient.Get(context.TODO(), key, existingRuntimeObj); err != nil {
		if !errors.IsNotFound(err) {
			return controllerutil.OperationResultNone, err
		}
		if owner != nil && scheme != nil {
			if err = controllerutil.SetControllerReference(owner, object, scheme); err != nil {
				return controllerutil.OperationResultNone, err
			}
		}
		if err := kclient.Create(context.TODO(), object.(runtime.Object)); err != nil {
			return controllerutil.OperationResultNone, err
		}
		return controllerutil.OperationResultCreated, nil
	}

	changed := false

	if !apiequality.Semantic.DeepEqual(existingObj.GetAnnotations(), object.GetAnnotations()) {
		existingObj.SetAnnotations(object.GetAnnotations())
		changed = true
	}
	if !apiequality.Semantic.DeepEqual(existingObj.GetLabels(), object.GetLabels()) {
		existingObj.SetLabels(object.GetLabels())
		changed = true
	}

	// deep copy spec so we can apply and detect changes
	// particularly with using merge, it's difficult to know what's changed, so we'd have to apply
	// updates and confirm
	existingCopy := existingRuntimeObj.DeepCopyObject()

	existingSpec := existingVal.Elem().FieldByName("Spec")
	targetSpec := reflect.ValueOf(object).Elem().FieldByName("Spec")
	if merge {
		objects.MergeObject(existingSpec.Addr().Interface(), targetSpec.Addr().Interface())
	} else {
		existingSpec.Set(targetSpec)
	}
	copiedSpec := reflect.ValueOf(existingCopy).Elem().FieldByName("Spec")
	if !apiequality.Semantic.DeepEqual(existingSpec.Addr().Interface(), copiedSpec.Addr().Interface()) {
		changed = true
	}

	if !changed {
		return controllerutil.OperationResultNone, nil
	}

	if err := kclient.Update(context.TODO(), existingRuntimeObj); err != nil {
		return controllerutil.OperationResultNone, err
	}

	// use existing since its status is set
	reflect.ValueOf(object).Elem().Set(existingVal.Elem())
	return controllerutil.OperationResultUpdated, nil
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
