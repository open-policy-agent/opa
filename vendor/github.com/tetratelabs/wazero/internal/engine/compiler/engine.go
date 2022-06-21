package compiler

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"unsafe"

	"github.com/tetratelabs/wazero/internal/buildoptions"
	"github.com/tetratelabs/wazero/internal/platform"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/internal/wasmdebug"
	"github.com/tetratelabs/wazero/internal/wasmruntime"
	"github.com/tetratelabs/wazero/internal/wazeroir"
)

type (
	// engine is a Compiler implementation of wasm.Engine
	engine struct {
		enabledFeatures wasm.Features
		codes           map[wasm.ModuleID][]*code // guarded by mutex.
		mux             sync.RWMutex
		// setFinalizer defaults to runtime.SetFinalizer, but overridable for tests.
		setFinalizer func(obj interface{}, finalizer interface{})
	}

	// moduleEngine implements wasm.ModuleEngine
	moduleEngine struct {
		// name is the name the module was instantiated with used for error handling.
		name string

		// functions are the functions in a module instances.
		// The index is module instance-scoped. We intentionally avoid using map
		// as the underlying memory region is accessed by assembly directly by using
		// codesElement0Address.
		functions []*function

		importedFunctionCount uint32
	}

	// callEngine holds context per moduleEngine.Call, and shared across all the
	// function calls originating from the same moduleEngine.Call execution.
	callEngine struct {
		// These contexts are read and written by compiled code.
		// Note: structs are embedded to reduce the costs to access fields inside them. Also, this eases field offset
		// calculation.
		globalContext
		moduleContext
		valueStackContext
		exitContext
		archContext

		// The following fields are not accessed by compiled code directly.

		// valueStack is the go-allocated stack for holding Wasm values.
		// Note: We never edit len or cap in compiled code, so we won't get screwed when GC comes in.
		valueStack []uint64

		// callFrameStack is initially callFrameStack[callFrameStackPointer].
		// The currently executed function call frame lives at callFrameStack[callFrameStackPointer-1]
		// and that is equivalent to  engine.callFrameTop().
		callFrameStack []callFrame
	}

	// globalContext holds the data which is constant across multiple function calls.
	globalContext struct {
		// valueStackElement0Address is &engine.valueStack[0] as uintptr.
		// Note: this is updated when growing the stack in builtinFunctionGrowValueStack.
		valueStackElement0Address uintptr
		// valueStackLen is len(engine.valueStack[0]).
		// Note: this is updated when growing the stack in builtinFunctionGrowValueStack.
		valueStackLen uint64

		// callFrameStackElementZeroAddress is &engine.callFrameStack[0] as uintptr.
		// Note: this is updated when growing the stack in builtinFunctionGrowCallFrameStack.
		callFrameStackElementZeroAddress uintptr
		// callFrameStackLen is len(engine.callFrameStack).
		// Note: this is updated when growing the stack in builtinFunctionGrowCallFrameStack.
		callFrameStackLen uint64
		// callFrameStackPointer points at the next empty slot on the call frame stack.
		// For example, for the next function call, we push the new callFrame onto
		// callFrameStack[callFrameStackPointer]. This value is incremented/decremented in assembly
		// when making function calls or returning from them.
		callFrameStackPointer uint64
	}

	// moduleContext holds the per-function call specific module information.
	// This is subject to be manipulated from compiled native code whenever we make function calls.
	moduleContext struct {
		// moduleInstanceAddress is the address of module instance from which we initialize
		// the following fields. This is set whenever we enter a function or return from function calls.
		// This is only used by Compiler code so mark this as nolint.
		moduleInstanceAddress uintptr //nolint

		// globalElement0Address is the address of the first element in the global slice,
		// i.e. &ModuleInstance.Globals[0] as uintptr.
		globalElement0Address uintptr
		// memoryElement0Address is the address of the first element in the global slice,
		// i.e. &ModuleInstance.Memory.Buffer[0] as uintptr.
		memoryElement0Address uintptr
		// memorySliceLen is the length of the memory buffer, i.e. len(ModuleInstance.Memory.Buffer).
		memorySliceLen uint64
		// tableElement0Address is the address of the first item in the tables slice,
		// i.e. &ModuleInstance.Tables[0] as uintptr.
		tablesElement0Address uintptr

		// functionsElement0Address is &moduleContext.functions[0] as uintptr.
		functionsElement0Address uintptr

		// typeIDsElement0Address holds the &ModuleInstance.TypeIDs[0] as uintptr.
		typeIDsElement0Address uintptr

		// dataInstancesElement0Address holds the &ModuleInstance.DataInstances[0] as uintptr.
		dataInstancesElement0Address uintptr

		// elementInstancesElement0Address holds the &ModuleInstance.ElementInstances[0] as uintptr.
		elementInstancesElement0Address uintptr
	}

	// valueStackContext stores the data to access engine.valueStack.
	valueStackContext struct {
		// stackPointer on .valueStack field which is accessed by [stackBasePointer] + [stackPointer].
		//
		// Note: stackPointer is not used in assembly since the native code knows exact position of
		// each variable in the value stack from the info from compilation.
		// Therefore, only updated when native code exit from the Compiler world and go back to the Go function.
		stackPointer uint64

		// stackBasePointer is updated whenever we make function calls.
		// Background: Functions might be compiled as if they use the stack from the bottom.
		// However, in reality, they have to use it from the middle of the stack depending on
		// when these function calls are made. So instead of accessing stack via stackPointer alone,
		// functions are compiled, so they access the stack via [stackBasePointer](fixed for entire function) + [stackPointer].
		// More precisely, stackBasePointer is set to [callee's stack pointer] + [callee's stack base pointer] - [caller's params].
		// This way, compiled functions can be independent of the timing of functions calls made against them.
		//
		// Note: This is saved on callFrameTop().returnStackBasePointer whenever making function call.
		// Also, this is changed whenever we make function call or return from functions where we execute jump instruction.
		// In either case, the caller of "jmp" instruction must set this field properly.
		stackBasePointer uint64
	}

	// exitContext will be manipulated whenever compiled native code returns into the Go function.
	exitContext struct {
		// Where we store the status code of Compiler execution.
		statusCode nativeCallStatusCode

		// Set when statusCode == compilerStatusCallBuiltInFunction
		// Indicating the function call index.
		builtinFunctionCallIndex wasm.Index
	}

	// callFrame holds the information to which the caller function can return.
	// callFrame is created for currently executed function frame as well,
	// so some fields are not yet set when native code is currently executing it.
	// That is, callFrameTop().returnAddress or returnStackBasePointer are not set
	// until it makes a function call.
	callFrame struct {
		// Set when making function call from this function frame, or for the initial function frame to call from
		// callEngine.execWasmFunction.
		returnAddress uintptr
		// Set when making function call from this function frame.
		returnStackBasePointer uint64
		// Set when making function call to this function frame.
		function *function
		// _ is a necessary padding to make the size of callFrame struct a power of 2.
		_ [8]byte
	}

	// Function corresponds to function instance in Wasm, and is created from `code`.
	function struct {
		// codeInitialAddress is the pre-calculated pointer pointing to the initial byte of .codeSegment slice.
		// That mean codeInitialAddress always equals uintptr(unsafe.Pointer(&.codeSegment[0]))
		// and we cache the value (uintptr(unsafe.Pointer(&.codeSegment[0]))) to this field,
		// so we don't need to repeat the calculation on each function call.
		codeInitialAddress uintptr
		// stackPointerCeil is the max of the stack pointer this function can reach. Lazily applied via maybeGrowValueStack.
		stackPointerCeil uint64
		// source is the source function instance from which this is compiled.
		source *wasm.FunctionInstance
		// moduleInstanceAddress holds the address of source.ModuleInstance.
		moduleInstanceAddress uintptr
		// parent holds code from which this is crated.
		parent *code
	}

	// code corresponds to a function in a module (not instantiated one). This holds the machine code
	// compiled by wazero compiler.
	code struct {
		// codeSegment is holding the compiled native code as a byte slice.
		codeSegment []byte
		// See the doc for codeStaticData type.
		staticData codeStaticData
		// stackPointerCeil is the max of the stack pointer this function can reach. Lazily applied via maybeGrowValueStack.
		stackPointerCeil uint64

		// indexInModule is the index of this function in the module. For logging purpose.
		indexInModule wasm.Index
		// sourceModule is the module from which this function is compiled. For logging purpose.
		sourceModule *wasm.Module
	}

	// staticData holds the read-only data (i.e. outside codeSegment which is marked as executable) per function.
	// This is used to store jump tables for br_table instructions.
	// The primary index is the logical separation of multiple data, for example data[0] and data[1]
	// correspond to different jump tables for different br_table instructions.
	codeStaticData = [][]byte
)

