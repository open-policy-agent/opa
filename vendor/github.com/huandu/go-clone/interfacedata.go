package clone

import (
	"reflect"
	"unsafe"
)

const sizeOfPointers = unsafe.Sizeof((interface{})(0)) / unsafe.Sizeof(uintptr(0))

// interfaceData is the underlying data of an interface.
// As the reflect.Value's interfaceData method is deprecated,
// it may be broken in any Go release.
// It's better to create a custom to hold the data.
//
// The type of interfaceData fields must be poniters.
// It's a way to cheat Go compile to generate calls to write barrier
// when copying interfaces.
type interfaceData struct {
	_ [sizeOfPointers]unsafe.Pointer
}

var reflectValuePtrOffset uintptr

func init() {
	t := reflect.TypeOf(reflect.Value{})
	found := false
	fields := t.NumField()

	for i := 0; i < fields; i++ {
		field := t.Field(i)

		if field.Type.Kind() == reflect.UnsafePointer {
			found = true
			reflectValuePtrOffset = field.Offset
			break
		}
	}

	if !found {
		panic("go-clone: fail to find internal ptr field in reflect.Value")
	}
}

// parseReflectValue returns the underlying interface data in a reflect value.
// It assumes that v is an interface value.
func parseReflectValue(v reflect.Value) interfaceData {
	pv := (unsafe.Pointer)(uintptr(unsafe.Pointer(&v)) + reflectValuePtrOffset)
	ptr := *(*unsafe.Pointer)(pv)
	return *(*interfaceData)(ptr)
}
