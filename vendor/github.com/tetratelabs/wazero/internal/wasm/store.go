package wasm

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/tetratelabs/wazero/api"
	experimentalapi "github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/internal/ieee754"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/sys"
)

type (
	// Store is the runtime representation of "instantiated" Wasm module and objects.
	// Multiple modules can be instantiated within a single store, and each instance,
	// (e.g. function instance) can be referenced by other module instances in a Store via Module.ImportSection.
	//
	// Every type whose name ends with "Instance" suffix belongs to exactly one store.
	//
	// Note that store is not thread (concurrency) safe, meaning that using single Store
	// via multiple goroutines might result in race conditions. In that case, the invocation
	// and access to any methods and field of Store must be guarded by mutex.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#store%E2%91%A0
	Store struct {
		// EnabledFeatures are read-only to allow optimizations.
		EnabledFeatures Features

		// Engine is a global context for a Store which is in responsible for compilation and execution of Wasm modules.
		Engine Engine

		// typeIDs maps each FunctionType.String() to a unique FunctionTypeID. This is used at runtime to
		// do type-checks on indirect function calls.
		typeIDs map[string]FunctionTypeID

		// functionMaxTypes represents the limit on the number of function types in a store.
		// Note: this is fixed to 2^27 but have this a field for testability.
		functionMaxTypes uint32

		// namespaces are all Namespace instances for this store including the default one.
		namespaces []*Namespace // guarded by mux

		// mux is used to guard the fields from concurrent access.
		mux sync.RWMutex
	}

	// ModuleInstance represents instantiated wasm module.
	// The difference from the spec is that in wazero, a ModuleInstance holds pointers
	// to the instances, rather than "addresses" (i.e. index to Store.Functions, Globals, etc) for convenience.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#syntax-moduleinst
	ModuleInstance struct {
		Name      string
		Exports   map[string]*ExportInstance
		Functions []*FunctionInstance
		Globals   []*GlobalInstance
		// Memory is set when Module.MemorySection had a memory, regardless of whether it was exported.
		Memory *MemoryInstance
		Tables []*TableInstance
		Types  []*FunctionType

		// CallCtx holds default function call context from this function instance.
		CallCtx *CallContext

		// Engine implements function calls for this module.
		Engine ModuleEngine

		// TypeIDs is index-correlated with types and holds typeIDs which is uniquely assigned to a type by store.
		// This is necessary to achieve fast runtime type checking for indirect function calls at runtime.
		TypeIDs []FunctionTypeID

		// DataInstances holds data segments bytes of the module.
		// This is only used by bulk memory operations.
		//
		// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/runtime.html#data-instances
		DataInstances []DataInstance

		// ElementInstances holds the element instance, and each holds the references to either functions
		// or external objects (unimplemented).
		ElementInstances []ElementInstance
	}

	// DataInstance holds bytes corresponding to the data segment in a module.
	//
	// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/runtime.html#data-instances
	DataInstance = []byte

	// ExportInstance represents an exported instance in a Store.
	// The difference from the spec is that in wazero, a ExportInstance holds pointers
	// to the instances, rather than "addresses" (i.e. index to Store.Functions, Globals, etc) for convenience.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#syntax-exportinst
	ExportInstance struct {
		Type     ExternType
		Function *FunctionInstance
		Global   *GlobalInstance
		Memory   *MemoryInstance
		Table    *TableInstance
	}

	// FunctionInstance represents a function instance in a Store.
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#function-instances%E2%91%A0
	FunctionInstance struct {
		// DebugName is for debugging purpose, and is used to augment stack traces.
		DebugName string

		// Kind describes how this function should be called.
		Kind FunctionKind

		// Type is the signature of this function.
		Type *FunctionType

		// LocalTypes holds types of locals, set when Kind == FunctionKindWasm
		LocalTypes []ValueType

		// Body is the function body in WebAssembly Binary Format, set when Kind == FunctionKindWasm
		Body []byte

		// GoFunc holds the runtime representation of host functions.
		// This is nil when Kind == FunctionKindWasm. Otherwise, all the above fields are ignored as they are
		// specific to Wasm functions.
		GoFunc *reflect.Value

		// Fields above here are settable prior to instantiation. Below are set by the Store during instantiation.

		// ModuleInstance holds the pointer to the module instance to which this function belongs.
		Module *ModuleInstance

		// TypeID is assigned by a store for FunctionType.
		TypeID FunctionTypeID

		// Idx holds the index of this function instance in the function index namespace (beginning with imports).
		Idx Index

		// The below metadata are used in function listeners, prior to instantiation:

		// moduleName is the defining module's name
		moduleName string

		// name is the module-defined name of this function
		name string

		// paramNames is non-nil when all parameters have names.
		paramNames []string

		// exportNames is non-nil when the function is exported.
		exportNames []string

		// FunctionListener holds a listener to notify when this function is called.
		FunctionListener experimentalapi.FunctionListener
	}

	// GlobalInstance represents a global instance in a store.
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#global-instances%E2%91%A0
	GlobalInstance struct {
		Type *GlobalType
		// Val holds a 64-bit representation of the actual value.
		Val uint64
		// ValHi is only used for vector type globals, and holds the higher bits of the vector.
		ValHi uint64
		// ^^ TODO: this should be guarded with atomics when mutable
	}

	// FunctionTypeID is a uniquely assigned integer for a function type.
	// This is wazero specific runtime object and specific to a store,
	// and used at runtime to do type-checks on indirect function calls.
	FunctionTypeID uint32
)

