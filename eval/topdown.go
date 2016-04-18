// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package eval

import "github.com/open-policy-agent/opa/opalog"
import "fmt"

var _ = fmt.Println

// Bindings is the structure used to ground expressions.
type Bindings map[opalog.Var]opalog.Value

// TopDownContext contains the state of the evaluation process.
//
// TODO(tsandall): profile perf/memory usage with current approach;
// the current approach copies the TopDownContext structure for each
// step and binding. This avoids the need to undo steps and bindings
// each time the proof fails but this may be too expensive.
type TopDownContext struct {
	Rule     *opalog.Rule
	Bindings Bindings
	Index    int
	Previous *TopDownContext
	Store    Storage
}

// Current returns the current expression to evaluate.
func (ctx *TopDownContext) Current() *opalog.Expr {
	return ctx.Rule.Body[ctx.Index]
}

// Step returns a new context to evaluate the next expression.
func (ctx *TopDownContext) Step() *TopDownContext {
	return &TopDownContext{
		Rule:     ctx.Rule,
		Bindings: ctx.Bindings,
		Index:    ctx.Index + 1,
		Previous: ctx.Previous,
		Store:    ctx.Store,
	}
}

// Bind returns a new TopDownContext with Bindings that map the variable to the value.
//
// If the caller attempts to bind a variable to itself (e.g., "x = x"), no mapping is added. This
// is effectively a no-op.
//
// If the call would introduce a recursive binding, nil is returned. This represents an undefined
// context that cannot be evaluated. E.g., "x = [x]".
//
// Binding a variable to some value also updates existing bindings to that variable.
func (ctx *TopDownContext) Bind(variable opalog.Var, value opalog.Value) *TopDownContext {
	if variable.Equal(value) {
		return ctx
	}
	occurs := walkValue(value, func(other opalog.Value) bool {
		if variable.Equal(other) {
			return true
		}
		return false
	})
	if occurs {
		return nil
	}
	cpy := *ctx
	cpy.Bindings = make(Bindings)
	for k, v := range ctx.Bindings {
		cpy.Bindings[k] = plugValue(v, Bindings{variable: value})
	}
	cpy.Bindings[variable] = value
	return &cpy
}

// TopDownIterator is the interface for processing contexts.
type TopDownIterator func(*TopDownContext) error

// TopDown runs the evaluation algorithm on the contxet and calls the iterator
// foreach context that contains bindings that satisfy all of the expressions
// inside the rule.
func TopDown(ctx *TopDownContext, iter TopDownIterator) error {
	return evalRule(ctx, iter)
}

// Undefined represents the absence of bindings that satisfy a completely defined rule.
// See the documentation for TopDownQuery for more details.
type Undefined struct{}

func (undefined Undefined) String() string {
	return "<undefined>"
}

// TopDownQuery returns the result of executing the evaluation algorithm and collecting
// all of the binding values that satisfy all of the expressions in the rule. In the case
// of rules that are completely definined (i.e., rules that are not defined incrementally),
// the Undefined value is returned to indicate that there are no bindings that satisfy all
// of the expressions in the rule.
func TopDownQuery(ctx *TopDownContext) (interface{}, error) {
	switch ctx.Rule.DocKind() {
	case opalog.SetDoc:
		result := []interface{}{}
		key := ctx.Rule.Key.Value.(opalog.Var)
		err := TopDown(ctx, func(ctx *TopDownContext) error {
			value, err := dereferenceVar(key, ctx)
			if err != nil {
				return err
			}
			result = append(result, value)
			return nil
		})
		return result, err

	case opalog.ObjectDoc:
		result := map[string]interface{}{}
		key := ctx.Rule.Key.Value.(opalog.Var)
		value := ctx.Rule.Value.Value.(opalog.Var)
		err := TopDown(ctx, func(ctx *TopDownContext) error {
			key, err := dereferenceVar(key, ctx)
			if err != nil {
				return err
			}
			asStr, ok := key.(string)
			if !ok {
				return fmt.Errorf("cannot produce object with non-string key: %v", key)
			}
			value, err := dereferenceVar(value, ctx)
			if err != nil {
				return err
			}
			result[asStr] = value
			return nil
		})
		return result, err

	case opalog.ScalarDoc:
		isDefined := false
		err := TopDown(ctx, func(ctx *TopDownContext) error {
			isDefined = true
			return nil
		})
		if err != nil {
			return nil, err
		}
		if !isDefined {
			return Undefined{}, nil
		}
		return valueToInterface(ctx.Rule.Value.Value, ctx)
	}
	panic(fmt.Sprintf("illegal argument: %v", ctx.Rule.DocKind()))
}

