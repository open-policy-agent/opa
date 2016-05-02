// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package eval

import "github.com/open-policy-agent/opa/opalog"
import "fmt"

// TopDownContext contains the state of the evaluation process.
//
// TODO(tsandall): profile perf/memory usage with current approach;
// the current approach copies the TopDownContext structure for each
// step and binding. This avoids the need to undo steps and bindings
// each time the proof fails but this may be too expensive.
type TopDownContext struct {
	Query    opalog.Body
	Bindings *hashMap
	Index    int
	Previous *TopDownContext
	Store    Storage
	Tracer   Tracer
}

// BindRef returns a new TopDownContext with bindings that map the reference to the value.
func (ctx *TopDownContext) BindRef(ref opalog.Ref, value opalog.Value) *TopDownContext {
	cpy := *ctx
	cpy.Bindings = newHashMap()
	ctx.Bindings.Iter(func(k opalog.Value, v opalog.Value) bool {
		cpy.Bindings.Put(k, v)
		return false
	})
	cpy.Bindings.Put(ref, value)
	return &cpy
}

// BindVar returns a new TopDownContext with bindings that map the variable to the value.
//
// If the caller attempts to bind a variable to itself (e.g., "x = x"), no mapping is added. This
// is effectively a no-op.
//
// If the call would introduce a recursive binding, nil is returned. This represents an undefined
// context that cannot be evaluated. E.g., "x = [x]".
//
// Binding a variable to some value also updates existing bindings to that variable.
//
// TODO(tsandall): potential optimization:
//
// The current implementation eagerly flattens the bindings by updating existing
// bindings each time a new binding is added. E.g., if Bind(y, [1,2,x]) and Bind(x, 3) are called
// (one after the other), the binding for "y" will be updated. It may be possible to defer this
// to a later stage, e.g., when plugging the values.
func (ctx *TopDownContext) BindVar(variable opalog.Var, value opalog.Value) *TopDownContext {
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
	cpy.Bindings = newHashMap()
	tmp := newHashMap()
	tmp.Put(variable, value)
	ctx.Bindings.Iter(func(k opalog.Value, v opalog.Value) bool {
		cpy.Bindings.Put(k, plugValue(v, tmp))
		return false
	})
	cpy.Bindings.Put(variable, value)
	return &cpy
}

// Child returns a new context to evaluate a rule that was referenced by this context.
func (ctx *TopDownContext) Child(rule *opalog.Rule, bindings *hashMap) *TopDownContext {
	cpy := *ctx
	cpy.Query = rule.Body
	cpy.Bindings = bindings
	cpy.Previous = ctx
	return &cpy
}

// Current returns the current expression to evaluate.
func (ctx *TopDownContext) Current() *opalog.Expr {
	return ctx.Query[ctx.Index]
}

// Step returns a new context to evaluate the next expression.
func (ctx *TopDownContext) Step() *TopDownContext {
	cpy := *ctx
	cpy.Index++
	return &cpy
}

func (ctx *TopDownContext) trace(f string, a ...interface{}) {
	if ctx.Tracer == nil {
		return
	}
	if ctx.Tracer.Enabled() {
		ctx.Tracer.Trace(ctx, f, a...)
	}
}

func (ctx *TopDownContext) traceEval() {
	ctx.trace("Eval %v", ctx.Current())
}

func (ctx *TopDownContext) traceTry(expr *opalog.Expr) {
	ctx.trace(" Try %v", expr)
}

func (ctx *TopDownContext) traceSuccess(expr *opalog.Expr) {
	ctx.trace("  Success %v", expr)
}

func (ctx *TopDownContext) traceFinish() {
	ctx.trace("   Finish %v", ctx.Bindings)
}

// TopDownIterator is the interface for processing contexts.
type TopDownIterator func(*TopDownContext) error

// TopDown runs the evaluation algorithm on the contxet and calls the iterator
// foreach context that contains bindings that satisfy all of the expressions
// inside the body.
func TopDown(ctx *TopDownContext, iter TopDownIterator) error {
	return evalContext(ctx, iter)
}

// TopDownQueryParams defines input parameters for the query interface.
type TopDownQueryParams struct {
	Store  Storage
	Tracer Tracer
	Path   []string
}

