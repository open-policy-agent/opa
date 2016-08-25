// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
)

// Context contains the state of the evaluation process.
type Context struct {
	Query    ast.Body
	Compiler *ast.Compiler
	Globals  *storage.Bindings
	Locals   *storage.Bindings
	Index    int
	Previous *Context
	Store    *storage.Storage
	Tracer   Tracer

	txn   storage.Transaction
	cache *contextcache
}

// NewContext creates a new Context with no bindings.
func NewContext(query ast.Body, compiler *ast.Compiler, store *storage.Storage, txn storage.Transaction) *Context {
	return &Context{
		Query:    query,
		Compiler: compiler,
		Globals:  storage.NewBindings(),
		Locals:   storage.NewBindings(),
		Store:    store,
		txn:      txn,
		cache:    newContextCache(),
	}
}

// Binding returns the value bound to the given key.
func (ctx *Context) Binding(k ast.Value) ast.Value {
	if v := ctx.Locals.Get(k); v != nil {
		return v
	}
	if ctx.Globals != nil {
		if v := ctx.Globals.Get(k); v != nil {
			return v
		}
	}
	return nil
}

// Undo represents a binding that can be undone.
type Undo struct {
	Key   ast.Value
	Value ast.Value
	Prev  *Undo
}

// Bind updates the context to include a binding from the key to the value. The return
// value is used to return the context to the state before the binding was added.
func (ctx *Context) Bind(key ast.Value, value ast.Value, prev *Undo) *Undo {
	o := ctx.Locals.Get(key)
	ctx.Locals.Put(key, value)
	return &Undo{key, o, prev}
}

// Unbind updates the context by removing the binding represented by the undo.
func (ctx *Context) Unbind(undo *Undo) {
	for u := undo; u != nil; u = u.Prev {
		if u.Value != nil {
			ctx.Locals.Put(u.Key, u.Value)
		} else {
			ctx.Locals.Delete(u.Key)
		}
	}
}

// Child returns a new context to evaluate a query that was referenced by this context.
func (ctx *Context) Child(query ast.Body, locals *storage.Bindings) *Context {
	cpy := *ctx
	cpy.Query = query
	cpy.Locals = locals
	cpy.Previous = ctx
	cpy.Index = 0
	return &cpy
}

// Current returns the current expression to evaluate.
func (ctx *Context) Current() *ast.Expr {
	return ctx.Query[ctx.Index]
}

// Step returns a new context to evaluate the next expression.
func (ctx *Context) Step() *Context {
	cpy := *ctx
	cpy.Index++
	return &cpy
}

func (ctx *Context) trace(f string, a ...interface{}) {
	if ctx.Tracer == nil {
		return
	}
	if ctx.Tracer.Enabled() {
		ctx.Tracer.Trace(ctx, f, a...)
	}
}

func (ctx *Context) traceEval() {
	ctx.trace("Eval %v", ctx.Current())
}

func (ctx *Context) traceTry(expr *ast.Expr) {
	ctx.trace(" Try %v", expr)
}

func (ctx *Context) traceSuccess(expr *ast.Expr) {
	ctx.trace("  Success %v", expr)
}

func (ctx *Context) traceFinish() {
	ctx.trace("   Finish %v", ctx.Locals)
}

// contextcache stores the result of rule evaluation for a query. The contextcache
// is inherited by child contexts. The cache is consulted when virtual document
// references are evaluated. If a miss occurs, the virtual document is generated
// and the cache is updated.
type contextcache struct {
	partialobjs map[*ast.Rule]map[ast.Value]ast.Value
	complete    map[*ast.Rule]ast.Value
}

func newContextCache() *contextcache {
	return &contextcache{
		partialobjs: map[*ast.Rule]map[ast.Value]ast.Value{},
		complete:    map[*ast.Rule]ast.Value{},
	}
}

// Error is the error type returned by the Eval and Query functions when
// an evaluation error occurs.
type Error struct {
	Code    int
	Message string
}

const (

	// InternalErr represents an unknown evaluation error
	InternalErr = iota

	// UnboundGlobalErr indicates a global variable without a binding was
	// encountered during evaluation.
	UnboundGlobalErr = iota

	// ConflictErr indicates multiple (conflicting) values were produced
	// while generating a virtual document. E.g., given two rules that share
	// the same name: p = false :- true, p = true :- true, a query "p" would
	// evaluate p to "true" and "false".
	ConflictErr = iota
)

func (e *Error) Error() string {
	return fmt.Sprintf("evaluation error (code: %v): %v", e.Code, e.Message)
}

// IsUnboundGlobal returns true if the error e is an UnboundGlobalErr
func IsUnboundGlobal(e error) bool {
	if e, ok := e.(*Error); ok {
		return e.Code == UnboundGlobalErr
	}
	return false
}

func unboundGlobalVarErr(r ast.Ref) error {
	return &Error{
		Code:    UnboundGlobalErr,
		Message: fmt.Sprintf("unbound variable %v: %v", r[0], r),
	}
}

func conflictErr(query interface{}, kind string, rule *ast.Rule) error {
	return &Error{
		Code:    ConflictErr,
		Message: fmt.Sprintf("multiple values for %v: rules must produce exactly one value for %v: check rule definition(s): %v", query, kind, rule.Name),
	}
}

// Iterator is the interface for processing contexts.
type Iterator func(*Context) error

// Continue binds the key to the value in the current context and invokes the iterator.
// This is a helper function for simple cases where a single value (e.g., a variable) needs
// to be bound to a value in order for the evaluation the proceed.
func Continue(ctx *Context, key, value ast.Value, iter Iterator) error {
	undo := ctx.Bind(key, value, nil)
	err := iter(ctx)
	ctx.Unbind(undo)
	return err
}

