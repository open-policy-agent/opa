// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/util"
)

// DefaultRootDocument is the default root document.
//
// All package directives inside source files are implicitly prefixed with the
// DefaultRootDocument value.
var DefaultRootDocument = VarTerm("data")

// InputRootDocument names the document containing query arguments.
var InputRootDocument = VarTerm("input")

// RootDocumentNames contains the names of top-level documents that can be
// referred to in modules and queries.
var RootDocumentNames = &Set{
	DefaultRootDocument,
	InputRootDocument,
}

// DefaultRootRef is a reference to the root of the default document.
//
// All refs to data in the policy engine's storage layer are prefixed with this ref.
var DefaultRootRef = Ref{DefaultRootDocument}

// InputRootRef is a reference to the root of the input document.
//
// All refs to query arguments are prefixed with this ref.
var InputRootRef = Ref{InputRootDocument}

// RootDocumentRefs contains the prefixes of top-level documents that all
// non-local references start with.
var RootDocumentRefs = &Set{
	NewTerm(DefaultRootRef),
	NewTerm(InputRootRef),
}

// SystemDocumentKey is the name of the top-level key that identifies the system
// document.
var SystemDocumentKey = String("system")

// ReservedVars is the set of names that refer to implicitly ground vars.
var ReservedVars = NewVarSet(
	DefaultRootDocument.Value.(Var),
	InputRootDocument.Value.(Var),
)

// Wildcard represents the wildcard variable as defined in the language.
var Wildcard = &Term{Value: Var("_")}

// WildcardPrefix is the special character that all wildcard variables are
// prefixed with when the statement they are contained in is parsed.
var WildcardPrefix = "$"

// Keywords contains strings that map to language keywords.
var Keywords = [...]string{
	"not",
	"package",
	"import",
	"as",
	"default",
	"with",
	"null",
	"true",
	"false",
}

// IsKeyword returns true if s is a language keyword.
func IsKeyword(s string) bool {
	for _, x := range Keywords {
		if x == s {
			return true
		}
	}
	return false
}

type (
	// Statement represents a single statement in a policy module.
	Statement interface {
		Loc() *Location
	}
)

type (

	// Module represents a collection of policies (defined by rules)
	// within a namespace (defined by the package) and optional
	// dependencies on external documents (defined by imports).
	Module struct {
		Package *Package  `json:"package"`
		Imports []*Import `json:"imports,omitempty"`
		Rules   []*Rule   `json:"rules,omitempty"`
	}

	// Comment contains the raw text from the comment in the definition.
	Comment struct {
		Text     []byte
		Location *Location
	}

	// Package represents the namespace of the documents produced
	// by rules inside the module.
	Package struct {
		Location *Location `json:"-"`
		Path     Ref       `json:"path"`
	}

	// Import represents a dependency on a document outside of the policy
	// namespace. Imports are optional.
	Import struct {
		Location *Location `json:"-"`
		Path     *Term     `json:"path"`
		Alias    Var       `json:"alias,omitempty"`
	}

	// Rule represents a rule as defined in the language. Rules define the
	// content of documents that represent policy decisions.
	Rule struct {
		Default bool  `json:"default,omitempty"`
		Head    *Head `json:"head"`
		Body    Body  `json:"body"`
	}

	// Head represents the head of a rule.
	Head struct {
		Location *Location `json:"-"`
		Name     Var       `json:"name"`
		Key      *Term     `json:"key,omitempty"`
		Value    *Term     `json:"value,omitempty"`
	}

	// Body represents one or more expressios contained inside a rule.
	Body []*Expr

	// Expr represents a single expression contained inside the body of a rule.
	Expr struct {
		Location *Location   `json:"-"`
		Index    int         `json:"index"`
		Negated  bool        `json:"negated,omitempty"`
		Terms    interface{} `json:"terms"`
		With     []*With     `json:"with,omitempty"`
	}

	// With represents a modifier on an expression.
	With struct {
		Location *Location `json:"-"`
		Target   *Term     `json:"target"`
		Value    *Term     `json:"value"`
	}
)

