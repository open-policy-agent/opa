package wazero

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/u64"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// ModuleBuilder is a way to define a WebAssembly 1.0 (20191205) in Go.
//
// Ex. Below defines and instantiates a module named "env" with one function:
//
//	ctx := context.Background()
//	r := wazero.NewRuntime()
//	defer r.Close(ctx) // This closes everything this Runtime created.
//
//	hello := func() {
//		fmt.Fprintln(stdout, "hello!")
//	}
//	env, _ := r.NewModuleBuilder("env").
//		ExportFunction("hello", hello).
//		Instantiate(ctx, r)
//
// If the same module may be instantiated multiple times, it is more efficient to separate steps. Ex.
//
//	compiled, _ := r.NewModuleBuilder("env").
//		ExportFunction("get_random_string", getRandomString).
//		Compile(ctx, wazero.NewCompileConfig())
//
//	env1, _ := r.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName("env.1"))
//
//	env2, _ := r.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName("env.2"))
//
// Notes
//
//	* ModuleBuilder is mutable. WithXXX functions return the same instance for chaining.
//	* WithXXX methods do not return errors, to allow chaining. Any validation errors are deferred until Build.
//	* Insertion order is not retained. Anything defined by this builder is sorted lexicographically on Build.
type ModuleBuilder interface {
	// Note: until golang/go#5860, we can't use example tests to embed code in interface godocs.

	// ExportFunction adds a function written in Go, which a WebAssembly module can import.
	// If a function is already exported with the same name, this overwrites it.
	//
	// Parameters
	//
	//	* name - the name to export. Ex "random_get"
	//	* goFunc - the `func` to export.
	//
	// Noting a context exception described later, all parameters or result types must match WebAssembly 1.0 (20191205) value
	// types. This means uint32, uint64, float32 or float64. Up to one result can be returned.
	//
	// Ex. This is a valid host function:
	//
	//	addInts := func(x uint32, uint32) uint32 {
	//		return x + y
	//	}
	//
	// Host functions may also have an initial parameter (param[0]) of type context.Context or api.Module.
	//
	// Ex. This uses a Go Context:
	//
	//	addInts := func(ctx context.Context, x uint32, uint32) uint32 {
	//		// add a little extra if we put some in the context!
	//		return x + y + ctx.Value(extraKey).(uint32)
	//	}
	//
	// Ex. This uses an api.Module to reads the parameters from memory. This is important because there are only numeric
	// types in Wasm. The only way to share other data is via writing memory and sharing offsets.
	//
	//	addInts := func(ctx context.Context, m api.Module, offset uint32) uint32 {
	//		x, _ := m.Memory().ReadUint32Le(ctx, offset)
	//		y, _ := m.Memory().ReadUint32Le(ctx, offset + 4) // 32 bits == 4 bytes!
	//		return x + y
	//	}
	//
	// If both parameters exist, they must be in order at positions zero and one.
	//
	// Ex. This uses propagates context properly when calling other functions exported in the api.Module:
	//	callRead := func(ctx context.Context, m api.Module, offset, byteCount uint32) uint32 {
	//		fn = m.ExportedFunction("__read")
	//		results, err := fn(ctx, offset, byteCount)
	//	--snip--
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#host-functions%E2%91%A2
	ExportFunction(name string, goFunc interface{}) ModuleBuilder

	// ExportFunctions is a convenience that calls ExportFunction for each key/value in the provided map.
	ExportFunctions(nameToGoFunc map[string]interface{}) ModuleBuilder

	// ExportMemory adds linear memory, which a WebAssembly module can import and become available via api.Memory.
	// If a memory is already exported with the same name, this overwrites it.
	//
	// Parameters
	//
	//	* name - the name to export. Ex "memory" for wasi_snapshot_preview1.ModuleSnapshotPreview1
	//	* minPages - the possibly zero initial size in pages (65536 bytes per page).
	//
	// For example, the WebAssembly 1.0 Text Format below is the equivalent of this builder method:
	//	// (memory (export "memory") 1)
	//	builder.ExportMemory(1)
	//
	// Notes
	//
	//	* This is allowed to grow to (4GiB) limited by api.MemorySizer. To bound it, use ExportMemoryWithMax.
	//	* Version 1.0 (20191205) of the WebAssembly spec allows at most one memory per module.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#memory-section%E2%91%A0
	ExportMemory(name string, minPages uint32) ModuleBuilder

	// ExportMemoryWithMax is like ExportMemory, but can prevent overuse of memory.
	//
	// For example, the WebAssembly 1.0 Text Format below is the equivalent of this builder method:
	//	// (memory (export "memory") 1 1)
	//	builder.ExportMemoryWithMax(1, 1)
	//
	// Note: api.MemorySizer determines the capacity.
	ExportMemoryWithMax(name string, minPages, maxPages uint32) ModuleBuilder

	// ExportGlobalI32 exports a global constant of type api.ValueTypeI32.
	// If a global is already exported with the same name, this overwrites it.
	//
	// For example, the WebAssembly 1.0 Text Format below is the equivalent of this builder method:
	//	// (global (export "canvas_width") i32 (i32.const 1024))
	//	builder.ExportGlobalI32("canvas_width", 1024)
	//
	// Note: The maximum value of v is math.MaxInt32 to match constraints of initialization in binary format.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#value-types%E2%91%A0 and
	// https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#syntax-globaltype
	ExportGlobalI32(name string, v int32) ModuleBuilder

	// ExportGlobalI64 exports a global constant of type api.ValueTypeI64.
	// If a global is already exported with the same name, this overwrites it.
	//
	// For example, the WebAssembly 1.0 Text Format below is the equivalent of this builder method:
	//	// (global (export "start_epoch") i64 (i64.const 1620216263544))
	//	builder.ExportGlobalI64("start_epoch", 1620216263544)
	//
	// Note: The maximum value of v is math.MaxInt64 to match constraints of initialization in binary format.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#value-types%E2%91%A0 and
	// https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#syntax-globaltype
	ExportGlobalI64(name string, v int64) ModuleBuilder

	// ExportGlobalF32 exports a global constant of type api.ValueTypeF32.
	// If a global is already exported with the same name, this overwrites it.
	//
	// For example, the WebAssembly 1.0 Text Format below is the equivalent of this builder method:
	//	// (global (export "math/pi") f32 (f32.const 3.1415926536))
	//	builder.ExportGlobalF32("math/pi", 3.1415926536)
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#syntax-globaltype
	ExportGlobalF32(name string, v float32) ModuleBuilder

	// ExportGlobalF64 exports a global constant of type api.ValueTypeF64.
	// If a global is already exported with the same name, this overwrites it.
	//
	// For example, the WebAssembly 1.0 Text Format below is the equivalent of this builder method:
	//	// (global (export "math/pi") f64 (f64.const 3.14159265358979323846264338327950288419716939937510582097494459))
	//	builder.ExportGlobalF64("math/pi", 3.14159265358979323846264338327950288419716939937510582097494459)
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#syntax-globaltype
	ExportGlobalF64(name string, v float64) ModuleBuilder

	// Compile returns a CompiledModule that can instantiated in any namespace (Namespace).
	//
	// Note: Closing the Namespace has the same effect as closing the result.
	Compile(context.Context, CompileConfig) (CompiledModule, error)

	// Instantiate is a convenience that calls Compile, then Namespace.InstantiateModule.
	//
	// Notes
	//
	//	* Closing the Namespace has the same effect as closing the result.
	//	* Fields in the builder are copied during instantiation: Later changes do not affect the instantiated result.
	//	* To avoid using configuration defaults, use Compile instead.
	Instantiate(context.Context, Namespace) (api.Module, error)
}

