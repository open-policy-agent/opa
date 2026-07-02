// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"cmp"
	"net/url"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

func builtinURIParse(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	str, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	parsed, err := url.Parse(string(str))
	if err != nil {
		return err
	}

	obj := ast.NewObject()

	if parsed.Scheme != "" {
		obj.Insert(ast.InternedTerm("scheme"), ast.StringTerm(parsed.Scheme))
	}
	if hostname := parsed.Hostname(); hostname != "" {
		obj.Insert(ast.InternedTerm("hostname"), ast.StringTerm(hostname))
	}
	if port := parsed.Port(); port != "" {
		obj.Insert(ast.InternedTerm("port"), ast.StringTerm(port))
	}
	if parsed.Path != "" {
		obj.Insert(ast.InternedTerm("path"), ast.StringTerm(parsed.Path))
		// raw_path is always set when path is present, so that users can
		// rely on it being available for custom path normalization.
		obj.Insert(ast.InternedTerm("raw_path"), ast.StringTerm(cmp.Or(parsed.RawPath, parsed.Path)))
	}
	if parsed.RawQuery != "" {
		// raw_query can be piped into urlquery.decode_object() for structured access
		obj.Insert(ast.InternedTerm("raw_query"), ast.StringTerm(parsed.RawQuery))
	}
	if parsed.Fragment != "" {
		obj.Insert(ast.InternedTerm("fragment"), ast.StringTerm(parsed.Fragment))
	}

	return iter(ast.NewTerm(obj))
}

func builtinURIIsValid(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	str, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return iter(ast.InternedTerm(false))
	}

	// Empty strings are technically valid relative references per RFC 3986,
	// but are rejected here to avoid unexpected results for policy use cases.
	if len(str) == 0 {
		return iter(ast.InternedTerm(false))
	}

	_, err = url.Parse(string(str))

	return iter(ast.InternedTerm(err == nil))
}

func init() {
	RegisterBuiltinFunc(ast.URIParse.Name, builtinURIParse)
	RegisterBuiltinFunc(ast.URIIsValid.Name, builtinURIIsValid)
}