// createFunction creates a new function which uses the native code compiled.
func (c *code) createFunction(f *wasm.FunctionInstance) *function {
	return &function{
		codeInitialAddress:    uintptr(unsafe.Pointer(&c.codeSegment[0])),
		stackPointerCeil:      c.stackPointerCeil,
		moduleInstanceAddress: uintptr(unsafe.Pointer(f.Module)),
		source:                f,
		parent:                c,
	}
}

// Native code reads/writes Go's structs with the following constants.
// See TestVerifyOffsetValue for how to derive these values.
const (
	// Offsets for moduleEngine.functions
	moduleEngineFunctionsOffset = 16

	// Offsets for callEngine globalContext.
	callEngineGlobalContextValueStackElement0AddressOffset     = 0
	callEngineGlobalContextValueStackLenOffset                 = 8
	callEngineGlobalContextCallFrameStackElement0AddressOffset = 16
	callEngineGlobalContextCallFrameStackLenOffset             = 24
	callEngineGlobalContextCallFrameStackPointerOffset         = 32

	// Offsets for callEngine moduleContext.
	callEngineModuleContextModuleInstanceAddressOffset           = 40
	callEngineModuleContextGlobalElement0AddressOffset           = 48
	callEngineModuleContextMemoryElement0AddressOffset           = 56
	callEngineModuleContextMemorySliceLenOffset                  = 64
	callEngineModuleContextTablesElement0AddressOffset           = 72
	callEngineModuleContextFunctionsElement0AddressOffset        = 80
	callEngineModuleContextTypeIDsElement0AddressOffset          = 88
	callEngineModuleContextDataInstancesElement0AddressOffset    = 96
	callEngineModuleContextElementInstancesElement0AddressOffset = 104

	// Offsets for callEngine valueStackContext.
	callEngineValueStackContextStackPointerOffset     = 112
	callEngineValueStackContextStackBasePointerOffset = 120

	// Offsets for callEngine exitContext.
	callEngineExitContextNativeCallStatusCodeOffset       = 128
	callEngineExitContextBuiltinFunctionCallAddressOffset = 132

	// Offsets for callFrame.
	callFrameDataSize                      = 32
	callFrameDataSizeMostSignificantSetBit = 5
	callFrameReturnAddressOffset           = 0
	callFrameReturnStackBasePointerOffset  = 8
	callFrameFunctionOffset                = 16

	// Offsets for function.
	functionCodeInitialAddressOffset    = 0
	functionStackPointerCeilOffset      = 8
	functionSourceOffset                = 16
	functionModuleInstanceAddressOffset = 24

	// Offsets for wasm.ModuleInstance.
	moduleInstanceGlobalsOffset          = 48
	moduleInstanceMemoryOffset           = 72
	moduleInstanceTablesOffset           = 80
	moduleInstanceEngineOffset           = 136
	moduleInstanceTypeIDsOffset          = 152
	moduleInstanceDataInstancesOffset    = 176
	moduleInstanceElementInstancesOffset = 200

	// Offsets for wasm.TableInstance.
	tableInstanceTableOffset    = 0
	tableInstanceTableLenOffset = 8

	// Offsets for wasm.FunctionInstance.
	functionInstanceTypeIDOffset = 96

	// Offsets for wasm.MemoryInstance.
	memoryInstanceBufferOffset    = 0
	memoryInstanceBufferLenOffset = 8

	// Offsets for wasm.GlobalInstance.
	globalInstanceValueOffset = 8

	// Offsets for Go's interface.
	// https://research.swtch.com/interfaces
	// https://github.com/golang/go/blob/release-branch.go1.17/src/runtime/runtime2.go#L207-L210
	interfaceDataOffset = 8

	// Consts for wasm.DataInstance.
	dataInstanceStructSize = 24

	// Consts for wasm.ElementInstance.
	elementInstanceStructSize = 32

	// pointerSizeLog2 satisfies: 1 << pointerSizeLog2 = sizeOf(uintptr)
	pointerSizeLog2 = 3
)

