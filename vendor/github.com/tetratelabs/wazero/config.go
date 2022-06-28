package wazero

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"math"
	"time"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/engine/compiler"
	"github.com/tetratelabs/wazero/internal/engine/interpreter"
	"github.com/tetratelabs/wazero/internal/platform"
	internalsys "github.com/tetratelabs/wazero/internal/sys"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/sys"
)

// RuntimeConfig controls runtime behavior, with the default implementation as NewRuntimeConfig
//
// Ex. To explicitly limit to Wasm Core 1.0 features as opposed to relying on defaults:
//	rConfig = wazero.NewRuntimeConfig().WithWasmCore1()
//
// Note: RuntimeConfig is immutable. Each WithXXX function returns a new instance including the corresponding change.
type RuntimeConfig interface {

	// WithFeatureBulkMemoryOperations adds instructions modify ranges of memory or table entries
	// ("bulk-memory-operations"). This defaults to false as the feature was not finished in WebAssembly 1.0.
	//
	// Here are the notable effects:
	//	* Adds `memory.fill`, `memory.init`, `memory.copy` and `data.drop` instructions.
	//	* Adds `table.init`, `table.copy` and `elem.drop` instructions.
	//	* Introduces a "passive" form of element and data segments.
	//	* Stops checking "active" element and data segment boundaries at compile-time, meaning they can error at runtime.
	//
	// Note: "bulk-memory-operations" is mixed with the "reference-types" proposal
	// due to the WebAssembly Working Group merging them "mutually dependent".
	// Therefore, enabling this feature results in enabling WithFeatureReferenceTypes, and vice-versa.
	//
	// See https://github.com/WebAssembly/spec/blob/main/proposals/bulk-memory-operations/Overview.md
	// https://github.com/WebAssembly/spec/blob/main/proposals/reference-types/Overview.md and
	// https://github.com/WebAssembly/spec/pull/1287
	WithFeatureBulkMemoryOperations(bool) RuntimeConfig

	// WithFeatureMultiValue enables multiple values ("multi-value"). This defaults to false as the feature was not
	// finished in WebAssembly 1.0 (20191205).
	//
	// Here are the notable effects:
	//	* Function (`func`) types allow more than one result
	//	* Block types (`block`, `loop` and `if`) can be arbitrary function types
	//
	// See https://github.com/WebAssembly/spec/blob/main/proposals/multi-value/Overview.md
	WithFeatureMultiValue(bool) RuntimeConfig

	// WithFeatureMutableGlobal allows globals to be mutable. This defaults to true as the feature was finished in
	// WebAssembly 1.0 (20191205).
	//
	// When false, an api.Global can never be cast to an api.MutableGlobal, and any wasm that includes global vars
	// will fail to parse.
	WithFeatureMutableGlobal(bool) RuntimeConfig

	// WithFeatureNonTrappingFloatToIntConversion enables non-trapping float-to-int conversions.
	// ("nontrapping-float-to-int-conversion"). This defaults to false as the feature was not in WebAssembly 1.0.
	//
	// The only effect of enabling is allowing the following instructions, which return 0 on NaN instead of panicking.
	//	* `i32.trunc_sat_f32_s`
	//	* `i32.trunc_sat_f32_u`
	//	* `i32.trunc_sat_f64_s`
	//	* `i32.trunc_sat_f64_u`
	//	* `i64.trunc_sat_f32_s`
	//	* `i64.trunc_sat_f32_u`
	//	* `i64.trunc_sat_f64_s`
	//	* `i64.trunc_sat_f64_u`
	//
	// See https://github.com/WebAssembly/spec/blob/main/proposals/nontrapping-float-to-int-conversion/Overview.md
	WithFeatureNonTrappingFloatToIntConversion(bool) RuntimeConfig

	// WithFeatureReferenceTypes enables various instructions and features related to table and new reference types.
	//
	//	* Introduction of new value types: `funcref` and `externref`.
	//	* Support for the following new instructions:
	//	 * `ref.null`
	//	 * `ref.func`
	//	 * `ref.is_null`
	//	 * `table.fill`
	//	 * `table.get`
	//	 * `table.grow`
	//	 * `table.set`
	//	 * `table.size`
	//	* Support for multiple tables per module:
	//	 * `call_indirect`, `table.init`, `table.copy` and `elem.drop` instructions can take non-zero table index.
	//	 * Element segments can take non-zero table index.
	//
	// Note: "reference-types" is mixed with the "bulk-memory-operations" proposal
	// due to the WebAssembly Working Group merging them "mutually dependent".
	// Therefore, enabling this feature results in enabling WithFeatureBulkMemoryOperations, and vice-versa.
	//
	// See https://github.com/WebAssembly/spec/blob/main/proposals/bulk-memory-operations/Overview.md
	// https://github.com/WebAssembly/spec/blob/main/proposals/reference-types/Overview.md and
	// https://github.com/WebAssembly/spec/pull/1287
	WithFeatureReferenceTypes(enabled bool) RuntimeConfig

	// WithFeatureSignExtensionOps enables sign extension instructions ("sign-extension-ops"). This defaults to false
	// as the feature was not in WebAssembly 1.0.
	//
	// Here are the notable effects:
	//	* Adds instructions `i32.extend8_s`, `i32.extend16_s`, `i64.extend8_s`, `i64.extend16_s` and `i64.extend32_s`
	//
	// See https://github.com/WebAssembly/spec/blob/main/proposals/sign-extension-ops/Overview.md
	WithFeatureSignExtensionOps(bool) RuntimeConfig

	// WithFeatureSIMD enables the vector value type and vector instructions (aka SIMD). This defaults to false
	// as the feature was not in WebAssembly 1.0.
	//
	// See https://github.com/WebAssembly/spec/blob/main/proposals/simd/SIMD.md
	WithFeatureSIMD(bool) RuntimeConfig

	// WithWasmCore1 enables features included in the WebAssembly Core Specification 1.0. Selecting this
	// overwrites any currently accumulated features with only those included in this W3C recommendation.
	//
	// This is default because as of mid 2022, this is the only version that is a Web Standard (W3C Recommendation).
	//
	// You can select the latest draft of the WebAssembly Core Specification 2.0 instead via WithWasmCore2. You can
	// also enable or disable individual features via `WithXXX` methods. Ex.
	//	rConfig = wazero.NewRuntimeConfig().WithWasmCore1().WithFeatureMutableGlobal(false)
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/
	WithWasmCore1() RuntimeConfig

	// WithWasmCore2 enables features included in the WebAssembly Core Specification 2.0 (20220419). Selecting this
	// overwrites any currently accumulated features with only those included in this W3C working draft.
	//
	// This is not default because it is not yet incomplete and also not yet a Web Standard (W3C Recommendation).
	//
	// Even after selecting this, you can enable or disable individual features via `WithXXX` methods. Ex.
	//	rConfig = wazero.NewRuntimeConfig().WithWasmCore2().WithFeatureMutableGlobal(false)
	//
	// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/
	WithWasmCore2() RuntimeConfig
}

