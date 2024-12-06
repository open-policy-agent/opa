// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package plugins implements plugin management for the policy engine.
package plugins

import (
	"github.com/open-policy-agent/opa/internal/report"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/gorilla/mux"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/hooks"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/resolver/wasm"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown/print"
	"github.com/open-policy-agent/opa/tracing"
	v1 "github.com/open-policy-agent/opa/v1/plugins"
)

// Factory defines the interface OPA uses to instantiate your plugin.
//
// When OPA processes it's configuration it looks for factories that
// have been registered by calling runtime.RegisterPlugin. Factories
// are registered to a name which is used to key into the
// configuration blob. If your plugin has not been configured, your
// factory will not be invoked.
//
//	plugins:
//	  my_plugin1:
//	    some_key: foo
//	  # my_plugin2:
//	  #   some_key2: bar
//
// If OPA was started with the configuration above and received two
// calls to runtime.RegisterPlugins (one with NAME "my_plugin1" and
// one with NAME "my_plugin2"), it would only invoke the factory for
// for my_plugin1.
//
// OPA instantiates and reconfigures plugins in two steps. First, OPA
// will call Validate to check the configuration. Assuming the
// configuration is valid, your factory should return a configuration
// value that can be used to construct your plugin. Second, OPA will
// call New to instantiate your plugin providing the configuration
// value returned from the Validate call.
//
// Validate receives a slice of bytes representing plugin
// configuration and returns a configuration value that can be used to
// instantiate your plugin. The manager is provided to give access to
// the OPA's compiler, storage layer, and global configuration. Your
// Validate function will typically:
//
//  1. Deserialize the raw config bytes
//  2. Validate the deserialized config for semantic errors
//  3. Inject default values
//  4. Return a deserialized/parsed config
//
// New receives a valid configuration for your plugin and returns a
// plugin object. Your New function will typically:
//
//  1. Cast the config value to it's own type
//  2. Instantiate a plugin object
//  3. Return the plugin object
//  4. Update status via `plugins.Manager#UpdatePluginStatus`
//
// After a plugin has been created subsequent status updates can be
// send anytime the plugin enters a ready or error state.
type Factory = v1.Factory

// Plugin defines the interface OPA uses to manage your plugin.
//
// When OPA starts it will start all of the plugins it was configured
// to instantiate. Each time a new plugin is configured (via
// discovery), OPA will start it. You can use the Start call to spawn
// additional goroutines or perform initialization tasks.
//
// Currently OPA will not call Stop on plugins.
//
// When OPA receives new configuration for your plugin via discovery
// it will first Validate the configuration using your factory and
// then call Reconfigure.
type Plugin = v1.Plugin

// Triggerable defines the interface plugins use for manual plugin triggers.
type Triggerable = v1.Triggerable

// State defines the state that a Plugin instance is currently
// in with pre-defined states.
type State = v1.State

const (
	// StateNotReady indicates that the Plugin is not in an error state, but isn't
	// ready for normal operation yet. This should only happen at
	// initialization time.
	StateNotReady = v1.StateNotReady

	// StateOK signifies that the Plugin is operating normally.
	StateOK = v1.StateOK

	// StateErr indicates that the Plugin is in an error state and should not
	// be considered as functional.
	StateErr = v1.StateErr

	// StateWarn indicates the Plugin is operating, but in a potentially dangerous or
	// degraded state. It may be used to indicate manual remediation is needed, or to
	// alert admins of some other noteworthy state.
	StateWarn = v1.StateWarn
)

// TriggerMode defines the trigger mode utilized by a Plugin for bundle download,
// log upload etc.
type TriggerMode = v1.TriggerMode

const (
	// TriggerPeriodic represents periodic polling mechanism
	TriggerPeriodic = v1.TriggerPeriodic

	// TriggerManual represents manual triggering mechanism
	TriggerManual = v1.TriggerManual

	// DefaultTriggerMode represents default trigger mechanism
	DefaultTriggerMode = v1.DefaultTriggerMode
)

// Status has a Plugin's current status plus an optional Message.
type Status = v1.Status

// StatusListener defines a handler to register for status updates.
type StatusListener v1.StatusListener

// Manager implements lifecycle management of plugins and gives plugins access
// to engine-wide components like storage.
type Manager = v1.Manager

// SetCompilerOnContext puts the compiler into the storage context. Calling this
// function before committing updated policies to storage allows the manager to
// skip parsing and compiling of modules. Instead, the manager will use the
// compiler that was stored on the context.
func SetCompilerOnContext(context *storage.Context, compiler *ast.Compiler) {
	v1.SetCompilerOnContext(context, compiler)
}