// ContinueN binds N keys to N values. The key/value pairs are passed in as alternating pairs, e.g.,
// key-1, value-1, key-2, value-2, ..., key-N, value-N.
func ContinueN(ctx *Context, iter Iterator, x ...ast.Value) error {
	var prev *Undo
	for i := 0; i < len(x)/2; i++ {
		offset := i * 2
		prev = ctx.Bind(x[offset], x[offset+1], prev)
	}
	err := iter(ctx)
	ctx.Unbind(prev)
	return err
}

// Eval runs the evaluation algorithm on the context and calls the iterator
// for each context that contains bindings that satisfy all of the expressions
// inside the body.
func Eval(ctx *Context, iter Iterator) error {
	return evalContext(ctx, iter)
}

// PlugExpr returns a copy of expr with bound terms substituted for values in ctx.
func PlugExpr(expr *ast.Expr, ctx *Context) *ast.Expr {
	plugged := *expr
	switch ts := plugged.Terms.(type) {
	case []*ast.Term:
		var buf []*ast.Term
		buf = append(buf, ts[0])
		for _, term := range ts[1:] {
			buf = append(buf, PlugTerm(term, ctx))
		}
		plugged.Terms = buf
	case *ast.Term:
		plugged.Terms = PlugTerm(ts, ctx)
	default:
		panic(fmt.Sprintf("illegal argument: %v", ts))
	}
	return &plugged
}

// PlugTerm returns a copy of term with bound terms substituted for values in ctx.
func PlugTerm(term *ast.Term, ctx *Context) *ast.Term {
	switch v := term.Value.(type) {
	case ast.Var:
		return &ast.Term{Value: PlugValue(v, ctx)}

	case ast.Ref:
		plugged := *term
		plugged.Value = PlugValue(v, ctx)
		return &plugged

	case ast.Array:
		plugged := *term
		plugged.Value = PlugValue(v, ctx)
		return &plugged

	case ast.Object:
		plugged := *term
		plugged.Value = PlugValue(v, ctx)
		return &plugged

	case *ast.ArrayComprehension:
		plugged := *term
		plugged.Value = PlugValue(v, ctx)
		return &plugged

	default:
		if !term.IsGround() {
			panic("unreachable")
		}
		return term
	}
}

// PlugValue returns a copy of v with bound terms substituted for values in ctx.
func PlugValue(v ast.Value, ctx *Context) ast.Value {

	switch v := v.(type) {
	case ast.Var:
		if b := ctx.Binding(v); b != nil {
			return PlugValue(b, ctx)
		}
		return v

	case *ast.ArrayComprehension:
		b := ctx.Binding(v)
		if b == nil {
			return v
		}
		return b

	case ast.Ref:
		if b := ctx.Binding(v); b != nil {
			return b
		}
		buf := make(ast.Ref, len(v))
		buf[0] = v[0]
		for i, p := range v[1:] {
			buf[i+1] = PlugTerm(p, ctx)
		}
		return buf

	case ast.Array:
		buf := make(ast.Array, len(v))
		for i, e := range v {
			buf[i] = PlugTerm(e, ctx)
		}
		return buf

	case ast.Object:
		buf := make(ast.Object, len(v))
		for i, e := range v {
			k := PlugTerm(e[0], ctx)
			v := PlugTerm(e[1], ctx)
			buf[i] = [...]*ast.Term{k, v}
		}
		return buf

	default:
		if !v.IsGround() {
			panic(fmt.Sprintf("illegal value: %v %v", ctx, v))
		}
		return v
	}
}

// QueryParams defines input parameters for the query interface.
type QueryParams struct {
	Compiler *ast.Compiler
	Store    *storage.Storage
	Globals  *storage.Bindings
	Tracer   Tracer
	Path     []interface{}
}

// NewQueryParams returns a new QueryParams.
func NewQueryParams(compiler *ast.Compiler, store *storage.Storage, globals *storage.Bindings, path []interface{}) *QueryParams {
	return &QueryParams{
		Compiler: compiler,
		Store:    store,
		Globals:  globals,
		Path:     path,
	}
}

// NewContext returns a new Context that can be used to do evaluation.
func (q *QueryParams) NewContext(body ast.Body, txn storage.Transaction) *Context {
	ctx := &Context{
		Query:    body,
		Compiler: q.Compiler,
		Globals:  q.Globals,
		Locals:   storage.NewBindings(),
		Store:    q.Store,
		Tracer:   q.Tracer,
		txn:      txn,
		cache:    newContextCache(),
	}
	return ctx
}

// Query returns the document identified by the path.
//
// If the storage node identified by the path is a collection of rules, then the TopDown
// algorithm is run to generate the virtual document defined by the rules.
func Query(params *QueryParams) (interface{}, error) {

	ref := ast.Ref{ast.DefaultRootDocument}
	for _, v := range params.Path {
		switch v := v.(type) {
		case float64:
			ref = append(ref, ast.NumberTerm(v))
		case string:
			ref = append(ref, ast.StringTerm(v))
		case bool:
			ref = append(ref, ast.BooleanTerm(v))
		case nil:
			ref = append(ref, ast.NullTerm())
		default:
			return nil, fmt.Errorf("bad path element: %v (%T)", v, v)
		}
	}

	txn, err := params.Store.NewTransaction()
	if err != nil {
		return nil, err
	}

	defer params.Store.Close(txn)

	node, err := params.Store.Read(txn, ref)
	if err != nil {
		return nil, err
	}

	switch node := node.(type) {
	case []*ast.Rule:
		if len(node) == 0 {
			return Undefined{}, nil
		}
		// This assumes that all the rules identified by the path are of the same
		// type. This is checked at compile time.
		switch node[0].DocKind() {
		case ast.CompleteDoc:
			return queryCompleteDoc(params, txn, node)
		case ast.PartialObjectDoc:
			return queryPartialObjectDoc(params, txn, node)
		case ast.PartialSetDoc:
			return queryPartialSetDoc(params, txn, node)
		default:
			return nil, fmt.Errorf("illegal document type %T: %v", node[0].DocKind(), ref)
		}
	default:
		return node, nil
	}
}

