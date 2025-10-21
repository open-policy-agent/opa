// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"

	"github.com/open-policy-agent/opa/internal/purl"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

func builtinPurlIsValid(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	str, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return iter(ast.InternedTerm(false))
	}

	_, err = purl.FromString(string(str))
	return iter(ast.InternedTerm(err == nil))
}

func builtinPurlParse(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	str, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	parsedPurl, err := purl.FromString(string(str))
	if err != nil {
		return fmt.Errorf("invalid PURL %q: %w", str, err)
	}

	// Create object with required fields
	obj := ast.NewObject(
		[2]*ast.Term{ast.InternedTerm("type"), ast.StringTerm(parsedPurl.Type)},
		[2]*ast.Term{ast.InternedTerm("name"), ast.StringTerm(parsedPurl.Name)},
	)

	// Add optional fields only if present
	if parsedPurl.Namespace != "" {
		obj.Insert(ast.InternedTerm("namespace"), ast.StringTerm(parsedPurl.Namespace))
	}
	if parsedPurl.Version != "" {
		obj.Insert(ast.InternedTerm("version"), ast.StringTerm(parsedPurl.Version))
	}
	if parsedPurl.Subpath != "" {
		obj.Insert(ast.InternedTerm("subpath"), ast.StringTerm(parsedPurl.Subpath))
	}

	// Add qualifiers only if present
	if len(parsedPurl.Qualifiers) > 0 {
		qualifiers := ast.NewObject()
		for _, q := range parsedPurl.Qualifiers {
			qualifiers.Insert(ast.StringTerm(q.Key), ast.StringTerm(q.Value))
		}
		obj.Insert(ast.InternedTerm("qualifiers"), ast.NewTerm(qualifiers))
	}

	return iter(ast.NewTerm(obj))
}

func init() {
	RegisterBuiltinFunc(ast.PurlIsValid.Name, builtinPurlIsValid)
	RegisterBuiltinFunc(ast.PurlParse.Name, builtinPurlParse)
}