// moduleBuilder implements ModuleBuilder
type moduleBuilder struct {
	r            *runtime
	moduleName   string
	nameToGoFunc map[string]interface{}
	nameToMemory map[string]*wasm.Memory
	nameToGlobal map[string]*wasm.Global
}

// NewModuleBuilder implements Runtime.NewModuleBuilder
func (r *runtime) NewModuleBuilder(moduleName string) ModuleBuilder {
	return &moduleBuilder{
		r:            r,
		moduleName:   moduleName,
		nameToGoFunc: map[string]interface{}{},
		nameToMemory: map[string]*wasm.Memory{},
		nameToGlobal: map[string]*wasm.Global{},
	}
}

// ExportFunction implements ModuleBuilder.ExportFunction
func (b *moduleBuilder) ExportFunction(name string, goFunc interface{}) ModuleBuilder {
	b.nameToGoFunc[name] = goFunc
	return b
}

// ExportFunctions implements ModuleBuilder.ExportFunctions
func (b *moduleBuilder) ExportFunctions(nameToGoFunc map[string]interface{}) ModuleBuilder {
	for k, v := range nameToGoFunc {
		b.ExportFunction(k, v)
	}
	return b
}

// ExportMemory implements ModuleBuilder.ExportMemory
func (b *moduleBuilder) ExportMemory(name string, minPages uint32) ModuleBuilder {
	b.nameToMemory[name] = &wasm.Memory{Min: minPages}
	return b
}

