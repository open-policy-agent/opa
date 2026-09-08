// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"encoding/json"
	"fmt"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/test/parsercases"
)

func regoVersion(s string) (ast.RegoVersion, error) {
	switch s {
	case "", "v1":
		return ast.RegoV1, nil
	case "v0":
		return ast.RegoV0, nil
	case "v0-compat-v1":
		return ast.RegoV0CompatV1, nil
	}
	return ast.RegoUndefined, fmt.Errorf("unknown rego_version %q", s)
}

func capabilities(tc parsercases.TestCase) *ast.Capabilities {
	return ast.CapabilitiesForThisVersion(ast.CapabilitiesExperimentalKeywords(tc.ExperimentalKeywords))
}

func parserOptions(tc parsercases.TestCase) (ast.ParserOptions, error) {
	v, err := regoVersion(tc.RegoVersion)
	if err != nil {
		return ast.ParserOptions{}, err
	}
	return ast.ParserOptions{
		RegoVersion:       v,
		Capabilities:      capabilities(tc),
		ProcessAnnotation: tc.Annotations,
		FutureKeywords:    tc.FutureKeywords,
		AllFutureKeywords: tc.AllFutureKeywords,
	}, nil
}

func parseModule(tc parsercases.TestCase, module string) (*ast.Module, error) {
	popts, err := parserOptions(tc)
	if err != nil {
		return nil, err
	}
	return ast.ParseModuleWithOpts(parsercases.DefaultModuleName, module, popts)
}

// MarshalAST renders the AST of m as a want_ast fixture, under whichever
// marshalling options are currently in effect.
//
// Comments are dropped, for the reason locations are off by default: the module
// is right there in the fixture, so recording them again asserts nothing a reader
// cannot see, while requiring an implementation to retain them in a particular
// shape. OPA's own is not one to hold anyone to — Comment marshals its text as
// base64 and its position unconditionally, ignoring the toggle that exists for it.
func MarshalAST(m *ast.Module) (string, error) {
	m.Comments = nil

	bs, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return parsercases.FormatAST(bs)
}
