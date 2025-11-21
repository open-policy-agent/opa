// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package ref implements internal helpers for references
package ref

import (
	"errors"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/util"
)

// ParseDataPath returns a ref from the slash separated path s rooted at data.
// All path segments are treated as identifier strings.
func ParseDataPath(s string) (ast.Ref, error) {
	path, ok := storage.ParsePath(util.WithPrefix(s, "/"))
	if !ok {
		return nil, errors.New("invalid path")
	}

	return path.Ref(ast.DefaultRootDocument), nil
}

// ArrayPath will take an ast.Array and build an ast.Ref using the ast.Terms in the Array
func ArrayPath(a *ast.Array) ast.Ref {
	ref := make(ast.Ref, 0, a.Len())

	a.Foreach(func(term *ast.Term) {
		ref = append(ref, term)
	})

	return ref
}