type builtinFunction func(*TopDownContext, *opalog.Expr, TopDownIterator) error

var builtinFunctions = map[opalog.Var]builtinFunction{
	opalog.Var("="): evalEq,
}

// dereferenceVar is used to lookup the variable binding and convert the value to
// a native Go type.
func dereferenceVar(v opalog.Var, ctx *TopDownContext) (interface{}, error) {
	binding, ok := ctx.Bindings[v]
	if !ok {
		return nil, fmt.Errorf("unbound variable: %v", v)
	}

	return valueToInterface(binding, ctx)
}

func evalEq(ctx *TopDownContext, expr *opalog.Expr, iter TopDownIterator) error {

	operands := expr.Terms.([]*opalog.Term)
	a := operands[1].Value
	b := operands[2].Value

	return evalEqUnify(ctx, a, b, iter)
}

func evalEqGround(ctx *TopDownContext, a opalog.Value, b opalog.Value, iter TopDownIterator) error {
	av, err := valueToInterface(a, ctx)
	if err != nil {
		return err
	}
	bv, err := valueToInterface(b, ctx)
	if err != nil {
		return err
	}
	if Compare(av, bv) == 0 {
		return iter(ctx)
	}
	return nil
}

// evalEqUnify is the top level of the unification implementation.
//
// When evaluating equality expressions, OPA tries to unify variables
// with values or other variables in the expression.
//
// The simplest case for unification is an expression of the form "<var> = ???".
// In this case, the variable is unified/bound to the other side the expression
// and evaluation continues to the next expression.
//
// In cases involving composites, OPA tries to unify elements in the same position
// of collections. For example, given an expression "[1,2,3] = [1,x,y]", OPA will
// unify variables x and y with the numbers 2 and 3. This process happens recursively,
// such that unification can happen on deeply embedded values.
//
// In cases involving references, OPA assumes that the references are ground at this stage.
// As a result, references are just special cases of the normal scalar/composite unification.
func evalEqUnify(ctx *TopDownContext, a opalog.Value, b opalog.Value, iter TopDownIterator) error {

	// Plug bindings into both terms because this will be called recursively and there may be
	// new bindings that have been made as part of unification.
	a = plugValue(a, ctx.Bindings)
	b = plugValue(b, ctx.Bindings)

	switch a := a.(type) {
	case opalog.Var:
		return evalEqUnifyVar(ctx, a, b, iter)
	case opalog.Object:
		return evalEqUnifyObject(ctx, a, b, iter)
	case opalog.Array:
		return evalEqUnifyArray(ctx, a, b, iter)
	default:
		switch b := b.(type) {
		case opalog.Var:
			return evalEqUnifyVar(ctx, b, a, iter)
		case opalog.Array:
			return evalEqUnifyArray(ctx, b, a, iter)
		case opalog.Object:
			return evalEqUnifyObject(ctx, b, a, iter)
		default:
			return evalEqGround(ctx, a, b, iter)
		}
	}

}

func evalEqUnifyArray(ctx *TopDownContext, a opalog.Array, b opalog.Value, iter TopDownIterator) error {
	switch b := b.(type) {
	case opalog.Var:
		return evalEqUnifyVar(ctx, b, a, iter)
	case opalog.Ref:
		return evalEqUnifyArrayRef(ctx, a, b, iter)
	case opalog.Array:
		return evalEqUnifyArrays(ctx, a, b, iter)
	default:
		return nil
	}
}

func evalEqUnifyArrayRef(ctx *TopDownContext, a opalog.Array, b opalog.Ref, iter TopDownIterator) error {

	r, err := ctx.Store.Lookup(b)
	if err != nil {
		return err
	}

	slice, ok := r.([]interface{})
	if !ok {
		return nil
	}

	if len(a) != len(slice) {
		return nil
	}

	for i := range a {
		if isScalar(a[i].Value) {
			v, err := valueToInterface(a[i].Value, ctx)
			if err != nil {
				return err
			}
			if Compare(v, slice[i]) != 0 {
				return nil
			}
		} else {
			var tmp *TopDownContext
			child := make(opalog.Ref, len(b), len(b)+1)
			copy(child, b)
			child = append(child, opalog.NumberTerm(float64(i)))
			err := evalEqUnify(ctx, a[i].Value, child, func(ctx *TopDownContext) error {
				tmp = ctx
				return nil
			})
			if err != nil {
				return err
			}
			if tmp == nil {
				return nil
			}
			ctx = tmp
		}
	}
	return iter(ctx)
}

