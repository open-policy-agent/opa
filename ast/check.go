// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util"
)

// exprChecker defines the interface for executing type checking on a single
// expression. The exprChecker must update the provided TypeEnv with inferred
// types of vars.
type exprChecker func(*TypeEnv, *Expr) *Error

// typeChecker implements type checking on queries and rules. Errors are
// accumulated on the typeChecker so that a single run can report multiple
// issues.
type typeChecker struct {
	errs         Errors
	exprCheckers map[String]exprChecker

	// When checking the types of functions, their inputs need to initially
	// be assumed as types.Any. In order to fill the TypeEnv with more accurate
	// type information for the inputs, we need to overwrite this types.Any
	// after we've infered more accurate typing from the function bodies.
	inFunc bool
}

// newTypeChecker returns a new typeChecker object that has no errors.
func newTypeChecker() *typeChecker {
	tc := &typeChecker{}
	tc.exprCheckers = map[String]exprChecker{
		Equality.Name: tc.checkExprEq,
	}
	return tc
}

// CheckBody runs type checking on the body and returns a TypeEnv if no errors
// are found. The resulting TypeEnv wraps the provided one. The resulting
// TypeEnv will be able to resolve types of vars contained in the body.
func (tc *typeChecker) CheckBody(env *TypeEnv, body Body) (*TypeEnv, Errors) {

	if env == nil {
		env = NewTypeEnv()
	} else {
		env = env.wrap()
	}

	WalkExprs(body, func(expr *Expr) bool {
		vis := newRefChecker(env)
		Walk(vis, expr)
		for _, err := range vis.errs {
			tc.err(err)
		}

		if err := tc.checkExpr(env, expr); err != nil {
			tc.err(err)
		}
		return false
	})

	return env, tc.errs
}

func (tc *typeChecker) checkLanguageBuiltins(env *TypeEnv) *TypeEnv {
	if env == nil {
		env = NewTypeEnv()
	} else {
		env = env.wrap()
	}

	for _, bi := range Builtins {
		env.PutFunc(bi.Name, bi.Args)
	}

	return env
}

// CheckTypes runs type checking on the rules and funcs and returns a TypeEnv if no
// errors are found. The resulting TypeEnv wraps the provided one. The
// resulting TypeEnv will be able to resolve types of refs that refer to rules
// and funcs.
func (tc *typeChecker) CheckTypes(env *TypeEnv, sorted []util.T) (*TypeEnv, Errors) {
	if env == nil {
		env = NewTypeEnv()
	} else {
		env.wrap()
	}

	for _, s := range sorted {
		switch s := s.(type) {
		case *Rule:
			tc.checkRule(env, s)
		case *Func:
			// TODO(mmussomele, tsandall): Currently this infers
			// function input/output types from the body. We'll want
			// to spend some time thinking about whether or not we
			// want to keep that.
			tc.checkFunc(env, s)
		}
	}

	return env, tc.errs
}

func (tc *typeChecker) checkFunc(env *TypeEnv, fn *Func) {
	tc.inFunc = true
	defer func() {
		tc.inFunc = false
	}()

	cpy := env.wrap()
	for _, arg := range fn.Head.Args {
		WalkVars(arg, func(v Var) bool {
			cpy.tree.PutOne(v, types.A)
			return false
		})
	}

	prev := len(tc.errs)
	cpy, err := tc.CheckBody(cpy, fn.Body)

	// If this function did not error, there is no reason to return early,
	// as that means that its dependencies compiled fine.
	if len(err) > prev {
		return
	}
	name := fn.PathString()

	// Ensure that multiple definitions of this function have consistent argument
	// lengths.
	cur := env.GetFunc(name)
	numArgs := len(fn.Head.Args)
	if cur != nil && len(cur)-1 != numArgs {
		tc.err(NewError(TypeErr, fn.Head.Loc(), "function definitions for %s have different number of arguments (%d vs %d)", name, numArgs, len(cur)-1))
		return
	}

	var argTypes []types.Type
	for i, arg := range fn.Head.Args {
		tpe := mergeTypes(cpy.Get(arg), cur, i)
		argTypes = append(argTypes, tpe)
	}

	out := mergeTypes(cpy.Get(fn.Head.Output), cur, numArgs)
	argTypes = append(argTypes, out)
	env.PutFunc(name, argTypes)
}

