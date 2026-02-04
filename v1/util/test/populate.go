// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// PopulateAllFields uses reflection to populate all fields of a struct with test data.
// This is useful for testing code that must handle all fields but when new fields
// might be added and missed. It must be possible to set all fields, or the helper
// will fail until the fields are supported.
// Caveats: only supports types needed at time of implementation, will not work
// on recursive structs.
func PopulateAllFields[T any](t *testing.T) *T {
	t.Helper()

	var instance T
	instancePtr := &instance
	instanceType := reflect.TypeOf(instance)
	instanceValue := reflect.ValueOf(instancePtr).Elem()

	populateStruct(t, instanceType, instanceValue)

	return instancePtr
}

func populateStruct(t *testing.T, structType reflect.Type, structValue reflect.Value) {
	t.Helper()

	for i := range structType.NumField() {
		field := structType.Field(i)
		fieldValue := structValue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		if !populateDefaultTypes(t, field.Type, fieldValue, i) {
			t.Fatalf("Unknown field type %s for field %s - update PopulateAllFields()", field.Type, field.Name)
		}
	}
}

func populateDefaultTypes(t *testing.T, fieldType reflect.Type, fieldValue reflect.Value, index int) bool {
	t.Helper()

	switch fieldType.Kind() {
	case reflect.Slice:
		if fieldType == reflect.TypeOf(json.RawMessage{}) {
			fieldValue.Set(reflect.ValueOf(fmt.Appendf(nil, `{"test": "bar-%d"}`, index)))
			return true
		}

	case reflect.Ptr:
		switch fieldType.Elem().Kind() {
		case reflect.String:
			testString := fmt.Sprintf("test-value-%d", index)
			fieldValue.Set(reflect.ValueOf(&testString))

			return true
		case reflect.Int:
			testInt := 100 + index // unique value per field
			fieldValue.Set(reflect.ValueOf(&testInt))

			return true
		case reflect.Int64:
			testInt64 := int64(200 + index) // unique value per field
			fieldValue.Set(reflect.ValueOf(&testInt64))

			return true
		case reflect.Struct:
			newStruct := reflect.New(fieldType.Elem())
			populateStruct(t, fieldType.Elem(), newStruct.Elem())
			fieldValue.Set(newStruct)

			return true
		case reflect.Bool:
			newBool := true
			fieldValue.Set(reflect.ValueOf(&newBool))

			return true
		}

	case reflect.Bool:
		fieldValue.SetBool(true)

		return true

	case reflect.Struct:
		populateStruct(t, fieldType, fieldValue)

		return true

	case reflect.Map:
		switch {
		case fieldType.Key().Kind() == reflect.String && fieldType.Elem().Kind() == reflect.String:
			fieldValue.Set(reflect.ValueOf(map[string]string{
				"env":     fmt.Sprintf("test-%d", index),
				"version": fmt.Sprintf("1.%d", index),
			}))

			return true

		case fieldType.Key().Kind() == reflect.String &&
			fieldType.Elem() == reflect.TypeOf(json.RawMessage{}):

			fieldValue.Set(reflect.ValueOf(map[string]json.RawMessage{
				"key1": fmt.Appendf(nil, `{"test": "bar-%d"}`, index),
				"key2": fmt.Appendf(nil, `{"foo": "baz-%d"}`, index),
			}))

			return true

		case fieldType.Key().Kind() == reflect.String &&
			fieldType.Elem().Kind() == reflect.Ptr &&
			fieldType.Elem().Elem().Kind() == reflect.Struct:

			elemType := fieldType.Elem().Elem()

			mapVal := reflect.MakeMap(fieldType)

			for _, key := range []string{"test1", "test2"} {
				newElem := reflect.New(elemType)

				populateStruct(t, elemType, newElem.Elem())

				mapVal.SetMapIndex(reflect.ValueOf(key), newElem)
			}

			fieldValue.Set(mapVal)

			return true
		}
	}

	return false
}