// Index implements the same method as documented on experimental.FunctionDefinition.
func (f *FunctionInstance) Index() uint32 {
	return f.Idx
}

// Name implements the same method as documented on experimental.FunctionDefinition.
func (f *FunctionInstance) Name() string {
	return f.name
}

// ModuleName implements the same method as documented on experimental.FunctionDefinition.
func (f *FunctionInstance) ModuleName() string {
	return f.moduleName
}

// ExportNames implements the same method as documented on experimental.FunctionDefinition.
func (f *FunctionInstance) ExportNames() []string {
	return f.exportNames
}

// ParamNames implements the same method as documented on experimental.FunctionDefinition.
func (f *FunctionInstance) ParamNames() []string {
	return f.paramNames
}

// The wazero specific limitations described at RATIONALE.md.
const (
	maximumFunctionTypes = 1 << 27
)

// addSections adds section elements to the ModuleInstance
func (m *ModuleInstance) addSections(module *Module, importedFunctions, functions []*FunctionInstance,
	importedGlobals, globals []*GlobalInstance, tables []*TableInstance, memory, importedMemory *MemoryInstance,
	types []*FunctionType, typeIDs []FunctionTypeID) {

	m.Types = types
	m.TypeIDs = typeIDs

	m.Functions = append(m.Functions, importedFunctions...)
	for i, f := range functions {
		// Associate each function with the type instance and the module instance's pointer.
		f.Module = m
		f.TypeID = typeIDs[module.FunctionSection[i]]
		m.Functions = append(m.Functions, f)
	}

	m.Globals = append(m.Globals, importedGlobals...)
	m.Globals = append(m.Globals, globals...)

	m.Tables = tables

	if importedMemory != nil {
		m.Memory = importedMemory
	} else {
		m.Memory = memory
	}

	m.buildExports(module.ExportSection)
	m.buildDataInstances(module.DataSection)
}

func (m *ModuleInstance) buildDataInstances(segments []*DataSegment) {
	for _, d := range segments {
		m.DataInstances = append(m.DataInstances, d.Init)
	}
}

func (m *ModuleInstance) buildElementInstances(elements []*ElementSegment) {
	m.ElementInstances = make([]ElementInstance, len(elements))
	for i, elm := range elements {
		if elm.Type == RefTypeFuncref && elm.Mode == ElementModePassive {
			// Only passive elements can be access as element instances.
			// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/syntax/modules.html#element-segments
			m.ElementInstances[i] = *m.Engine.CreateFuncElementInstance(elm.Init)
		}
	}
}

