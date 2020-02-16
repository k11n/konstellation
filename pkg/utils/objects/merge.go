package objects

import (
	"fmt"
	"reflect"

	"github.com/imdario/mergo"
)

var (
	transformers = mergeTransformers{}
)

// fill in non-empty fields from src to dest
func MergeObject(dest, src interface{}) {
	err := mergo.MergeWithOverwrite(dest, src, mergo.WithTransformers(transformers))
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
			MergeObject(dstPtr.Interface(), srcVal.Interface())
			dstVal.Set(dstPtr.Elem())
		}
	}
	return nil
}

type mergeTransformers struct {
}

func (t mergeTransformers) Transformer(oType reflect.Type) func(dst, src reflect.Value) error {
	switch oType.Kind() {
	case reflect.Slice:
		return mergeSliceValue
	default:
		return nil
	}
}
