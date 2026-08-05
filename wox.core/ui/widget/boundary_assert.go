package widget

import "reflect"

type boundaryAssertionT interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// AssertEqualCoversAllFields verifies every exported props field except callbacks explicitly tagged boundary:"stable".
func AssertEqualCoversAllFields[T BoundaryProps[T]](t boundaryAssertionT, sample T) {
	t.Helper()
	value := reflect.ValueOf(sample)
	if value.Kind() != reflect.Struct {
		t.Fatalf("Boundary props %T must be a struct", sample)
		return
	}
	propsType := value.Type()
	for index := 0; index < value.NumField(); index++ {
		fieldType := propsType.Field(index)
		if fieldType.Tag.Get("boundary") == "stable" {
			continue
		}
		if !fieldType.IsExported() {
			t.Fatalf("Boundary props %T field %s must be exported", sample, fieldType.Name)
			return
		}
		candidate := reflect.New(propsType).Elem()
		candidate.Set(value)
		if !mutateBoundaryTestValue(candidate.Field(index)) {
			t.Fatalf("Boundary props %T field %s cannot be varied; provide a non-zero sample value", sample, fieldType.Name)
			return
		}
		changed := candidate.Interface().(T)
		if sample.Equal(changed) || changed.Equal(sample) {
			t.Errorf("Boundary props %T Equal does not cover field %s", sample, fieldType.Name)
		}
	}
}

func mutateBoundaryTestValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Bool:
		value.SetBool(!value.Bool())
	case reflect.String:
		value.SetString(value.String() + "__boundary_changed__")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(value.Int() + 1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		value.SetUint(value.Uint() + 1)
	case reflect.Float32, reflect.Float64:
		value.SetFloat(value.Float() + 1)
	case reflect.Complex64, reflect.Complex128:
		value.SetComplex(value.Complex() + 1)
	case reflect.Pointer:
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		} else {
			value.SetZero()
		}
	case reflect.Interface, reflect.Func, reflect.Chan:
		if !value.IsNil() {
			value.SetZero()
		} else if value.Kind() == reflect.Func {
			value.Set(reflect.MakeFunc(value.Type(), func([]reflect.Value) []reflect.Value {
				results := make([]reflect.Value, value.Type().NumOut())
				for index := range results {
					results[index] = reflect.Zero(value.Type().Out(index))
				}
				return results
			}))
		} else if value.Kind() == reflect.Chan {
			value.Set(reflect.MakeChan(value.Type(), 0))
		} else {
			return false
		}
	case reflect.Slice:
		changed := reflect.MakeSlice(value.Type(), value.Len()+1, value.Len()+1)
		reflect.Copy(changed, value)
		value.Set(changed)
	case reflect.Map:
		if value.IsNil() {
			changed := reflect.MakeMap(value.Type())
			changed.SetMapIndex(reflect.Zero(value.Type().Key()), reflect.Zero(value.Type().Elem()))
			value.Set(changed)
		} else {
			value.SetZero()
		}
	case reflect.Array:
		for index := 0; index < value.Len(); index++ {
			if mutateBoundaryTestValue(value.Index(index)) {
				return true
			}
		}
		return false
	case reflect.Struct:
		for index := 0; index < value.NumField(); index++ {
			if value.Field(index).CanSet() && mutateBoundaryTestValue(value.Field(index)) {
				return true
			}
		}
		return false
	default:
		return false
	}
	return true
}