func (m *ModuleInstance) buildExports(exports []*Export) {
	m.Exports = make(map[string]*ExportInstance, len(exports))
	for _, exp := range exports {
		index := exp.Index
		var ei *ExportInstance
		switch exp.Type {
		case ExternTypeFunc:
			ei = &ExportInstance{Type: exp.Type, Function: m.Functions[index]}
		case ExternTypeGlobal:
			ei = &ExportInstance{Type: exp.Type, Global: m.Globals[index]}
		case ExternTypeMemory:
			ei = &ExportInstance{Type: exp.Type, Memory: m.Memory}
		case ExternTypeTable:
			ei = &ExportInstance{Type: exp.Type, Table: m.Tables[index]}
		}

		// We already validated the duplicates during module validation phase.
		m.Exports[exp.Name] = ei
	}
}

func (m *ModuleInstance) validateData(data []*DataSegment) (err error) {
	for i, d := range data {
		if !d.IsPassive() {
			offset := int(executeConstExpression(m.Globals, d.OffsetExpression).(int32))
			ceil := offset + len(d.Init)
			if offset < 0 || ceil > len(m.Memory.Buffer) {
				return fmt.Errorf("%s[%d] out of bounds memory access", SectionIDName(SectionIDElement), i)
			}
		}
	}
	return
}

func (m *ModuleInstance) applyData(data []*DataSegment) error {
	for i, d := range data {
		if !d.IsPassive() {
			offset := executeConstExpression(m.Globals, d.OffsetExpression).(int32)
			if offset < 0 || int(offset)+len(d.Init) > len(m.Memory.Buffer) {
				return fmt.Errorf("%s[%d] out of bounds memory access", SectionIDName(SectionIDElement), i)
			}
			copy(m.Memory.Buffer[offset:], d.Init)
		}
	}
	return nil
}

// GetExport returns an export of the given name and type or errs if not exported or the wrong type.
func (m *ModuleInstance) getExport(name string, et ExternType) (*ExportInstance, error) {
	exp, ok := m.Exports[name]
	if !ok {
		return nil, fmt.Errorf("%q is not exported in module %q", name, m.Name)
	}
	if exp.Type != et {
		return nil, fmt.Errorf("export %q in module %q is a %s, not a %s", name, m.Name, ExternTypeName(exp.Type), ExternTypeName(et))
	}
	return exp, nil
}

func NewStore(enabledFeatures Features, engine Engine) (*Store, *Namespace) {
	ns := newNamespace()
	return &Store{
		EnabledFeatures:  enabledFeatures,
		Engine:           engine,
		namespaces:       []*Namespace{ns},
		typeIDs:          map[string]FunctionTypeID{},
		functionMaxTypes: maximumFunctionTypes,
	}, ns
}

// NewNamespace implements the same method as documented on wazero.Runtime.
func (s *Store) NewNamespace(_ context.Context) *Namespace {
	// TODO: The above context isn't yet used. If it is, ensure it defaults to context.Background.

	ns := newNamespace()
	s.mux.Lock()
	defer s.mux.Unlock()
	s.namespaces = append(s.namespaces, ns)
	return ns
}

// Instantiate uses name instead of the Module.NameSection ModuleName as it allows instantiating the same module under
// different names safely and concurrently.
//
// * ctx: the default context used for function calls.
// * name: the name of the module.
// * sys: the system context, which will be closed (SysContext.Close) on CallContext.Close.
//
// Note: Module.Validate must be called prior to instantiation.
func (s *Store) Instantiate(
	ctx context.Context,
	ns *Namespace,
	module *Module,
	name string,
	sys *sys.Context,
	functionListenerFactory experimentalapi.FunctionListenerFactory,
) (*CallContext, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Collect any imported modules to avoid locking the namespace too long.
	importedModuleNames := map[string]struct{}{}
	for _, i := range module.ImportSection {
		importedModuleNames[i.Module] = struct{}{}
	}

	// Read-Lock the namespace and ensure imports needed are present.
	importedModules, err := ns.requireModules(importedModuleNames)
	if err != nil {
		return nil, err
	}

	// Write-Lock the namespace and claim the name of the current module.
	if err = ns.requireModuleName(name); err != nil {
		return nil, err
	}

	// Instantiate the module and add it to the namespace so that other modules can import it.
	if callCtx, err := s.instantiate(ctx, ns, module, name, sys, functionListenerFactory, importedModules); err != nil {
		ns.deleteModule(name)
		return nil, err
	} else {
		// Now that the instantiation is complete without error, add it.
		// This makes the module visible for import, and ensures it is closed when the namespace is.
		ns.addModule(callCtx.module)
		return callCtx, nil
	}
}