// NewRuntimeConfig returns a RuntimeConfig using the compiler if it is supported in this environment,
// or the interpreter otherwise.
func NewRuntimeConfig() RuntimeConfig {
	return newRuntimeConfig()
}

type runtimeConfig struct {
	enabledFeatures wasm.Features
	newEngine       func(wasm.Features) wasm.Engine
}

// engineLessConfig helps avoid copy/pasting the wrong defaults.
var engineLessConfig = &runtimeConfig{
	enabledFeatures: wasm.Features20191205,
}

// NewRuntimeConfigCompiler compiles WebAssembly modules into
// runtime.GOARCH-specific assembly for optimal performance.
//
// The default implementation is AOT (Ahead of Time) compilation, applied at
// Runtime.CompileModule. This allows consistent runtime performance, as well
// the ability to reduce any first request penalty.
//
// Note: While this is technically AOT, this does not imply any action on your
// part. wazero automatically performs ahead-of-time compilation as needed when
// Runtime.CompileModule is invoked.
//
// Warning: This panics at runtime if the runtime.GOOS or runtime.GOARCH does not
// support Compiler. Use NewRuntimeConfig to safely detect and fallback to
// NewRuntimeConfigInterpreter if needed.
func NewRuntimeConfigCompiler() RuntimeConfig {
	ret := engineLessConfig.clone()
	ret.newEngine = compiler.NewEngine
	return ret
}