func evalEqUnifyArrays(ctx *TopDownContext, a opalog.Array, b opalog.Array, iter TopDownIterator) error {
	aLen := len(a)
	bLen := len(b)
	if aLen != bLen {
		return nil
	}
	for i := 0; i < aLen; i++ {
		ai := a[i].Value
		bi := b[i].Value
		var tmp *TopDownContext
		err := evalEqUnify(ctx, ai, bi, func(ctx *TopDownContext) error {
			tmp = ctx
			return nil
		})
		if err != nil {
			return err
		}
		if tmp == nil {
			return nil
		}
		ctx = tmp
	}
	return iter(ctx)
}

// evalEqUnifyObject attempts to unify the object "a" with some other value "b".
// TODO(tsandal): unification of object keys (or unordered sets in general) is not
// supported because it would be too expensive. We may revisit this in the future.
func evalEqUnifyObject(ctx *TopDownContext, a opalog.Object, b opalog.Value, iter TopDownIterator) error {
	switch b := b.(type) {
	case opalog.Var:
		return evalEqUnifyVar(ctx, b, a, iter)
	case opalog.Ref:
		return evalEqUnifyObjectRef(ctx, a, b, iter)
	case opalog.Object:
		return evalEqUnifyObjects(ctx, a, b, iter)
	default:
		return nil
	}
}

func evalEqUnifyObjectRef(ctx *TopDownContext, a opalog.Object, b opalog.Ref, iter TopDownIterator) error {

	r, err := ctx.Store.Lookup(b)

	if err != nil {
		return err
	}

	for i := range a {
		if !a[i][0].IsGround() {
			return fmt.Errorf("cannot unify object with variable key: %v", a[i][0])
		}
	}

	obj, ok := r.(map[string]interface{})
	if !ok {
		return nil
	}

	if len(obj) != len(a) {
		return nil
	}

	for i := range a {
		// TODO(tsandall): support non-string keys in storage.
		k, ok := a[i][0].Value.(opalog.String)
		if !ok {
			return fmt.Errorf("cannot unify object with non-string key: %v", a[i][0])
		}

		_, ok = obj[string(k)]
		if !ok {
			return nil
		}

		child := make(opalog.Ref, len(b), len(b)+1)
		copy(child, b)
		child = append(child, a[i][0])
		var tmp *TopDownContext
		err := evalEqUnify(ctx, a[i][1].Value, child, func(ctx *TopDownContext) error {
			tmp = ctx
			return nil
		})
		if err != nil {
			return err
		}
		if tmp == nil {
			return nil
		}
		ctx = tmp
	}
	return iter(ctx)
}

func evalEqUnifyObjects(ctx *TopDownContext, a opalog.Object, b opalog.Object, iter TopDownIterator) error {

	if len(a) != len(b) {
		return nil
	}

	for i := range a {
		if !a[i][0].IsGround() {
			return fmt.Errorf("cannot unify object with variable key: %v", a[i][0])
		}
		if !b[i][0].IsGround() {
			return fmt.Errorf("cannot unify object with variable key: %v", b[i][0])
		}
	}

	for i := range a {
		var tmp *TopDownContext
		for j := range b {
			if b[j][0].Equal(a[i][0]) {
				err := evalEqUnify(ctx, a[i][1].Value, b[j][1].Value, func(ctx *TopDownContext) error {
					tmp = ctx
					return nil
				})
				if err != nil {
					return err
				}
				if tmp == nil {
					break
				}
			}
		}
		if tmp == nil {
			return nil
		}
		ctx = tmp
	}

	return iter(ctx)
}

func evalEqUnifyVar(ctx *TopDownContext, a opalog.Var, b opalog.Value, iter TopDownIterator) error {
	ctx = ctx.Bind(a, b)
	if ctx == nil {
		return nil
	}
	return iter(ctx)
}

