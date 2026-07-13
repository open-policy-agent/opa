// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"context"

	"github.com/open-policy-agent/opa/v1/metrics"
)

type ExternalRuleSource interface {
	// Refs returns the package refs that this source provides rules for.
	// A source can provide rules for multiple packages.
	Refs() []Ref

	// Init returns an initialized [ExternalRuleIndex]. A `Ref` is provided
	// so we know which package we're preparing if multiple Refs are external.
	Init(context.Context, Ref) (ExternalRuleIndex, error)
}

// ExternalRuleIndex mirrors RuleIndex.Lookup(), but add a [context.Context] parameter.
type ExternalRuleIndex interface {
	// Opts returns the options for the ExternalRuleIndex. Returns nil if no
	// options are configured.
	Opts() *ExternalSourceOptions

	// Lookup returns rules and optionally an updated ExternalRuleIndex instance.
	// The returned ExternalRuleIndex (if non-nil) will be used for subsequent
	// Lookup calls within the same evaluation context, allowing plugins to
	// maintain per-evaluation state.
	//
	// Plugins can use two strategies:
	// 1. Immutable: Return a new ExternalRuleIndex instance with updated state
	// 2. Mutable: Update internal state and return self
	//
	// If the plugin does not need per-evaluation state, it can return nil for
	// the ExternalRuleIndex, and the original instance will continue to be used.
	Lookup(context.Context, ...LookupOption) ([]*Rule, ExternalRuleIndex, error)
}

// ExternalRuleIndexCloser is an optional interface for resource cleanup.
type ExternalRuleIndexCloser interface {
	ExternalRuleIndex
	Close() error
}

// ParametrizedExternalRuleIndex is an optional interface implemented by external
// rule indexes that serve a family of sub-references under their registered
// prefix rather than a single exact ref. The Ref such a source is registered
// under is treated as a PREFIX: the leading elements of a query reference that
// follow the prefix are consumed as ground lookup parameters (handed to Lookup
// via LookupOptions.Params) rather than as descents into a static rule tree.
// This lets one registered source serve an unbounded family of sub-references —
// one distinct set of rules per parameter tuple — without registering each
// concretely, so references whose key only comes into existence at runtime
// resolve without a recompile.
//
// An index that does not implement this interface behaves as a conventional
// exact-ref source (equivalent to an arity of 0).
type ParametrizedExternalRuleIndex interface {
	ExternalRuleIndex

	// ParamArity reports how many elements following the registered prefix this
	// index consumes as lookup parameters, given the reference tail (the query
	// reference elements after the prefix, or an empty Ref when none follow).
	//
	// The count may vary with the tail's *shape* — e.g. keying off a leading
	// discriminator segment — which lets a single prefix back an uneven-depth
	// tree. It must NOT depend on parameter *values*: ParamArity is consulted
	// before the parameters are plugged, so the tail may contain non-ground
	// elements, and the count decides the caching boundary. Returning 0 makes
	// the reference resolve as a conventional exact ref.
	//
	// The parameter elements the count selects must be ground at evaluation
	// time. A non-ground parameter yields an undefined result, except under
	// partial evaluation where the reference is saved for residualization.
	ParamArity(tail Ref) int
}

// ExternalSourceOptions contains options for registering an external rule source.
type ExternalSourceOptions struct {
	// VisibleRefs controls which parts of the surrounding rule tree the external
	// source can reference during compilation. By default (nil), the source is
	// fully isolated and cannot access any surrounding policy. An empty slice
	// is equivalent to nil (fully isolated).
	//
	// To allow access to the entire rule tree, use []Ref{MustParseRef("data")}.
	// To allow access to specific subtrees only, list them explicitly, e.g.
	// []Ref{MustParseRef("data.helpers")}. The external source can then
	// reference rules under those prefixes but nothing else.
	VisibleRefs []Ref

	// SkippedStages allows external sources to skip stages in the dynamic compiler
	// used with the externally-provided Rego. If, for example, the `[]*Rule` returned
	// has already been compiled, we can skip all stages.
	//
	// For pre-compiled rules, prefer starting from AllStages() and removing only
	// the stages you need (e.g. SetModuleTree, SetRuleTree, BuildRuleIndices).
	// This is forward-compatible: new compiler stages added in future releases
	// will be skipped automatically rather than running unexpectedly.
	SkippedStages []StageID

	// DistinguishAbsentFromUnknown controls how the resolver passed to Lookup
	// (via LookupOptions.Resolver) reports references that do not resolve to a
	// concrete value.
	//
	// When false (default), the legacy behavior is preserved for backwards
	// compatibility: only input references are resolvable, and any input
	// reference that cannot be resolved — whether it is genuinely absent from a
	// concrete input or symbolic under partial evaluation — surfaces as
	// UnknownValueErr. The two cases are indistinguishable.
	//
	// When true, the source opts into the same save-set-aware resolver the
	// built-in rule indexer uses: a reference that is unknown under partial
	// evaluation returns UnknownValueErr, while a reference that is simply
	// absent from an otherwise-concrete input resolves to (nil, nil). This lets
	// a source tell "deliberately symbolic" apart from "concretely missing"
	// on a per-reference basis (e.g. input.foo unknown while input.bar is
	// known). See ValueResolver and IsUnknownValueErr.
	DistinguishAbsentFromUnknown bool
}

// LookupOption is a functional option for ExternalRuleIndex.Lookup calls.
type LookupOption func(*LookupOptions)

// LookupOptions contains options for ExternalRuleIndex.Lookup calls.
type LookupOptions struct {
	metrics          metrics.Metrics
	resolver         ValueResolver
	requestMetadata  map[string]any
	responseMetadata map[string]any
	params           []Value
}

// Metrics returns the metrics instance from the options, or nil if not set.
func (o *LookupOptions) Metrics() metrics.Metrics {
	if o == nil {
		return nil
	}
	return o.metrics
}

func (o *LookupOptions) Resolver() ValueResolver {
	return o.resolver
}

func (o *LookupOptions) RequestMetadata() map[string]any {
	return o.requestMetadata
}

func (o *LookupOptions) ResponseMetadata() map[string]any {
	return o.responseMetadata
}

// Params returns the parameter values for a parametrized external source (see
// ParametrizedExternalRuleIndex). The slice holds the ground key values that
// followed the registered prefix in the query reference, in order. It is empty
// for conventional (non-parametrized) sources.
func (o *LookupOptions) Params() []Value {
	return o.params
}

// LookupMetrics returns a LookupOption that sets the metrics instance
// for the Lookup call.
func LookupMetrics(m metrics.Metrics) LookupOption {
	return func(opts *LookupOptions) {
		opts.metrics = m
	}
}

func LookupResolver(r ValueResolver) LookupOption {
	return func(opts *LookupOptions) {
		opts.resolver = r
	}
}

func LookupRequestMetadata(m map[string]any) LookupOption {
	return func(opts *LookupOptions) {
		opts.requestMetadata = m
	}
}

func LookupResponseMetadata(m map[string]any) LookupOption {
	return func(opts *LookupOptions) {
		opts.responseMetadata = m
	}
}

// LookupParams returns a LookupOption that sets the parameter values handed to a
// parametrized external source (see ParametrizedExternalRuleIndex).
func LookupParams(params []Value) LookupOption {
	return func(opts *LookupOptions) {
		opts.params = params
	}
}