// Compare returns an integer indicating whether mod is less than, equal to,
// or greater than other.
func (mod *Module) Compare(other *Module) int {
	if cmp := mod.Package.Compare(other.Package); cmp != 0 {
		return cmp
	}
	if cmp := importsCompare(mod.Imports, other.Imports); cmp != 0 {
		return cmp
	}
	return rulesCompare(mod.Rules, other.Rules)
}

// Copy returns a deep copy of mod.
func (mod *Module) Copy() *Module {
	cpy := *mod
	cpy.Rules = make([]*Rule, len(mod.Rules))
	for i := range mod.Rules {
		cpy.Rules[i] = mod.Rules[i].Copy()
	}
	cpy.Imports = make([]*Import, len(mod.Imports))
	for i := range mod.Imports {
		cpy.Imports[i] = mod.Imports[i].Copy()
	}
	cpy.Package = mod.Package.Copy()
	return &cpy
}

// Equal returns true if mod equals other.
func (mod *Module) Equal(other *Module) bool {
	return mod.Compare(other) == 0
}

func (mod *Module) String() string {
	buf := []string{}
	buf = append(buf, mod.Package.String())
	if len(mod.Imports) > 0 {
		buf = append(buf, "")
		for _, imp := range mod.Imports {
			buf = append(buf, imp.String())
		}
	}
	if len(mod.Rules) > 0 {
		buf = append(buf, "")
		for _, rule := range mod.Rules {
			buf = append(buf, rule.String())
		}
	}
	return strings.Join(buf, "\n")
}

// NewComment returns a new Comment object.
func NewComment(text []byte) *Comment {
	return &Comment{
		Text: text,
	}
}

// Loc returns the location of the comment in the definition.
func (c *Comment) Loc() *Location {
	return c.Location
}

func (c *Comment) String() string {
	return "#" + string(c.Text)
}

// Compare returns an integer indicating whether pkg is less than, equal to,
// or greater than other.
func (pkg *Package) Compare(other *Package) int {
	return Compare(pkg.Path, other.Path)
}

// Copy returns a deep copy of pkg.
func (pkg *Package) Copy() *Package {
	cpy := *pkg
	cpy.Path = pkg.Path.Copy()
	return &cpy
}

