// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package authorizer provides authorization handlers to the server.
package authorizer

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/server/identifier"
	"github.com/open-policy-agent/opa/server/types"
	"github.com/open-policy-agent/opa/server/writer"
	"github.com/open-policy-agent/opa/storage"
)

// Basic provides policy-based authorization over incoming requests.
type Basic struct {
	inner    http.Handler
	compiler func() *ast.Compiler
	store    storage.Store
	runtime  *ast.Term
	decision func() ast.Ref
}

// Runtime returns an argument that sets the runtime on the authorizer.
func Runtime(term *ast.Term) func(*Basic) {
	return func(b *Basic) {
		b.runtime = term
	}
}

// Decision returns an argument that sets the path of the authorization decision
// to query.
func Decision(ref func() ast.Ref) func(*Basic) {
	return func(b *Basic) {
		b.decision = ref
	}
}

// NewBasic returns a new Basic object.
func NewBasic(inner http.Handler, compiler func() *ast.Compiler, store storage.Store, opts ...func(*Basic)) http.Handler {
	b := &Basic{
		inner:    inner,
		compiler: compiler,
		store:    store,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

func (h *Basic) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	input, err := makeInput(r)
	if err != nil {
		writer.ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
		return
	}

	rego := rego.New(
		rego.Query(h.decision().String()),
		rego.Compiler(h.compiler()),
		rego.Store(h.store),
		rego.Input(input),
		rego.Runtime(h.runtime),
	)

	rs, err := rego.Eval(r.Context())

	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	if len(rs) == 0 {
		// Authorizer was configured but no policy defined. This indicates an internal error or misconfiguration.
		writer.Error(w, http.StatusInternalServerError, types.NewErrorV1(types.CodeInternal, types.MsgUnauthorizedUndefinedError))
		return
	}

	switch allowed := rs[0].Expressions[0].Value.(type) {
	case bool:
		if allowed {
			h.inner.ServeHTTP(w, r)
			return
		}
	}

	writer.Error(w, http.StatusUnauthorized, types.NewErrorV1(types.CodeUnauthorized, types.MsgUnauthorizedError))
}

func makeInput(r *http.Request) (interface{}, error) {
	path, err := parsePath(r.URL.Path)
	if err != nil {
		return nil, err
	}

	method := strings.ToUpper(r.Method)
	query := r.URL.Query()

	input := map[string]interface{}{
		"path":    path,
		"method":  method,
		"params":  query,
		"headers": r.Header,
	}

	identity, ok := identifier.Identity(r)
	if ok {
		input["identity"] = identity
	}

	return input, nil
}

func parsePath(path string) ([]interface{}, error) {
	if len(path) == 0 {
		return []interface{}{}, nil
	}
	parts := strings.Split(path[1:], "/")
	for i := range parts {
		var err error
		parts[i], err = url.PathUnescape(parts[i])
		if err != nil {
			return nil, err
		}
	}
	sl := make([]interface{}, len(parts))
	for i := range sl {
		sl[i] = parts[i]
	}
	return sl, nil
}
