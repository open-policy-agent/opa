// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package parser

import "github.com/open-policy-agent/opa/ast"

// FIXME: Do we need this type alias? It (and return types) will contain ast pkg types, so an ast import will still be needed in some user-side code.
// SDK might feel more cohesive if it's kept ..
//type ParseOptions = ast.ParserOptions

// ParseBody returns exactly one body.
// If multiple bodies are parsed, an error is returned.
func ParseBody(input string) (ast.Body, error) {
	return ParseBodyWithOpts(input, ast.ParserOptions{SkipRules: true})
}

// ParseBodyWithOpts returns exactly one body. It does _not_ set SkipRules: true on its own,
// but respects whatever ParserOptions it's been given.
func ParseBodyWithOpts(input string, popts ast.ParserOptions) (ast.Body, error) {
	return ast.ParseBodyWithOpts(input, setDefaultRegoVersion(popts))
}

func MustParseBody(input string) ast.Body {
	return MustParseBodyWithOpts(input, ast.ParserOptions{SkipRules: true})
}

func MustParseBodyWithOpts(input string, popts ast.ParserOptions) ast.Body {
	return ast.MustParseBodyWithOpts(input, setDefaultRegoVersion(popts))
}

// ParseModule returns a parsed Module object.
// For details on Module objects and their fields, see policy.go.
// Empty input will return nil, nil.
func ParseModule(filename, input string) (*ast.Module, error) {
	return ParseModuleWithOpts(filename, input, ast.ParserOptions{})
}

// ParseModuleWithOpts returns a parsed Module object, and has an additional input ParserOptions
// For details on Module objects and their fields, see policy.go.
// Empty input will return nil, nil.
func ParseModuleWithOpts(filename, input string, popts ast.ParserOptions) (*ast.Module, error) {
	return ast.ParseModuleWithOpts(filename, input, setDefaultRegoVersion(popts))
}

func MustParseModule(input string) *ast.Module {
	return MustParseModuleWithOpts(input, ast.ParserOptions{})
}

func MustParseModuleWithOpts(input string, popts ast.ParserOptions) *ast.Module {
	return ast.MustParseModuleWithOpts(input, setDefaultRegoVersion(popts))
}

// TODO: Add more parse functions (do we also want MustParse* variants?)

func setDefaultRegoVersion(opts ast.ParserOptions) ast.ParserOptions {
	if opts.RegoVersion == ast.RegoUndefined {
		opts.RegoVersion = ast.RegoV1
	}
	return opts
}
