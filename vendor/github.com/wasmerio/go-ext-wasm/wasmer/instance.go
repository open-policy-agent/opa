package wasmer

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// InstanceError represents any kind of errors related to a WebAssembly instance. It
// is returned by `Instance` functions only.
type InstanceError struct {
	// Error message.
	message string
}

// NewInstanceError constructs a new `InstanceError`.
func NewInstanceError(message string) *InstanceError {
	return &InstanceError{message}
}

// `InstanceError` is an actual error. The `Error` function returns
// the error message.
func (error *InstanceError) Error() string {
	return error.message
}

// ExportedFunctionError represents any kind of errors related to a
// WebAssembly exported function. It is returned by `Instance`
// functions only.
type ExportedFunctionError struct {
	functionName string
	message      string
}

// NewExportedFunctionError constructs a new `ExportedFunctionError`,
// where `functionName` is the name of the exported function, and
// `message` is the error message. If the error message contains `%s`,
// then this parameter will be replaced by `functionName`.
func NewExportedFunctionError(functionName string, message string) *ExportedFunctionError {
	return &ExportedFunctionError{functionName, message}
}

// ExportedFunctionError is an actual error. The `Error` function
// returns the error message.
func (error *ExportedFunctionError) Error() string {
	return fmt.Sprintf(error.message, error.functionName)
}

// Instance represents a WebAssembly instance.
type Instance struct {
	// The underlying WebAssembly instance.
	instance *cWasmerInstanceT

	// The imported functions and memories. Use the
	// `NewInstanceWithImports` constructor to set it.
	imports *Imports

	// All functions exported by the WebAssembly instance, indexed
	// by their name as a string. An exported function is a
	// regular variadic Go closure. Arguments are untyped. Since
	// WebAssembly only supports: `i32`, `i64`, `f32` and `f64`
	// types, the accepted Go types are: `int8`, `uint8`, `int16`,
	// `uint16`, `int32`, `uint32`, `int64`, `int`, `uint`, `float32`
	// and `float64`. In addition to those types, the `Value` type
	// (from this project) is accepted. The conversion from a Go
	// value to a WebAssembly value is done automatically except for
	// the `Value` type (where type is coerced, that's the intent
	// here). The WebAssembly type is automatically inferred. Note
	// that the returned value is of kind `Value`, and not a
	// standard Go type.
	Exports map[string]func(...interface{}) (Value, error)

	// The exported memory of a WebAssembly instance.
	Memory *Memory

	contextDataIndex *int
}

// NewInstance constructs a new `Instance` with no imports.
func NewInstance(bytes []byte) (Instance, error) {
	return NewInstanceWithImports(bytes, NewImports())
}

// NewInstanceWithImports constructs a new `Instance` with imports.
func NewInstanceWithImports(bytes []byte, imports *Imports) (Instance, error) {
	return newInstanceWithImports(
		imports,
		func(wasmImportsCPointer *cWasmerImportT, numberOfImports int) (*cWasmerInstanceT, error) {
			var instance *cWasmerInstanceT

			var compileResult = cWasmerInstantiate(
				&instance,
				(*cUchar)(unsafe.Pointer(&bytes[0])),
				cUint(len(bytes)),
				wasmImportsCPointer,
				cInt(numberOfImports),
			)

			if compileResult != cWasmerOk {
				var lastError, err = GetLastError()
				var errorMessage = "Failed to instantiate the module:\n    %s"

				if err != nil {
					errorMessage = fmt.Sprintf(errorMessage, "(unknown details)")
				} else {
					errorMessage = fmt.Sprintf(errorMessage, lastError)
				}

				return nil, NewInstanceError(errorMessage)
			}

			return instance, nil
		},
	)
}

func newInstanceWithImports(
	imports *Imports,
	instanceBuilder func(*cWasmerImportT, int) (*cWasmerInstanceT, error),
) (Instance, error) {
	var numberOfImports = len(imports.imports)
	var wasmImports = make([]cWasmerImportT, numberOfImports)
	var importNth = 0

	for importName, importImport := range imports.imports {
		wasmImports[importNth] = *getCWasmerImport(importName, importImport)
		importNth++
	}

	var wasmImportsCPointer *cWasmerImportT

	if numberOfImports > 0 {
		wasmImportsCPointer = (*cWasmerImportT)(unsafe.Pointer(&wasmImports[0]))
	}

	instance, err := instanceBuilder(wasmImportsCPointer, numberOfImports)

	var emptyInstance = Instance{instance: nil, imports: nil, Exports: nil, Memory: nil}

	if err != nil {
		return emptyInstance, err
	}

	exports, memoryPointer, err := getExportsFromInstance(instance)

	if err != nil {
		return emptyInstance, err
	}

	return Instance{instance: instance, imports: imports, Exports: exports, Memory: memoryPointer}, nil
}