// NewRuntimeConfigInterpreter interprets WebAssembly modules instead of compiling them into assembly.
func NewRuntimeConfigInterpreter() RuntimeConfig {
	ret := engineLessConfig.clone()
	ret.newEngine = interpreter.NewEngine
	return ret
}

// clone makes a deep copy of this runtime config.
func (c *runtimeConfig) clone() *runtimeConfig {
	ret := *c // copy except maps which share a ref
	return &ret
}

// WithFeatureBulkMemoryOperations implements RuntimeConfig.WithFeatureBulkMemoryOperations
func (c *runtimeConfig) WithFeatureBulkMemoryOperations(enabled bool) RuntimeConfig {
	ret := c.clone()
	ret.enabledFeatures = ret.enabledFeatures.Set(wasm.FeatureBulkMemoryOperations, enabled)
	// bulk-memory-operations proposal is mutually-dependant with reference-types proposal.
	ret.enabledFeatures = ret.enabledFeatures.Set(wasm.FeatureReferenceTypes, enabled)
	return ret
}

// WithFeatureMultiValue implements RuntimeConfig.WithFeatureMultiValue
func (c *runtimeConfig) WithFeatureMultiValue(enabled bool) RuntimeConfig {
	ret := c.clone()
	ret.enabledFeatures = ret.enabledFeatures.Set(wasm.FeatureMultiValue, enabled)
	return ret
}

// WithFeatureMutableGlobal implements RuntimeConfig.WithFeatureMutableGlobal
func (c *runtimeConfig) WithFeatureMutableGlobal(enabled bool) RuntimeConfig {
	ret := c.clone()
	ret.enabledFeatures = ret.enabledFeatures.Set(wasm.FeatureMutableGlobal, enabled)
	return ret
}

// WithFeatureNonTrappingFloatToIntConversion implements RuntimeConfig.WithFeatureNonTrappingFloatToIntConversion
func (c *runtimeConfig) WithFeatureNonTrappingFloatToIntConversion(enabled bool) RuntimeConfig {
	ret := c.clone()
	ret.enabledFeatures = ret.enabledFeatures.Set(wasm.FeatureNonTrappingFloatToIntConversion, enabled)
	return ret
}

// WithFeatureReferenceTypes implements RuntimeConfig.WithFeatureReferenceTypes
func (c *runtimeConfig) WithFeatureReferenceTypes(enabled bool) RuntimeConfig {
	ret := c.clone()
	ret.enabledFeatures = ret.enabledFeatures.Set(wasm.FeatureReferenceTypes, enabled)
	// reference-types proposal is mutually-dependant with bulk-memory-operations proposal.
	ret.enabledFeatures = ret.enabledFeatures.Set(wasm.FeatureBulkMemoryOperations, enabled)
	return ret
}

// WithFeatureSignExtensionOps implements RuntimeConfig.WithFeatureSignExtensionOps
func (c *runtimeConfig) WithFeatureSignExtensionOps(enabled bool) RuntimeConfig {
	ret := c.clone()
	ret.enabledFeatures = ret.enabledFeatures.Set(wasm.FeatureSignExtensionOps, enabled)
	return ret
}

// WithFeatureSIMD implements RuntimeConfig.WithFeatureSIMD
func (c *runtimeConfig) WithFeatureSIMD(enabled bool) RuntimeConfig {
	ret := c.clone()
	ret.enabledFeatures = ret.enabledFeatures.Set(wasm.FeatureSIMD, enabled)
	return ret
}

