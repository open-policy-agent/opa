// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package parser

import (
	"io"

	"github.com/open-policy-agent/opa/ast"
	astJSON "github.com/open-policy-agent/opa/ast/json"
)

type Parser interface {
	Parse() ([]ast.Statement, []*ast.Comment, ast.Errors)
}

type Options struct {
	p *ast.Parser
}

type Option func(*Options)

// Filename provides the filename for Location details
// on parsed statements.
func Filename(filename string) Option {
	return func(o *Options) {
		o.p = o.p.WithFilename(filename)
	}
}

// Reader provides the io.Reader that the parser will
// use as its source.
func Reader(r io.Reader) Option {
	return func(o *Options) {
		o.p = o.p.WithReader(r)
	}
}

// ProcessAnnotation enables or disables the processing of
// annotations by the Parser
func ProcessAnnotation(annotations bool) Option {
	return func(o *Options) {
		o.p = o.p.WithProcessAnnotation(annotations)
	}
}

// FutureKeywords enables "future" keywords, i.e., keywords that can
// be imported via
//
//	import future.keywords.kw
//	import future.keywords.other
//
// but in a more direct way. The equivalent of this import would be
//
//	WithFutureKeywords("kw", "other")
func FutureKeywords(kws ...string) Option {
	return func(o *Options) {
		o.p = o.p.WithFutureKeywords(kws...)
	}
}

// AllFutureKeywords enables all "future" keywords, i.e., the
// ParserOption equivalent of
//
//	import future.keywords
func AllFutureKeywords(yes bool) Option {
	return func(o *Options) {
		o.p = o.p.WithAllFutureKeywords(yes)
	}
}

// Capabilities sets the capabilities structure on the parser.
func Capabilities(caps *ast.Capabilities) Option {
	return func(o *Options) {
		o.p = o.p.WithCapabilities(caps)
	}
}

// SkipRules instructs the parser not to attempt to parse Rule statements.
func SkipRules(skip bool) Option {
	return func(o *Options) {
		o.p = o.p.WithSkipRules(skip)
	}
}

// JSONOptions sets the Options which will be set on nodes to configure
// their JSON marshaling behavior.
func JSONOptions(opts *astJSON.Options) Option {
	return func(o *Options) {
		o.p = o.p.WithJSONOptions(opts)
	}
}

// RegoVersion sets the expected RegoVersion for parsed modules.
func RegoVersion(v ast.RegoVersion) Option {
	return func(o *Options) {
		o.p = o.p.WithRegoVersion(v)
	}
}

// NewParser creates and initializes a Parser.
//
// Returns a Parser that expects modules using the v1 syntax.
// To parse modules with the v0 syntax, use the [RegoVersion] option.
func NewParser(opts ...Option) Parser {
	p := ast.NewParser()

	options := Options{
		p: p,
	}

	for _, opt := range opts {
		opt(&options)
	}

	if p.ParserOptions().RegoVersion == ast.RegoUndefined {
		p = p.WithRegoVersion(ast.RegoV1)
	}

	return p
}