// Returns the exports, whether it has memory or an error
func getExportsFromInstance(
	instance *cWasmerInstanceT,
) (
	map[string]func(...interface{}) (Value, error),
	*Memory,
	error,
) {
	var exports = make(map[string]func(...interface{}) (Value, error))
	var wasmExports *cWasmerExportsT
	var memoryPointer *Memory
	cWasmerInstanceExports(instance, &wasmExports)
	defer cWasmerExportsDestroy(wasmExports)

	var numberOfExports = int(cWasmerExportsLen(wasmExports))

	for nth := 0; nth < numberOfExports; nth++ {
		var wasmExport = cWasmerExportsGet(wasmExports, cInt(nth))
		var wasmExportKind = cWasmerExportKind(wasmExport)

		switch wasmExportKind {
		case cWasmMemory:
			var wasmMemory *cWasmerMemoryT

			if cWasmerExportToMemory(wasmExport, &wasmMemory) != cWasmerOk {
				return nil, nil, NewInstanceError("Failed to extract the exported memory.")
			}

			var memory = newBorrowedMemory(wasmMemory)
			memoryPointer = &memory

		case cWasmFunction:
			var wasmExportName = cWasmerExportName(wasmExport)
			var exportedFunctionName = cGoStringN((*cChar)(unsafe.Pointer(wasmExportName.bytes)), (cInt)(wasmExportName.bytes_len))
			var wasmFunction = cWasmerExportToFunc(wasmExport)
			var wasmFunctionInputsArity cUint32T

			if cWasmerExportFuncParamsArity(wasmFunction, &wasmFunctionInputsArity) != cWasmerOk {
				return nil, nil, NewExportedFunctionError(exportedFunctionName, "Failed to read the input arity of the `%s` exported function.")
			}

			var wasmFunctionInputSignatures = make([]cWasmerValueTag, int(wasmFunctionInputsArity))

			if wasmFunctionInputsArity > 0 {
				var wasmFunctionInputSignaturesCPointer = (*cWasmerValueTag)(unsafe.Pointer(&wasmFunctionInputSignatures[0]))

				if cWasmerExportFuncParams(wasmFunction, wasmFunctionInputSignaturesCPointer, wasmFunctionInputsArity) != cWasmerOk {
					return nil, nil, NewExportedFunctionError(exportedFunctionName, "Failed to read the signature of the `%s` exported function.")
				}
			}

			var wasmFunctionOutputsArity cUint32T

			if cWasmerExportFuncResultsArity(wasmFunction, &wasmFunctionOutputsArity) != cWasmerOk {
				return nil, nil, NewExportedFunctionError(exportedFunctionName, "Failed to read the output arity of the `%s` exported function.")
			}

			var numberOfExpectedArguments = int(wasmFunctionInputsArity)

			var wasmInputs = make([]cWasmerValueT, wasmFunctionInputsArity)
			var wasmOutputs = make([]cWasmerValueT, wasmFunctionOutputsArity)

			type wasmFunctionNameHolder struct {
				CPointer *cChar
			}

			wasmFunctionName := &wasmFunctionNameHolder{
				CPointer: cCString(exportedFunctionName),
			}

			runtime.SetFinalizer(wasmFunctionName, func(h *wasmFunctionNameHolder) {
				cFree(unsafe.Pointer(h.CPointer))
			})

			exports[exportedFunctionName] = func(arguments ...interface{}) (Value, error) {
				var numberOfGivenArguments = len(arguments)
				var diff = numberOfExpectedArguments - numberOfGivenArguments

				if diff > 0 {
					return I32(0), NewExportedFunctionError(exportedFunctionName, fmt.Sprintf("Missing %d argument(s) when calling the `%%s` exported function; Expect %d argument(s), given %d.", diff, numberOfExpectedArguments, numberOfGivenArguments))
				} else if diff < 0 {
					return I32(0), NewExportedFunctionError(exportedFunctionName, fmt.Sprintf("Given %d extra argument(s) when calling the `%%s` exported function; Expect %d argument(s), given %d.", -diff, numberOfExpectedArguments, numberOfGivenArguments))
				}

				for nth, value := range arguments {
					var wasmInputType = wasmFunctionInputSignatures[nth]

					switch wasmInputType {
					case cWasmI32:
						wasmInputs[nth].tag = cWasmI32
						var pointer = (*int32)(unsafe.Pointer(&wasmInputs[nth].value))

						switch value.(type) {
						case int8:
							*pointer = int32(value.(int8))
						case uint8:
							*pointer = int32(value.(uint8))
						case int16:
							*pointer = int32(value.(int16))
						case uint16:
							*pointer = int32(value.(uint16))
						case int32:
							*pointer = int32(value.(int32))
						case int:
							*pointer = int32(value.(int))
						case uint:
							*pointer = int32(value.(uint))
						case Value:
							var value = value.(Value)

							if value.GetType() != TypeI32 {
								return I32(0), NewExportedFunctionError(exportedFunctionName, fmt.Sprintf("Argument #%d of the `%%s` exported function must be of type `i32`, cannot cast given value to this type.", nth+1))
							}

							*pointer = value.ToI32()
						default:
							return I32(0), NewExportedFunctionError(exportedFunctionName, fmt.Sprintf("Argument #%d of the `%%s` exported function must be of type `i32`, cannot cast given value to this type.", nth+1))
						}
					case cWasmI64:
						wasmInputs[nth].tag = cWasmI64
						var pointer = (*int64)(unsafe.Pointer(&wasmInputs[nth].value))

						switch value.(type) {
						case int8:
							*pointer = int64(value.(int8))
						case uint8:
							*pointer = int64(value.(uint8))
						case int16:
							*pointer = int64(value.(int16))
						case uint16:
							*pointer = int64(value.(uint16))
						case int32:
							*pointer = int64(value.(int32))
						case uint32:
							*pointer = int64(value.(uint32))
						case int64:
							*pointer = int64(value.(int64))
						case int:
							*pointer = int64(value.(int))
						case uint:
							*pointer = int64(value.(uint))
						case Value:
							var value = value.(Value)

							if value.GetType() != TypeI64 {
								return I32(0), NewExportedFunctionError(exportedFunctionName, fmt.Sprintf("Argument #%d of the `%%s` exported function must be of type `i64`, cannot cast given value to this type.", nth+1))
							}

							*pointer = value.ToI64()
						default:
							return I32(0), NewExportedFunctionError(exportedFunctionName, fmt.Sprintf("Argument #%d of the `%%s` exported function must be of type `i64`, cannot cast given value to this type.", nth+1))
						}
					case cWasmF32:
						wasmInputs[nth].tag = cWasmF32
						var pointer = (*float32)(unsafe.Pointer(&wasmInputs[nth].value))

						switch value.(type) {
						case float32:
							*pointer = value.(float32)
						case Value:
							var value = value.(Value)

							if value.GetType() != TypeF32 {
								return I32(0), NewExportedFunctionError(exportedFunctionName, fmt.Sprintf("Argument #%d of the `%%s` exported function must be of type `f32`, cannot cast given value to this type.", nth+1))
							}

							*pointer = value.ToF32()
						default:
							return I32(0), NewExportedFunctionError(exportedFunctionName, fmt.Sprintf("Argument #%d of the `%%s` exported function must be of type `f32`, cannot cast given value to this type.", nth+1))
						}
					case cWasmF64:
						wasmInputs[nth].tag = cWasmF64
						var pointer = (*float64)(unsafe.Pointer(&wasmInputs[nth].value))

						switch value.(type) {
						case float32:
							*pointer = float64(value.(float32))
						case float64:
							*pointer = value.(float64)
						case Value:
							var value = value.(Value)

							if value.GetType() != TypeF64 {
								return I32(0), NewExportedFunctionError(exportedFunctionName, fmt.Sprintf("Argument #%d of the `%%s` exported function must be of type `f64`, cannot cast given value to this type.", nth+1))
							}

							*pointer = value.ToF64()
						default:
							return I32(0), NewExportedFunctionError(exportedFunctionName, fmt.Sprintf("Argument #%d of the `%%s` exported function must be of type `f64`, cannot cast given value to this type.", nth+1))
						}
					default:
						panic(fmt.Sprintf("Invalid arguments type when calling the `%s` exported function.", exportedFunctionName))
					}
				}

				var wasmInputsCPointer *cWasmerValueT

				if wasmFunctionInputsArity > 0 {
					wasmInputsCPointer = (*cWasmerValueT)(unsafe.Pointer(&wasmInputs[0]))
				} else {
					wasmInputsCPointer = (*cWasmerValueT)(unsafe.Pointer(&wasmInputs))
				}

				var wasmOutputsCPointer *cWasmerValueT

				if wasmFunctionOutputsArity > 0 {
					wasmOutputsCPointer = (*cWasmerValueT)(unsafe.Pointer(&wasmOutputs[0]))
				} else {
					wasmOutputsCPointer = (*cWasmerValueT)(unsafe.Pointer(&wasmOutputs))
				}

				var callResult = cWasmerInstanceCall(
					instance,
					wasmFunctionName.CPointer,
					wasmInputsCPointer,
					wasmFunctionInputsArity,
					wasmOutputsCPointer,
					wasmFunctionOutputsArity,
				)

				if callResult != cWasmerOk {
					return I32(0), NewExportedFunctionError(exportedFunctionName, "Failed to call the `%s` exported function.")
				}

				if wasmFunctionOutputsArity > 0 {
					var result = wasmOutputs[0]

					switch result.tag {
					case cWasmI32:
						pointer := (*int32)(unsafe.Pointer(&result.value))

						return I32(*pointer), nil
					case cWasmI64:
						pointer := (*int64)(unsafe.Pointer(&result.value))

						return I64(*pointer), nil
					case cWasmF32:
						pointer := (*float32)(unsafe.Pointer(&result.value))

						return F32(*pointer), nil
					case cWasmF64:
						pointer := (*float64)(unsafe.Pointer(&result.value))

						return F64(*pointer), nil
					default:
						panic("unreachable")
					}
				} else {
					return void(), nil
				}
			}
		}
	}
	return exports, memoryPointer, nil
}

