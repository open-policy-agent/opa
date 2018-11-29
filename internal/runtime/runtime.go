// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package runtime contains utilities to return runtime information on the OPA instance.
package runtime

import (
	"os"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
)

// Params controls the types of runtime information to return.
type Params struct {
	Config []byte
}

// Term returns the runtime information as an ast.Term object.
func Term(params Params) (*ast.Term, error) {

	obj := ast.NewObject()

	if params.Config != nil {

		var x interface{}
		if err := util.Unmarshal(params.Config, &x); err != nil {
			return nil, err
		}

		v, err := ast.InterfaceToValue(x)
		if err != nil {
			return nil, err
		}

		obj.Insert(ast.StringTerm("config"), ast.NewTerm(v))
	}

	env := ast.NewObject()

	for _, s := range os.Environ() {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) == 1 {
			env.Insert(ast.StringTerm(parts[0]), ast.NullTerm())
		} else if len(parts) > 1 {
			env.Insert(ast.StringTerm(parts[0]), ast.StringTerm(parts[1]))
		}
	}

	obj.Insert(ast.StringTerm("env"), ast.NewTerm(env))

	return ast.NewTerm(obj), nil
}