func (tc *typeChecker) checkRule(env *TypeEnv, rule *Rule) {
	cpy, err := tc.CheckBody(env, rule.Body)

	if len(err) == 0 {
		path := rule.Path()
		var tpe types.Type

		switch rule.Head.DocKind() {
		case CompleteDoc:
			typeV := cpy.Get(rule.Head.Value)
			if typeV != nil {
				exist := env.tree.Get(path)
				tpe = types.Or(typeV, exist)
			}
		case PartialObjectDoc:
			// TODO(tsandall): partial object keys require 'optional' support
			// in types.Object. For now, treat partial objects as having
			// dynamic keys where the value type is constrained.
			typeV := cpy.Get(rule.Head.Value)
			if typeV != nil {
				exist := env.tree.Get(path)
				typeV = types.Or(types.Values(exist), typeV)
				tpe = types.NewObject(nil, typeV)
			}
		case PartialSetDoc:
			typeK := cpy.Get(rule.Head.Key)
			if typeK != nil {
				exist := env.tree.Get(path)
				typeK = types.Or(types.Keys(exist), typeK)
				tpe = types.NewSet(typeK)
			}
		}
		if tpe != nil {
			env.tree.Put(path, tpe)
		}
	}
}

func (tc *typeChecker) checkExpr(env *TypeEnv, expr *Expr) *Error {
	if !expr.IsBuiltin() {
		return nil
	}

	checker := tc.exprCheckers[expr.Name()]
	if checker != nil {
		return checker(env, expr)
	}

	return tc.checkExprBuiltin(env, expr)
}

func (tc *typeChecker) checkExprBuiltin(env *TypeEnv, expr *Expr) *Error {
	name := expr.Name()
	expArgs := env.GetFunc(name)
	if expArgs == nil {
		return NewError(TypeErr, expr.Location, "undefined built-in function %v", name)
	}

	args := expr.Operands()
	pre := make([]types.Type, len(args))
	for i := range args {
		pre[i] = env.Get(args[i])
	}

	if len(args) < len(expArgs) {
		return newArgError(expr.Location, name, "too few arguments", pre, expArgs)
	} else if len(args) > len(expArgs) {
		return newArgError(expr.Location, name, "too many arguments", pre, expArgs)
	}

	for i := range args {
		if !tc.unify1(env, args[i], expArgs[i]) {
			post := make([]types.Type, len(args))
			for i := range args {
				post[i] = env.Get(args[i])
			}
			return newArgError(expr.Location, name, "invalid argument(s)", post, expArgs)
		}
	}

	return nil
}

func (tc *typeChecker) checkExprEq(env *TypeEnv, expr *Expr) *Error {

	a, b := expr.Operand(0), expr.Operand(1)
	typeA, typeB := env.Get(a), env.Get(b)

	if !tc.unify2(env, a, typeA, b, typeB) {
		err := NewError(TypeErr, expr.Location, "match error")
		err.Details = &UnificationErrDetail{
			Left:  typeA,
			Right: typeB,
		}
		return err
	}

	return nil
}

func (tc *typeChecker) unify2(env *TypeEnv, a *Term, typeA types.Type, b *Term, typeB types.Type) bool {

	nilA := types.Nil(typeA)
	nilB := types.Nil(typeB)

	if nilA && !nilB {
		return tc.unify1(env, a, typeB)
	} else if nilB && !nilA {
		return tc.unify1(env, b, typeA)
	} else if !nilA && !nilB {
		return unifies(typeA, typeB)
	}

	switch a := a.Value.(type) {
	case Array:
		switch b := b.Value.(type) {
		case Array:
			if len(a) == len(b) {
				for i := range a {
					if !tc.unify2(env, a[i], env.Get(a[i]), b[i], env.Get(b[i])) {
						return false
					}
				}
				return true
			}
		}
	case Object:
		switch b := b.Value.(type) {
		case Object:
			c := a.Intersect(b)
			if len(a) == len(b) && len(b) == len(c) {
				for i := range c {
					if !tc.unify2(env, c[i][1], env.Get(c[i][1]), c[i][2], env.Get(c[i][2])) {
						return false
					}
				}
				return true
			}
		}
	}

	return false
}

