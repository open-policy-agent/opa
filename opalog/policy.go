// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opalog

import "fmt"
import "strings"

// DefaultRootDocument is the default root document.
// All package directives inside source files are implicitly
// prefixed with the DefaultRootDocument value.
var DefaultRootDocument = VarTerm("data")

// Keywords is an array of reserved keywords in the language.
// These are reserved names that cannot be used for variables.
var Keywords = [...]string{
	"package", "import", "not",
}

type (
	// Module represents a collection of policies (defined by rules)
	// within a namespace (defined by the package) and optional
	// dependencies on external documents (defined by imports).
	Module struct {
		Package *Package
		Imports []*Import
		Rules   []*Rule
	}

	// Package represents the namespace of the documents produced
	// by rules inside the module.
	Package struct {
		Location *Location
		Path     Ref
	}

	// Import represents a dependency on a document outside of the policy
	// namespace. Imports are optional.
	Import struct {
		Location *Location
		Path     Value
		Alias    Var
	}

	// Rule represents a rule as defined by Opalog. Rules define the
	// content of documents that represent policy decisions.
	Rule struct {
		Location *Location
		Name     Var
		Key      *Term
		Value    *Term
		Body     Body
	}

	// Body represents one or more expressios contained inside a rule.
	Body []*Expr

	// Expr represents a single expression contained inside the body of a rule.
	Expr struct {
		Location *Location
		Negated  bool
		Terms    interface{}
	}
)

// Equal returns true if this Module equals the other Module.
// Two modules are equal if they contain the same package,
// ordered imports, and ordered rules.
func (mod *Module) Equal(other *Module) bool {
	if !mod.Package.Equal(other.Package) {
		return false
	}
	if len(mod.Imports) != len(other.Imports) {
		return false
	}
	for i := range mod.Imports {
		if !mod.Imports[i].Equal(other.Imports[i]) {
			return false
		}
	}
	if len(mod.Rules) != len(other.Rules) {
		return false
	}
	for i := range mod.Rules {
		if !mod.Rules[i].Equal(other.Rules[i]) {
			return false
		}
	}
	return true
}

// Equal returns true if this Package has the same path as the other Package.
func (pkg *Package) Equal(other *Package) bool {
	return pkg.Path.Equal(other.Path)
}

func (pkg *Package) String() string {
	return fmt.Sprintf("package %v", pkg.Path)
}

// Equal returns true if this Import has the same path and alias as the other Import.
func (imp *Import) Equal(other *Import) bool {
	return imp.Alias.Equal(other.Alias) && imp.Path.Equal(other.Path)
}

func (imp *Import) String() string {
	buf := []string{"import", imp.Path.String()}
	if len(imp.Alias) > 0 {
		buf = append(buf, "as "+imp.Alias.String())
	}
	return strings.Join(buf, " ")
}

// Equal returns true if this Rule has the same name, arguments, and body as the other Rule.
func (rule *Rule) Equal(other *Rule) bool {
	if !rule.Name.Equal(other.Name) {
		return false
	}
	if !rule.Key.Equal(other.Key) {
		return false
	}
	if !rule.Value.Equal(other.Value) {
		return false
	}
	return rule.Body.Equal(other.Body)
}

// DocKind represents the collection of document types that can be produced by rules.
type DocKind int

const (
	// CompleteDoc represents a document that is completely defined by the rule.
	CompleteDoc = iota

	// PartialSetDoc represents a set document that is partially defined by the rule.
	PartialSetDoc = iota

	// PartialObjectDoc represents an object document that is partially defined by the rule.
	PartialObjectDoc = iota
)

// DocKind returns the type of document produced by this rule.
func (rule *Rule) DocKind() DocKind {
	if rule.Key != nil {
		if rule.Value != nil {
			return PartialObjectDoc
		}
		return PartialSetDoc
	}
	return CompleteDoc
}

func (rule *Rule) String() string {
	var buf []string
	if rule.Key != nil {
		buf = append(buf, rule.Name.String()+"["+rule.Key.String()+"]")
	} else {
		buf = append(buf, rule.Name.String())
	}
	if rule.Value != nil {
		buf = append(buf, "=")
		buf = append(buf, rule.Value.String())
	}
	if len(rule.Body) >= 0 {
		buf = append(buf, ":-")
		buf = append(buf, rule.Body.String())
	}
	return strings.Join(buf, " ")
}

// Equal returns true if this Body is equal to the other Body.
// Two bodies are equal if consist of equal, ordered expressions.
func (body Body) Equal(other Body) bool {
	if len(body) != len(other) {
		return false
	}
	for i := range body {
		if !body[i].Equal(other[i]) {
			return false
		}
	}
	return true
}

func (body Body) String() string {
	var buf []string
	for _, v := range body {
		buf = append(buf, v.String())
	}
	return strings.Join(buf, ", ")
}

// Complement returns a copy of this expression with the negation flag flipped.
func (expr *Expr) Complement() *Expr {
	cpy := *expr
	cpy.Negated = !cpy.Negated
	return &cpy
}

// Equal returns true if this Expr equals the other Expr.
// Two expressions are considered equal if both expressions are negated (or not),
// are built-ins (or not), and have the same ordered terms.
func (expr *Expr) Equal(other *Expr) bool {
	if expr.Negated != other.Negated {
		return false
	}
	switch t := expr.Terms.(type) {
	case *Term:
		switch u := other.Terms.(type) {
		case *Term:
			return t.Equal(u)
		}
	case []*Term:
		switch u := other.Terms.(type) {
		case []*Term:
			return termSliceEqual(t, u)
		}
	}
	return false
}

// IsEquality returns true if this is an equality expression.
func (expr *Expr) IsEquality() bool {
	terms, ok := expr.Terms.([]*Term)
	if !ok {
		return false
	}
	if len(terms) != 3 {
		return false
	}
	return terms[0].Equal(VarTerm("="))
}

var builtinNames = map[string]string{
	"=":  "eq",
	"<":  "lt",
	">":  "gt",
	"<=": "lte",
	">=": "gte",
	"!=": "ne",
}

func (expr *Expr) String() string {
	var buf []string
	if expr.Negated {
		buf = append(buf, "not")
	}
	switch t := expr.Terms.(type) {
	case []*Term:
		var args []string
		for _, v := range t[1:] {
			args = append(args, v.String())
		}
		name, ok := builtinNames[string(t[0].Value.(Var))]
		if !ok {
			name = t[0].String()
		}
		builtinStr := fmt.Sprintf("%s(%s)", name, strings.Join(args, ", "))
		buf = append(buf, builtinStr)
	case *Term:
		buf = append(buf, t.String())
	}
	return strings.Join(buf, " ")
}

// NewBuiltinExpr creates a new Expr object with the supplied terms.
// The builtin operator must be the first term.
func NewBuiltinExpr(terms ...*Term) *Expr {
	return &Expr{Terms: terms}
}