// TopDownQuery returns the document identified by the path.
//
// If the storage node identified by the path is a collection of rules, then the TopDown
// algorithm is run to generate the virtual document defined by the rules.
func TopDownQuery(params *TopDownQueryParams) (interface{}, error) {

	var ref opalog.Ref
	ref = append(ref, opalog.VarTerm(params.Path[0]))
	for _, v := range params.Path[1:] {
		ref = append(ref, opalog.StringTerm(v))
	}

	node, err := params.Store.Lookup(ref)
	if err != nil {
		return nil, err
	}

	switch node := node.(type) {
	case []*opalog.Rule:
		if len(node) == 0 {
			return Undefined{}, nil
		}
		// This assumes that all the rules identified by the path are of the same
		// type. This is checked at compile time.
		switch node[0].DocKind() {
		case opalog.CompleteDoc:
			return topDownQueryCompleteDoc(params, node)
		case opalog.PartialObjectDoc:
			return topDownQueryPartialObjectDoc(params, node)
		case opalog.PartialSetDoc:
			return topDownQueryPartialSetDoc(params, node)
		default:
			return nil, fmt.Errorf("invalid document (kind: %v): %v", node[0].DocKind(), ref)
		}
	default:
		return node, nil
	}
}

// Undefined represents the absence of bindings that satisfy a completely defined rule.
// See the documentation for TopDownQuery for more details.
type Undefined struct{}

func (undefined Undefined) String() string {
	return "<undefined>"
}

type builtinFunction func(*TopDownContext, *opalog.Expr, TopDownIterator) error

var builtinFunctions = map[opalog.Var]builtinFunction{
	opalog.Var("="): evalEq,
}

// dereferenceVar is used to lookup the variable binding and convert the value to
// a native Go type.
func dereferenceVar(v opalog.Var, ctx *TopDownContext) (interface{}, error) {
	binding := ctx.Bindings.Get(v)
	if binding == nil {
		return nil, fmt.Errorf("unbound variable: %v", v)
	}
	return valueToInterface(binding, ctx)
}

func evalContext(ctx *TopDownContext, iter TopDownIterator) error {

	if ctx.Index >= len(ctx.Query) {
		ctx.traceFinish()
		return iter(ctx)
	}

	ctx.traceEval()

	if ctx.Current().Negated {
		return evalContextNegated(ctx, iter)
	}

	return evalTerms(ctx, func(ctx *TopDownContext) error {
		return evalExpr(ctx, func(ctx *TopDownContext) error {
			ctx = ctx.Step()
			return evalContext(ctx, iter)
		})
	})
}

func evalContextNegated(ctx *TopDownContext, iter TopDownIterator) error {

	negation := *ctx
	negation.Query = opalog.Body([]*opalog.Expr{ctx.Current().Complement()})
	negation.Index = 0
	negation.Previous = ctx

	isDefined := false

	err := evalContext(&negation, func(*TopDownContext) error {
		isDefined = true
		return nil
	})

	if err != nil {
		return err
	}

	if !isDefined {
		return evalContext(ctx.Step(), iter)
	}

	return nil
}

func evalEq(ctx *TopDownContext, expr *opalog.Expr, iter TopDownIterator) error {

	operands := expr.Terms.([]*opalog.Term)
	a := operands[1].Value
	b := operands[2].Value

	return evalEqUnify(ctx, a, b, func(ctx *TopDownContext) error {
		ctx.traceSuccess(expr)
		return iter(ctx)
	})
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
	ctx = ctx.BindVar(a, b)
	if ctx == nil {
		return nil
	}
	return iter(ctx)
}

func evalExpr(ctx *TopDownContext, iter TopDownIterator) error {
	expr := plugExpr(ctx.Current(), ctx.Bindings)
	ctx.traceTry(expr)
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
			return fmt.Errorf("implicit cast not supported: %v", tv)
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

	// Handle head of reference.
	if isVar && len(path) == 0 {

		// Check if head has a binding. E.g., x = [1,2,3], x[i] = 1
		binding := ctx.Bindings.Get(headVar)
		if binding != nil {
			return evalRefRuleResult(ctx, ref, ref[1:], binding, iter)
		}

		path = append(path, head)
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
		case []*opalog.Rule:
			for _, rule := range node {
				if err := evalRefRule(ctx, ref, path, rule, iter); err != nil {
					return err
				}
			}
			return nil
		default:
			return evalRefRec(ctx, tail, iter, path)
		}
	}

	// Handle constants in reference.
	if !isVar {
		path = append(path, head)
		return evalRefRec(ctx, tail, iter, path)
	}

	// Binding already exists for variable. Treat it as a constant.
	binding := ctx.Bindings.Get(headVar)
	if binding != nil {
		path = append(path, &opalog.Term{Value: binding})
		return evalRefRec(ctx, tail, iter, path)
	}

	// Binding does not exist, we will lookup the collection and enumerate
	// the keys below.
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
			cpy := ctx.BindVar(headVar, opalog.String(key))
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
			cpy := ctx.BindVar(headVar, opalog.Number(i))
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

