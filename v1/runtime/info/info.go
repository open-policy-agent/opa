// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package params provides utilities to create runtime context information for
// OPA instances.
package info

import (
	"os"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/version"
)

// Options controls the types of runtime context information to return.
type Options struct {
	Config                 []byte
	IsAuthorizationEnabled bool
	SkipKnownSchemaCheck   bool
}

func New() (*ast.Term, error) {
	return NewWithOptions(Options{})
}

// NewWithOptions returns the runtime context information as an ast.Term object.
func NewWithOptions(opts Options) (*ast.Term, error) {
	obj := ast.NewObject()

	if opts.Config != nil {
		var x any
		if err := util.Unmarshal(opts.Config, &x); err != nil {
			return nil, err
		}

		v, err := ast.InterfaceToValue(x)
		if err != nil {
			return nil, err
		}

		obj.Insert(ast.InternedTerm("config"), ast.NewTerm(v))
	}

	env := ast.NewObject()

	for _, s := range os.Environ() {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) == 1 {
			env.Insert(ast.StringTerm(parts[0]), ast.InternedNullTerm)
		} else if len(parts) > 1 {
			env.Insert(ast.StringTerm(parts[0]), ast.StringTerm(parts[1]))
		}
	}

	obj.Insert(ast.InternedTerm("env"), ast.NewTerm(env))
	obj.Insert(ast.InternedTerm("version"), ast.StringTerm(version.Version))
	obj.Insert(ast.InternedTerm("commit"), ast.StringTerm(version.Vcs))
	obj.Insert(ast.InternedTerm("authorization_enabled"), ast.InternedTerm(opts.IsAuthorizationEnabled))
	obj.Insert(ast.InternedTerm("skip_known_schema_check"), ast.InternedTerm(opts.SkipKnownSchemaCheck))

	return ast.NewTerm(obj), nil
}
