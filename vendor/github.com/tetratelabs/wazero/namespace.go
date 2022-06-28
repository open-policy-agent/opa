package wazero

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	experimentalapi "github.com/tetratelabs/wazero/experimental"
	internalsys "github.com/tetratelabs/wazero/internal/sys"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/sys"
)

// Namespace contains instantiated modules, which cannot conflict until they are closed.
type Namespace interface {
	// Module returns exports from an instantiated module in this namespace or nil if there aren't any.
	Module(moduleName string) api.Module

	// InstantiateModule instantiates the module namespace or errs if the configuration was invalid.
	// When the context is nil, it defaults to context.Background.
	//
	// Ex.
	//	module, _ := n.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName("prod"))
	//
	// While CompiledModule is pre-validated, there are a few situations which can cause an error:
	//	* The module name is already in use.
	//	* The module has a table element initializer that resolves to an index outside the Table minimum size.
	//	* The module has a start function, and it failed to execute.
	InstantiateModule(ctx context.Context, compiled CompiledModule, config ModuleConfig) (api.Module, error)

	// CloseWithExitCode closes all modules initialized in this Namespace with the provided exit code.
	// An error is returned if any module returns an error when closed.
	//
	// Ex.
	//	n := r.NewNamespace(ctx)
	//	defer n.CloseWithExitCode(ctx, 2) // This closes all modules in this Namespace.
	//
	//	Everything below here can be closed, but will anyway due to above.
	//	_, _ = wasi_snapshot_preview1.InstantiateSnapshotPreview1(ctx, n)
	//	mod, _ := n.InstantiateModuleFromBinary(ctx, wasm)
	//
	// See Closer
	CloseWithExitCode(ctx context.Context, exitCode uint32) error

	// Closer closes modules initialized in this Namespace by delegating to CloseWithExitCode with an exit code of zero.
	//
	// Ex.
	//	n := r.NewNamespace(ctx)
	//	defer n.Close(ctx) // This closes all modules in this Namespace.
	api.Closer
}

// namespace allows decoupling of public interfaces from internal representation.
type namespace struct {
	store *wasm.Store
	ns    *wasm.Namespace
}

// Module implements Namespace.Module.
func (ns *namespace) Module(moduleName string) api.Module {
	return ns.ns.Module(moduleName)
}

// InstantiateModule implements Namespace.InstantiateModule
func (ns *namespace) InstantiateModule(
	ctx context.Context,
	compiled CompiledModule,
	mConfig ModuleConfig,
) (mod api.Module, err error) {
	code, ok := compiled.(*compiledModule)
	if !ok {
		panic(fmt.Errorf("unsupported wazero.CompiledModule implementation: %#v", compiled))
	}

	config, ok := mConfig.(*moduleConfig)
	if !ok {
		panic(fmt.Errorf("unsupported wazero.ModuleConfig implementation: %#v", mConfig))
	}

	var sysCtx *internalsys.Context
	if sysCtx, err = config.toSysContext(); err != nil {
		return
	}

	name := config.name
	if name == "" && code.module.NameSection != nil && code.module.NameSection.ModuleName != "" {
		name = code.module.NameSection.ModuleName
	}

	var functionListenerFactory experimentalapi.FunctionListenerFactory
	if ctx != nil { // Test to see if internal code are using an experimental feature.
		if fnlf := ctx.Value(experimentalapi.FunctionListenerFactoryKey{}); fnlf != nil {
			functionListenerFactory = fnlf.(experimentalapi.FunctionListenerFactory)
		}
	}

	// Instantiate the module in the appropriate namespace.
	mod, err = ns.store.Instantiate(ctx, ns.ns, code.module, name, sysCtx, functionListenerFactory)
	if err != nil {
		// If there was an error, don't leak the compiled module.
		if code.closeWithModule {
			_ = code.Close(ctx) // don't overwrite the error
		}
		return
	}

	// Attach the code closer so that anything afterwards closes the compiled code when closing the module.
	if code.closeWithModule {
		mod.(*wasm.CallContext).CodeCloser = code
	}

	// Now, invoke any start functions, failing at first error.
	for _, fn := range config.startFunctions {
		start := mod.ExportedFunction(fn)
		if start == nil {
			continue
		}
		if _, err = start.Call(ctx); err != nil {
			_ = mod.Close(ctx) // Don't leak the module on error.
			if _, ok = err.(*sys.ExitError); ok {
				return // Don't wrap an exit error
			}
			err = fmt.Errorf("module[%s] function[%s] failed: %w", name, fn, err)
			return
		}
	}
	return
}

// Close implements api.Closer embedded in Namespace.
func (ns *namespace) Close(ctx context.Context) error {
	return ns.CloseWithExitCode(ctx, 0)
}

// CloseWithExitCode implements Namespace.CloseWithExitCode
func (ns *namespace) CloseWithExitCode(ctx context.Context, exitCode uint32) error {
	return ns.ns.CloseWithExitCode(ctx, exitCode)
}