func evalRefRule(ctx *TopDownContext, ref opalog.Ref, path opalog.Ref, rule *opalog.Rule, iter TopDownIterator) error {

	switch rule.DocKind() {
	case opalog.PartialSetDoc:
		return evalRefRulePartialSetDoc(ctx, ref, path, rule, iter)
	case opalog.PartialObjectDoc:
		return evalRefRulePartialObjectDoc(ctx, ref, path, rule, iter)
	case opalog.CompleteDoc:
		return evalRefRuleCompleteDoc(ctx, ref, path, rule, iter)
	default:
		panic(fmt.Sprintf("illegal argument: %v", rule))
	}

}

func evalRefRuleCompleteDoc(ctx *TopDownContext, ref opalog.Ref, path opalog.Ref, rule *opalog.Rule, iter TopDownIterator) error {
	suffix := ref[len(path):]
	if len(suffix) == 0 {
		return fmt.Errorf("not implemented: %v %v %v", ref, path, rule)
	}

	bindings := newHashMap()
	child := ctx.Child(rule, bindings)

	return TopDown(child, func(child *TopDownContext) error {
		switch v := rule.Value.Value.(type) {
		case opalog.Object:
			return evalRefRuleResult(ctx, ref, suffix, v, iter)
		case opalog.Array:
			return evalRefRuleResult(ctx, ref, suffix, v, iter)
		default:
			return fmt.Errorf("cannot dereference value (%T) in %v", rule.Value.Value, rule)
		}
	})
}

func evalRefRulePartialObjectDoc(ctx *TopDownContext, ref opalog.Ref, path opalog.Ref, rule *opalog.Rule, iter TopDownIterator) error {
	suffix := ref[len(path):]
	if len(suffix) == 0 {
		return fmt.Errorf("not implemented: %v %v %v", ref, path, rule)
	}

	if !suffix[0].IsGround() {
		child := ctx.Child(rule, newHashMap())
		return TopDown(child, func(child *TopDownContext) error {
			key := child.Bindings.Get(rule.Key.Value)
			if key == nil {
				return fmt.Errorf("unbound variable: %v", rule.Key)
			}
			value := child.Bindings.Get(rule.Value.Value)
			if value == nil {
				return fmt.Errorf("unbound variable: %v", rule.Value)
			}
			ctx = ctx.BindVar(suffix[0].Value.(opalog.Var), key)
			return evalRefRuleResult(ctx, ref, ref[len(path)+1:], value, iter)
		})
	}

	bindings := newHashMap()
	bindings.Put(rule.Key.Value, suffix[0].Value)
	child := ctx.Child(rule, bindings)

	return TopDown(child, func(child *TopDownContext) error {
		value := child.Bindings.Get(rule.Value.Value)
		if value == nil {
			return fmt.Errorf("unbound variable: %v", rule.Value)
		}
		return evalRefRuleResult(ctx, ref, ref[len(path)+1:], value, iter)
	})
}

func evalRefRulePartialSetDoc(ctx *TopDownContext, ref opalog.Ref, path opalog.Ref, rule *opalog.Rule, iter TopDownIterator) error {
	suffix := ref[len(path):]
	if len(suffix) == 0 {
		return fmt.Errorf("not implemented: %v %v %v", ref, path, rule)
	}

	if len(suffix) > 1 {
		// TODO(tsandall): attempting to dereference set lookup
		// results in undefined value, catch this using static analysis
		return nil
	}

	if !suffix[0].IsGround() {
		child := ctx.Child(rule, newHashMap())
		return TopDown(child, func(child *TopDownContext) error {
			value := child.Bindings.Get(rule.Key.Value)
			if value == nil {
				return fmt.Errorf("unbound variable: %v", rule.Key)
			}
			// Take the output of the child context and bind (1) the value to
			// the variable from this context and (2) the reference to true
			// so that expression will be defined. E.g., given a simple rule:
			// "p = true :- q[x]", we say that "p" should be defined if "q"
			// is defined for some value "x".
			ctx = ctx.BindVar(suffix[0].Value.(opalog.Var), value)
			ctx = ctx.BindRef(ref[:len(path)+1], opalog.Boolean(true))
			return iter(ctx)
		})
	}

	bindings := newHashMap()
	bindings.Put(rule.Key.Value, suffix[0].Value)
	child := ctx.Child(rule, bindings)
	return TopDown(child, func(child *TopDownContext) error {
		// See comment above for explanation of why the reference is bound to true.
		ctx = ctx.BindRef(ref[:len(path)+1], opalog.Boolean(true))
		return iter(ctx)
	})

}

