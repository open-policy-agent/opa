package wasm

import (
	"context"
	"errors"
)

// Engine is a Store-scoped mechanism to compile functions declared or imported by a module.
// This is a top-level type implemented by an interpreter or compiler.
type Engine interface {
	// CompileModule implements the same method as documented on wasm.Engine.
	CompileModule(ctx context.Context, module *Module) error

	// CompiledModuleCount is exported for testing, to track the size of the compilation cache.
	CompiledModuleCount() uint32

	// DeleteCompiledModule releases compilation caches for the given module (source).
	// Note: it is safe to call this function for a module from which module instances are instantiated even when these
	// module instances have outstanding calls.
	DeleteCompiledModule(module *Module)

	// NewModuleEngine compiles down the function instances in a module, and returns ModuleEngine for the module.
	//
	// * name is the name the module was instantiated with used for error handling.
	// * module is the source module from which moduleFunctions are instantiated. This is used for caching.
	// * importedFunctions: functions this module imports, already compiled in this engine.
	// * moduleFunctions: functions declared in this module that must be compiled.
	// * tables: possibly shared tables used by this module. When nil tableInit will be nil.
	// * tableInit: a mapping of Table's index to a mapping of TableInstance.Table index to the function index it should point to.
	//
	// Note: Input parameters must be pre-validated with wasm.Module Validate, to ensure no fields are invalid
	// due to reasons such as out-of-bounds.
	NewModuleEngine(
		name string,
		module *Module,
		importedFunctions, moduleFunctions []*FunctionInstance,
		tables []*TableInstance,
		tableInits []TableInitEntry,
	) (ModuleEngine, error)
}

// ModuleEngine implements function calls for a given module.
type ModuleEngine interface {
	// Name returns the name of the module this engine was compiled for.
	Name() string

	// Call invokes a function instance f with given parameters.
	Call(ctx context.Context, m *CallContext, f *FunctionInstance, params ...uint64) (results []uint64, err error)

	// CreateFuncElementInstance creates an ElementInstance whose references are engine-specific function pointers
	// corresponding to the given `indexes`.
	CreateFuncElementInstance(indexes []*Index) *ElementInstance

	// InitializeFuncrefGlobals initializes the globals of Funcref type as the opaque pointer values of engine specific compiled functions.
	InitializeFuncrefGlobals(globals []*GlobalInstance)
}

// TableInitEntry is normalized element segment used for initializing tables by engines.
type TableInitEntry struct {
	TableIndex Index
	// Offset is the offset in the table from which the table is initialized by engine.
	Offset Index
	// FunctionIndexes contains nullable function indexes.
	FunctionIndexes []*Index
}

// ErrElementOffsetOutOfBounds is the error raised when the active element offset exceeds the table length.
// Before FeatureReferenceTypes, this was checked statically before instantiation, after the proposal,
// this must be raised as runtime error (as in assert_trap in spectest), not even an instantiation error.
//
// See https://github.com/WebAssembly/spec/blob/d39195773112a22b245ffbe864bab6d1182ccb06/test/core/linking.wast#L264-L274
var ErrElementOffsetOutOfBounds = errors.New("element offset ouf of bounds")
