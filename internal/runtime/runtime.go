// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package runtime contains utilities to return runtime information on the OPA instance.
package runtime

import (
	"os"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/version"
)

// Params controls the types of runtime information to return.
type Params struct {
	Config                 []byte
	IsAuthorizationEnabled bool
	SkipKnownSchemaCheck   bool
}

// Term returns the runtime information as an ast.Term object.
func Term(params Params) (*ast.Term, error) {

	obj := ast.NewObject()

	if params.Config != nil {

		var x any
		if err := util.Unmarshal(params.Config, &x); err != nil {
			return nil, err
		}

		v, err := ast.InterfaceToValue(x)
		if err != nil {
			return nil, err
		}

		obj.Insert(ast.InternedStringTerm("config"), ast.NewTerm(v))
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

	obj.Insert(ast.InternedStringTerm("env"), ast.NewTerm(env))
	obj.Insert(ast.InternedStringTerm("version"), ast.StringTerm(version.Version))
	obj.Insert(ast.InternedStringTerm("commit"), ast.StringTerm(version.Vcs))
	obj.Insert(ast.InternedStringTerm("authorization_enabled"), ast.InternedBooleanTerm(params.IsAuthorizationEnabled))
	obj.Insert(ast.InternedStringTerm("skip_known_schema_check"), ast.InternedBooleanTerm(params.SkipKnownSchemaCheck))

	return ast.NewTerm(obj), nil
}