// HasMemory checks whether the instance has at least one exported memory.
func (instance *Instance) HasMemory() bool {
	return nil != instance.Memory
}

var (

	// In order to avoid passing illegal Go pointers across the
	// CGo FFI, store Instance Context Data in instanceContextData
	// and simply pass the index through the FFI instead.
	//
	// See `Instance.SetContextData` and `InstanceContext.Data`.
	instancesContextData      = make(map[int]interface{})
	nextContextDataIndex      int
	instancesContextDataMutex sync.RWMutex
)

// SetContextData assigns a data that can be used by all imported functions.
// Each imported function receives as its first argument an instance context
// (see `InstanceContext`). An instance context can hold any kind of data,
// including data that contain Go references such as slices, maps, or structs
// with reference types or pointers. It is important to understand that data is
// global to the instance, and thus is shared by all imported functions.
func (instance *Instance) SetContextData(data interface{}) {
	instancesContextDataMutex.Lock()

	if instance.contextDataIndex == nil {
		instance.contextDataIndex = new(int)
		*instance.contextDataIndex = nextContextDataIndex
		nextContextDataIndex++

		// When instance is garbage-collected, clean up its
		// `instanceContextData`.  Set the finalizer on the
		// unexported `instance.contextDataIndex`, instead of
		// directly on the instance, to allow users of this
		// package to set their own finalizer on the `Instance`
		// for other reasons.
		runtime.SetFinalizer(instance.contextDataIndex, func(index *int) {
			// Launch a goroutine to avoid blocking other
			// finalizers while waiting for the mutex lock.
			go func() {
				instancesContextDataMutex.Lock()
				delete(instancesContextData, *index)
				instancesContextDataMutex.Unlock()
			}()
		})
	}

	instancesContextData[*instance.contextDataIndex] = data
	instancesContextDataMutex.Unlock()

	cWasmerInstanceContextDataSet(
		instance.instance,
		unsafe.Pointer(instance.contextDataIndex),
	)
}

// Close closes/frees an `Instance`.
func (instance *Instance) Close() {
	if instance.imports != nil {
		instance.imports.Close()
	}

	if instance.instance != nil {
		cWasmerInstanceDestroy(instance.instance)
	}
}