// Undefined represents the absence of bindings that satisfy a completely defined rule.
// See the documentation for Query for more details.
type Undefined struct{}

func (undefined Undefined) String() string {
	return "<undefined>"
}

// ValueToInterface returns the underlying Go value associated with an AST value.
// If the value is a reference, the reference is fetched from storage. Composite
// AST values such as objects and arrays are converted recursively.
func ValueToInterface(v ast.Value, ctx *Context) (interface{}, error) {

	switch v := v.(type) {

	// Scalars easily convert to native values.
	case ast.Null:
		return nil, nil
	case ast.Boolean:
		return bool(v), nil
	case ast.Number:
		return float64(v), nil
	case ast.String:
		return string(v), nil

	// Recursively convert array into []interface{}...
	case ast.Array:
		buf := []interface{}{}
		for _, x := range v {
			x1, err := ValueToInterface(x.Value, ctx)
			if err != nil {
				return nil, err
			}
			buf = append(buf, x1)
		}
		return buf, nil

	// Recursively convert object into map[string]interface{}...
	case ast.Object:
		buf := map[string]interface{}{}
		for _, x := range v {
			k, err := ValueToInterface(x[0].Value, ctx)
			if err != nil {
				return nil, err
			}
			asStr, stringKey := k.(string)
			if !stringKey {
				return nil, fmt.Errorf("illegal object key: %v", k)
			}
			v, err := ValueToInterface(x[1].Value, ctx)
			if err != nil {
				return nil, err
			}
			buf[asStr] = v
		}
		return buf, nil

	// References convert to native values via lookup.
	case ast.Ref:
		return ctx.Store.Read(ctx.txn, v)

	default:
		v = PlugValue(v, ctx)
		if !v.IsGround() {
			return nil, fmt.Errorf("unbound value: %v", v)
		}
		return ValueToInterface(v, ctx)
	}
}

// ValueToSlice returns the underlying Go value associated with an AST value.
// If the value is a reference, the reference is fetched from storage.
func ValueToSlice(v ast.Value, ctx *Context) ([]interface{}, error) {
	x, err := ValueToInterface(v, ctx)
	if err != nil {
		return nil, err
	}
	s, ok := x.([]interface{})
	if !ok {
		return nil, fmt.Errorf("illegal argument: %v", x)
	}
	return s, nil
}

// ValueToFloat64 returns the underlying Go value associated with an AST value.
// If the value is a reference, the reference is fetched from storage.
func ValueToFloat64(v ast.Value, ctx *Context) (float64, error) {
	x, err := ValueToInterface(v, ctx)
	if err != nil {
		return 0, err
	}
	f, ok := x.(float64)
	if !ok {
		return 0, fmt.Errorf("illegal argument: %v", v)
	}
	return f, nil
}

// ValueToString returns the underlying Go value associated with an AST value.
// If the value is a reference, the reference is fetched from storage.
func ValueToString(v ast.Value, ctx *Context) (string, error) {
	x, err := ValueToInterface(v, ctx)
	if err != nil {
		return "", err
	}
	s, ok := x.(string)
	if !ok {
		return "", fmt.Errorf("illegal argument: %v", v)
	}
	return s, nil
}

// ValueToStrings returns a slice of strings associated with an AST value.
func ValueToStrings(v ast.Value, ctx *Context) ([]string, error) {
	sl, err := ValueToSlice(v, ctx)
	if err != nil {
		return nil, err
	}
	r := make([]string, len(sl))
	for i, x := range sl {
		var ok bool
		r[i], ok = x.(string)
		if !ok {
			return nil, fmt.Errorf("illegal argument: %v", x)
		}
	}
	return r, nil
}

func evalContext(ctx *Context, iter Iterator) error {

	if ctx.Index >= len(ctx.Query) {
		ctx.traceFinish()
		return iter(ctx)
	}

	ctx.traceEval()

	if ctx.Current().Negated {
		return evalContextNegated(ctx, iter)
	}

	return evalTerms(ctx, func(ctx *Context) error {
		return evalExpr(ctx, func(ctx *Context) error {
			ctx = ctx.Step()
			return evalContext(ctx, iter)
		})
	})
}

func evalContextNegated(ctx *Context, iter Iterator) error {

	negation := *ctx
	negation.Query = ast.Body([]*ast.Expr{ctx.Current().Complement()})
	negation.Index = 0
	negation.Previous = ctx

	isTrue := false

	err := evalContext(&negation, func(*Context) error {
		isTrue = true
		return nil
	})

	if err != nil {
		return err
	}

	if !isTrue {
		return evalContext(ctx.Step(), iter)
	}

	return nil
}