// nativeCallStatusCode represents the result of `nativecall`.
// This is set by the native code.
type nativeCallStatusCode uint32

const (
	// nativeCallStatusCodeReturned means the nativecall reaches the end of function, and returns successfully.
	nativeCallStatusCodeReturned nativeCallStatusCode = iota
	// nativeCallStatusCodeCallHostFunction means the nativecall returns to make a host function call.
	nativeCallStatusCodeCallHostFunction
	// nativeCallStatusCodeCallBuiltInFunction means the nativecall returns to make a builtin function call.
	nativeCallStatusCodeCallBuiltInFunction
	// nativeCallStatusCodeUnreachable means the function invocation reaches "unreachable" instruction.
	nativeCallStatusCodeUnreachable
	// nativeCallStatusCodeInvalidFloatToIntConversion means an invalid conversion of integer to floats happened.
	nativeCallStatusCodeInvalidFloatToIntConversion
	// nativeCallStatusCodeMemoryOutOfBounds means an out-of-bounds memory access happened.
	nativeCallStatusCodeMemoryOutOfBounds
	// nativeCallStatusCodeInvalidTableAccess means either offset to the table was out of bounds of table, or
	// the target element in the table was uninitialized during call_indirect instruction.
	nativeCallStatusCodeInvalidTableAccess
	// nativeCallStatusCodeTypeMismatchOnIndirectCall means the type check failed during call_indirect.
	nativeCallStatusCodeTypeMismatchOnIndirectCall
	nativeCallStatusIntegerOverflow
	nativeCallStatusIntegerDivisionByZero
)

// causePanic causes a panic with the corresponding error to the nativeCallStatusCode.
func (s nativeCallStatusCode) causePanic() {
	var err error
	switch s {
	case nativeCallStatusIntegerOverflow:
		err = wasmruntime.ErrRuntimeIntegerOverflow
	case nativeCallStatusIntegerDivisionByZero:
		err = wasmruntime.ErrRuntimeIntegerDivideByZero
	case nativeCallStatusCodeInvalidFloatToIntConversion:
		err = wasmruntime.ErrRuntimeInvalidConversionToInteger
	case nativeCallStatusCodeUnreachable:
		err = wasmruntime.ErrRuntimeUnreachable
	case nativeCallStatusCodeMemoryOutOfBounds:
		err = wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess
	case nativeCallStatusCodeInvalidTableAccess:
		err = wasmruntime.ErrRuntimeInvalidTableAccess
	case nativeCallStatusCodeTypeMismatchOnIndirectCall:
		err = wasmruntime.ErrRuntimeIndirectCallTypeMismatch
	}
	panic(err)
}

func (s nativeCallStatusCode) String() (ret string) {
	switch s {
	case nativeCallStatusCodeReturned:
		ret = "returned"
	case nativeCallStatusCodeCallHostFunction:
		ret = "call_host_function"
	case nativeCallStatusCodeCallBuiltInFunction:
		ret = "call_builtin_function"
	case nativeCallStatusCodeUnreachable:
		ret = "unreachable"
	case nativeCallStatusCodeInvalidFloatToIntConversion:
		ret = "invalid float to int conversion"
	case nativeCallStatusCodeMemoryOutOfBounds:
		ret = "memory out of bounds"
	case nativeCallStatusCodeInvalidTableAccess:
		ret = "invalid table access"
	case nativeCallStatusCodeTypeMismatchOnIndirectCall:
		ret = "type mismatch on indirect call"
	case nativeCallStatusIntegerOverflow:
		ret = "integer overflow"
	case nativeCallStatusIntegerDivisionByZero:
		ret = "integer division by zero"
	default:
		panic("BUG")
	}
	return
}

// String implements fmt.Stringer
func (c *callFrame) String() string {
	return fmt.Sprintf(
		"[%s: return address=0x%x, return stack base pointer=%d]",
		c.function.source.DebugName, c.returnAddress, c.returnStackBasePointer,
	)
}

// releaseCode is a runtime.SetFinalizer function that munmaps the code.codeSegment.
func releaseCode(compiledFn *code) {
	codeSegment := compiledFn.codeSegment
	if codeSegment == nil {
		return // already released
	}

	// Setting this to nil allows tests to know the correct finalizer function was called.
	compiledFn.codeSegment = nil
	if err := platform.MunmapCodeSegment(codeSegment); err != nil {
		// munmap failure cannot recover, and happen asynchronously on the finalizer thread. While finalizer
		// functions can return errors, they are ignored. To make these visible for troubleshooting, we panic
		// with additional context. module+funcidx should be enough, but if not, we can add more later.
		panic(fmt.Errorf("compiler: failed to munmap code segment for %s.function[%d]: %w", compiledFn.sourceModule.NameSection.ModuleName,
			compiledFn.indexInModule, err))
	}
}

// CompiledModuleCount implements the same method as documented on wasm.Engine.
func (e *engine) CompiledModuleCount() uint32 {
	return uint32(len(e.codes))
}

// DeleteCompiledModule implements the same method as documented on wasm.Engine.
func (e *engine) DeleteCompiledModule(module *wasm.Module) {
	e.deleteCodes(module)
}

