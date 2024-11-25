// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package format implements formatting of Rego source files.
package format

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/types"
	v1 "github.com/open-policy-agent/opa/v1/format"
)

// Opts lets you control the code formatting via `AstWithOpts()`.
type Opts = v1.Opts

// Source formats a Rego source file. The bytes provided must describe a complete
// Rego module. If they don't, Source will return an error resulting from the attempt
// to parse the bytes.
func Source(filename string, src []byte) ([]byte, error) {
	return SourceWithOpts(filename, src, Opts{
		RegoVersion: ast.DefaultRegoVersion,
		ParserOptions: &ast.ParserOptions{
			RegoVersion: ast.DefaultRegoVersion,
		},
	})
}

func SourceWithOpts(filename string, src []byte, opts Opts) ([]byte, error) {
	if opts.RegoVersion == ast.RegoUndefined {
		opts.RegoVersion = ast.DefaultRegoVersion
	}
	if opts.ParserOptions == nil {
		opts.ParserOptions = &ast.ParserOptions{}
	}
	if opts.ParserOptions.RegoVersion == ast.RegoUndefined {
		opts.ParserOptions.RegoVersion = ast.DefaultRegoVersion
	}

	return v1.SourceWithOpts(filename, src, opts)
}

// MustAst is a helper function to format a Rego AST element. If any errors
// occurs this function will panic. This is mostly used for test
func MustAst(x interface{}) []byte {
	bs, err := Ast(x)
	if err != nil {
		panic(err)
	}
	return bs
}

// MustAstWithOpts is a helper function to format a Rego AST element. If any errors
// occurs this function will panic. This is mostly used for test
func MustAstWithOpts(x interface{}, opts Opts) []byte {
	bs, err := AstWithOpts(x, opts)
	if err != nil {
		panic(err)
	}
	return bs
}

// Ast formats a Rego AST element. If the passed value is not a valid AST
// element, Ast returns nil and an error. If AST nodes are missing locations
// an arbitrary location will be used.
func Ast(x interface{}) ([]byte, error) {
	return AstWithOpts(x, Opts{
		RegoVersion: ast.DefaultRegoVersion,
	})
}

func AstWithOpts(x interface{}, opts Opts) ([]byte, error) {
	if opts.RegoVersion == ast.RegoUndefined {
		opts.RegoVersion = ast.DefaultRegoVersion
	}

	return v1.AstWithOpts(x, opts)
}

// ArgErrDetail but for `fmt` checks since compiler has not run yet.
type ArityFormatErrDetail = v1.ArityFormatErrDetail

// arityMismatchError but for `fmt` checks since the compiler has not run yet.
func ArityFormatMismatchError(operands []*ast.Term, operator string, loc *ast.Location, f *types.Function) *ast.Error {
	return v1.ArityFormatMismatchError(operands, operator, loc, f)
}