func evalExpr(ctx *Context, iter Iterator) error {
	expr := PlugExpr(ctx.Current(), ctx)
	ctx.traceTry(expr)
	switch tt := expr.Terms.(type) {
	case []*ast.Term:
		builtin := builtinFunctions[tt[0].Value.(ast.Var)]
		if builtin == nil {
			// Operator validation is done at compile-time so we panic here because
			// this should never happen.
			panic(fmt.Sprintf("illegal built-in: %v", tt[0]))
		}
		return builtin(ctx, expr, func(ctx *Context) error {
			ctx.traceSuccess(expr)
			return iter(ctx)
		})
	case *ast.Term:
		v := tt.Value
		if r, ok := v.(ast.Ref); ok {
			var err error
			v, err = lookupValue(ctx, r)
			if err != nil {
				return err
			}
		}
		if !v.Equal(ast.Boolean(false)) {
			if v.IsGround() {
				ctx.traceSuccess(expr)
				return iter(ctx)
			}
		}
		return nil
	default:
		panic(fmt.Sprintf("illegal argument: %v", tt))
	}
}

// evalRef evaluates the ast.Ref ref and calls the Iterator iter once for each
// instance of ref that would be defined. If an error occurs during the evaluation
// process, the return value is non-nil. Also, if iter returns an error, the return
// value is non-nil.
func evalRef(ctx *Context, ref, path ast.Ref, iter Iterator) error {

	if len(ref) == 0 {
		// If this reference refers to a local variable, evaluate against the binding.
		// Otherwise, evaluate against the database.
		if !path[0].Equal(ast.DefaultRootDocument) {
			v := ctx.Binding(path[0].Value)
			if v == nil {
				return unboundGlobalVarErr(path)
			}
			return evalRefRuleResult(ctx, path, path[1:], v, iter)
		}
		return evalRefRec(ctx, ast.Ref{path[0]}, path[1:], iter)
	}

	head, tail := ref[0], ref[1:]
	n, ok := head.Value.(ast.Ref)
	if !ok {
		path = append(path, head)
		return evalRef(ctx, tail, path, iter)
	}

	return evalRef(ctx, n, ast.Ref{}, func(ctx *Context) error {
		var undo *Undo
		if b := ctx.Binding(n); b == nil {
			p := PlugValue(n, ctx).(ast.Ref)
			v, err := lookupValue(ctx, p)
			if err != nil {
				return err
			}
			undo = ctx.Bind(n, v, nil)
		}
		tmp := append(path, head)
		err := evalRef(ctx, tail, tmp, iter)
		if undo != nil {
			ctx.Unbind(undo)
		}
		return err
	})
}

func evalRefRec(ctx *Context, path, tail ast.Ref, iter Iterator) error {

	if len(tail) == 0 {
		ok, err := lookupExists(ctx, path)
		if err == nil && ok {
			return iter(ctx)
		}
		return err
	}

	if tail[0].IsGround() {
		// Check if the node exists. If the node does not exist, stop.
		// If the node exists and is a rule, evaluate the rule to produce a virtual doc.
		// Otherwise, process the rest of the reference.
		path = append(path, PlugTerm(tail[0], ctx))
		rules, err := lookupRule(ctx, path)

		if err != nil {
			if storage.IsNotFound(err) {
				return nil
			}
			return err
		}

		if rules != nil {
			ref := append(path, tail[1:]...)
			return evalRefRule(ctx, ref, path, rules, iter)
		}

		return evalRefRec(ctx, path, tail[1:], iter)
	}

	// Check if the variable has a binding.
	// If there is a binding, process the rest of the reference normally.
	// If there is no binding, enumerate the collection referred to by the path.
	plugged := PlugTerm(tail[0], ctx)

	if plugged.IsGround() {
		path = append(path, plugged)
		return evalRefRec(ctx, path, tail[1:], iter)
	}

	return evalRefRecWalkColl(ctx, path, tail, iter)
}

func evalRefRecWalkColl(ctx *Context, path, tail ast.Ref, iter Iterator) error {

	node, err := ctx.Store.Read(ctx.txn, path)
	if err != nil {
		if storage.IsNotFound(err) {
			return nil
		}
		return err
	}

	head := tail[0].Value.(ast.Var)
	tail = tail[1:]

	switch node := node.(type) {
	case map[string]interface{}:
		for key := range node {
			undo := ctx.Bind(head, ast.String(key), nil)
			path = append(path, ast.StringTerm(key))
			err := evalRefRec(ctx, path, tail, iter)
			ctx.Unbind(undo)
			if err != nil {
				return err
			}
			path = path[:len(path)-1]
		}
		return nil
	case []interface{}:
		for i := range node {
			undo := ctx.Bind(head, ast.Number(i), nil)
			path = append(path, ast.NumberTerm(float64(i)))
			err := evalRefRec(ctx, path, tail, iter)
			ctx.Unbind(undo)
			if err != nil {
				return err
			}
			path = path[:len(path)-1]
		}
		return nil
	default:
		return fmt.Errorf("non-collection document: %v", path)
	}
}