// CompileModule implements the same method as documented on wasm.Engine.
func (e *engine) CompileModule(ctx context.Context, module *wasm.Module) error {
	if _, ok := e.getCodes(module); ok { // cache hit!
		return nil
	}

	funcs := make([]*code, 0, len(module.FunctionSection))

	if module.IsHostModule() {
		for funcIndex := range module.HostFunctionSection {
			compiled, err := compileHostFunction(module.TypeSection[module.FunctionSection[funcIndex]])
			if err != nil {
				return fmt.Errorf("function[%d/%d] %w", funcIndex, len(module.FunctionSection)-1, err)
			}

			// As this uses mmap, we need a finalizer in case moduleEngine.Close was never called. Regardless, we need a
			// finalizer due to how moduleEngine.Close is implemented.
			e.setFinalizer(compiled, releaseCode)

			compiled.indexInModule = wasm.Index(funcIndex)
			compiled.sourceModule = module
			funcs = append(funcs, compiled)
		}
	} else {
		irs, err := wazeroir.CompileFunctions(ctx, e.enabledFeatures, module)
		if err != nil {
			return err
		}

		for funcIndex := range module.FunctionSection {
			compiled, err := compileWasmFunction(e.enabledFeatures, irs[funcIndex])
			if err != nil {
				return fmt.Errorf("function[%d/%d] %w", funcIndex, len(module.FunctionSection)-1, err)
			}

			// As this uses mmap, we need to munmap on the compiled machine code when it's GCed.
			e.setFinalizer(compiled, releaseCode)

			compiled.indexInModule = wasm.Index(funcIndex)
			compiled.sourceModule = module

			funcs = append(funcs, compiled)
		}
	}
	e.addCodes(module, funcs)
	return nil
}

// NewModuleEngine implements the same method as documented on wasm.Engine.
func (e *engine) NewModuleEngine(name string, module *wasm.Module, importedFunctions, moduleFunctions []*wasm.FunctionInstance, tables []*wasm.TableInstance, tableInits []wasm.TableInitEntry) (wasm.ModuleEngine, error) {
	imported := uint32(len(importedFunctions))
	me := &moduleEngine{
		name:                  name,
		functions:             make([]*function, 0, imported+uint32(len(moduleFunctions))),
		importedFunctionCount: imported,
	}

	for _, f := range importedFunctions {
		cf := f.Module.Engine.(*moduleEngine).functions[f.Idx]
		me.functions = append(me.functions, cf)
	}

	codes, ok := e.getCodes(module)
	if !ok {
		return nil, fmt.Errorf("source module for %s must be compiled before instantiation", name)
	}

	for i, c := range codes {
		f := moduleFunctions[i]
		function := c.createFunction(f)
		me.functions = append(me.functions, function)
	}

	for _, init := range tableInits {
		references := tables[init.TableIndex].References
		if int(init.Offset)+(len(init.FunctionIndexes)) > len(references) {
			return me, wasm.ErrElementOffsetOutOfBounds
		}

		for i, funcIdx := range init.FunctionIndexes {
			if funcIdx != nil {
				references[init.Offset+uint32(i)] = uintptr(unsafe.Pointer(me.functions[*funcIdx]))
			}
		}
	}
	return me, nil
}

func (e *engine) deleteCodes(module *wasm.Module) {
	e.mux.Lock()
	defer e.mux.Unlock()
	delete(e.codes, module.ID)
}

func (e *engine) addCodes(module *wasm.Module, fs []*code) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.codes[module.ID] = fs
}

func (e *engine) getCodes(module *wasm.Module) (fs []*code, ok bool) {
	e.mux.RLock()
	defer e.mux.RUnlock()
	fs, ok = e.codes[module.ID]
	return
}

// Name implements the same method as documented on wasm.ModuleEngine.
func (e *moduleEngine) Name() string {
	return e.name
}

// CreateFuncElementInstance implements the same method as documented on wasm.ModuleEngine.
func (e *moduleEngine) CreateFuncElementInstance(indexes []*wasm.Index) *wasm.ElementInstance {
	refs := make([]wasm.Reference, len(indexes))
	for i, index := range indexes {
		if index != nil {
			refs[i] = uintptr(unsafe.Pointer(e.functions[*index]))
		}
	}
	return &wasm.ElementInstance{
		References: refs,
		Type:       wasm.RefTypeFuncref,
	}
}

// InitializeFuncrefGlobals implements the same method as documented on wasm.InitializeFuncrefGlobals.
func (e *moduleEngine) InitializeFuncrefGlobals(globals []*wasm.GlobalInstance) {
	for _, g := range globals {
		if g.Type.ValType == wasm.ValueTypeFuncref {
			if int64(g.Val) == wasm.GlobalInstanceNullFuncRefValue {
				g.Val = 0 // Null funcref is expressed as zero.
			} else {
				// Lowers the stored function index into the interpreter specific function's opaque pointer.
				g.Val = uint64(uintptr(unsafe.Pointer(e.functions[g.Val])))
			}
		}
	}
}

// Call implements the same method as documented on wasm.ModuleEngine.
func (e *moduleEngine) Call(ctx context.Context, callCtx *wasm.CallContext, f *wasm.FunctionInstance, params ...uint64) (results []uint64, err error) {
	// Note: The input parameters are pre-validated, so a compiled function is only absent on close. Updates to
	// code on close aren't locked, neither is this read.
	compiled := e.functions[f.Idx]
	if compiled == nil { // Lazy check the cause as it could be because the module was already closed.
		if err = callCtx.FailIfClosed(); err == nil {
			panic(fmt.Errorf("BUG: %s.func[%d] was nil before close", e.name, f.Idx))
		}
		return
	}

	paramCount := len(params)
	if f.Type.ParamNumInUint64 != paramCount {
		return nil, fmt.Errorf("expected %d params, but passed %d", f.Type.ParamNumInUint64, paramCount)
	}

	ce := e.newCallEngine()

	// We ensure that this Call method never panics as
	// this Call method is indirectly invoked by embedders via store.CallFunction,
	// and we have to make sure that all the runtime errors, including the one happening inside
	// host functions, will be captured as errors, not panics.
	defer func() {
		// If the module closed during the call, and the call didn't err for another reason, set an ExitError.
		if err == nil {
			err = callCtx.FailIfClosed()
		}
		// TODO: ^^ Will not fail if the function was imported from a closed module.

		if v := recover(); v != nil {
			builder := wasmdebug.NewErrorBuilder()
			// Handle edge-case where the host function is called directly by Go.
			if ce.globalContext.callFrameStackPointer == 0 {
				fn := compiled.source
				builder.AddFrame(fn.DebugName, fn.ParamTypes(), fn.ResultTypes())
			}
			for i := uint64(0); i < ce.globalContext.callFrameStackPointer; i++ {
				fn := ce.callFrameStack[ce.globalContext.callFrameStackPointer-1-i].function.source
				builder.AddFrame(fn.DebugName, fn.ParamTypes(), fn.ResultTypes())
			}
			err = builder.FromRecovered(v)
		}
	}()

	if f.Kind == wasm.FunctionKindWasm {
		for _, v := range params {
			ce.pushValue(v)
		}
		ce.execWasmFunction(ctx, callCtx, compiled)
		results = wasm.PopValues(f.Type.ResultNumInUint64, ce.popValue)
	} else {
		results = wasm.CallGoFunc(ctx, callCtx, compiled.source, params)
	}
	return
}