// WithWasmCore1 implements RuntimeConfig.WithWasmCore1
func (c *runtimeConfig) WithWasmCore1() RuntimeConfig {
	ret := c.clone()
	ret.enabledFeatures = wasm.Features20191205
	return ret
}

// WithWasmCore2 implements RuntimeConfig.WithWasmCore2
func (c *runtimeConfig) WithWasmCore2() RuntimeConfig {
	ret := c.clone()
	ret.enabledFeatures = wasm.Features20220419
	return ret
}

// CompiledModule is a WebAssembly 1.0 module ready to be instantiated (Runtime.InstantiateModule) as an api.Module.
//
// In WebAssembly terminology, this is a decoded, validated, and possibly also compiled module. wazero avoids using
// the name "Module" for both before and after instantiation as the name conflation has caused confusion.
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#semantic-phases%E2%91%A0
//
// Note: Closing the wazero.Runtime closes any CompiledModule it compiled.
type CompiledModule interface {
	// Close releases all the allocated resources for this CompiledModule.
	//
	// Note: It is safe to call Close while having outstanding calls from an api.Module instantiated from this.
	Close(context.Context) error
}

type compiledModule struct {
	module *wasm.Module
	// compiledEngine holds an engine on which `module` is compiled.
	compiledEngine wasm.Engine

	// closeWithModule prevents leaking compiled code when a module is compiled implicitly.
	closeWithModule bool
}

// Close implements CompiledModule.Close
func (c *compiledModule) Close(_ context.Context) error {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	c.compiledEngine.DeleteCompiledModule(c.module)
	// It is possible the underlying may need to return an error later, but in any case this matches api.Module.Close.
	return nil
}

// CompileConfig allows you to override what was decoded from wasm, prior to compilation (ModuleBuilder.Compile or
// Runtime.CompileModule).
//
// For example, WithImportRenamer allows you to override hard-coded names that don't match your requirements.
//
// Note: CompileConfig is immutable. Each WithXXX function returns a new instance including the corresponding change.
type CompileConfig interface {

	// WithImportRenamer can rename imports or break them into different modules. No default.
	// A nil function is invalid and ignored.
	//
	// Note: This is currently not relevant for ModuleBuilder as it has no means to define imports.
	WithImportRenamer(api.ImportRenamer) CompileConfig

	// WithMemorySizer are the allocation parameters used for a Wasm memory.
	// The default is to set cap=min and max=65536 if unset. A nil function is invalid and ignored.
	WithMemorySizer(api.MemorySizer) CompileConfig
}

type compileConfig struct {
	importRenamer api.ImportRenamer
	memorySizer   api.MemorySizer
}

// NewCompileConfig returns a CompileConfig that can be used for configuring module compilation.
func NewCompileConfig() CompileConfig {
	return &compileConfig{
		importRenamer: nil,
		memorySizer:   wasm.MemorySizer,
	}
}

// clone makes a deep copy of this compile config.
func (c *compileConfig) clone() *compileConfig {
	ret := *c // copy except maps which share a ref
	return &ret
}

// WithImportRenamer implements CompileConfig.WithImportRenamer
func (c *compileConfig) WithImportRenamer(importRenamer api.ImportRenamer) CompileConfig {
	if importRenamer == nil {
		return c
	}
	ret := c.clone()
	ret.importRenamer = importRenamer
	return ret
}

// WithMemorySizer implements CompileConfig.WithMemorySizer
func (c *compileConfig) WithMemorySizer(memorySizer api.MemorySizer) CompileConfig {
	if memorySizer == nil {
		return c
	}
	ret := c.clone()
	ret.memorySizer = memorySizer
	return ret
}

