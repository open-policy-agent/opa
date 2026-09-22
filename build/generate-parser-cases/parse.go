// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"encoding/json"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/test/conformance"
	"github.com/open-policy-agent/opa/v1/test/parsercases"
)

// parserOptions translates the case's parse options onto OPA's parser. Every decision
// is the schema's; nothing here is derived from the case. The runner's parseCaseOptions
// is the same translation, and TestTranslateParserOptionsCarryEveryField pins it.
func parserOptions(tc parsercases.TestCase) (ast.ParserOptions, error) {
	opts, err := tc.ParseOptions()
	if err != nil {
		return ast.ParserOptions{}, err
	}
	return translateParserOptions(opts)
}

func translateParserOptions(opts conformance.ParseOptions) (ast.ParserOptions, error) {
	v, err := corpusgen.RegoVersion(opts.RegoVersion)
	if err != nil {
		return ast.ParserOptions{}, err
	}

	return ast.ParserOptions{
		RegoVersion:       v,
		Capabilities:      ast.CapabilitiesForThisVersion(ast.CapabilitiesExperimentalKeywords(opts.ExperimentalKeywords)),
		ProcessAnnotation: opts.ProcessAnnotations,
		FutureKeywords:    opts.FutureKeywords,
		AllFutureKeywords: opts.AllFutureKeywords,
		SkipRules:         opts.SkipRules,
	}, nil
}

// parseCase reads the case's Rego through the entry point it is written for: a
// *ast.Module for a module case, an ast.Body for a body case.
func parseCase(tc parsercases.TestCase) (any, error) {
	popts, err := parserOptions(tc)
	if err != nil {
		return nil, err
	}
	if tc.BodyCase() {
		return ast.ParseBodyWithOpts(tc.Body, popts)
	}
	return ast.ParseModuleWithOpts(parsercases.DefaultModuleName, tc.Module, popts)
}

// MarshalAST renders the AST of node as a want_ast fixture, under whichever
// marshalling options are currently in effect. A module marshals as an object, a
// body as an array of expressions.
//
// Comments are dropped, for the reason locations are off by default: the module
// is right there in the fixture, so recording them again asserts nothing a reader
// cannot see, while requiring an implementation to retain them in a particular
// shape. OPA's own is not one to hold anyone to — Comment marshals its text as
// base64 and its position unconditionally, ignoring the toggle that exists for it.
func MarshalAST(node any) (string, error) {
	if m, ok := node.(*ast.Module); ok {
		m.Comments = nil
	}

	bs, err := json.Marshal(node)
	if err != nil {
		return "", err
	}
	return conformance.FormatAST(bs)
}