func evalRefRule(ctx *Context, ref ast.Ref, path ast.Ref, rules []*ast.Rule, iter Iterator) error {

	suffix := ref[len(path):]

	switch rules[0].DocKind() {

	case ast.CompleteDoc:
		return evalRefRuleCompleteDoc(ctx, ref, suffix, rules, iter)

	case ast.PartialObjectDoc:
		if len(suffix) == 0 {
			return evalRefRulePartialObjectDocFull(ctx, ref, rules, iter)
		}
		for _, rule := range rules {
			err := evalRefRulePartialObjectDoc(ctx, ref, path, rule, iter)
			if err != nil {
				return err
			}
		}

	case ast.PartialSetDoc:
		if len(suffix) == 0 {
			return fmt.Errorf("not implemented: full evaluation of virtual set documents: %v", ref)
		}
		for _, rule := range rules {
			err := evalRefRulePartialSetDoc(ctx, ref, path, rule, iter)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func evalRefRuleCompleteDoc(ctx *Context, ref ast.Ref, suffix ast.Ref, rules []*ast.Rule, iter Iterator) error {

	var result ast.Value

	// Check if we have cached the result of evaluating this rule set already.
	for _, rule := range rules {
		if doc, ok := ctx.cache.complete[rule]; ok {
			return evalRefRuleResult(ctx, ref, suffix, doc, iter)
		}
	}

	for _, rule := range rules {

		bindings := storage.NewBindings()
		child := ctx.Child(rule.Body, bindings)

		err := Eval(child, func(child *Context) error {
			if result == nil {
				result = PlugValue(rule.Value.Value, child)
			} else {
				r := PlugValue(rule.Value.Value, child)
				if !result.Equal(r) {
					return conflictErr(ref, "complete documents", rule)
				}
			}
			return nil
		})

		if err != nil {
			return err
		}
	}

	if result != nil {
		// Add the result to the cache. All of the rules have either produced the same value
		// or only one of them has produced a value. As such, we can cache the result on any
		// of them.
		ctx.cache.complete[rules[0]] = result
		return evalRefRuleResult(ctx, ref, suffix, result, iter)
	}

	return nil
}

func evalRefRulePartialObjectDoc(ctx *Context, ref ast.Ref, path ast.Ref, rule *ast.Rule, iter Iterator) error {
	suffix := ref[len(path):]

	key := PlugValue(suffix[0].Value, ctx)

	// There are two cases being handled below. The first deals with non-ground
	// keys. If the key is not ground, we evaluate the child query and copy the
	// key binding from the child context into this context. The second deals
	// with ground keys. In that case, we initialize the child context with a key
	// binding and evaluate the child query. This reduces the amount of processing
	// the child query has to do.
	//
	// In the first case, we do not unify the keys because the unification does not
	// namespace variables within their context. As a result, we could end up with
	// a recursive binding if we unified "key" with "rule.Key.Value". If unification
	// is improved to handle namespacing, this can be revisited.
	if !key.IsGround() {
		child := ctx.Child(rule.Body, storage.NewBindings())
		return Eval(child, func(child *Context) error {
			key := PlugValue(rule.Key.Value, child)
			if !key.IsGround() {
				return fmt.Errorf("unbound variable: %v", key)
			}
			value := PlugValue(rule.Value.Value, child)
			if !value.IsGround() {
				return fmt.Errorf("unbound variable: %v", value)
			}
			undo := ctx.Bind(suffix[0].Value.(ast.Var), key, nil)
			err := evalRefRuleResult(ctx, ref, ref[len(path)+1:], value, iter)
			ctx.Unbind(undo)
			return err
		})
	}

	// Check if the rule has already been evaluated with this key. If it has,
	// proceed with the cached value. Otherwise, evaluate the rule and update
	// the cache.
	if docs, ok := ctx.cache.partialobjs[rule]; ok {
		k := key
		if r, ok := key.(ast.Ref); ok {
			var err error
			k, err = lookupValue(ctx, r)
			if err != nil {
				if storage.IsNotFound(err) {
					return nil
				}
				return err
			}
		}
		if doc, ok := docs[k]; ok {
			return evalRefRuleResult(ctx, ref, ref[len(path)+1:], doc, iter)
		}
	}

	child := ctx.Child(rule.Body, storage.NewBindings())
	_, err := evalEqUnify(child, key, rule.Key.Value, nil, func(child *Context) error {
		return Eval(child, func(child *Context) error {
			value := PlugValue(rule.Value.Value, child)
			if !value.IsGround() {
				return fmt.Errorf("unbound variable: %v", value)
			}
			cache, ok := ctx.cache.partialobjs[rule]
			if !ok {
				cache = map[ast.Value]ast.Value{}
				ctx.cache.partialobjs[rule] = cache
			}
			k := key
			if r, ok := key.(ast.Ref); ok {
				var err error
				k, err = lookupValue(ctx, r)
				if err != nil {
					if storage.IsNotFound(err) {
						return nil
					}
					return err
				}
			}
			cache[k] = value
			return evalRefRuleResult(ctx, ref, ref[len(path)+1:], value.(ast.Value), iter)
		})
	})

	return err

}

func evalRefRulePartialObjectDocFull(ctx *Context, ref ast.Ref, rules []*ast.Rule, iter Iterator) error {

	var result ast.Object

	// keys is a set of key bindings that we obtain while
	// evaluating the child query. If we obtain multiple
	// values for a single key, it represents a conflictErr
	// and we raise an error.
	keys := util.NewHashMap(func(a, b util.T) bool {
		return a.(ast.Value).Equal(b.(ast.Value))
	}, func(x util.T) int {
		return x.(ast.Value).Hash()
	})

	for _, rule := range rules {

		bindings := storage.NewBindings()
		child := ctx.Child(rule.Body, bindings)

		err := Eval(child, func(child *Context) error {
			key := PlugValue(rule.Key.Value, child)
			if !key.IsGround() {
				return fmt.Errorf("unbound variable: %v", key)
			}
			if _, ok := keys.Get(key); ok {
				return conflictErr(ref, "object document keys", rule)
			}
			keys.Put(key, ast.Null{})
			value := PlugValue(rule.Value.Value, child)
			if !value.IsGround() {
				return fmt.Errorf("unbound variable: %v", value)
			}
			result = append(result, ast.Item(&ast.Term{Value: key}, &ast.Term{Value: value}))
			return nil
		})

		if err != nil {
			return err
		}
	}

	return Continue(ctx, ref, result, iter)
}

func evalRefRulePartialSetDoc(ctx *Context, ref ast.Ref, path ast.Ref, rule *ast.Rule, iter Iterator) error {

	suffix := ref[len(path):]
	key := PlugValue(suffix[0].Value, ctx)

	// See comment in evalRefRulePartialObjectDoc about the two branches below.
	if !key.IsGround() {
		child := ctx.Child(rule.Body, storage.NewBindings())

		// TODO(tsandall): Currently this evaluates the child query without any
		// bindings from the current context. In cases where the key is partially
		// ground this may be quite inefficient. An optimization would be to unify
		// variables in the child query with ground values in the current context/query.
		// To do this, the unification may need to be improved to namespace variables
		// across contexts (otherwise we could end up with recursive bindings).
		return Eval(child, func(child *Context) error {
			value := PlugValue(rule.Key.Value, child)
			if !value.IsGround() {
				return fmt.Errorf("unbound variable: %v", rule.Value)
			}

			undo, err := evalEqUnify(ctx, key, value, nil, func(child *Context) error {
				return Continue(ctx, ref[:len(path)+1], ast.Boolean(true), iter)
			})

			if err != nil {
				return err
			}
			ctx.Unbind(undo)
			return nil
		})
	}

	child := ctx.Child(rule.Body, storage.NewBindings())

	_, err := evalEqUnify(child, key, rule.Key.Value, nil, func(child *Context) error {
		return Eval(child, func(child *Context) error {
			return Continue(ctx, ref[:len(path)+1], ast.Boolean(true), iter)
		})
	})

	return err
}

func evalRefRuleResult(ctx *Context, ref ast.Ref, suffix ast.Ref, result ast.Value, iter Iterator) error {

	s := make(ast.Ref, len(suffix))
	for i := range suffix {
		s[i] = PlugTerm(suffix[i], ctx)
	}

	return evalRefRuleResultRec(ctx, result, s, ast.Ref{}, func(ctx *Context, v ast.Value) error {
		return Continue(ctx, ref, v, iter)
	})
}

func evalRefRuleResultRec(ctx *Context, v ast.Value, ref, path ast.Ref, iter func(*Context, ast.Value) error) error {
	if len(ref) == 0 {
		return iter(ctx, v)
	}
	switch v := v.(type) {
	case ast.Array:
		return evalRefRuleResultRecArray(ctx, v, ref, path, iter)
	case ast.Object:
		return evalRefRuleResultRecObject(ctx, v, ref, path, iter)
	case ast.Ref:
		return evalRefRuleResultRecRef(ctx, v, ref, path, iter)
	}
	return nil
}

func evalRefRuleResultRecArray(ctx *Context, arr ast.Array, ref, path ast.Ref, iter func(*Context, ast.Value) error) error {
	head, tail := ref[0], ref[1:]
	switch n := head.Value.(type) {
	case ast.Number:
		idx := int(n)
		if ast.Number(idx) != n {
			return nil
		}
		if idx >= len(arr) {
			return nil
		}
		el := arr[idx]
		path = append(path, head)
		return evalRefRuleResultRec(ctx, el.Value, tail, path, iter)
	case ast.Var:
		for i := range arr {
			idx := ast.Number(i)
			undo := ctx.Bind(n, idx, nil)
			path = append(path, &ast.Term{Value: idx})
			if err := evalRefRuleResultRec(ctx, arr[i].Value, tail, path, iter); err != nil {
				return err
			}
			ctx.Unbind(undo)
			path = path[:len(path)-1]
		}
	}
	return nil
}

func evalRefRuleResultRecObject(ctx *Context, obj ast.Object, ref, path ast.Ref, iter func(*Context, ast.Value) error) error {
	head, tail := ref[0], ref[1:]
	switch k := head.Value.(type) {
	case ast.String:
		match := -1
		for idx, i := range obj {
			x := i[0].Value
			if r, ok := i[0].Value.(ast.Ref); ok {
				var err error
				if x, err = lookupValue(ctx, r); err != nil {
					if storage.IsNotFound(err) {
						return nil
					}
					return err
				}
			}
			if x.Equal(k) {
				match = idx
				break
			}
		}
		if match == -1 {
			return nil
		}
		path = append(path, head)
		return evalRefRuleResultRec(ctx, obj[match][1].Value, tail, path, iter)
	case ast.Var:
		for _, i := range obj {
			undo := ctx.Bind(k, i[0].Value, nil)
			path = append(path, i[0])
			if err := evalRefRuleResultRec(ctx, i[1].Value, tail, path, iter); err != nil {
				return err
			}
			ctx.Unbind(undo)
			path = path[:len(path)-1]
		}
	}
	return nil
}

func evalRefRuleResultRecRef(ctx *Context, v, ref, path ast.Ref, iter func(*Context, ast.Value) error) error {
	b := append(v, ref...)
	return evalRefRec(ctx, v, ref, func(ctx *Context) error {
		return iter(ctx, PlugValue(b, ctx))
	})
}

func evalTerms(ctx *Context, iter Iterator) error {

	expr := ctx.Current()

	// Attempt to evaluate the terms using indexing. Indexing can be used
	// if this is an equality expression where one side is a non-ground,
	// non-nested reference to a base document and the other side is a
	// reference or some ground term. If indexing is available for the terms,
	// the index is built lazily.
	if expr.IsEquality() {

		// The terms must be plugged otherwise the index may yield bindings
		// that would result in false positives.
		ts := expr.Terms.([]*ast.Term)
		t1 := PlugTerm(ts[1], ctx)
		t2 := PlugTerm(ts[2], ctx)
		r1, ok1 := t1.Value.(ast.Ref)
		r2, ok2 := t2.Value.(ast.Ref)

		if ok1 && !r1.IsGround() && !r1.IsNested() && (ok2 || t2.IsGround()) {
			ok, err := indexBuildLazy(ctx, r1)
			if err != nil {
				return errors.Wrapf(err, "index build failed on %v", r1)
			}
			if ok {
				return evalTermsIndexed(ctx, iter, r1, t2)
			}
		}

		if ok2 && !r2.IsGround() && !r2.IsNested() && (ok1 || t1.IsGround()) {
			ok, err := indexBuildLazy(ctx, r2)
			if err != nil {
				return errors.Wrapf(err, "index build failed on %v", r2)
			}
			if ok {
				return evalTermsIndexed(ctx, iter, r2, t1)
			}
		}
	}

	var ts []*ast.Term
	switch t := expr.Terms.(type) {
	case []*ast.Term:
		ts = t
	case *ast.Term:
		ts = append(ts, t)
	default:
		panic(fmt.Sprintf("illegal argument: %v", t))
	}

	return evalTermsRec(ctx, iter, ts)
}

func evalTermsComprehension(ctx *Context, comp ast.Value, iter Iterator) error {
	switch comp := comp.(type) {
	case *ast.ArrayComprehension:
		r := ast.Array{}
		c := ctx.Child(comp.Body, ctx.Locals)
		err := Eval(c, func(c *Context) error {
			r = append(r, PlugTerm(comp.Term, c))
			return nil
		})
		if err != nil {
			return err
		}
		return Continue(ctx, comp, r, iter)
	default:
		panic(fmt.Sprintf("illegal argument: %v %v", ctx, comp))
	}
}

func evalTermsIndexed(ctx *Context, iter Iterator, indexed ast.Ref, nonIndexed *ast.Term) error {

	iterateIndex := func(ctx *Context) error {

		// Evaluate the non-indexed term.
		plugged := PlugTerm(nonIndexed, ctx)
		nonIndexedValue, err := ValueToInterface(plugged.Value, ctx)
		if err != nil {
			return err
		}

		// Iterate the bindings for the indexed term that when applied to the reference
		// would locate the non-indexed value obtained above.
		return ctx.Store.Index(ctx.txn, indexed, nonIndexedValue, func(bindings *storage.Bindings) error {
			var prev *Undo
			bindings.Iter(func(k, v ast.Value) bool {
				prev = ctx.Bind(k, v, prev)
				return false
			})
			err := iter(ctx)
			ctx.Unbind(prev)
			return err
		})

	}

	return evalTermsRec(ctx, iterateIndex, []*ast.Term{nonIndexed})
}

func evalTermsRec(ctx *Context, iter Iterator, ts []*ast.Term) error {

	if len(ts) == 0 {
		return iter(ctx)
	}

	head := ts[0]
	tail := ts[1:]

	switch head := head.Value.(type) {
	case ast.Ref:
		return evalRef(ctx, head, ast.Ref{}, func(ctx *Context) error {
			return evalTermsRec(ctx, iter, tail)
		})
	case ast.Array:
		return evalTermsRecArray(ctx, head, 0, func(ctx *Context) error {
			return evalTermsRec(ctx, iter, tail)
		})
	case ast.Object:
		return evalTermsRecObject(ctx, head, 0, func(ctx *Context) error {
			return evalTermsRec(ctx, iter, tail)
		})
	case *ast.ArrayComprehension:
		return evalTermsComprehension(ctx, head, func(ctx *Context) error {
			return evalTermsRec(ctx, iter, tail)
		})
	default:
		return evalTermsRec(ctx, iter, tail)
	}
}

func evalTermsRecArray(ctx *Context, arr ast.Array, idx int, iter Iterator) error {
	if idx >= len(arr) {
		return iter(ctx)
	}
	switch v := arr[idx].Value.(type) {
	case ast.Ref:
		return evalRef(ctx, v, ast.Ref{}, func(ctx *Context) error {
			return evalTermsRecArray(ctx, arr, idx+1, iter)
		})
	case ast.Array:
		return evalTermsRecArray(ctx, v, 0, func(ctx *Context) error {
			return evalTermsRecArray(ctx, arr, idx+1, iter)
		})
	case ast.Object:
		return evalTermsRecObject(ctx, v, 0, func(ctx *Context) error {
			return evalTermsRecArray(ctx, arr, idx+1, iter)
		})
	case *ast.ArrayComprehension:
		return evalTermsComprehension(ctx, v, func(ctx *Context) error {
			return evalTermsRecArray(ctx, arr, idx+1, iter)
		})
	default:
		return evalTermsRecArray(ctx, arr, idx+1, iter)
	}
}

func evalTermsRecObject(ctx *Context, obj ast.Object, idx int, iter Iterator) error {
	if idx >= len(obj) {
		return iter(ctx)
	}
	switch k := obj[idx][0].Value.(type) {
	case ast.Ref:
		return evalRef(ctx, k, ast.Ref{}, func(ctx *Context) error {
			switch v := obj[idx][1].Value.(type) {
			case ast.Ref:
				return evalRef(ctx, v, ast.Ref{}, func(ctx *Context) error {
					return evalTermsRecObject(ctx, obj, idx+1, iter)
				})
			case ast.Array:
				return evalTermsRecArray(ctx, v, 0, func(ctx *Context) error {
					return evalTermsRecObject(ctx, obj, idx+1, iter)
				})
			case ast.Object:
				return evalTermsRecObject(ctx, v, 0, func(ctx *Context) error {
					return evalTermsRecObject(ctx, obj, idx+1, iter)
				})
			case *ast.ArrayComprehension:
				return evalTermsComprehension(ctx, v, func(ctx *Context) error {
					return evalTermsRecObject(ctx, obj, idx+1, iter)
				})
			default:
				return evalTermsRecObject(ctx, obj, idx+1, iter)
			}
		})
	default:
		switch v := obj[idx][1].Value.(type) {
		case ast.Ref:
			return evalRef(ctx, v, ast.Ref{}, func(ctx *Context) error {
				return evalTermsRecObject(ctx, obj, idx+1, iter)
			})
		case ast.Array:
			return evalTermsRecArray(ctx, v, 0, func(ctx *Context) error {
				return evalTermsRecObject(ctx, obj, idx+1, iter)
			})
		case ast.Object:
			return evalTermsRecObject(ctx, v, 0, func(ctx *Context) error {
				return evalTermsRecObject(ctx, obj, idx+1, iter)
			})
		case *ast.ArrayComprehension:
			return evalTermsComprehension(ctx, v, func(ctx *Context) error {
				return evalTermsRecObject(ctx, obj, idx+1, iter)
			})
		default:
			return evalTermsRecObject(ctx, obj, idx+1, iter)
		}
	}
}

// indexBuildLazy returns true if there is an index built for this term. If there is no index
// currently built for the term, but the term is a candidate for indexing, ther index will be
// built on the fly.
func indexBuildLazy(ctx *Context, ref ast.Ref) (bool, error) {

	if ref.IsGround() || ref.IsNested() {
		return false, nil
	}

	// Check if index was already built.
	if ctx.Store.IndexExists(ref) {
		return true, nil
	}

	// Ignore refs against variables.
	if !ref[0].Equal(ast.DefaultRootDocument) {
		return false, nil
	}

	// Ignore refs against virtual docs.
	tmp := ast.Ref{ref[0], ref[1]}
	r, err := lookupRule(ctx, tmp)
	if err != nil || r != nil {
		return false, err
	}

	for _, p := range ref[2:] {

		if !p.Value.IsGround() {
			break
		}

		tmp = append(tmp, p)
		r, err := lookupRule(ctx, tmp)
		if err != nil || r != nil {
			return false, err
		}
	}

	if err := ctx.Store.BuildIndex(ctx.txn, ref); err != nil {
		switch err := err.(type) {
		case *storage.Error:
			if err.Code == storage.IndexingNotSupportedErr {
				return false, nil
			}
		}
		return false, err
	}

	return true, nil
}

func lookupExists(ctx *Context, ref ast.Ref) (bool, error) {
	_, err := ctx.Store.Read(ctx.txn, ref)
	if err != nil {
		if storage.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func lookupRule(ctx *Context, ref ast.Ref) ([]*ast.Rule, error) {
	r, err := ctx.Store.Read(ctx.txn, ref)
	if err != nil {
		return nil, err
	}
	switch r := r.(type) {
	case ([]*ast.Rule):
		return r, nil
	default:
		return nil, nil
	}
}

func lookupValue(ctx *Context, ref ast.Ref) (ast.Value, error) {
	r, err := ctx.Store.Read(ctx.txn, ref)
	if err != nil {
		return nil, err
	}
	return ast.InterfaceToValue(r)
}

func queryCompleteDoc(params *QueryParams, txn storage.Transaction, rules []*ast.Rule) (interface{}, error) {

	var result ast.Value
	var resultContext *Context

	for _, rule := range rules {
		ctx := params.NewContext(rule.Body, txn)

		err := Eval(ctx, func(ctx *Context) error {
			if result == nil {
				result = PlugValue(rule.Value.Value, ctx)
			} else {
				r := PlugValue(rule.Value.Value, ctx)
				if !result.Equal(r) {
					return conflictErr(params.Path, "complete documents", rule)
				}
			}
			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	if result == nil {
		return Undefined{}, nil
	}

	return ValueToInterface(result, resultContext)
}

func queryPartialObjectDoc(params *QueryParams, txn storage.Transaction, rules []*ast.Rule) (interface{}, error) {

	result := map[string]interface{}{}
	keys := map[string]struct{}{}

	for _, rule := range rules {
		ctx := params.NewContext(rule.Body, txn)
		err := Eval(ctx, func(ctx *Context) error {
			k, err := ValueToInterface(rule.Key.Value, ctx)
			if err != nil {
				return err
			}
			key, ok := k.(string)
			if !ok {
				return fmt.Errorf("illegal object key: %v", k)
			}
			if _, ok := keys[key]; ok {
				return conflictErr(params.Path, "object document keys", rule)
			}
			value, err := ValueToInterface(rule.Value.Value, ctx)
			if err != nil {
				return err
			}
			keys[key] = struct{}{}
			result[key] = value
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func queryPartialSetDoc(params *QueryParams, txn storage.Transaction, rules []*ast.Rule) (interface{}, error) {
	result := []interface{}{}
	for _, rule := range rules {
		ctx := params.NewContext(rule.Body, txn)
		err := Eval(ctx, func(ctx *Context) error {
			r, err := ValueToInterface(rule.Key.Value, ctx)
			if err != nil {
				return err
			}
			result = append(result, r)
			return nil
		})

		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