// ModuleConfig configures resources needed by functions that have low-level interactions with the host operating
// system. Using this, resources such as STDIN can be isolated, so that the same module can be safely instantiated
// multiple times.
//
// Ex.
//	// Initialize base configuration:
//	config := wazero.NewModuleConfig().WithStdout(buf).WithSysNanotime()
//
//	// Assign different configuration on each instantiation
//	module, _ := r.InstantiateModule(ctx, compiled, config.WithName("rotate").WithArgs("rotate", "angle=90", "dir=cw"))
//
// While wazero supports Windows as a platform, host functions using ModuleConfig follow a UNIX dialect.
// See RATIONALE.md for design background and relationship to WebAssembly System Interfaces (WASI).
//
// Note: ModuleConfig is immutable. Each WithXXX function returns a new instance including the corresponding change.
type ModuleConfig interface {

	// WithArgs assigns command-line arguments visible to an imported function that reads an arg vector (argv). Defaults to
	// none. Runtime.InstantiateModule errs if any arg is empty.
	//
	// These values are commonly read by the functions like "args_get" in "wasi_snapshot_preview1" although they could be
	// read by functions imported from other modules.
	//
	// Similar to os.Args and exec.Cmd Env, many implementations would expect a program name to be argv[0]. However, neither
	// WebAssembly nor WebAssembly System Interfaces (WASI) define this. Regardless, you may choose to set the first
	// argument to the same value set via WithName.
	//
	// Note: This does not default to os.Args as that violates sandboxing.
	//
	// See https://linux.die.net/man/3/argv and https://en.wikipedia.org/wiki/Null-terminated_string
	WithArgs(...string) ModuleConfig

	// WithEnv sets an environment variable visible to a Module that imports functions. Defaults to none.
	// Runtime.InstantiateModule errs if the key is empty or contains a NULL(0) or equals("") character.
	//
	// Validation is the same as os.Setenv on Linux and replaces any existing value. Unlike exec.Cmd Env, this does not
	// default to the current process environment as that would violate sandboxing. This also does not preserve order.
	//
	// Environment variables are commonly read by the functions like "environ_get" in "wasi_snapshot_preview1" although
	// they could be read by functions imported from other modules.
	//
	// While similar to process configuration, there are no assumptions that can be made about anything OS-specific. For
	// example, neither WebAssembly nor WebAssembly System Interfaces (WASI) define concerns processes have, such as
	// case-sensitivity on environment keys. For portability, define entries with case-insensitively unique keys.
	//
	// See https://linux.die.net/man/3/environ and https://en.wikipedia.org/wiki/Null-terminated_string
	WithEnv(key, value string) ModuleConfig

	// WithFS assigns the file system to use for any paths beginning at "/". Defaults to not found.
	// Note: This sets WithWorkDirFS to the same file-system unless already set.
	//
	// Ex. This sets a read-only, embedded file-system to serve files under the root ("/") and working (".") directories:
	//
	//	//go:embed testdata/index.html
	//	var testdataIndex embed.FS
	//
	//	rooted, err := fs.Sub(testdataIndex, "testdata")
	//	require.NoError(t, err)
	//
	//	// "index.html" is accessible as both "/index.html" and "./index.html" because we didn't use WithWorkDirFS.
	//	config := wazero.NewModuleConfig().WithFS(rooted)
	//
	WithFS(fs.FS) ModuleConfig

	// WithName configures the module name. Defaults to what was decoded or overridden via CompileConfig.WithModuleName.
	WithName(string) ModuleConfig

	// WithStartFunctions configures the functions to call after the module is instantiated. Defaults to "_start".
	//
	// Note: If any function doesn't exist, it is skipped. However, all functions that do exist are called in order.
	WithStartFunctions(...string) ModuleConfig

	// WithStderr configures where standard error (file descriptor 2) is written. Defaults to io.Discard.
	//
	// This writer is most commonly used by the functions like "fd_write" in "wasi_snapshot_preview1" although it could
	// be used by functions imported from other modules.
	//
	// Notes
	//
	//	* The caller is responsible to close any io.Writer they supply: It is not closed on api.Module Close.
	//	* This does not default to os.Stderr as that both violates sandboxing and prevents concurrent modules.
	//
	// See https://linux.die.net/man/3/stderr
	WithStderr(io.Writer) ModuleConfig

	// WithStdin configures where standard input (file descriptor 0) is read. Defaults to return io.EOF.
	//
	// This reader is most commonly used by the functions like "fd_read" in "wasi_snapshot_preview1" although it could
	// be used by functions imported from other modules.
	//
	// Notes
	//
	//	* The caller is responsible to close any io.Reader they supply: It is not closed on api.Module Close.
	//	* This does not default to os.Stdin as that both violates sandboxing and prevents concurrent modules.
	//
	// See https://linux.die.net/man/3/stdin
	WithStdin(io.Reader) ModuleConfig

	// WithStdout configures where standard output (file descriptor 1) is written. Defaults to io.Discard.
	//
	// This writer is most commonly used by the functions like "fd_write" in "wasi_snapshot_preview1" although it could
	// be used by functions imported from other modules.
	//
	// Notes
	//
	//	* The caller is responsible to close any io.Writer they supply: It is not closed on api.Module Close.
	//	* This does not default to os.Stdout as that both violates sandboxing and prevents concurrent modules.
	//
	// See https://linux.die.net/man/3/stdout
	WithStdout(io.Writer) ModuleConfig

	// WithWalltime configures the wall clock, sometimes referred to as the
	// real time clock. Defaults to a fake result that increases by 1ms on
	// each reading.
	//
	// Ex. To override with your own clock:
	//	moduleConfig = moduleConfig.
	//		WithWalltime(func(context.Context) (sec int64, nsec int32) {
	//			return clock.walltime()
	//		}, sys.ClockResolution(time.Microsecond.Nanoseconds()))
	//
	// Note: This does not default to time.Now as that violates sandboxing. Use
	// WithSysWalltime for a usable implementation.
	WithWalltime(sys.Walltime, sys.ClockResolution) ModuleConfig

	// WithSysWalltime uses time.Now for sys.Walltime with a resolution of 1us
	// (1000ns).
	//
	// See WithWalltime
	WithSysWalltime() ModuleConfig

	// WithNanotime configures the monotonic clock, used to measure elapsed
	// time in nanoseconds. Defaults to a fake result that increases by 1ms
	// on each reading.
	//
	// Ex. To override with your own clock:
	//	moduleConfig = moduleConfig.
	//		WithNanotime(func(context.Context) int64 {
	//			return clock.nanotime()
	//		}, sys.ClockResolution(time.Microsecond.Nanoseconds()))
	//
	// Notes:
	//	* This does not default to time.Since as that violates sandboxing.
	//	* Some compilers implement sleep by looping on sys.Nanotime (ex. Go).
	//	* If you set this, you should probably set WithNanosleep also.
	//	* Use WithSysNanotime for a usable implementation.
	WithNanotime(sys.Nanotime, sys.ClockResolution) ModuleConfig

	// WithSysNanotime uses time.Now for sys.Nanotime with a resolution of 1us.
	//
	// See WithNanotime
	WithSysNanotime() ModuleConfig

	// WithNanosleep configures the how to pause the current goroutine for at
	// least the configured nanoseconds. Defaults to return immediately.
	//
	// Ex. To override with your own sleep function:
	//	moduleConfig = moduleConfig.
	//		WithNanosleep(func(ctx context.Context, ns int64) {
	//			rel := unix.NsecToTimespec(ns)
	//			remain := unix.Timespec{}
	//			for { // loop until no more time remaining
	//				err := unix.ClockNanosleep(unix.CLOCK_MONOTONIC, 0, &rel, &remain)
	//			--snip--
	//
	// Notes:
	//	* This primarily supports `poll_oneoff` for relative clock events.
	//	* This does not default to time.Sleep as that violates sandboxing.
	//	* Some compilers implement sleep by looping on sys.Nanotime (ex. Go).
	//	* If you set this, you should probably set WithNanotime also.
	//	* Use WithSysNanosleep for a usable implementation.
	WithNanosleep(sys.Nanosleep) ModuleConfig

	// WithSysNanosleep uses time.Sleep for sys.Nanosleep.
	//
	// See WithNanosleep
	WithSysNanosleep() ModuleConfig

	// WithRandSource configures a source of random bytes. Defaults to crypto/rand.Reader.
	//
	// This reader is most commonly used by the functions like "random_get" in "wasi_snapshot_preview1" or "seed" in
	// AssemblyScript standard "env" although it could be used by functions imported from other modules.
	//
	// Note: The caller is responsible to close any io.Reader they supply: It is not closed on api.Module Close.
	WithRandSource(io.Reader) ModuleConfig

	// WithWorkDirFS indicates the file system to use for any paths beginning at "./". Defaults to the same as WithFS.
	//
	// Ex. This sets a read-only, embedded file-system as the root ("/"), and a mutable one as the working directory ("."):
	//
	//	//go:embed appA
	//	var rootFS embed.FS
	//
	//	// Files relative to this source under appA are available under "/" and files relative to "/work/appA" under ".".
	//	config := wazero.NewModuleConfig().WithFS(rootFS).WithWorkDirFS(os.DirFS("/work/appA"))
	//
	// Note: os.DirFS documentation includes important notes about isolation, which also applies to fs.Sub. As of Go 1.18,
	// the built-in file-systems are not jailed (chroot). See https://github.com/golang/go/issues/42322
	WithWorkDirFS(fs.FS) ModuleConfig
}

