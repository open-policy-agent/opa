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

// MustParseExpr returns a parsed expression.
// If an error occurs during parsing, panic.
func MustParseExpr(input string) *Expr {
	parsed, err := ParseExpr(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseImports returns a slice of imports.
// If an error occurs during parsing, panic.
func MustParseImports(input string) []*Import {
	parsed, err := ParseImports(input)
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

// MustParsePackage returns a Package.
// If an error occurs during parsing, panic.
func MustParsePackage(input string) *Package {
	parsed, err := ParsePackage(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseStatements returns a slice of parsed statements.
// If an error occurs during parsing, panic.
func MustParseStatements(input string) []Statement {
	parsed, err := ParseStatements("", input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseStatement returns exactly one statement.
// If an error occurs during parsing, panic.
func MustParseStatement(input string) Statement {
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

// ParseRuleFromBody attempts to return a rule from a body. Equality expressions
// of the form <var> = <term> can be converted into rules of the form <var> =
// <term> { true }. This is a concise way of defining constants inside modules.
func ParseRuleFromBody(body Body) (*Rule, error) {

	if len(body) != 1 {
		return nil, fmt.Errorf("multiple %vs cannot be used for %v", ExprTypeName, HeadTypeName)
	}

	expr := body[0]
	if !expr.IsEquality() {
		return nil, fmt.Errorf("non-equality %v cannot be used for %v", ExprTypeName, HeadTypeName)
	}

	if len(expr.With) > 0 {
		return nil, fmt.Errorf("%vs using %v cannot be used for %v", ExprTypeName, WithTypeName, HeadTypeName)
	}

	terms := expr.Terms.([]*Term)
	var name Var

	switch v := terms[1].Value.(type) {
	case Var:
		name = v
	case Ref:
		if v.Equal(InputRootRef) || v.Equal(DefaultRootRef) {
			name = Var(v.String())
		} else {
			return nil, fmt.Errorf("%v cannot be used for name of %v", RefTypeName, RuleTypeName)
		}
	default:
		return nil, fmt.Errorf("%v cannot be used for name of %v", TypeName(v), RuleTypeName)
	}

	rule := &Rule{
		Head: &Head{
			Location: expr.Location,
			Name:     name,
			Value:    terms[2],
		},
		Body: NewBody(
			&Expr{Terms: BooleanTerm(true)},
		),
	}

	return rule, nil
}

// ParseImports returns a slice of Import objects.
func ParseImports(input string) ([]*Import, error) {
	stmts, err := ParseStatements("", input)
	if err != nil {
		return nil, err
	}
	result := []*Import{}
	for _, stmt := range stmts {
		if imp, ok := stmt.(*Import); ok {
			result = append(result, imp)
		} else {
			return nil, fmt.Errorf("expected import but got %T", stmt)
		}
	}
	return result, nil
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

	result := Body{}

	for _, stmt := range stmts {
		switch stmt := stmt.(type) {
		case Body:
			result = append(result, stmt...)
		case *Comment:
			// skip
		default:
			return nil, fmt.Errorf("expected body but got %T", stmt)
		}
	}

	setExprIndices(result)

	return result, nil
}

// ParseExpr returns exactly one expression.
// If multiple expressions are parsed, an error is returned.
func ParseExpr(input string) (*Expr, error) {
	body, err := ParseBody(input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse expression")
	}
	if len(body) != 1 {
		return nil, fmt.Errorf("expected exactly one expression but got: %v", body)
	}
	return body[0], nil
}

// ParsePackage returns exactly one Package.
// If multiple statements are parsed, an error is returned.
func ParsePackage(input string) (*Package, error) {
	stmt, err := ParseStatement(input)
	if err != nil {
		return nil, err
	}
	pkg, ok := stmt.(*Package)
	if !ok {
		return nil, fmt.Errorf("expected package but got %T", stmt)
	}
	return pkg, nil
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
func ParseStatement(input string) (Statement, error) {
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
func ParseStatements(filename, input string) ([]Statement, error) {

	parsed, err := Parse(filename, []byte(input))
	if err != nil {
		switch err := err.(type) {
		case errList:
			return nil, convertErrList(filename, err)
		default:
			return nil, err
		}
	}

	sl := parsed.([]interface{})
	stmts := make([]Statement, 0, len(sl))

	for _, x := range sl {
		if rules, ok := x.([]*Rule); ok {
			for _, rule := range rules {
				stmts = append(stmts, rule)
			}
		} else {
			// Unchecked cast should be safe. A panic indicates grammar is
			// out-of-sync.
			stmts = append(stmts, x.(Statement))
		}
	}

	postProcess(filename, stmts)

	return stmts, err
}

func convertErrList(filename string, errs errList) error {
	r := make(Errors, len(errs))
	for i, e := range errs {
		switch e := e.(type) {
		case *parserError:
			r[i] = formatParserError(filename, e)
		default:
			r[i] = NewError(ParseErr, nil, e.Error())
		}
	}
	return r
}

func formatParserError(filename string, e *parserError) *Error {
	loc := NewLocation(nil, filename, e.pos.line, e.pos.col)
	return NewError(ParseErr, loc, e.Inner.Error())
}

func parseModule(stmts []Statement) (*Module, error) {

	if len(stmts) == 0 {
		return nil, nil
	}

	var errs Errors

	_package, ok := stmts[0].(*Package)
	if !ok {
		loc := stmts[0].(Statement).Loc()
		errs = append(errs, NewError(ParseErr, loc, "expected %v", PackageTypeName))
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
			rule, err := ParseRuleFromBody(stmt)
			if err != nil {
				errs = append(errs, NewError(ParseErr, stmt[0].Location, err.Error()))
			} else {
				mod.Rules = append(mod.Rules, rule)
			}
		case *Package:
			errs = append(errs, NewError(ParseErr, stmt.Loc(), "unexpected "+PackageTypeName))
		case *Comment: // Drop comments for now.
		default:
			panic("illegal value") // Indicates grammar is out-of-sync with code.
		}
	}

	if len(errs) == 0 {
		return mod, nil
	}

	return nil, errs
}

func postProcess(filename string, stmts []Statement) error {
	setFilename(filename, stmts)

	if err := mangleDataVars(stmts); err != nil {
		return err
	}

	if err := mangleInputVars(stmts); err != nil {
		return err
	}

	mangleWildcards(stmts)
	mangleExprIndices(stmts)

	return nil
}

func mangleDataVars(stmts []Statement) error {
	for i := range stmts {
		vt := newVarToRefTransformer(DefaultRootDocument.Value.(Var), DefaultRootRef)
		stmt, err := Transform(vt, stmts[i])
		if err != nil {
			return err
		}
		stmts[i] = stmt.(Statement)
	}
	return nil
}

func mangleInputVars(stmts []Statement) error {
	for i := range stmts {
		vt := newVarToRefTransformer(InputRootDocument.Value.(Var), InputRootRef)
		stmt, err := Transform(vt, stmts[i])
		if err != nil {
			return err
		}
		stmts[i] = stmt.(Statement)
	}
	return nil
}

func mangleExprIndices(stmts []Statement) {
	for _, stmt := range stmts {
		setExprIndices(stmt)
	}
}

func setExprIndices(x interface{}) {
	WalkBodies(x, func(b Body) bool {
		for i, expr := range b {
			expr.Index = i
		}
		return false
	})
}

func mangleWildcards(stmts []Statement) {

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
	case *Set:
		vis.mangleSlice(*x)
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

func setFilename(filename string, stmts []Statement) {
	for _, stmt := range stmts {
		vis := &GenericVisitor{func(x interface{}) bool {
			switch x := x.(type) {
			case *Package:
				x.Location.File = filename
			case *Import:
				x.Location.File = filename
			case *Head:
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

type varToRefTransformer struct {
	orig   Var
	target Ref
	// skip set to true to avoid recursively processing the result of
	// transformation.
	skip bool
}

func newVarToRefTransformer(orig Var, target Ref) *varToRefTransformer {
	return &varToRefTransformer{
		orig:   orig,
		target: target,
		skip:   false,
	}
}

func (vt *varToRefTransformer) Transform(x interface{}) (interface{}, error) {
	if vt.skip {
		vt.skip = false
		return x, nil
	}
	switch x := x.(type) {
	case *Head:
		// The next AST node will be the rule name (which should not be
		// transformed).
		vt.skip = true
	case Ref:
		// The next AST node will be the ref head (which should not be transformed).
		vt.skip = true
	case Var:
		if x.Equal(vt.orig) {
			vt.skip = true
			return vt.target, nil
		}
	}
	return x, nil
}
