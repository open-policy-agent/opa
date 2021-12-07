// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"io/ioutil"
	"path/filepath"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

func builtinReadFile(a ast.Value) (ast.Value, error) {
	s, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}
	path, err := filepath.Abs(string(s))
	if err != nil {
		return nil, err
	}
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ast.String(string(content[:])), nil
}

func init() {
	RegisterFunctionalBuiltin1(ast.ReadFile.Name, builtinReadFile)
}