func evalRefRuleResult(ctx *TopDownContext, ref opalog.Ref, suffix opalog.Ref, result opalog.Value, iter TopDownIterator) error {

	switch result := result.(type) {
	case opalog.Ref:
		// Below we concatenate the result of evaluating a rule with the rest of the reference
		// to be evaluated.
		//
		// E.g., given two rules: q[k] = v :- a[k] = v and p = true :- q[k].a[i] = 100
		// when evaluating the expression in "p", this function will be called with a binding for
		// "q[k]" and "k". This leaves the remaining part of the reference (i.e., ["a"][i])
		// needing to be processed. This is done by substituting the prefix of the original reference
		// with the binding. In this case, the prefix is "q[k]" and the binding value would be
		// "a[<some key>]".
		var binding opalog.Ref
		binding = append(binding, result...)
		binding = append(binding, suffix...)
		return evalRefRec(ctx, suffix, func(ctx *TopDownContext) error {
			ctx = ctx.BindRef(ref, plugValue(binding, ctx.Bindings))
			return iter(ctx)
		}, result)

	case opalog.Array:
		if len(suffix) > 0 {
			return result.Query(suffix, func(keys map[opalog.Var]opalog.Value, value opalog.Value) error {
				ctx = ctx.BindRef(ref, value)
				for k, v := range keys {
					ctx = ctx.BindVar(k, v)
				}
				return iter(ctx)
			})
		}
		ctx.BindRef(ref, result)
		return iter(ctx)

	case opalog.Object:
		if len(suffix) > 0 {
			return result.Query(suffix, func(keys map[opalog.Var]opalog.Value, value opalog.Value) error {
				ctx = ctx.BindRef(ref, value)
				for k, v := range keys {
					ctx = ctx.BindVar(k, v)
				}
				return iter(ctx)
			})
		}
		ctx.BindRef(ref, result)
		return iter(ctx)

	default:
		if len(suffix) > 0 {
			return nil
		}
		ctx = ctx.BindRef(ref, result)
		return iter(ctx)
	}
}

// evalTerms is used to get bindings for variables in individual terms.
//
// Before an expression is evaluated, this function is called to find bindings
// for variables used in references inside the expression. Finding bindings for
// variables used in references involves iterating collections in storage or
// evaluating rules identified by the references. In either case, this function
// will invoke the iterator with each set of bindings that should be evaluated.
//
// TODO(tsandall): extend to support indexing.
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
	case opalog.Array:
		return evalTermsRecArray(ctx, head, 0, func(ctx *TopDownContext) error {
			return evalTermsRec(ctx, iter, tail)
		})
	case opalog.Object:
		return evalTermsRecObject(ctx, head, 0, func(ctx *TopDownContext) error {
			return evalTermsRec(ctx, iter, tail)
		})
	default:
		return evalTermsRec(ctx, iter, tail)
	}
}

func evalTermsRecArray(ctx *TopDownContext, arr opalog.Array, idx int, iter TopDownIterator) error {
	if idx >= len(arr) {
		return iter(ctx)
	}
	switch v := arr[idx].Value.(type) {
	case opalog.Ref:
		return evalRef(ctx, v, func(ctx *TopDownContext) error {
			return evalTermsRecArray(ctx, arr, idx+1, iter)
		})
	case opalog.Array:
		return evalTermsRecArray(ctx, v, 0, func(ctx *TopDownContext) error {
			return evalTermsRecArray(ctx, arr, idx+1, iter)
		})
	case opalog.Object:
		return evalTermsRecObject(ctx, v, 0, func(ctx *TopDownContext) error {
			return evalTermsRecArray(ctx, arr, idx+1, iter)
		})
	default:
		return evalTermsRecArray(ctx, arr, idx+1, iter)
	}
}

