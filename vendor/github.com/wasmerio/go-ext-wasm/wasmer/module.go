package wasmer

import (
	"fmt"
	"io/ioutil"
	"unsafe"
)

// ReadBytes reads a `.wasm` file and returns its content as an array of bytes.
func ReadBytes(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

// Validate validates a sequence of bytes that is supposed to represent a valid
// WebAssembly module.
func Validate(bytes []byte) bool {
	return true == cWasmerValidate((*cUchar)(unsafe.Pointer(&bytes[0])), cUint(len(bytes)))
}

// ModuleError represents any kind of errors related to a WebAssembly
// module.
type ModuleError struct {
	// Error message.
	message string
}

// NewModuleError constructs a new `ModuleError`.
func NewModuleError(message string) *ModuleError {
	return &ModuleError{message}
}

// `ModuleError` is an actual error. The `Error` function returns the
// error message.
func (error *ModuleError) Error() string {
	return error.message
}

// ExportDescriptor represents an export descriptor of a WebAssembly
// module. It is different of an export of a WebAssembly instance. An
// export descriptor only has a name and a kind/type.
type ExportDescriptor struct {
	// The export name.
	Name string

	// The export kind/type.
	Kind ImportExportKind
}

// ImportExportKind represents an import/export descriptor kind/type.
type ImportExportKind int

const (
	// ImportExportKindFunction represents an import/export descriptor of kind function.
	ImportExportKindFunction = ImportExportKind(cWasmFunction)

	// ImportExportKindGlobal represents an import/export descriptor of kind global.
	ImportExportKindGlobal = ImportExportKind(cWasmGlobal)

	// ImportExportKindMemory represents an import/export descriptor of kind memory.
	ImportExportKindMemory = ImportExportKind(cWasmMemory)

	// ImportExportKindTable represents an import/export descriptor of kind table.
	ImportExportKindTable = ImportExportKind(cWasmTable)
)

// ImportDescriptor represents an import descriptor of a WebAssembly
// module. It is different of an import of a WebAssembly instance. An
// import descriptor only has a name, a namespace, and a kind/type.
type ImportDescriptor struct {
	// The import name.
	Name string

	// The import namespace.
	Namespace string

	// The import kind/type.
	Kind ImportExportKind
}

// Module represents a WebAssembly module.
type Module struct {
	module  *cWasmerModuleT
	Exports []ExportDescriptor
	Imports []ImportDescriptor
}

// Compile compiles a WebAssembly module from bytes.
func Compile(bytes []byte) (Module, error) {
	var module *cWasmerModuleT

	var compileResult = cWasmerCompile(
		&module,
		(*cUchar)(unsafe.Pointer(&bytes[0])),
		cUint(len(bytes)),
	)

	var emptyModule = Module{module: nil}

	if compileResult != cWasmerOk {
		return emptyModule, NewModuleError("Failed to compile the module.")
	}

	var exports = moduleExports(module)
	var imports = moduleImports(module)

	return Module{module, exports, imports}, nil
}

func moduleExports(module *cWasmerModuleT) []ExportDescriptor {
	var exportDescriptors *cWasmerExportDescriptorsT
	cWasmerExportDescriptors(module, &exportDescriptors)
	defer cWasmerExportDescriptorsDestroy(exportDescriptors)

	var numberOfExportDescriptors = int(cWasmerExportDescriptorsLen(exportDescriptors))
	var exports = make([]ExportDescriptor, numberOfExportDescriptors)

	for nth := 0; nth < numberOfExportDescriptors; nth++ {
		var exportDescriptor = cWasmerExportDescriptorsGet(exportDescriptors, cInt(nth))
		var exportKind = cWasmerExportDescriptorKind(exportDescriptor)
		var wasmExportName = cWasmerExportDescriptorName(exportDescriptor)
		var exportName = cGoStringN((*cChar)(unsafe.Pointer(wasmExportName.bytes)), (cInt)(wasmExportName.bytes_len))

		exports[nth] = ExportDescriptor{
			Name: exportName,
			Kind: ImportExportKind(exportKind),
		}
	}

	return exports
}

func moduleImports(module *cWasmerModuleT) []ImportDescriptor {
	var importDescriptors *cWasmerImportDescriptorsT
	cWasmerImportDescriptors(module, &importDescriptors)
	defer cWasmerImportDescriptorsDestroy(importDescriptors)

	var numberOfImportDescriptors = int(cWasmerImportDescriptorsLen(importDescriptors))
	var imports = make([]ImportDescriptor, numberOfImportDescriptors)

	for nth := 0; nth < numberOfImportDescriptors; nth++ {
		var importDescriptor = cWasmerImportDescriptorsGet(importDescriptors, cInt(nth))
		var importKind = cWasmerImportDescriptorKind(importDescriptor)
		var wasmImportName = cWasmerImportDescriptorName(importDescriptor)
		var importName = cGoStringN((*cChar)(unsafe.Pointer(wasmImportName.bytes)), (cInt)(wasmImportName.bytes_len))
		var wasmImportNamespace = cWasmerImportDescriptorModuleName(importDescriptor)
		var importNamespace = cGoStringN((*cChar)(unsafe.Pointer(wasmImportNamespace.bytes)), (cInt)(wasmImportNamespace.bytes_len))

		imports[nth] = ImportDescriptor{
			Name:      importName,
			Namespace: importNamespace,
			Kind:      ImportExportKind(importKind),
		}
	}

	return imports
}

// Instantiate creates a new instance of the WebAssembly module.
func (module *Module) Instantiate() (Instance, error) {
	return module.InstantiateWithImports(NewImports())
}

// InstantiateWithImports creates a new instance with imports of the WebAssembly module.
func (module *Module) InstantiateWithImports(imports *Imports) (Instance, error) {
	return newInstanceWithImports(
		imports,
		func(wasmImportsCPointer *cWasmerImportT, numberOfImports int) (*cWasmerInstanceT, error) {
			var instance *cWasmerInstanceT

			var instantiateResult = cWasmerModuleInstantiate(
				module.module,
				&instance,
				wasmImportsCPointer,
				cInt(numberOfImports),
			)

			if instantiateResult != cWasmerOk {
				var lastError, err = GetLastError()
				var errorMessage = "Failed to instantiate the module:\n    %s"

				if err != nil {
					errorMessage = fmt.Sprintf(errorMessage, "(unknown details)")
				} else {
					errorMessage = fmt.Sprintf(errorMessage, lastError)
				}

				return nil, NewModuleError(errorMessage)
			}

			return instance, nil
		},
	)
}

// InstantiateWithImportObject creates a new instance of a WebAssembly module with an
// `ImportObject`
func (module *Module) InstantiateWithImportObject(importObject *ImportObject) (Instance, error) {
	var instance *cWasmerInstanceT
	var emptyInstance = Instance{instance: nil, imports: nil, Exports: nil, Memory: nil}

	var instantiateResult = cWasmerModuleImportInstantiate(&instance, module.module, importObject.inner)

	if instantiateResult != cWasmerOk {
		var lastError, err = GetLastError()
		var errorMessage = "Failed to instantiate the module:\n    %s"

		if err != nil {
			errorMessage = fmt.Sprintf(errorMessage, "(unknown details)")
		} else {
			errorMessage = fmt.Sprintf(errorMessage, lastError)
		}

		return emptyInstance, NewModuleError(errorMessage)
	}

	exports, memoryPointer, err := getExportsFromInstance(instance)

	if err != nil {
		return emptyInstance, err
	}

	imports, err := importObject.Imports()

	if err != nil {
		return emptyInstance, NewModuleError(fmt.Sprintf("Could not get imports from ImportObject: %s", err))
	}

	return Instance{instance: instance, imports: imports, Exports: exports, Memory: memoryPointer}, nil
}

// Serialize serializes the current module into a sequence of
// bytes. Those bytes can be deserialized into a module with
// `DeserializeModule`.
func (module *Module) Serialize() ([]byte, error) {
	var serializedModule *cWasmerSerializedModuleT
	var serializeResult = cWasmerModuleSerialize(&serializedModule, module.module)
	defer cWasmerSerializedModuleDestroy(serializedModule)

	if serializeResult != cWasmerOk {
		return nil, NewModuleError("Failed to serialize the module.")
	}

	return cWasmerSerializedModuleBytes(serializedModule), nil
}

// DeserializeModule deserializes a sequence of bytes into a
// module. Ideally, those bytes must come from `Module.Serialize`.
func DeserializeModule(serializedModuleBytes []byte) (Module, error) {
	var emptyModule = Module{module: nil}

	if len(serializedModuleBytes) < 1 {
		return emptyModule, NewModuleError("Serialized module bytes are empty.")
	}

	var serializedModule *cWasmerSerializedModuleT
	var deserializeBytesResult = cWasmerSerializedModuleFromBytes(
		&serializedModule,
		(*cUint8T)(unsafe.Pointer(&serializedModuleBytes[0])),
		cInt(len(serializedModuleBytes)),
	)
	defer cWasmerSerializedModuleDestroy(serializedModule)

	if deserializeBytesResult != cWasmerOk {
		return emptyModule, NewModuleError("Failed to reconstitute the serialized module from the given bytes.")
	}

	var module *cWasmerModuleT
	var deserializeResult = cWasmerModuleDeserialize(&module, serializedModule)

	if deserializeResult != cWasmerOk {
		return emptyModule, NewModuleError("Failed to deserialize the module.")
	}

	var exports = moduleExports(module)
	var imports = moduleImports(module)

	return Module{module, exports, imports}, nil
}

// Close closes/frees a `Module`.
func (module *Module) Close() {
	if module.module != nil {
		cWasmerModuleDestroy(module.module)
	}
}