type moduleConfig struct {
	name               string
	startFunctions     []string
	stdin              io.Reader
	stdout             io.Writer
	stderr             io.Writer
	randSource         io.Reader
	walltime           *sys.Walltime
	walltimeResolution sys.ClockResolution
	nanotime           *sys.Nanotime
	nanotimeResolution sys.ClockResolution
	nanosleep          *sys.Nanosleep
	args               []string
	// environ is pair-indexed to retain order similar to os.Environ.
	environ []string
	// environKeys allow overwriting of existing values.
	environKeys map[string]int
	fs          *internalsys.FSConfig
}

// NewModuleConfig returns a ModuleConfig that can be used for configuring module instantiation.
func NewModuleConfig() ModuleConfig {
	return &moduleConfig{
		startFunctions: []string{"_start"},
		environKeys:    map[string]int{},
		fs:             internalsys.NewFSConfig(),
	}
}

// clone makes a deep copy of this module config.
func (c *moduleConfig) clone() *moduleConfig {
	ret := *c // copy except maps which share a ref
	ret.environKeys = make(map[string]int, len(c.environKeys))
	for key, value := range c.environKeys {
		ret.environKeys[key] = value
	}
	ret.fs = c.fs.Clone()
	return &ret
}

// WithArgs implements ModuleConfig.WithArgs
func (c *moduleConfig) WithArgs(args ...string) ModuleConfig {
	ret := c.clone()
	ret.args = args
	return ret
}