func NewEngine(enabledFeatures wasm.Features) wasm.Engine {
	return newEngine(enabledFeatures)
}

func newEngine(enabledFeatures wasm.Features) *engine {
	return &engine{
		enabledFeatures: enabledFeatures,
		codes:           map[wasm.ModuleID][]*code{},
		setFinalizer:    runtime.SetFinalizer,
	}
}

// Do not make these variables as constants, otherwise there would be
// dangerous memory access from native code.
//
// Background: Go has a mechanism called "goroutine stack-shrink" where Go
// runtime shrinks Goroutine's stack when it is GCing. Shrinking means that
// all the contents on the goroutine stack will be relocated by runtime,
// Therefore, the memory address of these contents change undeterministically.
// Not only shrinks, but also Go runtime grows the goroutine stack at any point
// of function call entries, which also might end up relocating contents.
//
// On the other hand, we hold pointers to the data region of value stack and
// call-frame stack slices and use these raw pointers from native code.
// Therefore, it is dangerous if these two stacks are allocated on stack
// as these stack's address might be changed by Goroutine which we cannot
// detect.
//
// By declaring these values as `var`, slices created via `make([]..., var)`
// will never be allocated on stack [1]. This means accessing these slices via
// raw pointers is safe: As of version 1.18, Go's garbage collector never relocates
// heap-allocated objects (aka no compaction of memory [2]).
//
// On Go upgrades, re-validate heap-allocation via `go build -gcflags='-m' ./internal/engine/compiler/...`.
//
//  [1] https://github.com/golang/go/blob/68ecdc2c70544c303aa923139a5f16caf107d955/src/cmd/compile/internal/escape/utils.go#L206-L208
//  [2] https://github.com/golang/go/blob/68ecdc2c70544c303aa923139a5f16caf107d955/src/runtime/mgc.go#L9
//  [3] https://mayurwadekar2.medium.com/escape-analysis-in-golang-ee40a1c064c1
//  [4] https://medium.com/@yulang.chu/go-stack-or-heap-2-slices-which-keep-in-stack-have-limitation-of-size-b3f3adfd6190
var (
	initialValueStackSize     = 64
	initialCallFrameStackSize = 16
)

func (e *moduleEngine) newCallEngine() *callEngine {
	ce := &callEngine{
		valueStack:     make([]uint64, initialValueStackSize),
		callFrameStack: make([]callFrame, initialCallFrameStackSize),
		archContext:    newArchContext(),
	}

	valueStackHeader := (*reflect.SliceHeader)(unsafe.Pointer(&ce.valueStack))
	callFrameStackHeader := (*reflect.SliceHeader)(unsafe.Pointer(&ce.callFrameStack))
	ce.globalContext = globalContext{
		valueStackElement0Address:        valueStackHeader.Data,
		valueStackLen:                    uint64(valueStackHeader.Len),
		callFrameStackElementZeroAddress: callFrameStackHeader.Data,
		callFrameStackLen:                uint64(callFrameStackHeader.Len),
		callFrameStackPointer:            0,
	}
	return ce
}

func (ce *callEngine) popValue() (ret uint64) {
	ce.valueStackContext.stackPointer--
	ret = ce.valueStack[ce.valueStackTopIndex()]
	return
}

func (ce *callEngine) pushValue(v uint64) {
	ce.valueStack[ce.valueStackTopIndex()] = v
	ce.valueStackContext.stackPointer++
}

func (ce *callEngine) callFrameTop() *callFrame {
	return &ce.callFrameStack[ce.globalContext.callFrameStackPointer-1]
}

func (ce *callEngine) callFrameAt(depth uint64) *callFrame {
	idx := ce.globalContext.callFrameStackPointer - 1 - depth
	return &ce.callFrameStack[idx]
}

func (ce *callEngine) valueStackTopIndex() uint64 {
	return ce.valueStackContext.stackBasePointer + ce.valueStackContext.stackPointer
}

const (
	builtinFunctionIndexMemoryGrow wasm.Index = iota
	builtinFunctionIndexGrowValueStack
	builtinFunctionIndexGrowCallFrameStack
	builtinFunctionIndexTableGrow
	// builtinFunctionIndexBreakPoint is internal (only for wazero developers). Disabled by default.
	builtinFunctionIndexBreakPoint
)