func (s *Store) instantiate(
	ctx context.Context,
	ns *Namespace,
	module *Module,
	name string,
	sys *sys.Context,
	functionListenerFactory experimentalapi.FunctionListenerFactory,
	modules map[string]*ModuleInstance,
) (*CallContext, error) {
	typeIDs, err := s.getFunctionTypeIDs(module.TypeSection)
	if err != nil {
		return nil, err
	}

	importedFunctions, importedGlobals, importedTables, importedMemory, err := resolveImports(module, modules)
	if err != nil {
		return nil, err
	}

	tables, tableInit, err := module.buildTables(importedTables, importedGlobals,
		// As of reference-types proposal, boundary check must be done after instantiation.
		s.EnabledFeatures.Get(FeatureReferenceTypes))
	if err != nil {
		return nil, err
	}
	globals, memory := module.buildGlobals(importedGlobals), module.buildMemory()

	// If there are no module-defined functions, assume this is a host module.
	var functions []*FunctionInstance
	var funcSection SectionID
	if module.HostFunctionSection == nil {
		funcSection = SectionIDFunction
		functions = module.buildFunctions(name, functionListenerFactory)
	} else {
		funcSection = SectionIDHostFunction
		functions = module.buildHostFunctions(name, functionListenerFactory)
	}

	// Now we have all instances from imports and local ones, so ready to create a new ModuleInstance.
	m := &ModuleInstance{Name: name}
	m.addSections(module, importedFunctions, functions, importedGlobals, globals, tables, importedMemory, memory, module.TypeSection, typeIDs)

	// As of reference types proposal, data segment validation must happen after instantiation,
	// and the side effect must persist even if there's out of bounds error after instantiation.
	// https://github.com/WebAssembly/spec/blob/d39195773112a22b245ffbe864bab6d1182ccb06/test/core/linking.wast#L395-L405
	if !s.EnabledFeatures.Get(FeatureReferenceTypes) {
		if err = m.validateData(module.DataSection); err != nil {
			return nil, err
		}
	}

	// Plus, we are ready to compile functions.
	m.Engine, err = s.Engine.NewModuleEngine(name, module, importedFunctions, functions, tables, tableInit)
	if err != nil && !errors.Is(err, ErrElementOffsetOutOfBounds) {
		// ErrElementOffsetOutOfBounds is not an instantiation error, but rather runtime error, so we ignore it as
		// in anyway the instantiated module and engines are fine and can be used for function invocations.
		// See comments on ErrElementOffsetOutOfBounds.
		return nil, fmt.Errorf("compilation failed: %w", err)
	}

	// After engine creation, we can create the funcref element instances and initialize funcref type globals.
	m.buildElementInstances(module.ElementSection)
	m.Engine.InitializeFuncrefGlobals(globals)

	// Now all the validation passes, we are safe to mutate memory instances (possibly imported ones).
	if err = m.applyData(module.DataSection); err != nil {
		return nil, err
	}

	// Compile the default context for calls to this module.
	m.CallCtx = NewCallContext(ns, m, sys)

	// Execute the start function.
	if module.StartSection != nil {
		funcIdx := *module.StartSection
		f := m.Functions[funcIdx]
		if _, err = f.Module.Engine.Call(ctx, m.CallCtx, f); err != nil {
			return nil, fmt.Errorf("start %s failed: %w", module.funcDesc(funcSection, funcIdx), err)
		}
	}

	return m.CallCtx, nil
}