// WithEnv implements ModuleConfig.WithEnv
func (c *moduleConfig) WithEnv(key, value string) ModuleConfig {
	ret := c.clone()
	// Check to see if this key already exists and update it.
	if i, ok := ret.environKeys[key]; ok {
		ret.environ[i+1] = value // environ is pair-indexed, so the value is 1 after the key.
	} else {
		ret.environKeys[key] = len(ret.environ)
		ret.environ = append(ret.environ, key, value)
	}
	return ret
}

// WithFS implements ModuleConfig.WithFS
func (c *moduleConfig) WithFS(fs fs.FS) ModuleConfig {
	ret := c.clone()
	ret.fs = ret.fs.WithFS(fs)
	return ret
}

// WithName implements ModuleConfig.WithName
func (c *moduleConfig) WithName(name string) ModuleConfig {
	ret := c.clone()
	ret.name = name
	return ret
}

// WithStartFunctions implements ModuleConfig.WithStartFunctions
func (c *moduleConfig) WithStartFunctions(startFunctions ...string) ModuleConfig {
	ret := c.clone()
	ret.startFunctions = startFunctions
	return ret
}

// WithStderr implements ModuleConfig.WithStderr
func (c *moduleConfig) WithStderr(stderr io.Writer) ModuleConfig {
	ret := c.clone()
	ret.stderr = stderr
	return ret
}

