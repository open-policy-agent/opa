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
	parsed, err := ParseModule(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseStatements returns a slice of parsed statements.
// If an error occurs during parsing, panic.
func MustParseStatements(input string) []interface{} {
	parsed, err := ParseStatements(input)
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

// ParseModule returns a parsed Module object.
// For details on Module objects and their fields, see policy.go.
// Empty input will return nil, nil.
func ParseModule(input string) (*Module, error) {
	stmts, err := ParseStatements(input)
	if err != nil {
		return nil, err
	}
	return parseModule(stmts)
}

// ParseModuleFile returns a parsed Module object.
func ParseModuleFile(filename string) (*Module, error) {
	parsed, err := ParseFile(filename)
	if err != nil {
		return nil, err
	}
	stmts := parsed.([]interface{})
	return parseModule(stmts)
}

// ParseBody returns exactly one body.
// If multiple bodies are parsed, an error is returned.
func ParseBody(input string) (Body, error) {
	stmts, err := ParseStatements(input)
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

// ParseRule returns exactly one rule.
// If multiple rules are parsed, an error is returned.
func ParseRule(input string) (*Rule, error) {
	stmts, err := ParseStatements(input)
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

// ParseStatements returns a slice of parsed statements.
// This is the default return value from the parser.
func ParseStatements(input string) ([]interface{}, error) {
	parsed, err := Parse("", []byte(input))
	if err != nil {
		return nil, err
	}
	stmts := parsed.([]interface{})
	return stmts, err
}

// ParseStatement returns exactly one statement.
// A statement might be a term, expression, rule, etc. Regardless,
// this function expects *exactly* one statement. If multiple
// statements are parsed, an error is returned.
func ParseStatement(input string) (interface{}, error) {
	stmts, err := ParseStatements(input)
	if err != nil {
		return nil, err
	}
	if len(stmts) != 1 {
		return nil, fmt.Errorf("expected exactly one statement")
	}
	return stmts[0], nil
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

// parseConstantRule attempts to return a rule from a Body.
// Equality expressions of the form <var> = <ground term> can be
// converted into rules of the form <var> = <ground term> :- true.
// This is a concise way of defining constants inside modules.
// This function handles the conversion.
func parseConstantRule(stmt Body) (*Rule, error) {
	if len(stmt) > 1 {
		return nil, fmt.Errorf("expression must be contained inside rule: %v", stmt)
	} else if len(stmt) == 1 {
		stmt := stmt[0]
		if !stmt.IsEquality() {
			return nil, fmt.Errorf("non-equality expression must be contained inside rule: %v", stmt)
		}
		terms := stmt.Terms.([]*Term)
		if !terms[2].IsGround() {
			return nil, fmt.Errorf("constant rule value must be ground: %v", stmt)
		}
		switch name := terms[1].Value.(type) {
		case Var:
			rule := &Rule{
				Location: stmt.Location,
				Name:     name,
				Value:    terms[2],
				Body: []*Expr{
					&Expr{Terms: BooleanTerm(true)},
				},
			}
			return rule, nil
		default:
			return nil, fmt.Errorf("rule name must be a variable: %v", stmt)
		}
	} else {
		panic("unreachable")
	}
}

func parseModule(stmts []interface{}) (*Module, error) {

	if len(stmts) == 0 {
		return nil, nil
	}

	_package, ok := stmts[0].(*Package)
	if !ok {
		return nil, fmt.Errorf("first statement must be package")
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
			rule, err := parseConstantRule(stmt)
			if err != nil {
				return nil, err
			}
			mod.Rules = append(mod.Rules, rule)
		}
	}

	return mod, nil
}