func evalExpr(ctx *TopDownContext, iter TopDownIterator) error {
	expr := plugExpr(ctx.Current(), ctx.Bindings)
	switch tt := expr.Terms.(type) {
	case []*opalog.Term:
		builtin := builtinFunctions[tt[0].Value.(opalog.Var)]
		if builtin == nil {
			// Operator validation is done at compile-time so we panic here because
			// this should never happen.
			panic("unreachable")
		}
		return builtin(ctx, expr, iter)
	case *opalog.Term:
		switch tv := tt.Value.(type) {
		case opalog.Boolean:
			if tv.Equal(opalog.Boolean(true)) {
				return iter(ctx)
			}
			return nil
		default:
			panic("not implemented: set membership and implicit casts")
		}
	default:
		panic(fmt.Sprintf("illegal argument: %v", tt))
	}
}

func evalRef(ctx *TopDownContext, ref opalog.Ref, iter TopDownIterator) error {
	return evalRefRec(ctx, ref, iter, opalog.EmptyRef())
}

func evalRefRec(ctx *TopDownContext, ref opalog.Ref, iter TopDownIterator, path opalog.Ref) error {

	if len(ref) == 0 {
		_, err := ctx.Store.Lookup(path)
		if err != nil {
			switch err := err.(type) {
			case *StorageError:
				if err.Code == StorageNotFoundErr {
					return nil
				}
			}
			return err
		}
		return iter(ctx)
	}

	head := ref[0]
	tail := ref[1:]

	headVar, isVar := head.Value.(opalog.Var)

	if !isVar || len(path) == 0 {
		path = append(path, head)
		return evalRefRec(ctx, tail, iter, path)
	}

	binding, isBound := ctx.Bindings[headVar]

	if isBound {
		path = append(path, &opalog.Term{Value: binding})
		return evalRefRec(ctx, tail, iter, path)
	}

	node, err := ctx.Store.Lookup(path)
	if err != nil {
		switch err := err.(type) {
		case *StorageError:
			if err.Code == StorageNotFoundErr {
				return nil
			}
		}
		return err
	}

	switch node := node.(type) {
	case map[string]interface{}:
		for key := range node {
			cpy := ctx.Bind(headVar, opalog.String(key))
			path = append(path, opalog.StringTerm(key))
			err := evalRefRec(cpy, tail, iter, path)
			if err != nil {
				return err
			}
			path = path[:len(path)-1]
		}
		return nil
	case []interface{}:
		for i := range node {
			cpy := ctx.Bind(headVar, opalog.Number(i))
			path = append(path, opalog.NumberTerm(float64(i)))
			err := evalRefRec(cpy, tail, iter, path)
			if err != nil {
				return err
			}
			path = path[:len(path)-1]
		}
		return nil
	default:
		return fmt.Errorf("unexpected non-composite encountered via reference %v at path: %v", ref, path)
	}
}

func evalRule(ctx *TopDownContext, iter TopDownIterator) error {
	if ctx.Index >= len(ctx.Rule.Body) {
		return iter(ctx)
	}
	return evalTerms(ctx, func(ctx *TopDownContext) error {
		return evalExpr(ctx, func(ctx *TopDownContext) error {
			ctx = ctx.Step()
			return evalRule(ctx, iter)
		})
	})
}

func evalTerms(ctx *TopDownContext, iter TopDownIterator) error {
	expr := ctx.Current()
	var ts []*opalog.Term
	switch t := expr.Terms.(type) {
	case []*opalog.Term:
		ts = t
	case *opalog.Term:
		ts = append(ts, t)
	default:
		panic(fmt.Sprintf("illegal argument: %v", t))
	}
	return evalTermsRec(ctx, iter, ts)
}

func evalTermsRec(ctx *TopDownContext, iter TopDownIterator, ts []*opalog.Term) error {

	if len(ts) == 0 {
		return iter(ctx)
	}

	head := ts[0]
	tail := ts[1:]

	switch head := head.Value.(type) {
	case opalog.Ref:
		return evalRef(ctx, head, func(ctx *TopDownContext) error {
			return evalTermsRec(ctx, iter, tail)
		})
	default:
		return evalTermsRec(ctx, iter, tail)
	}
}

func isScalar(v opalog.Value) bool {
	switch v.(type) {
	case opalog.Null:
		return true
	case opalog.Boolean:
		return true
	case opalog.Number:
		return true
	case opalog.String:
		return true
	}
	return false
}