// WithStdin implements ModuleConfig.WithStdin
func (c *moduleConfig) WithStdin(stdin io.Reader) ModuleConfig {
	ret := c.clone()
	ret.stdin = stdin
	return ret
}

// WithStdout implements ModuleConfig.WithStdout
func (c *moduleConfig) WithStdout(stdout io.Writer) ModuleConfig {
	ret := c.clone()
	ret.stdout = stdout
	return ret
}

// WithWalltime implements ModuleConfig.WithWalltime
func (c *moduleConfig) WithWalltime(walltime sys.Walltime, resolution sys.ClockResolution) ModuleConfig {
	ret := c.clone()
	ret.walltime = &walltime
	ret.walltimeResolution = resolution
	return ret
}

// We choose arbitrary resolutions here because there's no perfect alternative. For example, according to the
// source in time.go, windows monotonic resolution can be 15ms. This chooses arbitrarily 1us for wall time and
// 1ns for monotonic. See RATIONALE.md for more context.

// WithSysWalltime implements ModuleConfig.WithSysWalltime
func (c *moduleConfig) WithSysWalltime() ModuleConfig {
	return c.WithWalltime(platform.Walltime, sys.ClockResolution(time.Microsecond.Nanoseconds()))
}

// WithNanotime implements ModuleConfig.WithNanotime
func (c *moduleConfig) WithNanotime(nanotime sys.Nanotime, resolution sys.ClockResolution) ModuleConfig {
	ret := c.clone()
	ret.nanotime = &nanotime
	ret.nanotimeResolution = resolution
	return ret
}

// WithSysNanotime implements ModuleConfig.WithSysNanotime
func (c *moduleConfig) WithSysNanotime() ModuleConfig {
	return c.WithNanotime(platform.Nanotime, sys.ClockResolution(1))
}

// WithNanosleep implements ModuleConfig.WithNanosleep
func (c *moduleConfig) WithNanosleep(nanosleep sys.Nanosleep) ModuleConfig {
	ret := *c // copy
	ret.nanosleep = &nanosleep
	return &ret
}

// WithSysNanosleep implements ModuleConfig.WithSysNanosleep
func (c *moduleConfig) WithSysNanosleep() ModuleConfig {
	return c.WithNanosleep(platform.Nanosleep)
}

// WithRandSource implements ModuleConfig.WithRandSource
func (c *moduleConfig) WithRandSource(source io.Reader) ModuleConfig {
	ret := c.clone()
	ret.randSource = source
	return ret
}

// WithWorkDirFS implements ModuleConfig.WithWorkDirFS
func (c *moduleConfig) WithWorkDirFS(fs fs.FS) ModuleConfig {
	ret := c.clone()
	ret.fs = ret.fs.WithWorkDirFS(fs)
	return ret
}

// toSysContext creates a baseline wasm.Context configured by ModuleConfig.
func (c *moduleConfig) toSysContext() (sysCtx *internalsys.Context, err error) {
	var environ []string // Intentionally doesn't pre-allocate to reduce logic to default to nil.
	// Same validation as syscall.Setenv for Linux
	for i := 0; i < len(c.environ); i += 2 {
		key, value := c.environ[i], c.environ[i+1]
		if len(key) == 0 {
			err = errors.New("environ invalid: empty key")
			return
		}
		for j := 0; j < len(key); j++ {
			if key[j] == '=' { // NUL enforced in NewContext
				err = errors.New("environ invalid: key contains '=' character")
				return
			}
		}
		environ = append(environ, key+"="+value)
	}

	preopens, err := c.fs.Preopens()
	if err != nil {
		return nil, err
	}

	return internalsys.NewContext(
		math.MaxUint32,
		c.args,
		environ,
		c.stdin,
		c.stdout,
		c.stderr,
		c.randSource,
		c.walltime, c.walltimeResolution,
		c.nanotime, c.nanotimeResolution,
		c.nanosleep,
		preopens,
	)
}