func (tc *typeChecker) unify1(env *TypeEnv, term *Term, tpe types.Type) bool {
	switch v := term.Value.(type) {
	case Array:
		switch tpe := tpe.(type) {
		case *types.Array:
			return tc.unify1Array(env, v, tpe)
		case types.Any:
			if types.Compare(tpe, types.A) == 0 {
				for i := range v {
					tc.unify1(env, v[i], types.A)
				}
				return true
			}
			unifies := false
			for i := range tpe {
				unifies = tc.unify1(env, term, tpe[i]) || unifies
			}
			return unifies
		}
		return false
	case Object:
		switch tpe := tpe.(type) {
		case *types.Object:
			return tc.unify1Object(env, v, tpe)
		case types.Any:
			if types.Compare(tpe, types.A) == 0 {
				for i := range v {
					tc.unify1(env, v[i][1], types.A)
				}
				return true
			}
			unifies := false
			for i := range tpe {
				unifies = tc.unify1(env, term, tpe[i]) || unifies
			}
			return unifies
		}
		return false
	case *Set:
		switch tpe := tpe.(type) {
		case *types.Set:
			return tc.unify1Set(env, v, tpe)
		case types.Any:
			if types.Compare(tpe, types.A) == 0 {
				v.Iter(func(elem *Term) bool {
					tc.unify1(env, elem, types.A)
					return true
				})
				return true
			}
			unifies := false
			for i := range tpe {
				unifies = tc.unify1(env, term, tpe[i]) || unifies
			}
			return unifies
		}
		return false
	case *ArrayComprehension:
		return unifies(env.Get(v), tpe)
	case Ref:
		return unifies(env.Get(v), tpe)
	case Var:
		if exist := env.Get(v); exist != nil {
			if e, ok := exist.(types.Any); !ok || len(e) != 0 || !tc.inFunc {
				return unifies(exist, tpe)
			}
		}
		env.tree.PutOne(term.Value, tpe)
		return true
	default:
		if !IsConstant(v) {
			panic("unreachable")
		}
		return unifies(env.Get(term), tpe)
	}
}

func (tc *typeChecker) unify1Array(env *TypeEnv, val Array, tpe *types.Array) bool {
	if len(val) != tpe.Len() && tpe.Dynamic() == nil {
		return false
	}
	for i := range val {
		if !tc.unify1(env, val[i], tpe.Select(i)) {
			return false
		}
	}
	return true
}