func plugExpr(expr *opalog.Expr, bindings Bindings) *opalog.Expr {
	plugged := *expr
	switch ts := plugged.Terms.(type) {
	case []*opalog.Term:
		var buf []*opalog.Term
		buf = append(buf, ts[0])
		for _, term := range ts[1:] {
			buf = append(buf, plugTerm(term, bindings))
		}
		plugged.Terms = buf
	case *opalog.Term:
		plugged.Terms = plugTerm(ts, bindings)
	default:
		panic(fmt.Sprintf("illegal argument: %v", ts))
	}
	return &plugged
}

func plugTerm(term *opalog.Term, bindings Bindings) *opalog.Term {

	if term.IsGround() {
		return term
	}

	switch v := term.Value.(type) {
	case opalog.Var:
		return &opalog.Term{Value: plugValue(v, bindings)}

	case opalog.Ref:
		plugged := *term
		plugged.Value = plugValue(v, bindings)
		return &plugged

	case opalog.Array:
		plugged := *term
		plugged.Value = plugValue(v, bindings)
		return &plugged

	case opalog.Object:
		plugged := *term
		plugged.Value = plugValue(v, bindings)
		return &plugged

	default:
		panic("unreachable")
	}
}

func plugValue(v opalog.Value, bindings Bindings) opalog.Value {

	if v.IsGround() {
		return v
	}

	switch v := v.(type) {
	case opalog.Var:
		binding, ok := bindings[v]
		if !ok {
			return v
		}
		return binding

	case opalog.Ref:
		var buf opalog.Ref
		buf = append(buf, v[0])
		for _, p := range v[1:] {
			buf = append(buf, plugTerm(p, bindings))
		}
		return buf

	case opalog.Array:
		var buf opalog.Array
		for _, e := range v {
			buf = append(buf, plugTerm(e, bindings))
		}
		return buf

	case opalog.Object:
		var buf opalog.Object
		for _, e := range v {
			k := plugTerm(e[0], bindings)
			v := plugTerm(e[1], bindings)
			buf = append(buf, [...]*opalog.Term{k, v})
		}
		return buf

	default:
		panic("unreachable")
	}
}

// valueToInterface returns the underlying Go value associated with an AST value.
// If the value is a reference, the reference is fetched from storage. Composite
// AST values such as objects and arrays are converted recursively.
func valueToInterface(v opalog.Value, ctx *TopDownContext) (interface{}, error) {

	switch v := v.(type) {

	// Scalars easily convert to native values.
	case opalog.Null:
		return nil, nil
	case opalog.Boolean:
		return bool(v), nil
	case opalog.Number:
		return float64(v), nil
	case opalog.String:
		return string(v), nil

	// Recursively convert array into []interface{}...
	case opalog.Array:
		buf := []interface{}{}
		for _, x := range v {
			x1, err := valueToInterface(x.Value, ctx)
			if err != nil {
				return nil, err
			}
			buf = append(buf, x1)
		}
		return buf, nil

	// Recursively convert object into map[string]interface{}...
	case opalog.Object:
		buf := map[string]interface{}{}
		for _, x := range v {
			k, err := valueToInterface(x[0].Value, ctx)
			if err != nil {
				return nil, err
			}
			asStr, stringKey := k.(string)
			if !stringKey {
				return nil, fmt.Errorf("cannot convert object with non-string key to map: %v", k)
			}
			v, err := valueToInterface(x[1].Value, ctx)
			if err != nil {
				return nil, err
			}
			buf[asStr] = v
		}
		return buf, nil

	// References convert to native values via lookup.
	case opalog.Ref:
		return ctx.Store.Lookup(v)

	default:
		// If none of the above cases are hit, something is very wrong, e.g., the caller
		// is attempting to convert a variable to a native Go value (which represents an
		// issue with the callers logic.)
		panic(fmt.Sprintf("illegal argument: %v", v))
	}
}

// walkValue invokes the iterator for each AST value contained inside the supplied AST value.
// If walkValue is called with a scalar, the iterator is invoked exactly once.
// If walkValue is called with a reference, the iterator is invoked for each element in the reference.
func walkValue(value opalog.Value, iter func(opalog.Value) bool) bool {
	switch value := value.(type) {
	case opalog.Ref:
		for _, x := range value {
			if walkValue(x.Value, iter) {
				return true
			}
		}
		return false
	case opalog.Array:
		for _, x := range value {
			if walkValue(x.Value, iter) {
				return true
			}
		}
		return false
	case opalog.Object:
		for _, i := range value {
			if walkValue(i[0].Value, iter) {
				return true
			}
			if walkValue(i[1].Value, iter) {
				return true
			}
		}
		return false
	default:
		return iter(value)
	}
}