func (ce *callEngine) execWasmFunction(ctx context.Context, callCtx *wasm.CallContext, f *function) {
	// Push the initial callframe.
	ce.callFrameStack[0] = callFrame{returnAddress: f.codeInitialAddress, function: f}
	ce.globalContext.callFrameStackPointer++

entry:
	{
		frame := ce.callFrameTop()
		if buildoptions.IsDebugMode {
			fmt.Printf("callframe=%s, stackBasePointer: %d, stackPointer: %d\n",
				frame.String(), ce.valueStackContext.stackBasePointer, ce.valueStackContext.stackPointer)
		}

		// Call into the native code.
		nativecall(frame.returnAddress, uintptr(unsafe.Pointer(ce)), f.moduleInstanceAddress)

		// Check the status code from Compiler code.
		switch status := ce.exitContext.statusCode; status {
		case nativeCallStatusCodeReturned:
			// Meaning that all the function frames above the previous call frame stack pointer are executed.
		case nativeCallStatusCodeCallHostFunction:
			calleeHostFunction := ce.callFrameTop().function
			// Not "callFrameTop" but take the below of peek with "callFrameAt(1)" as the top frame is for host function,
			// but when making host function calls, we need to pass the memory instance of host function caller.
			callerFunction := ce.callFrameAt(1).function
			params := wasm.PopGoFuncParams(calleeHostFunction.source, ce.popValue)
			results := wasm.CallGoFunc(
				ctx,
				// Use the caller's memory, which might be different from the defining module on an imported function.
				callCtx.WithMemory(callerFunction.source.Module.Memory),
				calleeHostFunction.source,
				params,
			)
			for _, v := range results {
				ce.pushValue(v)
			}
			goto entry
		case nativeCallStatusCodeCallBuiltInFunction:
			switch ce.exitContext.builtinFunctionCallIndex {
			case builtinFunctionIndexMemoryGrow:
				callerFunction := ce.callFrameTop().function
				ce.builtinFunctionMemoryGrow(ctx, callerFunction.source.Module.Memory)
			case builtinFunctionIndexGrowValueStack:
				callerFunction := ce.callFrameTop().function
				ce.builtinFunctionGrowValueStack(callerFunction.stackPointerCeil)
			case builtinFunctionIndexGrowCallFrameStack:
				ce.builtinFunctionGrowCallFrameStack()
			case builtinFunctionIndexTableGrow:
				caller := ce.callFrameTop().function
				ce.builtinFunctionTableGrow(ctx, caller.source.Module.Tables)
			}
			if buildoptions.IsDebugMode {
				if ce.exitContext.builtinFunctionCallIndex == builtinFunctionIndexBreakPoint {
					runtime.Breakpoint()
				}
			}
			goto entry
		default:
			status.causePanic()
		}
	}
}

func (ce *callEngine) builtinFunctionGrowValueStack(stackPointerCeil uint64) {
	// Extends the valueStack's length to currentLen*2+stackPointerCeil.
	newLen := ce.globalContext.valueStackLen*2 + (stackPointerCeil)
	newStack := make([]uint64, newLen)
	top := ce.valueStackContext.stackBasePointer + ce.valueStackContext.stackPointer
	copy(newStack[:top], ce.valueStack[:top])
	ce.valueStack = newStack
	valueStackHeader := (*reflect.SliceHeader)(unsafe.Pointer(&ce.valueStack))
	ce.globalContext.valueStackElement0Address = valueStackHeader.Data
	ce.globalContext.valueStackLen = uint64(valueStackHeader.Len)
}

var callStackCeiling = uint64(buildoptions.CallStackCeiling)

func (ce *callEngine) builtinFunctionGrowCallFrameStack() {
	if callStackCeiling < uint64(len(ce.callFrameStack)+1) {
		panic(wasmruntime.ErrRuntimeCallStackOverflow)
	}

	// Double the callstack slice length.
	newLen := uint64(ce.globalContext.callFrameStackLen) * 2
	newStack := make([]callFrame, newLen)
	copy(newStack, ce.callFrameStack)
	ce.callFrameStack = newStack

	// Update the globalContext's fields as they become stale after the update ^^.
	stackSliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&newStack))
	ce.globalContext.callFrameStackLen = uint64(stackSliceHeader.Len)
	ce.globalContext.callFrameStackElementZeroAddress = stackSliceHeader.Data
}

func (ce *callEngine) builtinFunctionMemoryGrow(ctx context.Context, mem *wasm.MemoryInstance) {
	newPages := ce.popValue()

	if res, ok := mem.Grow(ctx, uint32(newPages)); !ok {
		ce.pushValue(uint64(0xffffffff)) // = -1 in signed 32-bit integer.
	} else {
		ce.pushValue(uint64(res))
	}

	// Update the moduleContext fields as they become stale after the update ^^.
	bufSliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&mem.Buffer))
	ce.moduleContext.memorySliceLen = uint64(bufSliceHeader.Len)
	ce.moduleContext.memoryElement0Address = bufSliceHeader.Data
}

func (ce *callEngine) builtinFunctionTableGrow(ctx context.Context, tables []*wasm.TableInstance) {
	tableIndex := ce.popValue()
	table := tables[tableIndex] // verified not to be out of range by the func validation at compilation phase.
	num := ce.popValue()
	ref := ce.popValue()
	res := table.Grow(ctx, uint32(num), uintptr(ref))
	ce.pushValue(uint64(res))
}

func compileHostFunction(sig *wasm.FunctionType) (*code, error) {
	compiler, err := newCompiler(&wazeroir.CompilationResult{Signature: sig})
	if err != nil {
		return nil, err
	}

	if err = compiler.compileHostFunction(); err != nil {
		return nil, err
	}

	c, _, _, err := compiler.compile()
	if err != nil {
		return nil, err
	}

	return &code{codeSegment: c}, nil
}