func (tc *typeChecker) unify1Object(env *TypeEnv, val Object, tpe *types.Object) bool {
	if len(val) != len(tpe.Keys()) && tpe.Dynamic() == nil {
		return false
	}
	for i := range val {
		if child := selectConstant(tpe, val[i][0]); child != nil {
			if !tc.unify1(env, val[i][1], child) {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

func (tc *typeChecker) unify1Set(env *TypeEnv, val *Set, tpe *types.Set) bool {
	of := types.Values(tpe)
	return !val.Iter(func(elem *Term) bool {
		return !tc.unify1(env, elem, of)
	})
}

func (tc *typeChecker) err(err *Error) {
	tc.errs = append(tc.errs, err)
}

type refChecker struct {
	env  *TypeEnv
	errs Errors
}

func newRefChecker(env *TypeEnv) *refChecker {
	return &refChecker{
		env:  env,
		errs: nil,
	}
}

func (rc *refChecker) Visit(x interface{}) Visitor {
	switch x := x.(type) {
	case *ArrayComprehension:
		return nil
	case Ref:
		if err := rc.checkRef(rc.env, rc.env.tree, x, 0); err != nil {
			rc.errs = append(rc.errs, err)
		}
	}
	return rc
}

func (rc *refChecker) checkRef(curr *TypeEnv, node *typeTreeNode, ref Ref, idx int) *Error {

	if idx == len(ref) {
		return nil
	}

	head := ref[idx]

	// Handle constant ref operands, i.e., strings or the ref head.
	if _, ok := head.Value.(String); ok || idx == 0 {

		child := node.Child(head.Value)
		if child == nil {
			return rc.checkRefNext(curr, ref)
		}

		if child.Leaf() {
			return rc.checkRefLeaf(child.Value(), ref, idx+1)
		}

		return rc.checkRef(curr, child, ref, idx+1)
	}

	// Handle dynamic ref operands.
	switch value := head.Value.(type) {

	case Var:

		if exist := rc.env.Get(value); exist != nil {
			if !unifies(types.S, exist) {
				return newRefError(ref[0].Location, ref, idx, nil, exist)
			}
		} else {
			rc.env.tree.PutOne(value, types.S)
		}

	case Ref:

		exist := rc.env.Get(value)
		if exist == nil {
			// If ref type is unknown, an error will already be reported so
			// stop here.
			return nil
		}

		if !unifies(types.S, exist) {
			return newRefError(ref[0].Location, ref, idx, nil, exist)
		}

	// Catch other ref operand types here. Non-leaf nodes must be referred to
	// with string values.
	default:
		return newRefError(ref[0].Location, ref, idx, nil, rc.env.Get(value))
	}

	// Run checking on remaining portion of the ref. Note, since the ref
	// potentially refers to data for which no type information exists,
	// checking should never fail.
	for _, child := range node.Children() {
		rc.checkRef(curr, child, ref, idx+1)
	}

	return nil
}

func (rc *refChecker) checkRefLeaf(tpe types.Type, ref Ref, idx int) *Error {

	if idx == len(ref) {
		return nil
	}

	head := ref[idx]

	keys := types.Keys(tpe)
	if keys == nil {
		return newRefError(ref[0].Location, ref, idx, tpe, nil)
	}

	switch value := head.Value.(type) {

	case Var:
		if exist := rc.env.Get(value); exist != nil {
			if !unifies(exist, keys) {
				return newRefError(ref[0].Location, ref, idx, tpe, exist)
			}
		} else {
			rc.env.tree.PutOne(value, types.Keys(tpe))
		}

	case Ref:
		if exist := rc.env.Get(value); exist != nil {
			if !unifies(exist, keys) {
				return newRefError(ref[0].Location, ref, idx, tpe, exist)
			}
		}

	default:
		child := selectConstant(tpe, head)
		if child == nil {
			return newRefError(ref[0].Location, ref, idx, tpe, nil)
		}
		return rc.checkRefLeaf(child, ref, idx+1)
	}

	return rc.checkRefLeaf(types.Values(tpe), ref, idx+1)
}

func (rc *refChecker) checkRefNext(curr *TypeEnv, ref Ref) *Error {

	if curr.next != nil {
		next := curr.next
		return rc.checkRef(next, next.tree, ref, 0)
	}

	if RootDocumentNames.Contains(ref[0]) {
		return rc.checkRefLeaf(types.A, ref, 0)
	}

	return NewError(TypeErr, ref[0].Location, "ref is unsafe: %v", ref)
}

func unifies(a, b types.Type) bool {

	if a == nil || b == nil {
		return false
	}

	if anyA, ok := a.(types.Any); ok {
		return unifiesAny(anyA, b)
	}

	if anyB, ok := b.(types.Any); ok {
		return unifiesAny(anyB, a)
	}

	switch a := a.(type) {
	case types.Null:
		_, ok := b.(types.Null)
		return ok
	case types.Boolean:
		_, ok := b.(types.Boolean)
		return ok
	case types.Number:
		_, ok := b.(types.Number)
		return ok
	case types.String:
		_, ok := b.(types.String)
		return ok
	case *types.Array:
		b, ok := b.(*types.Array)
		if !ok {
			return false
		}
		return unifiesArrays(a, b)
	case *types.Object:
		b, ok := b.(*types.Object)
		if !ok {
			return false
		}
		return unifiesObjects(a, b)
	case *types.Set:
		b, ok := b.(*types.Set)
		if !ok {
			return false
		}
		return unifies(types.Values(a), types.Values(b))
	default:
		panic("unreachable")
	}
}

func unifiesAny(a types.Any, b types.Type) bool {
	if types.Compare(a, types.A) == 0 {
		return true
	}
	for i := range a {
		if unifies(a[i], b) {
			return true
		}
	}
	return false
}

func unifiesArrays(a, b *types.Array) bool {

	if !unifiesArraysStatic(a, b) {
		return false
	}

	if !unifiesArraysStatic(b, a) {
		return false
	}

	return a.Dynamic() == nil || b.Dynamic() == nil || unifies(a.Dynamic(), b.Dynamic())
}

func unifiesArraysStatic(a, b *types.Array) bool {
	if a.Len() != 0 {
		for i := 0; i < a.Len(); i++ {
			if !unifies(a.Select(i), b.Select(i)) {
				return false
			}
		}
	}
	return true
}

func unifiesObjects(a, b *types.Object) bool {
	if !unifiesObjectsStatic(a, b) {
		return false
	}

	if !unifiesObjectsStatic(b, a) {
		return false
	}

	return a.Dynamic() == nil || b.Dynamic() == nil || unifies(a.Dynamic(), b.Dynamic())
}

func unifiesObjectsStatic(a, b *types.Object) bool {
	for _, k := range a.Keys() {
		if !unifies(a.Select(k), b.Select(k)) {
			return false
		}
	}
	return true
}

func mergeTypes(found types.Type, cur []types.Type, i int) types.Type {
	if found == nil {
		found = types.A
	}

	if cur == nil {
		return found
	}

	return types.Or(found, cur[i])
}

func builtinNameRef(name String) Ref {
	n, err := ParseRef(string(name))
	if err != nil {
		n = Ref([]*Term{{Value: name}})
	}
	return n
}

// ArgErrDetail represents a generic argument error.
type ArgErrDetail struct {
	Have []types.Type `json:"have"`
	Want []types.Type `json:"want"`
}

// Lines returns the string representation of the detail.
func (d *ArgErrDetail) Lines() []string {
	lines := make([]string, 2)
	lines[0] = fmt.Sprint("have : ", formatArgs(d.Have))
	lines[1] = fmt.Sprint("want : ", formatArgs(d.Want))
	return lines
}

// UnificationErrDetail represents a type mismatch error when two **values**
// are unified (as in x = y).
type UnificationErrDetail struct {
	Left  types.Type `json:"a"`
	Right types.Type `json:"b"`
}

// Lines returns the string representation of the detail.
func (a *UnificationErrDetail) Lines() []string {
	lines := make([]string, 2)
	lines[0] = fmt.Sprint("left  : ", types.Sprint(a.Left))
	lines[1] = fmt.Sprint("right : ", types.Sprint(a.Right))
	return lines
}

// RefErrDetail represents an undefined ref.
type RefErrDetail struct {
	Ref      Ref        `json:"ref"`
	Index    int        `json:"index"`
	Refers   types.Type `json:"refers"`
	Referrer types.Type `json:"referrer"`
}

// Lines returns the string representation of the detail.
func (r *RefErrDetail) Lines() []string {
	lines := []string{}
	lines = append(lines, fmt.Sprintf("prefix    : %v (valid)", r.Ref[:r.Index]))
	if r.Refers != nil {
		lines = append(lines, fmt.Sprintf("refers to : %v", r.Refers))
	}
	tail := r.Ref[r.Index:]
	var str string
	if len(tail) == 1 {
		switch v := tail[0].Value.(type) {
		case String:
			str = formatString(v)
		default:
			str = v.String()
		}
	} else {
		str = tail.String()
	}
	lines = append(lines, fmt.Sprintf("suffix    : %v (invalid)", str))
	if r.Referrer != nil {
		lines = append(lines, fmt.Sprintf("refers to : %v", r.Referrer))
	}
	return lines
}

func formatArgs(args []types.Type) string {
	buf := make([]string, len(args))
	for i := range args {
		buf[i] = types.Sprint(args[i])
	}
	return "(" + strings.Join(buf, ", ") + ")"
}

func newRefError(loc *Location, ref Ref, idx int, refers, referrer types.Type) *Error {
	err := NewError(TypeErr, loc, "undefined ref: %v", ref)
	err.Details = &RefErrDetail{
		Ref:      ref,
		Index:    idx,
		Refers:   refers,
		Referrer: referrer,
	}
	return err
}

func newArgError(loc *Location, builtinName String, msg string, have []types.Type, want []types.Type) *Error {
	err := NewError(TypeErr, loc, "%v: %v", builtinName, msg)
	err.Details = &ArgErrDetail{
		Have: have,
		Want: want,
	}
	return err
}
