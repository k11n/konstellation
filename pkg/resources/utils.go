package resources

import (
	"fmt"
	"reflect"
	"time"

	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
)

const (
	dateTimeFormat = "20060102-1504"
)

var (
	transformers = mergeTransformers{}
)

func NodepoolName() string {
	return fmt.Sprintf("%s-%s", NODEPOOL_PREFIX, time.Now().Format(dateTimeFormat))
}

func GetPodNames(pods []corev1.Pod) []string {
	podNames := make([]string, 0, len(pods))
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// fill in non-empty fields from src to dest
func MergeObject(dest, src interface{}) {
	err := mergo.Merge(dest, src, mergo.WithTransformers(transformers))
	if err != nil {
		fmt.Printf("error merging: %v\n", err)
	}
}

func MergeSlice(dest, src interface{}) {
	destVal := reflect.ValueOf(dest)
	srcVal := reflect.ValueOf(src)

	if destVal.Kind() != srcVal.Kind() {
		return
	}

	if destVal.Kind() != reflect.Ptr && destVal.Kind() != reflect.Interface {
		fmt.Printf("not an interface or pointer")
		return
	}

	if destVal.Elem().Kind() != srcVal.Elem().Kind() || destVal.Elem().Kind() != reflect.Slice {
		return
	}

	mergeSliceValue(destVal.Elem(), srcVal.Elem())
}

func mergeSliceValue(dst, src reflect.Value) error {
	if src.Len() != dst.Len() {
		// override it entirely
		dst.Set(src)
		return nil
	}
	// equal length.. so we'll merge underlying types
	for i := 0; i < src.Len(); i++ {
		srcVal := src.Index(i)
		dstVal := dst.Index(i)
		switch srcVal.Kind() {
		case reflect.Struct:
			dstPtr := reflect.New(dstVal.Type())
			dstPtr.Elem().Set(dstVal)
			// fmt.Printf("merging %v into %v\n", srcVal.Interface(), dstPtr.Interface())
			MergeObject(dstPtr.Interface(), srcVal.Interface())
			dstVal.Set(dstPtr.Elem())
			// fmt.Printf("merged: %v\n", dstVal)
		}
	}
	return nil
}

type mergeTransformers struct {
}

func (t mergeTransformers) Transformer(oType reflect.Type) func(dst, src reflect.Value) error {
	if oType.Kind() != reflect.Slice {
		return nil
	}
	return mergeSliceValue
}
