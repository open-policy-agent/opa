// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// This file contains extra functions for parsing Rego.
// Most of the parsing is handled by the auto-generated code in
// parser.go, however, there are additional utilities that are
// helpful for dealing with Rego source inputs (e.g., REPL
// statements, source files, etc.)

package ast

import (
	"fmt"

	"github.com/pkg/errors"
)

// MustParseBody returns a parsed body.
// If an error occurs during parsing, panic.
func MustParseBody(input string) Body {
	parsed, err := ParseBody(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseModule returns a parsed module.
// If an error occurs during parsing, panic.
func MustParseModule(input string) *Module {
	parsed, err := ParseModule("", input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseStatements returns a slice of parsed statements.
// If an error occurs during parsing, panic.
func MustParseStatements(input string) []interface{} {
	parsed, err := ParseStatements("", input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseStatement returns exactly one statement.
// If an error occurs during parsing, panic.
func MustParseStatement(input string) interface{} {
	parsed, err := ParseStatement(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseRef returns a parsed reference.
// If an error occurs during parsing, panic.
func MustParseRef(input string) Ref {
	parsed, err := ParseRef(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseRule returns a parsed rule.
// If an error occurs during parsing, panic.
func MustParseRule(input string) *Rule {
	parsed, err := ParseRule(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseTerm returns a parsed term.
// If an error occurs during parsing, panic.
func MustParseTerm(input string) *Term {
	parsed, err := ParseTerm(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// ParseConstantRule attempts to return a rule from a body.
// Equality expressions of the form <var> = <ground term> can be
// converted into rules of the form <var> = <ground term> :- true.
// This is a concise way of defining constants inside modules.
func ParseConstantRule(body Body) *Rule {
	if len(body) != 1 {
		return nil
	}
	expr := body[0]
	if !expr.IsEquality() {
		return nil
	}
	terms := expr.Terms.([]*Term)
	a, b := terms[1], terms[2]
	if !b.IsGround() {
		return nil
	}
	name, ok := a.Value.(Var)
	if !ok {
		return nil
	}
	return &Rule{
		Location: expr.Location,
		Name:     name,
		Value:    b,
		Body: []*Expr{
			&Expr{Terms: BooleanTerm(true)},
		},
	}
}

// ParseModule returns a parsed Module object.
// For details on Module objects and their fields, see policy.go.
// Empty input will return nil, nil.
func ParseModule(filename, input string) (*Module, error) {
	stmts, err := ParseStatements(filename, input)
	if err != nil {
		return nil, err
	}
	return parseModule(stmts)
}

// ParseBody returns exactly one body.
// If multiple bodies are parsed, an error is returned.
func ParseBody(input string) (Body, error) {
	stmts, err := ParseStatements("", input)
	if err != nil {
		return nil, err
	}
	if len(stmts) != 1 {
		return nil, fmt.Errorf("expected exactly one statement (body)")
	}
	body, ok := stmts[0].(Body)
	if !ok {
		return nil, fmt.Errorf("expected body but got %T", stmts[0])
	}
	return body, nil
}

// ParseTerm returns exactly one term.
// If multiple terms are parsed, an error is returned.
func ParseTerm(input string) (*Term, error) {
	body, err := ParseBody(input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse term")
	}
	if len(body) > 1 {
		return nil, fmt.Errorf("expected exactly one term but got %v", body)
	}
	term, ok := body[0].Terms.(*Term)
	if !ok {
		return nil, fmt.Errorf("expected term but got %v", body[0].Terms)
	}
	return term, nil
}

// ParseRef returns exactly one reference.
func ParseRef(input string) (Ref, error) {
	term, err := ParseTerm(input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse ref")
	}
	ref, ok := term.Value.(Ref)
	if !ok {
		return nil, fmt.Errorf("expected ref but got %v", term)
	}
	return ref, nil
}

// ParseRule returns exactly one rule.
// If multiple rules are parsed, an error is returned.
func ParseRule(input string) (*Rule, error) {
	stmts, err := ParseStatements("", input)
	if err != nil {
		return nil, err
	}
	if len(stmts) != 1 {
		return nil, fmt.Errorf("expected exactly one statement (rule)")
	}
	rule, ok := stmts[0].(*Rule)
	if !ok {
		return nil, fmt.Errorf("expected rule but got %T", stmts[0])
	}
	return rule, nil
}

// ParseStatement returns exactly one statement.
// A statement might be a term, expression, rule, etc. Regardless,
// this function expects *exactly* one statement. If multiple
// statements are parsed, an error is returned.
func ParseStatement(input string) (interface{}, error) {
	stmts, err := ParseStatements("", input)
	if err != nil {
		return nil, err
	}
	if len(stmts) != 1 {
		return nil, fmt.Errorf("expected exactly one statement")
	}
	return stmts[0], nil
}

// ParseStatements returns a slice of parsed statements.
// This is the default return value from the parser.
func ParseStatements(filename, input string) ([]interface{}, error) {
	parsed, err := Parse(filename, []byte(input))
	if err != nil {
		return nil, err
	}
	stmts := parsed.([]interface{})
	postProcess(filename, stmts)
	return stmts, err
}

func parseModule(stmts []interface{}) (*Module, error) {

	if len(stmts) == 0 {
		return nil, nil
	}

	_package, ok := stmts[0].(*Package)
	if !ok {
		loc := stmts[0].(Statement).Loc()
		return nil, loc.Errorf("expected package directive (%s must come after package directive)", stmts[0])
	}

	mod := &Module{
		Package: _package,
	}

	for _, stmt := range stmts[1:] {
		switch stmt := stmt.(type) {
		case *Import:
			mod.Imports = append(mod.Imports, stmt)
		case *Rule:
			mod.Rules = append(mod.Rules, stmt)
		case Body:
			rule := ParseConstantRule(stmt)
			if rule == nil {
				return nil, stmt[0].Location.Errorf("expected rule (%s must be declared inside a rule)", stmt[0].Location.Text)
			}
			mod.Rules = append(mod.Rules, rule)
		}
	}

	return mod, nil
}

func postProcess(filename string, stmts []interface{}) {
	setFilename(filename, stmts)
	mangleWildcards(stmts)
}

func mangleWildcards(stmts []interface{}) {

	mangler := &wildcardMangler{}
	for _, stmt := range stmts {
		Walk(mangler, stmt)
	}
}

type wildcardMangler struct {
	c int
}

func (vis *wildcardMangler) Visit(x interface{}) Visitor {
	switch x := x.(type) {
	case Object:
		for _, i := range x {
			vis.mangleSlice(i[:])
		}
	case Array:
		vis.mangleSlice(x)
	case Ref:
		vis.mangleSlice(x)
	case *Expr:
		switch ts := x.Terms.(type) {
		case []*Term:
			vis.mangleSlice(ts)
		case *Term:
			vis.mangle(ts)
		}
	}
	return vis
}

func (vis *wildcardMangler) mangle(x *Term) {
	if x.Equal(Wildcard) {
		name := fmt.Sprintf("%s%d", WildcardPrefix, vis.c)
		x.Value = Var(name)
		vis.c++
	}
}

func (vis *wildcardMangler) mangleSlice(xs []*Term) {
	for _, x := range xs {
		vis.mangle(x)
	}
}

func setFilename(filename string, stmts []interface{}) {
	for _, stmt := range stmts {
		vis := &GenericVisitor{func(x interface{}) bool {
			switch x := x.(type) {
			case *Package:
				x.Location.File = filename
			case *Import:
				x.Location.File = filename
			case *Rule:
				x.Location.File = filename
			case *Expr:
				x.Location.File = filename
			case *Term:
				x.Location.File = filename
			}
			return false
		}}
		Walk(vis, stmt)
	}
}
