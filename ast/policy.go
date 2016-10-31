// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DefaultRootDocument is the default root document.
// All package directives inside source files are implicitly
// prefixed with the DefaultRootDocument value.
var DefaultRootDocument = VarTerm("data")

// ReservedVars is the set of reserved variable names.
var ReservedVars = NewVarSet(DefaultRootDocument.Value.(Var))

// Wildcard represents the wildcard variable as defined in the language.
var Wildcard = &Term{Value: Var("_")}

// WildcardPrefix is the special character that all wildcard variables are
// prefixed with when the statement they are contained in is parsed.
var WildcardPrefix = "$"

type (

	// Module represents a collection of policies (defined by rules)
	// within a namespace (defined by the package) and optional
	// dependencies on external documents (defined by imports).
	Module struct {
		Package *Package
		Imports []*Import
		Rules   []*Rule
	}

	// Statement represents a single statement within a module.
	Statement interface {
		Loc() *Location
	}

	// Package represents the namespace of the documents produced
	// by rules inside the module.
	Package struct {
		Location *Location `json:"-"`
		Path     Ref
	}

	// Import represents a dependency on a document outside of the policy
	// namespace. Imports are optional.
	Import struct {
		Location *Location `json:"-"`
		Path     *Term
		Alias    Var `json:",omitempty"`
	}

	// Rule represents a rule as defined in the language. Rules define the
	// content of documents that represent policy decisions.
	Rule struct {
		Location *Location `json:"-"`
		Name     Var
		Key      *Term `json:",omitempty"`
		Value    *Term `json:",omitempty"`
		Body     Body
	}

	// Head represents the head of a rule.
	// TODO(tsandall): refactor Rule to contain a Head.
	Head struct {
		Name  Var
		Key   *Term
		Value *Term
	}

	// Body represents one or more expressios contained inside a rule.
	Body []*Expr

	// Expr represents a single expression contained inside the body of a rule.
	Expr struct {
		Location *Location `json:"-"`
		Index    int
		Negated  bool `json:",omitempty"`
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

func (mod *Module) String() string {
	buf := []string{}
	buf = append(buf, mod.Package.String())
	buf = append(buf, "")
	for _, imp := range mod.Imports {
		buf = append(buf, imp.String())
	}
	buf = append(buf, "")
	for _, rule := range mod.Rules {
		buf = append(buf, rule.String())
	}
	return strings.Join(buf, "\n")
}

// Equal returns true if this Package has the same path as the other Package.
func (pkg *Package) Equal(other *Package) bool {
	return pkg.Path.Equal(other.Path)
}

// Loc returns the location of the Package in the definition.
func (pkg *Package) Loc() *Location {
	return pkg.Location
}

func (pkg *Package) String() string {
	// Omit head as all packages have the DefaultRootDocument prepended at parse time.
	path := make(Ref, len(pkg.Path)-1)
	path[0] = VarTerm(string(pkg.Path[1].Value.(String)))
	copy(path[1:], pkg.Path[2:])
	return fmt.Sprintf("package %v", path)
}

// Equal returns true if this Import has the same path and alias as the other Import.
func (imp *Import) Equal(other *Import) bool {
	return imp.Alias.Equal(other.Alias) && imp.Path.Equal(other.Path)
}

// Loc returns the location of the Import in the definition.
func (imp *Import) Loc() *Location {
	return imp.Location
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

// HeadVars returns map where keys represent all of the variables found in the
// head of the rule. The values of the map are ignored.
func (rule *Rule) HeadVars() VarSet {
	vis := &varVisitor{vars: VarSet{}}
	if rule.Key != nil {
		Walk(vis, rule.Key)
	}
	if rule.Value != nil {
		Walk(vis, rule.Value)
	}
	return vis.vars
}

// Head returns the rule's head.
func (rule *Rule) Head() *Head {
	return &Head{
		Name:  rule.Name,
		Key:   rule.Key,
		Value: rule.Value,
	}
}

// Loc returns the location of the Rule in the definition.
func (rule *Rule) Loc() *Location {
	return rule.Location
}

// Path returns a reference that identifies the rule under ns.
func (rule *Rule) Path(ns Ref) Ref {
	return ns.Append(StringTerm(string(rule.Name)))
}

func (rule *Rule) String() string {
	buf := []string{rule.Head().String()}
	if len(rule.Body) >= 0 {
		buf = append(buf, ":-")
		buf = append(buf, rule.Body.String())
	}
	return strings.Join(buf, " ")
}

func (head *Head) String() string {
	var buf []string
	if head.Key != nil {
		buf = append(buf, head.Name.String()+"["+head.Key.String()+"]")
	} else {
		buf = append(buf, head.Name.String())
	}
	if head.Value != nil {
		buf = append(buf, "=")
		buf = append(buf, head.Value.String())
	}
	return strings.Join(buf, " ")
}

// NewBody returns a new Body containing the given expressions. The indices of
// the immediate expressions will be reset.
func NewBody(exprs ...*Expr) Body {
	for i, expr := range exprs {
		expr.Index = i
	}
	return Body(exprs)
}

// Contains returns true if this body contains the given expression.
func (body Body) Contains(x *Expr) bool {
	for _, e := range body {
		if e.Equal(x) {
			return true
		}
	}
	return false
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

// Hash returns the hash code for the Body.
func (body Body) Hash() int {
	s := 0
	for _, e := range body {
		s += e.Hash()
	}
	return s
}

// IsGround returns true if all of the expressions in the Body are ground.
func (body Body) IsGround() bool {
	for _, e := range body {
		if !e.IsGround() {
			return false
		}
	}
	return true
}

// Loc returns the location of the Body in the definition.
func (body Body) Loc() *Location {
	return body[0].Location
}

// OutputVars returns a VarSet containing the variables that would be bound by evaluating
// the body.
func (body Body) OutputVars(safe VarSet) VarSet {
	o := safe.Copy()
	for _, e := range body {
		o.Update(e.OutputVars(o))
	}
	return o.Diff(safe)
}

func (body Body) String() string {
	var buf []string
	for _, v := range body {
		buf = append(buf, v.String())
	}
	return strings.Join(buf, ", ")
}

// Vars returns a VarSet containing all of the variables in the body. If skipClosures is true,
// variables contained inside closures within the body will be ignored.
func (body Body) Vars(skipClosures bool) VarSet {
	vis := &varVisitor{
		vars:         VarSet{},
		skipClosures: skipClosures,
	}
	Walk(vis, body)
	return vis.vars
}

// Complement returns a copy of this expression with the negation flag flipped.
func (expr *Expr) Complement() *Expr {
	cpy := *expr
	cpy.Negated = !cpy.Negated
	return &cpy
}

// Equal returns true if this Expr equals the other Expr.  Two expressions are
// considered equal if both expressions are negated (or not), exist at the same
// position in the containing query, are built-ins (or not), and have the same
// ordered terms.
func (expr *Expr) Equal(other *Expr) bool {
	if expr.Negated != other.Negated {
		return false
	}
	if expr.Index != other.Index {
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

// Hash returns the hash code of the Expr.
func (expr *Expr) Hash() int {
	s := expr.Index
	switch ts := expr.Terms.(type) {
	case []*Term:
		for _, t := range ts {
			s += t.Value.Hash()
		}
	case *Term:
		s += ts.Value.Hash()
	}
	if expr.Negated {
		s++
	}
	return s
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
	return terms[0].Value.Equal(Equality.Name)
}

// IsGround returns true if all of the expression terms are ground.
func (expr *Expr) IsGround() bool {
	switch ts := expr.Terms.(type) {
	case []*Term:
		for _, t := range ts[1:] {
			if !t.IsGround() {
				return false
			}
		}
	case *Term:
		return ts.IsGround()
	}
	return true
}

// OutputVars returns a VarSet containing variables that would be bound by evaluating
// this expression.
func (expr *Expr) OutputVars(safe VarSet) VarSet {
	if !expr.Negated {
		switch terms := expr.Terms.(type) {
		case *Term:
			return expr.outputVarsRefs()
		case []*Term:
			name := terms[0].Value.(Var)
			if b := BuiltinMap[name]; b != nil {
				if b.Name.Equal(Equality.Name) {
					return expr.outputVarsEquality(safe)
				}
				return expr.outputVarsBuiltins(b, safe)
			}
		}
	}
	return VarSet{}
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
		name := t[0].Value.(Var).String()
		s := fmt.Sprintf("%s(%s)", name, strings.Join(args, ", "))
		buf = append(buf, s)
	case *Term:
		buf = append(buf, t.String())
	}
	return strings.Join(buf, " ")
}

// UnmarshalJSON parses the byte array and stores the result in expr.
func (expr *Expr) UnmarshalJSON(bs []byte) error {
	v := map[string]interface{}{}
	if err := json.Unmarshal(bs, &v); err != nil {
		return err
	}
	return unmarshalExpr(expr, v)
}

// Vars returns a VarSet containing all of the variables in the expression.
// If skipClosures is true then variables contained inside closures within this
// expression will not be included in the VarSet.
func (expr *Expr) Vars(skipClosures bool) VarSet {
	vis := &varVisitor{
		skipClosures: skipClosures,
		vars:         VarSet{},
	}
	Walk(vis, expr)
	return vis.vars
}

func (expr *Expr) outputVarsBuiltins(b *Builtin, safe VarSet) VarSet {

	o := expr.outputVarsRefs()
	terms := expr.Terms.([]*Term)

	// Check that all input terms are ground or safe.
	for i, t := range terms[1:] {
		if b.IsTargetPos(i) {
			continue
		}
		if t.Value.IsGround() {
			continue
		}
		vis := &varVisitor{
			skipClosures:     true,
			skipObjectKeys:   true,
			skipRefHead:      true,
			skipBuiltinNames: true,
			vars:             VarSet{},
		}
		Walk(vis, t)
		unsafe := vis.vars.Diff(o).Diff(safe)
		if len(unsafe) > 0 {
			return VarSet{}
		}
	}

	// Add vars in target positions to result.
	for i, t := range terms[1:] {
		if v, ok := t.Value.(Var); ok {
			if b.IsTargetPos(i) {
				o.Add(v)
			}
		}
	}

	return o
}

func (expr *Expr) outputVarsEquality(safe VarSet) VarSet {
	ts := expr.Terms.([]*Term)
	o := expr.outputVarsRefs()
	o.Update(safe)
	o.Update(Unify(o, ts[1], ts[2]))
	return o.Diff(safe)
}

func (expr *Expr) outputVarsRefs() VarSet {
	o := VarSet{}
	WalkRefs(expr, func(r Ref) bool {
		o.Update(r.OutputVars())
		return false
	})
	return o
}

// NewBuiltinExpr creates a new Expr object with the supplied terms.
// The builtin operator must be the first term.
func NewBuiltinExpr(terms ...*Term) *Expr {
	return &Expr{Terms: terms}
}

type varVisitor struct {
	skipRefHead      bool
	skipObjectKeys   bool
	skipClosures     bool
	skipBuiltinNames bool
	vars             VarSet
}

func (vis *varVisitor) Visit(v interface{}) Visitor {
	if vis.skipObjectKeys {
		if o, ok := v.(Object); ok {
			for _, i := range o {
				Walk(vis, i[1])
			}
			return nil
		}
	}
	if vis.skipRefHead {
		if r, ok := v.(Ref); ok {
			for _, t := range r[1:] {
				Walk(vis, t)
			}
			return nil
		}
	}
	if vis.skipClosures {
		switch v.(type) {
		case *ArrayComprehension:
			return nil
		}
	}
	if vis.skipBuiltinNames {
		if v, ok := v.(*Expr); ok {
			if ts, ok := v.Terms.([]*Term); ok {
				for _, t := range ts[1:] {
					Walk(vis, t)
				}
				return nil
			}
		}
	}
	if v, ok := v.(Var); ok {
		vis.vars.Add(v)
	}
	return vis
}
