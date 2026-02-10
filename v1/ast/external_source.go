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
	SkippedStages []StageID
}

// LookupOption is a functional option for ExternalRuleIndex.Lookup calls.
type LookupOption func(*LookupOptions)

// LookupOptions contains options for ExternalRuleIndex.Lookup calls.
type LookupOptions struct {
	metrics          metrics.Metrics
	resolver         ValueResolver
	requestMetadata  map[string]any
	responseMetadata map[string]any
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