func evalTermsRecObject(ctx *TopDownContext, obj opalog.Object, idx int, iter TopDownIterator) error {
	if idx >= len(obj) {
		return iter(ctx)
	}
	switch k := obj[idx][0].Value.(type) {
	case opalog.Ref:
		return evalRef(ctx, k, func(ctx *TopDownContext) error {
			switch v := obj[idx][1].Value.(type) {
			case opalog.Ref:
				return evalRef(ctx, v, func(ctx *TopDownContext) error {
					return evalTermsRecObject(ctx, obj, idx+1, iter)
				})
			case opalog.Array:
				return evalTermsRecArray(ctx, v, 0, func(ctx *TopDownContext) error {
					return evalTermsRecObject(ctx, obj, idx+1, iter)
				})
			case opalog.Object:
				return evalTermsRecObject(ctx, v, 0, func(ctx *TopDownContext) error {
					return evalTermsRecObject(ctx, obj, idx+1, iter)
				})
			default:
				return evalTermsRecObject(ctx, obj, idx+1, iter)
			}
		})
	default:
		switch v := obj[idx][1].Value.(type) {
		case opalog.Ref:
			return evalRef(ctx, v, func(ctx *TopDownContext) error {
				return evalTermsRecObject(ctx, obj, idx+1, iter)
			})
		case opalog.Array:
			return evalTermsRecArray(ctx, v, 0, func(ctx *TopDownContext) error {
				return evalTermsRecObject(ctx, obj, idx+1, iter)
			})
		case opalog.Object:
			return evalTermsRecObject(ctx, v, 0, func(ctx *TopDownContext) error {
				return evalTermsRecObject(ctx, obj, idx+1, iter)
			})
		default:
			return evalTermsRecObject(ctx, obj, idx+1, iter)
		}
	}
}

func plugExpr(expr *opalog.Expr, bindings *hashMap) *opalog.Expr {
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

func plugTerm(term *opalog.Term, bindings *hashMap) *opalog.Term {
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
		if !term.IsGround() {
			panic("unreachable")
		}
		return term
	}
}

func plugValue(v opalog.Value, bindings *hashMap) opalog.Value {

	switch v := v.(type) {
	case opalog.Var:
		binding := bindings.Get(v)
		if binding == nil {
			return v
		}
		return binding

	case opalog.Ref:
		binding := bindings.Get(v)
		if binding != nil {
			return binding
		}
		if v.IsGround() {
			return v
		}
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
		if !v.IsGround() {
			panic("unreachable")
		}
		return v
	}
}

func topDownQueryCompleteDoc(params *TopDownQueryParams, rules []*opalog.Rule) (interface{}, error) {

	if len(rules) > 1 {
		return nil, fmt.Errorf("multiple conflicting rules: %v", rules[0].Name)
	}

	rule := rules[0]

	ctx := &TopDownContext{
		Query:    rule.Body,
		Bindings: newHashMap(),
		Store:    params.Store,
		Tracer:   params.Tracer,
	}

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

	return valueToInterface(rule.Value.Value, ctx)
}

func topDownQueryPartialObjectDoc(params *TopDownQueryParams, rules []*opalog.Rule) (interface{}, error) {

	result := map[string]interface{}{}

	for _, rule := range rules {
		ctx := &TopDownContext{
			Query:    rule.Body,
			Bindings: newHashMap(),
			Store:    params.Store,
			Tracer:   params.Tracer,
		}
		key := rule.Key.Value.(opalog.Var)
		value := rule.Value.Value.(opalog.Var)
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
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func topDownQueryPartialSetDoc(params *TopDownQueryParams, rules []*opalog.Rule) (interface{}, error) {
	result := []interface{}{}
	for _, rule := range rules {
		ctx := &TopDownContext{
			Query:    rule.Body,
			Bindings: newHashMap(),
			Store:    params.Store,
			Tracer:   params.Tracer,
		}
		key := rule.Key.Value.(opalog.Var)
		err := TopDown(ctx, func(ctx *TopDownContext) error {
			value, err := dereferenceVar(key, ctx)
			if err != nil {
				return err
			}
			result = append(result, value)
			return nil
		})

		if err != nil {
			return nil, err
		}
	}
	return result, nil
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