// Equal returns true if pkg is equal to other.
func (pkg *Package) Equal(other *Package) bool {
	return pkg.Compare(other) == 0
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

// IsValidImportPath returns an error indicating if the import path is invalid.
// If the import path is invalid, err is nil.
func IsValidImportPath(v Value) (err error) {
	switch v := v.(type) {
	case Var:
		if !v.Equal(DefaultRootDocument.Value) && !v.Equal(InputRootDocument.Value) {
			return fmt.Errorf("invalid path %v: path must begin with input or data", v)
		}
	case Ref:
		if err := IsValidImportPath(v[0].Value); err != nil {
			return fmt.Errorf("invalid path %v: path must begin with input or data", v)
		}
		for _, e := range v[1:] {
			if _, ok := e.Value.(String); !ok {
				return fmt.Errorf("invalid path %v: path elements must be %vs", v, StringTypeName)
			}
		}
	default:
		return fmt.Errorf("invalid path %v: path must be %v or %v", v, RefTypeName, VarTypeName)
	}
	return nil
}

// Compare returns an integer indicating whether imp is less than, equal to,
// or greater than other.
func (imp *Import) Compare(other *Import) int {
	if cmp := Compare(imp.Path, other.Path); cmp != 0 {
		return cmp
	}
	return Compare(imp.Alias, other.Alias)
}

// Copy returns a deep copy of imp.
func (imp *Import) Copy() *Import {
	cpy := *imp
	cpy.Path = imp.Path.Copy()
	return &cpy
}

// Equal returns true if imp is equal to other.
func (imp *Import) Equal(other *Import) bool {
	return imp.Compare(other) == 0
}

// Loc returns the location of the Import in the definition.
func (imp *Import) Loc() *Location {
	return imp.Location
}

// Name returns the variable that is used to refer to the imported virtual
// document. This is the alias if defined otherwise the last element in the
// path.
func (imp *Import) Name() Var {
	if len(imp.Alias) != 0 {
		return imp.Alias
	}
	switch v := imp.Path.Value.(type) {
	case Var:
		return v
	case Ref:
		if len(v) == 1 {
			return v[0].Value.(Var)
		}
		return Var(v[len(v)-1].Value.(String))
	}
	panic("illegal import")
}

func (imp *Import) String() string {
	buf := []string{"import", imp.Path.String()}
	if len(imp.Alias) > 0 {
		buf = append(buf, "as "+imp.Alias.String())
	}
	return strings.Join(buf, " ")
}

// Compare returns an integer indicating whether rule is less than, equal to,
// or greater than other.
func (rule *Rule) Compare(other *Rule) int {
	if cmp := rule.Head.Compare(other.Head); cmp != 0 {
		return cmp
	}
	if cmp := util.Compare(rule.Default, other.Default); cmp != 0 {
		return cmp
	}
	return rule.Body.Compare(other.Body)
}

// Copy returns a deep copy of rule.
func (rule *Rule) Copy() *Rule {
	cpy := *rule
	cpy.Head = rule.Head.Copy()
	cpy.Body = rule.Body.Copy()
	return &cpy
}

// Equal returns true if rule is equal to other.
func (rule *Rule) Equal(other *Rule) bool {
	return rule.Compare(other) == 0
}

// Loc returns the location of the Rule in the definition.
func (rule *Rule) Loc() *Location {
	return rule.Head.Location
}

// Path returns a reference that identifies the rule under ns.
func (rule *Rule) Path(ns Ref) Ref {
	return ns.Append(StringTerm(string(rule.Head.Name)))
}

func (rule *Rule) String() string {
	buf := []string{}
	if rule.Default {
		buf = append(buf, "default")
	}
	buf = append(buf, rule.Head.String())
	if !rule.Default {
		buf = append(buf, "{")
		buf = append(buf, rule.Body.String())
		buf = append(buf, "}")
	}
	return strings.Join(buf, " ")
}

// NewHead returns a new Head object. If args are provided, the first will be
// used for the key and the second will be used for the value.
func NewHead(name Var, args ...*Term) *Head {
	head := &Head{
		Name: name,
	}
	if len(args) == 0 {
		return head
	}
	head.Key = args[0]
	if len(args) == 1 {
		return head
	}
	head.Value = args[1]
	return head
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
func (head *Head) DocKind() DocKind {
	if head.Key != nil {
		if head.Value != nil {
			return PartialObjectDoc
		}
		return PartialSetDoc
	}
	return CompleteDoc
}

// Compare returns an integer indicating whether head is less than, equal to,
// or greater than other.
func (head *Head) Compare(other *Head) int {
	if cmp := Compare(head.Name, other.Name); cmp != 0 {
		return cmp
	}
	if cmp := Compare(head.Key, other.Key); cmp != 0 {
		return cmp
	}
	return Compare(head.Value, other.Value)
}

// Copy returns a deep copy of head.
func (head *Head) Copy() *Head {
	cpy := *head
	cpy.Key = head.Key.Copy()
	cpy.Value = head.Value.Copy()
	return &cpy
}

// Equal returns true if this head equals other.
func (head *Head) Equal(other *Head) bool {
	return head.Compare(other) == 0
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

// Vars returns a set of vars found in the head.
func (head *Head) Vars() VarSet {
	vis := &VarVisitor{vars: VarSet{}}
	if head.Key != nil {
		Walk(vis, head.Key)
	}
	if head.Value != nil {
		Walk(vis, head.Value)
	}
	return vis.vars
}

// NewBody returns a new Body containing the given expressions. The indices of
// the immediate expressions will be reset.
func NewBody(exprs ...*Expr) Body {
	for i, expr := range exprs {
		expr.Index = i
	}
	return Body(exprs)
}

// Append adds the expr to the body and updates the expr's index accordingly.
func (body *Body) Append(expr *Expr) {
	n := len(*body)
	expr.Index = n
	*body = append(*body, expr)
}

// Compare returns an integer indicating whether body is less than, equal to,
// or greater than other.
//
// If body is a subset of other, it is considered less than (and vice versa).
func (body Body) Compare(other Body) int {
	minLen := len(body)
	if len(other) < minLen {
		minLen = len(other)
	}
	for i := 0; i < minLen; i++ {
		if cmp := body[i].Compare(other[i]); cmp != 0 {
			return cmp
		}
	}
	if len(body) < len(other) {
		return -1
	}
	if len(other) < len(body) {
		return 1
	}
	return 0
}

// Copy returns a deep copy of body.
func (body Body) Copy() Body {
	cpy := make(Body, len(body))
	for i := range body {
		cpy[i] = body[i].Copy()
	}
	return cpy
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
func (body Body) Equal(other Body) bool {
	return body.Compare(other) == 0
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
	return strings.Join(buf, "; ")
}

// Vars returns a VarSet containing variables in body. The params can be set to
// control which vars are included.
func (body Body) Vars(params VarVisitorParams) VarSet {
	vis := NewVarVisitor().WithParams(params)
	Walk(vis, body)
	return vis.Vars()
}

// NewExpr returns a new Expr object.
func NewExpr(terms interface{}) *Expr {
	return &Expr{
		Negated: false,
		Terms:   terms,
		Index:   0,
		With:    nil,
	}
}

// Complement returns a copy of this expression with the negation flag flipped.
func (expr *Expr) Complement() *Expr {
	cpy := *expr
	cpy.Negated = !cpy.Negated
	return &cpy
}

// Equal returns true if this Expr equals the other Expr.
func (expr *Expr) Equal(other *Expr) bool {
	return expr.Compare(other) == 0
}

// Compare returns an integer indicating whether expr is less than, equal to,
// or greater than other.
//
// Expressions are compared as follows:
//
// 1. Preceding expression (by Index) is always less than the other expression.
// 2. Non-negated expressions are always less than than negated expressions.
// 3. Single term expressions are always less than built-in expressions.
//
// Otherwise, the expression terms are compared normally. If both expressions
// have the same terms, the modifiers are compared.
func (expr *Expr) Compare(other *Expr) int {

	switch {
	case expr.Index < other.Index:
		return -1
	case expr.Index > other.Index:
		return 1
	}

	switch {
	case expr.Negated && !other.Negated:
		return 1
	case !expr.Negated && other.Negated:
		return -1
	}

	switch t := expr.Terms.(type) {
	case *Term:
		u, ok := other.Terms.(*Term)
		if !ok {
			return -1
		}
		if cmp := Compare(t.Value, u.Value); cmp != 0 {
			return cmp
		}
	case []*Term:
		u, ok := other.Terms.([]*Term)
		if !ok {
			return 1
		}
		if cmp := termSliceCompare(t, u); cmp != 0 {
			return cmp
		}
	}

	return withSliceCompare(expr.With, other.With)
}

// Copy returns a deep copy of expr.
func (expr *Expr) Copy() *Expr {

	cpy := *expr

	switch ts := expr.Terms.(type) {
	case []*Term:
		cpyTs := make([]*Term, len(ts))
		for i := range ts {
			cpyTs[i] = ts[i].Copy()
		}
		cpy.Terms = cpyTs
	case *Term:
		cpy.Terms = ts.Copy()
	}

	cpy.With = make([]*With, len(expr.With))
	for i := range expr.With {
		cpy.With[i] = expr.With[i].Copy()
	}

	return &cpy
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
	for _, w := range expr.With {
		s += w.Hash()
	}
	return s
}

// IncludeWith returns a copy of expr with the with modifier appended.
func (expr *Expr) IncludeWith(target *Term, value *Term) *Expr {
	cpy := *expr
	cpy.With = append(cpy.With, &With{Target: target, Value: value})
	return &cpy
}

// NoWith returns a copy of expr where the with modifier has been removed.
func (expr *Expr) NoWith() *Expr {
	expr.With = nil
	return expr
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

		// Currently the with modifier does not produce any outputs. Any
		// variables in the value must be safe before the expression can be
		// evaluated.
		if !expr.withModifierSafe(safe) {
			return VarSet{}
		}

		switch terms := expr.Terms.(type) {
		case *Term:
			return expr.outputVarsRefs(safe)
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
		name := t[0].Value.(Var)
		bi := BuiltinMap[name]
		var s string
		if bi != nil && len(bi.Infix) > 0 {
			switch len(bi.Args) {
			case 2:
				s = fmt.Sprintf("%v %v %v", t[1], bi.Infix, t[2])
			case 3:
				// Special case for "x = y <operand> z" built-ins.
				if len(bi.TargetPos) == 1 && bi.TargetPos[0] == 2 {
					s = fmt.Sprintf("%v = %v %v %v", t[3], t[1], bi.Infix, t[2])
				}
			}
		}
		if len(s) == 0 {
			var args []string
			for _, v := range t[1:] {
				args = append(args, v.String())
			}
			name := t[0].Value.(Var).String()
			s = fmt.Sprintf("%s(%s)", name, strings.Join(args, ", "))
		}

		buf = append(buf, s)

	case *Term:
		buf = append(buf, t.String())
	}

	for i := range expr.With {
		buf = append(buf, expr.With[i].String())
	}

	return strings.Join(buf, " ")
}

// UnmarshalJSON parses the byte array and stores the result in expr.
func (expr *Expr) UnmarshalJSON(bs []byte) error {
	v := map[string]interface{}{}
	if err := util.UnmarshalJSON(bs, &v); err != nil {
		return err
	}
	return unmarshalExpr(expr, v)
}

// Vars returns a VarSet containing variables in expr. The params can be set to
// control which vars are included.
func (expr *Expr) Vars(params VarVisitorParams) VarSet {
	vis := NewVarVisitor().WithParams(params)
	Walk(vis, expr)
	return vis.Vars()
}

func (expr *Expr) outputVarsBuiltins(b *Builtin, safe VarSet) VarSet {

	o := expr.outputVarsRefs(safe)
	terms := expr.Terms.([]*Term)

	// Check that all input terms are ground or safe.
	for i, t := range terms[1:] {
		if b.IsTargetPos(i) {
			continue
		}
		if t.Value.IsGround() {
			continue
		}
		vis := NewVarVisitor().WithParams(VarVisitorParams{
			SkipClosures:         true,
			SkipObjectKeys:       true,
			SkipRefHead:          true,
			SkipBuiltinOperators: true,
		})
		Walk(vis, t)
		unsafe := vis.Vars().Diff(o).Diff(safe)
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
	o := expr.outputVarsRefs(safe)
	o.Update(safe)
	o.Update(Unify(o, ts[1], ts[2]))
	return o.Diff(safe)
}

func (expr *Expr) outputVarsRefs(safe VarSet) VarSet {
	o := VarSet{}
	WalkRefs(expr, func(r Ref) bool {
		if safe.Contains(r[0].Value.(Var)) {
			o.Update(r.OutputVars())
			return false
		}
		return true
	})
	return o
}

func (expr *Expr) withModifierSafe(safe VarSet) bool {
	for _, with := range expr.With {
		unsafe := false
		WalkVars(with, func(v Var) bool {
			if !safe.Contains(v) {
				unsafe = true
				return true
			}
			return false
		})
		if unsafe {
			return false
		}
	}
	return true
}

// NewBuiltinExpr creates a new Expr object with the supplied terms.
// The builtin operator must be the first term.
func NewBuiltinExpr(terms ...*Term) *Expr {
	return &Expr{Terms: terms}
}

func (w *With) String() string {
	return "with " + w.Target.String() + " as " + w.Value.String()
}

// Equal returns true if this With is equals the other With.
func (w *With) Equal(other *With) bool {
	return Compare(w, other) == 0
}

// Compare returns an integer indicating whether w is less than, equal to, or
// greater than other.
func (w *With) Compare(other *With) int {
	if cmp := Compare(w.Target, other.Target); cmp != 0 {
		return cmp
	}
	return Compare(w.Value, other.Value)
}

// Copy returns a deep copy of w.
func (w *With) Copy() *With {
	cpy := *w
	cpy.Value = w.Value.Copy()
	cpy.Target = w.Target.Copy()
	return &cpy
}

// Hash returns the hash code of the With.
func (w With) Hash() int {
	return w.Target.Hash() + w.Value.Hash()
}

type ruleSlice []*Rule

func (s ruleSlice) Less(i, j int) bool { return Compare(s[i], s[j]) < 0 }
func (s ruleSlice) Swap(i, j int)      { x := s[i]; s[i] = s[j]; s[j] = x }
func (s ruleSlice) Len() int           { return len(s) }