// ExportMemoryWithMax implements ModuleBuilder.ExportMemoryWithMax
func (b *moduleBuilder) ExportMemoryWithMax(name string, minPages, maxPages uint32) ModuleBuilder {
	b.nameToMemory[name] = &wasm.Memory{Min: minPages, Max: maxPages, IsMaxEncoded: true}
	return b
}

// ExportGlobalI32 implements ModuleBuilder.ExportGlobalI32
func (b *moduleBuilder) ExportGlobalI32(name string, v int32) ModuleBuilder {
	b.nameToGlobal[name] = &wasm.Global{
		Type: &wasm.GlobalType{ValType: wasm.ValueTypeI32},
		// Treat constants as signed as their interpretation is not yet known per /RATIONALE.md
		Init: &wasm.ConstantExpression{Opcode: wasm.OpcodeI32Const, Data: leb128.EncodeInt32(v)},
	}
	return b
}

// ExportGlobalI64 implements ModuleBuilder.ExportGlobalI64
func (b *moduleBuilder) ExportGlobalI64(name string, v int64) ModuleBuilder {
	b.nameToGlobal[name] = &wasm.Global{
		Type: &wasm.GlobalType{ValType: wasm.ValueTypeI64},
		// Treat constants as signed as their interpretation is not yet known per /RATIONALE.md
		Init: &wasm.ConstantExpression{Opcode: wasm.OpcodeI64Const, Data: leb128.EncodeInt64(v)},
	}
	return b
}

// ExportGlobalF32 implements ModuleBuilder.ExportGlobalF32
func (b *moduleBuilder) ExportGlobalF32(name string, v float32) ModuleBuilder {
	b.nameToGlobal[name] = &wasm.Global{
		Type: &wasm.GlobalType{ValType: wasm.ValueTypeF32},
		Init: &wasm.ConstantExpression{Opcode: wasm.OpcodeF32Const, Data: u64.LeBytes(api.EncodeF32(v))},
	}
	return b
}

// ExportGlobalF64 implements ModuleBuilder.ExportGlobalF64
func (b *moduleBuilder) ExportGlobalF64(name string, v float64) ModuleBuilder {
	b.nameToGlobal[name] = &wasm.Global{
		Type: &wasm.GlobalType{ValType: wasm.ValueTypeF64},
		Init: &wasm.ConstantExpression{Opcode: wasm.OpcodeF64Const, Data: u64.LeBytes(api.EncodeF64(v))},
	}
	return b
}

// Compile implements ModuleBuilder.Compile
func (b *moduleBuilder) Compile(ctx context.Context, cConfig CompileConfig) (CompiledModule, error) {
	config, ok := cConfig.(*compileConfig)
	if !ok {
		panic(fmt.Errorf("unsupported wazero.CompileConfig implementation: %#v", cConfig))
	}

	// Verify the maximum limit here, so we don't have to pass it to wasm.NewHostModule
	for name, mem := range b.nameToMemory {
		var maxP *uint32
		if mem.IsMaxEncoded {
			maxP = &mem.Max
		}
		mem.Min, mem.Cap, mem.Max = config.memorySizer(mem.Min, maxP)
		if err := mem.Validate(); err != nil {
			return nil, fmt.Errorf("memory[%s] %v", name, err)
		}
	}

	module, err := wasm.NewHostModule(b.moduleName, b.nameToGoFunc, b.nameToMemory, b.nameToGlobal, b.r.enabledFeatures)
	if err != nil {
		return nil, err
	}

	if err = b.r.store.Engine.CompileModule(ctx, module); err != nil {
		return nil, err
	}

	return &compiledModule{module: module, compiledEngine: b.r.store.Engine}, nil
}

// Instantiate implements ModuleBuilder.Instantiate
func (b *moduleBuilder) Instantiate(ctx context.Context, ns Namespace) (api.Module, error) {
	if compiled, err := b.Compile(ctx, NewCompileConfig()); err != nil {
		return nil, err
	} else {
		compiled.(*compiledModule).closeWithModule = true
		return ns.InstantiateModule(ctx, compiled, NewModuleConfig())
	}
}