func resolveImports(module *Module, modules map[string]*ModuleInstance) (
	importedFunctions []*FunctionInstance,
	importedGlobals []*GlobalInstance,
	importedTables []*TableInstance,
	importedMemory *MemoryInstance,
	err error,
) {
	for idx, i := range module.ImportSection {
		m, ok := modules[i.Module]
		if !ok {
			err = fmt.Errorf("module[%s] not instantiated", i.Module)
			return
		}

		var imported *ExportInstance
		imported, err = m.getExport(i.Name, i.Type)
		if err != nil {
			return
		}

		switch i.Type {
		case ExternTypeFunc:
			typeIndex := i.DescFunc
			// TODO: this shouldn't be possible as invalid should fail validate
			if int(typeIndex) >= len(module.TypeSection) {
				err = errorInvalidImport(i, idx, fmt.Errorf("function type out of range"))
				return
			}
			expectedType := module.TypeSection[i.DescFunc]
			importedFunction := imported.Function

			actualType := importedFunction.Type
			if !expectedType.EqualsSignature(actualType.Params, actualType.Results) {
				err = errorInvalidImport(i, idx, fmt.Errorf("signature mismatch: %s != %s", expectedType, actualType))
				return
			}

			importedFunctions = append(importedFunctions, importedFunction)
		case ExternTypeTable:
			expected := i.DescTable
			importedTable := imported.Table
			if expected.Type != importedTable.Type {
				err = errorInvalidImport(i, idx, fmt.Errorf("table type mismatch: %s != %s",
					RefTypeName(expected.Type), RefTypeName(importedTable.Type)))
			}

			if expected.Min > importedTable.Min {
				err = errorMinSizeMismatch(i, idx, expected.Min, importedTable.Min)
				return
			}

			if expected.Max != nil {
				expectedMax := *expected.Max
				if importedTable.Max == nil {
					err = errorNoMax(i, idx, expectedMax)
					return
				} else if expectedMax < *importedTable.Max {
					err = errorMaxSizeMismatch(i, idx, expectedMax, *importedTable.Max)
					return
				}
			}
			importedTables = append(importedTables, importedTable)
		case ExternTypeMemory:
			expected := i.DescMem
			importedMemory = imported.Memory

			if expected.Min > memoryBytesNumToPages(uint64(len(importedMemory.Buffer))) {
				err = errorMinSizeMismatch(i, idx, expected.Min, importedMemory.Min)
				return
			}

			if expected.Max < importedMemory.Max {
				err = errorMaxSizeMismatch(i, idx, expected.Max, importedMemory.Max)
				return
			}
		case ExternTypeGlobal:
			expected := i.DescGlobal
			importedGlobal := imported.Global

			if expected.Mutable != importedGlobal.Type.Mutable {
				err = errorInvalidImport(i, idx, fmt.Errorf("mutability mismatch: %t != %t",
					expected.Mutable, importedGlobal.Type.Mutable))
				return
			}

			if expected.ValType != importedGlobal.Type.ValType {
				err = errorInvalidImport(i, idx, fmt.Errorf("value type mismatch: %s != %s",
					ValueTypeName(expected.ValType), ValueTypeName(importedGlobal.Type.ValType)))
				return
			}
			importedGlobals = append(importedGlobals, importedGlobal)
		}
	}
	return
}

func errorMinSizeMismatch(i *Import, idx int, expected, actual uint32) error {
	return errorInvalidImport(i, idx, fmt.Errorf("minimum size mismatch: %d > %d", expected, actual))
}

func errorNoMax(i *Import, idx int, expected uint32) error {
	return errorInvalidImport(i, idx, fmt.Errorf("maximum size mismatch: %d, but actual has no max", expected))
}

func errorMaxSizeMismatch(i *Import, idx int, expected, actual uint32) error {
	return errorInvalidImport(i, idx, fmt.Errorf("maximum size mismatch: %d < %d", expected, actual))
}