func compileWasmFunction(_ wasm.Features, ir *wazeroir.CompilationResult) (*code, error) {
	compiler, err := newCompiler(ir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize assembly builder: %w", err)
	}

	if err := compiler.compilePreamble(); err != nil {
		return nil, fmt.Errorf("failed to emit preamble: %w", err)
	}

	var skip bool
	for _, op := range ir.Operations {
		// Compiler determines whether skip the entire label.
		// For example, if the label doesn't have any caller,
		// we don't need to generate native code at all as we never reach the region.
		if op.Kind() == wazeroir.OperationKindLabel {
			skip = compiler.compileLabel(op.(*wazeroir.OperationLabel))
		}
		if skip {
			continue
		}

		if buildoptions.IsDebugMode {
			fmt.Printf("compiling op=%s: %s\n", op.Kind(), compiler)
		}
		var err error
		switch o := op.(type) {
		case *wazeroir.OperationLabel:
			// Label op is already handled ^^.
		case *wazeroir.OperationUnreachable:
			err = compiler.compileUnreachable()
		case *wazeroir.OperationBr:
			err = compiler.compileBr(o)
		case *wazeroir.OperationBrIf:
			err = compiler.compileBrIf(o)
		case *wazeroir.OperationBrTable:
			err = compiler.compileBrTable(o)
		case *wazeroir.OperationCall:
			err = compiler.compileCall(o)
		case *wazeroir.OperationCallIndirect:
			err = compiler.compileCallIndirect(o)
		case *wazeroir.OperationDrop:
			err = compiler.compileDrop(o)
		case *wazeroir.OperationSelect:
			err = compiler.compileSelect()
		case *wazeroir.OperationPick:
			err = compiler.compilePick(o)
		case *wazeroir.OperationSwap:
			err = compiler.compileSwap(o)
		case *wazeroir.OperationGlobalGet:
			err = compiler.compileGlobalGet(o)
		case *wazeroir.OperationGlobalSet:
			err = compiler.compileGlobalSet(o)
		case *wazeroir.OperationLoad:
			err = compiler.compileLoad(o)
		case *wazeroir.OperationLoad8:
			err = compiler.compileLoad8(o)
		case *wazeroir.OperationLoad16:
			err = compiler.compileLoad16(o)
		case *wazeroir.OperationLoad32:
			err = compiler.compileLoad32(o)
		case *wazeroir.OperationStore:
			err = compiler.compileStore(o)
		case *wazeroir.OperationStore8:
			err = compiler.compileStore8(o)
		case *wazeroir.OperationStore16:
			err = compiler.compileStore16(o)
		case *wazeroir.OperationStore32:
			err = compiler.compileStore32(o)
		case *wazeroir.OperationMemorySize:
			err = compiler.compileMemorySize()
		case *wazeroir.OperationMemoryGrow:
			err = compiler.compileMemoryGrow()
		case *wazeroir.OperationConstI32:
			err = compiler.compileConstI32(o)
		case *wazeroir.OperationConstI64:
			err = compiler.compileConstI64(o)
		case *wazeroir.OperationConstF32:
			err = compiler.compileConstF32(o)
		case *wazeroir.OperationConstF64:
			err = compiler.compileConstF64(o)
		case *wazeroir.OperationEq:
			err = compiler.compileEq(o)
		case *wazeroir.OperationNe:
			err = compiler.compileNe(o)
		case *wazeroir.OperationEqz:
			err = compiler.compileEqz(o)
		case *wazeroir.OperationLt:
			err = compiler.compileLt(o)
		case *wazeroir.OperationGt:
			err = compiler.compileGt(o)
		case *wazeroir.OperationLe:
			err = compiler.compileLe(o)
		case *wazeroir.OperationGe:
			err = compiler.compileGe(o)
		case *wazeroir.OperationAdd:
			err = compiler.compileAdd(o)
		case *wazeroir.OperationSub:
			err = compiler.compileSub(o)
		case *wazeroir.OperationMul:
			err = compiler.compileMul(o)
		case *wazeroir.OperationClz:
			err = compiler.compileClz(o)
		case *wazeroir.OperationCtz:
			err = compiler.compileCtz(o)
		case *wazeroir.OperationPopcnt:
			err = compiler.compilePopcnt(o)
		case *wazeroir.OperationDiv:
			err = compiler.compileDiv(o)
		case *wazeroir.OperationRem:
			err = compiler.compileRem(o)
		case *wazeroir.OperationAnd:
			err = compiler.compileAnd(o)
		case *wazeroir.OperationOr:
			err = compiler.compileOr(o)
		case *wazeroir.OperationXor:
			err = compiler.compileXor(o)
		case *wazeroir.OperationShl:
			err = compiler.compileShl(o)
		case *wazeroir.OperationShr:
			err = compiler.compileShr(o)
		case *wazeroir.OperationRotl:
			err = compiler.compileRotl(o)
		case *wazeroir.OperationRotr:
			err = compiler.compileRotr(o)
		case *wazeroir.OperationAbs:
			err = compiler.compileAbs(o)
		case *wazeroir.OperationNeg:
			err = compiler.compileNeg(o)
		case *wazeroir.OperationCeil:
			err = compiler.compileCeil(o)
		case *wazeroir.OperationFloor:
			err = compiler.compileFloor(o)
		case *wazeroir.OperationTrunc:
			err = compiler.compileTrunc(o)
		case *wazeroir.OperationNearest:
			err = compiler.compileNearest(o)
		case *wazeroir.OperationSqrt:
			err = compiler.compileSqrt(o)
		case *wazeroir.OperationMin:
			err = compiler.compileMin(o)
		case *wazeroir.OperationMax:
			err = compiler.compileMax(o)
		case *wazeroir.OperationCopysign:
			err = compiler.compileCopysign(o)
		case *wazeroir.OperationI32WrapFromI64:
			err = compiler.compileI32WrapFromI64()
		case *wazeroir.OperationITruncFromF:
			err = compiler.compileITruncFromF(o)
		case *wazeroir.OperationFConvertFromI:
			err = compiler.compileFConvertFromI(o)
		case *wazeroir.OperationF32DemoteFromF64:
			err = compiler.compileF32DemoteFromF64()
		case *wazeroir.OperationF64PromoteFromF32:
			err = compiler.compileF64PromoteFromF32()
		case *wazeroir.OperationI32ReinterpretFromF32:
			err = compiler.compileI32ReinterpretFromF32()
		case *wazeroir.OperationI64ReinterpretFromF64:
			err = compiler.compileI64ReinterpretFromF64()
		case *wazeroir.OperationF32ReinterpretFromI32:
			err = compiler.compileF32ReinterpretFromI32()
		case *wazeroir.OperationF64ReinterpretFromI64:
			err = compiler.compileF64ReinterpretFromI64()
		case *wazeroir.OperationExtend:
			err = compiler.compileExtend(o)
		case *wazeroir.OperationSignExtend32From8:
			err = compiler.compileSignExtend32From8()
		case *wazeroir.OperationSignExtend32From16:
			err = compiler.compileSignExtend32From16()
		case *wazeroir.OperationSignExtend64From8:
			err = compiler.compileSignExtend64From8()
		case *wazeroir.OperationSignExtend64From16:
			err = compiler.compileSignExtend64From16()
		case *wazeroir.OperationSignExtend64From32:
			err = compiler.compileSignExtend64From32()
		case *wazeroir.OperationDataDrop:
			err = compiler.compileDataDrop(o)
		case *wazeroir.OperationMemoryInit:
			err = compiler.compileMemoryInit(o)
		case *wazeroir.OperationMemoryCopy:
			err = compiler.compileMemoryCopy()
		case *wazeroir.OperationMemoryFill:
			err = compiler.compileMemoryFill()
		case *wazeroir.OperationTableInit:
			err = compiler.compileTableInit(o)
		case *wazeroir.OperationTableCopy:
			err = compiler.compileTableCopy(o)
		case *wazeroir.OperationElemDrop:
			err = compiler.compileElemDrop(o)
		case *wazeroir.OperationRefFunc:
			err = compiler.compileRefFunc(o)
		case *wazeroir.OperationTableGet:
			err = compiler.compileTableGet(o)
		case *wazeroir.OperationTableSet:
			err = compiler.compileTableSet(o)
		case *wazeroir.OperationTableGrow:
			err = compiler.compileTableGrow(o)
		case *wazeroir.OperationTableSize:
			err = compiler.compileTableSize(o)
		case *wazeroir.OperationTableFill:
			err = compiler.compileTableFill(o)
		case *wazeroir.OperationV128Const:
			err = compiler.compileV128Const(o)
		case *wazeroir.OperationV128Add:
			err = compiler.compileV128Add(o)
		case *wazeroir.OperationV128Sub:
			err = compiler.compileV128Sub(o)
		case *wazeroir.OperationV128Load:
			err = compiler.compileV128Load(o)
		case *wazeroir.OperationV128LoadLane:
			err = compiler.compileV128LoadLane(o)
		case *wazeroir.OperationV128Store:
			err = compiler.compileV128Store(o)
		case *wazeroir.OperationV128StoreLane:
			err = compiler.compileV128StoreLane(o)
		case *wazeroir.OperationV128ExtractLane:
			err = compiler.compileV128ExtractLane(o)
		case *wazeroir.OperationV128ReplaceLane:
			err = compiler.compileV128ReplaceLane(o)
		case *wazeroir.OperationV128Splat:
			err = compiler.compileV128Splat(o)
		case *wazeroir.OperationV128Shuffle:
			err = compiler.compileV128Shuffle(o)
		case *wazeroir.OperationV128Swizzle:
			err = compiler.compileV128Swizzle(o)
		case *wazeroir.OperationV128AnyTrue:
			err = compiler.compileV128AnyTrue(o)
		case *wazeroir.OperationV128AllTrue:
			err = compiler.compileV128AllTrue(o)
		case *wazeroir.OperationV128BitMask:
			err = compiler.compileV128BitMask(o)
		case *wazeroir.OperationV128And:
			err = compiler.compileV128And(o)
		case *wazeroir.OperationV128Not:
			err = compiler.compileV128Not(o)
		case *wazeroir.OperationV128Or:
			err = compiler.compileV128Or(o)
		case *wazeroir.OperationV128Xor:
			err = compiler.compileV128Xor(o)
		case *wazeroir.OperationV128Bitselect:
			err = compiler.compileV128Bitselect(o)
		case *wazeroir.OperationV128AndNot:
			err = compiler.compileV128AndNot(o)
		case *wazeroir.OperationV128Shr:
			err = compiler.compileV128Shr(o)
		case *wazeroir.OperationV128Shl:
			err = compiler.compileV128Shl(o)
		case *wazeroir.OperationV128Cmp:
			err = compiler.compileV128Cmp(o)
		case *wazeroir.OperationV128AddSat:
			err = compiler.compileV128AddSat(o)
		case *wazeroir.OperationV128SubSat:
			err = compiler.compileV128SubSat(o)
		case *wazeroir.OperationV128Mul:
			err = compiler.compileV128Mul(o)
		case *wazeroir.OperationV128Div:
			err = compiler.compileV128Div(o)
		case *wazeroir.OperationV128Neg:
			err = compiler.compileV128Neg(o)
		case *wazeroir.OperationV128Sqrt:
			err = compiler.compileV128Sqrt(o)
		case *wazeroir.OperationV128Abs:
			err = compiler.compileV128Abs(o)
		case *wazeroir.OperationV128Popcnt:
			err = compiler.compileV128Popcnt(o)
		case *wazeroir.OperationV128Min:
			err = compiler.compileV128Min(o)
		case *wazeroir.OperationV128Max:
			err = compiler.compileV128Max(o)
		case *wazeroir.OperationV128AvgrU:
			err = compiler.compileV128AvgrU(o)
		case *wazeroir.OperationV128Pmin:
			err = compiler.compileV128Pmin(o)
		case *wazeroir.OperationV128Pmax:
			err = compiler.compileV128Pmax(o)
		case *wazeroir.OperationV128Ceil:
			err = compiler.compileV128Ceil(o)
		case *wazeroir.OperationV128Floor:
			err = compiler.compileV128Floor(o)
		case *wazeroir.OperationV128Trunc:
			err = compiler.compileV128Trunc(o)
		case *wazeroir.OperationV128Nearest:
			err = compiler.compileV128Nearest(o)
		case *wazeroir.OperationV128Extend:
			err = compiler.compileV128Extend(o)
		case *wazeroir.OperationV128ExtMul:
			err = compiler.compileV128ExtMul(o)
		case *wazeroir.OperationV128Q15mulrSatS:
			err = compiler.compileV128Q15mulrSatS(o)
		case *wazeroir.OperationV128ExtAddPairwise:
			err = compiler.compileV128ExtAddPairwise(o)
		case *wazeroir.OperationV128FloatPromote:
			err = compiler.compileV128FloatPromote(o)
		case *wazeroir.OperationV128FloatDemote:
			err = compiler.compileV128FloatDemote(o)
		case *wazeroir.OperationV128FConvertFromI:
			err = compiler.compileV128FConvertFromI(o)
		case *wazeroir.OperationV128Dot:
			err = compiler.compileV128Dot(o)
		case *wazeroir.OperationV128Narrow:
			err = compiler.compileV128Narrow(o)
		case *wazeroir.OperationV128ITruncSatFromF:
			err = compiler.compileV128ITruncSatFromF(o)
		default:
			err = errors.New("unsupported")
		}
		if err != nil {
			return nil, fmt.Errorf("operation %s: %w", op.Kind().String(), err)
		}
	}

	c, staticData, stackPointerCeil, err := compiler.compile()
	if err != nil {
		return nil, fmt.Errorf("failed to compile: %w", err)
	}

	return &code{codeSegment: c, stackPointerCeil: stackPointerCeil, staticData: staticData}, nil
}