// GetCompilerOnContext gets the compiler cached on the storage context.
func GetCompilerOnContext(context *storage.Context) *ast.Compiler {
	return v1.GetCompilerOnContext(context)
}

// SetWasmResolversOnContext puts a set of Wasm Resolvers into the storage
// context. Calling this function before committing updated wasm modules to
// storage allows the manager to skip initializing modules before using them.
// Instead, the manager will use the compiler that was stored on the context.
func SetWasmResolversOnContext(context *storage.Context, rs []*wasm.Resolver) {
	v1.SetWasmResolversOnContext(context, rs)
}

// ValidateAndInjectDefaultsForTriggerMode validates the trigger mode and injects default values
func ValidateAndInjectDefaultsForTriggerMode(a, b *TriggerMode) (*TriggerMode, error) {
	return v1.ValidateAndInjectDefaultsForTriggerMode(a, b)
}

// Info sets the runtime information on the manager. The runtime information is
// propagated to opa.runtime() built-in function calls.
func Info(term *ast.Term) func(*Manager) {
	return v1.Info(term)
}

// InitBundles provides the initial set of bundles to load.
func InitBundles(b map[string]*bundle.Bundle) func(*Manager) {
	return v1.InitBundles(b)
}

// InitFiles provides the initial set of other data/policy files to load.
func InitFiles(f loader.Result) func(*Manager) {
	return v1.InitFiles(f)
}

// MaxErrors sets the error limit for the manager's shared compiler.
func MaxErrors(n int) func(*Manager) {
	return v1.MaxErrors(n)
}

// GracefulShutdownPeriod passes the configured graceful shutdown period to plugins
func GracefulShutdownPeriod(gracefulShutdownPeriod int) func(*Manager) {
	return v1.GracefulShutdownPeriod(gracefulShutdownPeriod)
}

// Logger configures the passed logger on the plugin manager (useful to
// configure default fields)
func Logger(logger logging.Logger) func(*Manager) {
	return v1.Logger(logger)
}

// ConsoleLogger sets the passed logger to be used by plugins that are
// configured with console logging enabled.
func ConsoleLogger(logger logging.Logger) func(*Manager) {
	return v1.ConsoleLogger(logger)
}

func EnablePrintStatements(yes bool) func(*Manager) {
	return v1.EnablePrintStatements(yes)
}

func PrintHook(h print.Hook) func(*Manager) {
	return v1.PrintHook(h)
}

func WithRouter(r *mux.Router) func(*Manager) {
	return v1.WithRouter(r)
}

// WithPrometheusRegister sets the passed prometheus.Registerer to be used by plugins
func WithPrometheusRegister(prometheusRegister prometheus.Registerer) func(*Manager) {
	return v1.WithPrometheusRegister(prometheusRegister)
}

// WithTracerProvider sets the passed *trace.TracerProvider to be used by plugins
func WithTracerProvider(tracerProvider *trace.TracerProvider) func(*Manager) {
	return v1.WithTracerProvider(tracerProvider)
}

// WithDistributedTracingOpts sets the options to be used by distributed tracing.
func WithDistributedTracingOpts(tr tracing.Options) func(*Manager) {
	return v1.WithDistributedTracingOpts(tr)
}

// WithHooks allows passing hooks to the plugin manager.
func WithHooks(hs hooks.Hooks) func(*Manager) {
	return v1.WithHooks(hs)
}

// WithParserOptions sets the parser options to be used by the plugin manager.
func WithParserOptions(opts ast.ParserOptions) func(*Manager) {
	return v1.WithParserOptions(opts)
}

// WithEnableTelemetry controls whether OPA will send telemetry reports to an external service.
func WithEnableTelemetry(enableTelemetry bool) func(*Manager) {
	return v1.WithEnableTelemetry(enableTelemetry)
}

// WithTelemetryGatherers allows registration of telemetry gatherers which enable injection of additional data in the
// telemetry report
func WithTelemetryGatherers(gs map[string]report.Gatherer) func(*Manager) {
	return v1.WithTelemetryGatherers(gs)
}

// New creates a new Manager using config.
func New(raw []byte, id string, store storage.Store, opts ...func(*Manager)) (*Manager, error) {
	options := make([]func(*Manager), 0, len(opts)+1)
	options = append(options, opts...)

	// Add option to apply default Rego version if not set. Must be last in list of options.
	options = append(options, func(m *Manager) {
		if m.ParserOptions().RegoVersion == ast.RegoUndefined {
			cpy := m.ParserOptions()
			cpy.RegoVersion = ast.DefaultRegoVersion
			WithParserOptions(cpy)(m)
		}
	})

	return v1.New(raw, id, store, options...)
}