func errorInvalidImport(i *Import, idx int, err error) error {
	return fmt.Errorf("import[%d] %s[%s.%s]: %w", idx, ExternTypeName(i.Type), i.Module, i.Name, err)
}

// Global initialization constant expression can only reference the imported globals.
// See the note on https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#constant-expressions%E2%91%A0
func executeConstExpression(importedGlobals []*GlobalInstance, expr *ConstantExpression) (v interface{}) {
	r := bytes.NewReader(expr.Data)
	switch expr.Opcode {
	case OpcodeI32Const:
		// Treat constants as signed as their interpretation is not yet known per /RATIONALE.md
		v, _, _ = leb128.DecodeInt32(r)
	case OpcodeI64Const:
		// Treat constants as signed as their interpretation is not yet known per /RATIONALE.md
		v, _, _ = leb128.DecodeInt64(r)
	case OpcodeF32Const:
		v, _ = ieee754.DecodeFloat32(r)
	case OpcodeF64Const:
		v, _ = ieee754.DecodeFloat64(r)
	case OpcodeGlobalGet:
		id, _, _ := leb128.DecodeUint32(r)
		g := importedGlobals[id]
		switch g.Type.ValType {
		case ValueTypeI32:
			v = int32(g.Val)
		case ValueTypeI64:
			v = int64(g.Val)
		case ValueTypeF32:
			v = api.DecodeF32(g.Val)
		case ValueTypeF64:
			v = api.DecodeF64(g.Val)
		case ValueTypeV128:
			v = [2]uint64{g.Val, g.ValHi}
		}
	case OpcodeRefNull:
		switch expr.Data[0] {
		case ValueTypeExternref:
			v = int64(0) // Extern reference types are opaque 64bit pointer at runtime.
		case ValueTypeFuncref:
			// For funcref types, the pointer value will be set by Engines, so
			// here we set the "invalid function index" (-1) to indicate that this should be null reference.
			v = GlobalInstanceNullFuncRefValue
		}
	case OpcodeRefFunc:
		// For ref.func const expresssion, we temporarily store the index as value,
		// and if this is the const expr for global, the value will be further downed to
		// opaque pointer of the engine-specific compiled function.
		v, _, _ = leb128.DecodeInt32(r)
	case OpcodeVecV128Const:
		v = [2]uint64{binary.LittleEndian.Uint64(expr.Data[0:8]), binary.LittleEndian.Uint64(expr.Data[8:16])}
	}
	return
}

// GlobalInstanceNullFuncRefValue is the temporary value for ValueTypeFuncref globals which are initialized via ref.null.
const GlobalInstanceNullFuncRefValue int64 = -1

func (s *Store) getFunctionTypeIDs(ts []*FunctionType) ([]FunctionTypeID, error) {
	// We take write-lock here as the following might end up mutating typeIDs map.
	s.mux.Lock()
	defer s.mux.Unlock()
	ret := make([]FunctionTypeID, len(ts))
	for i, t := range ts {
		inst, err := s.getFunctionTypeID(t)
		if err != nil {
			return nil, err
		}
		ret[i] = inst
	}
	return ret, nil
}

func (s *Store) getFunctionTypeID(t *FunctionType) (FunctionTypeID, error) {
	key := t.String()
	id, ok := s.typeIDs[key]
	if !ok {
		l := uint32(len(s.typeIDs))
		if l >= s.functionMaxTypes {
			return 0, fmt.Errorf("too many function types in a store")
		}
		id = FunctionTypeID(len(s.typeIDs))
		s.typeIDs[key] = id
	}
	return id, nil
}

// CloseWithExitCode implements the same method as documented on wazero.Runtime.
func (s *Store) CloseWithExitCode(ctx context.Context, exitCode uint32) (err error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	// Close modules in reverse initialization order.
	for i := len(s.namespaces) - 1; i >= 0; i-- {
		// If closing this namespace errs, proceed anyway to close the others.
		if e := s.namespaces[i].CloseWithExitCode(ctx, exitCode); e != nil && err == nil {
			err = e // first error
		}
	}
	s.namespaces = nil
	s.typeIDs = nil
	return
}
