// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package authorizer provides authorization handlers to the server.
package authorizer

import (
	"context"
	"net/http"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown/cache"
	"github.com/open-policy-agent/opa/topdown/print"
	v1 "github.com/open-policy-agent/opa/v1/server/authorizer"
)

// Basic provides policy-based authorization over incoming requests.
type Basic = v1.Basic

// Runtime returns an argument that sets the runtime on the authorizer.
func Runtime(term *ast.Term) func(*Basic) {
	return v1.Runtime(term)
}

// Decision returns an argument that sets the path of the authorization decision
// to query.
func Decision(ref func() ast.Ref) func(*Basic) {
	return v1.Decision(ref)
}

// PrintHook sets the object to use for handling print statement outputs.
func PrintHook(printHook print.Hook) func(*Basic) {
	return v1.PrintHook(printHook)
}

// EnablePrintStatements enables print() calls. If this option is not provided,
// print() calls will be erased from the policy. This option only applies to
// queries and policies that passed as raw strings, i.e., this function will not
// have any affect if the caller supplies the ast.Compiler instance.
func EnablePrintStatements(yes bool) func(r *Basic) {
	return v1.EnablePrintStatements(yes)
}

// InterQueryCache enables the inter-query cache on the authorizer
func InterQueryCache(interQueryCache cache.InterQueryCache) func(*Basic) {
	return v1.InterQueryCache(interQueryCache)
}

// InterQueryValueCache enables the inter-query value cache on the authorizer
func InterQueryValueCache(interQueryValueCache cache.InterQueryValueCache) func(*Basic) {
	return v1.InterQueryValueCache(interQueryValueCache)
}

// NewBasic returns a new Basic object.
func NewBasic(inner http.Handler, compiler func() *ast.Compiler, store storage.Store, opts ...func(*Basic)) http.Handler {
	return v1.NewBasic(inner, compiler, store, opts...)
}

// SetBodyOnContext adds the parsed input value to the context. This function is only
// exposed for test purposes.
func SetBodyOnContext(ctx context.Context, x interface{}) context.Context {
	return v1.SetBodyOnContext(ctx, x)
}

// GetBodyOnContext returns the parsed input from the request context if it exists.
// The authorizer saves the parsed input on the context when it runs.
func GetBodyOnContext(ctx context.Context) (interface{}, bool) {
	return v1.GetBodyOnContext(ctx)
}
