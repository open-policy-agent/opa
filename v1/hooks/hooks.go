// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package hooks

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/v1/config"
	topdown_cache "github.com/open-policy-agent/opa/v1/topdown/cache"
)

// Hook is a hook to be called in some select places in OPA's operation.
//
// The base Hook interface is any, and wherever a hook can occur, the calling code
// will check if your hook implements an appropriate interface. If so, your hook
// is called.
//
// This allows you to only hook in to behavior you care about, and it allows the
// OPA to add more hooks in the future.
//
// All hook interfaces in this package have Hook in the name. Hooks must be safe
// for concurrent use. It is expected that hooks are fast; if a hook needs to take
// time, then copy what you need and ensure the hook is async.
//
// When multiple instances of a hook are provided, they are all going to be executed
// in an unspecified order (it's a map-range call underneath). If you need hooks to
// be run in order, you can wrap them into another hook, and configure that one.
type Hook any

// Hooks is the type used for every struct in OPA that can work with hooks.
type Hooks struct {
	m map[Hook]struct{} // we are NOT providing a stable invocation ordering
}

// New creates a new instance of Hooks.
func New(hs ...Hook) Hooks {
	h := Hooks{m: make(map[Hook]struct{}, len(hs))}
	for i := range hs {
		h.m[hs[i]] = struct{}{}
	}
	return h
}

func (hs Hooks) Each(fn func(Hook)) {
	for h := range hs.m {
		fn(h)
	}
}

func (hs Hooks) Len() int {
	return len(hs.m)
}

// ConfigHook allows inspecting or rewriting the configuration when the plugin
// manager is processing it.
// Note that this hook is not run when the plugin manager is reconfigured. This
// usually only happens when there's a new config from a discovery bundle, and
// for processing _that_, there's `ConfigDiscoveryHook`.
type ConfigHook interface {
	OnConfig(context.Context, *config.Config) (*config.Config, error)
}

// ConfigHook allows inspecting or rewriting the discovered configuration when
// the discovery plugin is processing it.
type ConfigDiscoveryHook interface {
	OnConfigDiscovery(context.Context, *config.Config) (*config.Config, error)
}

// InterQueryCacheHook allows access to the server's inter-query cache instance.
// It's useful for out-of-tree handlers that also need to evaluate something.
// Using this hook, they can share the caches with the rest of OPA.
type InterQueryCacheHook interface {
	OnInterQueryCache(context.Context, topdown_cache.InterQueryCache) error
}

// InterQueryValueCacheHook allows access to the server's inter-query value cache
// instance.
type InterQueryValueCacheHook interface {
	OnInterQueryValueCache(context.Context, topdown_cache.InterQueryValueCache) error
}

func (hs Hooks) Validate() error {
	for h := range hs.m {
		switch h.(type) {
		case InterQueryCacheHook,
			InterQueryValueCacheHook,
			ConfigHook,
			ConfigDiscoveryHook: // OK
		default:
			return fmt.Errorf("unknown hook type %T", h)
		}
	}
	return nil
}
