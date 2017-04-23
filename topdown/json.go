// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"encoding/json"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/open-policy-agent/opa/util"
)

func builtinJSONMarshal(a ast.Value) (ast.Value, error) {

	asJSON, err := ast.JSON(a)
	if err != nil {
		return nil, err
	}

	bs, err := json.Marshal(asJSON)
	if err != nil {
		return nil, err
	}

	return ast.String(string(bs)), nil
}

func builtinJSONUnmarshal(a ast.Value) (ast.Value, error) {

	str, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}

	var x interface{}

	if err := util.UnmarshalJSON([]byte(str), &x); err != nil {
		return nil, err
	}

	return ast.InterfaceToValue(x)
}

func init() {
	RegisterFunctionalBuiltin1(ast.JSONMarshal.Name, builtinJSONMarshal)
	RegisterFunctionalBuiltin1(ast.JSONUnmarshal.Name, builtinJSONUnmarshal)
}
