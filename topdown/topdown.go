// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"sync"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
)

// Context contains the state of the evaluation process.
type Context struct {
	Query    ast.Body
	Compiler *ast.Compiler
	Globals  *ast.ValueMap
	Locals   *ast.ValueMap
	Index    int
	Previous *Context
	Store    *storage.Storage
	Tracer   Tracer

	// TODO(tsandall): make the transaction public and lazily create one in
	// Eval(). This way callers do not have to provide the transaction unless
	// they want to run evaluation multiple times against the same snapshot.
	txn   storage.Transaction
	cache *contextcache
	qid   uint64
	redos *redoStack
}

type queryIDFactory struct {
	next uint64
	mtx  sync.Mutex
}

func (f *queryIDFactory) Next() uint64 {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	next := f.next
	f.next++
	return next
}

func (f *queryIDFactory) Reset() {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	f.next = uint64(1)
}

var qidFactory = &queryIDFactory{
	next: uint64(1),
}

type redoStack struct {
	events []*redoStackElement
}

type redoStackElement struct {
	ctx *Context
	evt *Event
}

// NewContext creates a new Context with no bindings.
func NewContext(query ast.Body, compiler *ast.Compiler, store *storage.Storage, txn storage.Transaction) *Context {
	return &Context{
		Query:    query,
		Compiler: compiler,
		Locals:   ast.NewValueMap(),
		Store:    store,
		txn:      txn,
		cache:    newContextCache(),
		qid:      qidFactory.Next(),
		redos:    &redoStack{},
	}
}

// Binding returns the value bound to the given key.
func (ctx *Context) Binding(k ast.Value) ast.Value {
	if v := ctx.Locals.Get(k); v != nil {
		return v
	}
	if v := ctx.Globals.Get(k); v != nil {
		return v
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
func (ctx *Context) Child(query ast.Body, locals *ast.ValueMap) *Context {
	cpy := *ctx
	cpy.Query = query
	cpy.Locals = locals
	cpy.Previous = ctx
	cpy.Index = 0
	cpy.qid = qidFactory.Next()
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

func (ctx *Context) traceEnter(node interface{}) {
	if ctx.tracingEnabled() {
		evt := ctx.makeEvent(EnterOp, node)
		ctx.flushRedos(evt)
		ctx.Tracer.Trace(ctx, evt)
	}
}

func (ctx *Context) traceExit(node interface{}) {
	if ctx.tracingEnabled() {
		evt := ctx.makeEvent(ExitOp, node)
		ctx.flushRedos(evt)
		ctx.Tracer.Trace(ctx, evt)
	}
}

func (ctx *Context) traceEval(node interface{}) {
	if ctx.tracingEnabled() {
		evt := ctx.makeEvent(EvalOp, node)
		ctx.flushRedos(evt)
		ctx.Tracer.Trace(ctx, evt)
	}
}

func (ctx *Context) traceRedo(node interface{}) {
	if ctx.tracingEnabled() {
		evt := ctx.makeEvent(RedoOp, node)
		ctx.saveRedo(evt)
	}
}

func (ctx *Context) traceFail(node interface{}) {
	if ctx.tracingEnabled() {
		evt := ctx.makeEvent(FailOp, node)
		ctx.flushRedos(evt)
		ctx.Tracer.Trace(ctx, evt)
	}
}

func (ctx *Context) tracingEnabled() bool {
	return ctx.Tracer != nil && ctx.Tracer.Enabled()
}

func (ctx *Context) saveRedo(evt *Event) {

	buf := &redoStackElement{
		ctx: ctx,
		evt: evt,
	}

	// Search stack for redo that this (redo) event should follow.
	for len(ctx.redos.events) > 0 {
		idx := len(ctx.redos.events) - 1
		top := ctx.redos.events[idx]

		// Expression redo should follow rule/body redo from the same query.
		if evt.HasExpr() {
			if top.evt.QueryID == evt.QueryID && (top.evt.HasBody() || top.evt.HasRule()) {
				break
			}
		}

		// Rule/body redo should follow expression redo from the parent query.
		if evt.HasRule() || evt.HasBody() {
			if top.evt.QueryID == evt.ParentID && top.evt.HasExpr() {
				break
			}
		}

		// Top of stack can be discarded. This indicates the search terminated
		// without producing any more events.
		ctx.redos.events = ctx.redos.events[:idx]
	}

	ctx.redos.events = append(ctx.redos.events, buf)
}

func (ctx *Context) flushRedos(evt *Event) {

	idx := len(ctx.redos.events) - 1

	if idx != -1 {
		top := ctx.redos.events[idx]

		if top.evt.QueryID == evt.QueryID {
			for _, buf := range ctx.redos.events {
				ctx.Tracer.Trace(buf.ctx, buf.evt)
			}
		}

		ctx.redos.events = nil
	}

}

func (ctx *Context) makeEvent(op Op, node interface{}) *Event {
	evt := Event{
		Op:      op,
		Node:    node,
		QueryID: ctx.qid,
		Locals:  ctx.Locals.Copy(),
	}
	if ctx.Previous != nil {
		evt.ParentID = ctx.Previous.qid
	}
	return &evt
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

	// InternalErr represents an unknown evaluation error.
	InternalErr = iota

	// UnboundGlobalErr indicates a global variable without a binding was
	// encountered during evaluation.
	UnboundGlobalErr = iota

	// ConflictErr indicates multiple (conflicting) values were produced
	// while generating a virtual document. E.g., given two rules that share
	// the same name: p = false :- true, p = true :- true, a query "p" would
	// evaluate p to "true" and "false".
	ConflictErr = iota

	// TypeErr indicates evaluation stopped because an expression was applied to
	// a value of an inappropriate type.
	TypeErr = iota
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

// typeErrObjectKey returns a TypeErr with a message indicating the rule
// produced a key that of an inappropriate type.
func typeErrObjectKey(rule *ast.Rule, v ast.Value) error {
	return &Error{
		Code:    TypeErr,
		Message: rule.Location.Format("%v produced illegal object key type %T", rule.Name, v),
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
	ctx.traceEnter(ctx.Query)
	return evalContext(ctx, func(ctx *Context) error {
		ctx.traceExit(ctx.Query)
		if err := iter(ctx); err != nil {
			return err
		}
		ctx.traceRedo(ctx.Query)
		return nil
	})
}

// Binding defines the interface used to apply term bindings to terms,
// expressions, etc.
type Binding func(ast.Value) ast.Value

// PlugHead returns a copy of head with bound terms substituted for the binding.
func PlugHead(head *ast.Head, binding Binding) *ast.Head {
	plugged := *head
	if plugged.Key != nil {
		plugged.Key = PlugTerm(plugged.Key, binding)
	}
	if plugged.Value != nil {
		plugged.Value = PlugTerm(plugged.Value, binding)
	}
	return &plugged
}

// PlugExpr returns a copy of expr with bound terms substituted for the binding.
func PlugExpr(expr *ast.Expr, binding Binding) *ast.Expr {
	plugged := *expr
	switch ts := plugged.Terms.(type) {
	case []*ast.Term:
		var buf []*ast.Term
		buf = append(buf, ts[0])
		for _, term := range ts[1:] {
			buf = append(buf, PlugTerm(term, binding))
		}
		plugged.Terms = buf
	case *ast.Term:
		plugged.Terms = PlugTerm(ts, binding)
	default:
		panic(fmt.Sprintf("illegal argument: %v", ts))
	}
	return &plugged
}

// PlugTerm returns a copy of term with bound terms substituted for the binding.
func PlugTerm(term *ast.Term, binding Binding) *ast.Term {
	switch v := term.Value.(type) {
	case ast.Var:
		return &ast.Term{Value: PlugValue(v, binding)}

	case ast.Ref:
		plugged := *term
		plugged.Value = PlugValue(v, binding)
		return &plugged

	case ast.Array:
		plugged := *term
		plugged.Value = PlugValue(v, binding)
		return &plugged

	case ast.Object:
		plugged := *term
		plugged.Value = PlugValue(v, binding)
		return &plugged

	case *ast.ArrayComprehension:
		plugged := *term
		plugged.Value = PlugValue(v, binding)
		return &plugged

	default:
		if !term.IsGround() {
			panic("unreachable")
		}
		return term
	}
}

// PlugValue returns a copy of v with bound terms substituted for the binding.
func PlugValue(v ast.Value, binding func(ast.Value) ast.Value) ast.Value {

	switch v := v.(type) {
	case ast.Var:
		if b := binding(v); b != nil {
			return PlugValue(b, binding)
		}
		return v

	case *ast.ArrayComprehension:
		b := binding(v)
		if b == nil {
			return v
		}
		return b

	case ast.Ref:
		if b := binding(v); b != nil {
			return b
		}
		buf := make(ast.Ref, len(v))
		buf[0] = v[0]
		for i, p := range v[1:] {
			buf[i+1] = PlugTerm(p, binding)
		}
		return buf

	case ast.Array:
		buf := make(ast.Array, len(v))
		for i, e := range v {
			buf[i] = PlugTerm(e, binding)
		}
		return buf

	case ast.Object:
		buf := make(ast.Object, len(v))
		for i, e := range v {
			k := PlugTerm(e[0], binding)
			v := PlugTerm(e[1], binding)
			buf[i] = [...]*ast.Term{k, v}
		}
		return buf

	default:
		if !v.IsGround() {
			panic(fmt.Sprintf("illegal value: %v", v))
		}
		return v
	}
}

// QueryParams defines input parameters for the query interface.
type QueryParams struct {
	Compiler    *ast.Compiler
	Store       *storage.Storage
	Transaction storage.Transaction
	Globals     *ast.ValueMap
	Tracer      Tracer
	Path        []interface{}
}

// NewQueryParams returns a new QueryParams.
func NewQueryParams(compiler *ast.Compiler, store *storage.Storage, txn storage.Transaction, globals *ast.ValueMap, path []interface{}) *QueryParams {
	return &QueryParams{
		Compiler:    compiler,
		Store:       store,
		Transaction: txn,
		Globals:     globals,
		Path:        path,
	}
}

// NewContext returns a new Context that can be used to do evaluation.
func (q *QueryParams) NewContext(body ast.Body) *Context {
	ctx := NewContext(body, q.Compiler, q.Store, q.Transaction)
	ctx.Globals = q.Globals
	ctx.Tracer = q.Tracer
	return ctx
}

// Query returns the document identified by the path.
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

	// TODO(tsandall): set literals: once they are supported, this special case can
	// be removed.
	if rs := params.Compiler.GetRulesExact(ref); rs != nil {
		switch rs[0].DocKind() {
		case ast.PartialSetDoc:
			return queryPartialSetDoc(params, rs)
		}
	}

	// Construct and execute a query to obtain the value for the reference.
	query := ast.NewBody(ast.Equality.Expr(ast.RefTerm(ref...), ast.Wildcard))
	ctx := params.NewContext(query)
	var result interface{} = Undefined{}
	var err error

	err = Eval(ctx, func(ctx *Context) error {
		val := PlugValue(ast.Wildcard.Value, ctx.Binding)
		result, err = ValueToInterface(val, ctx)
		return err
	})

	if err != nil {
		return nil, err
	}

	return result, nil
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
				return nil, fmt.Errorf("object key type %T", k)
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
		v = PlugValue(v, ctx.Binding)
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
		return iter(ctx)
	}

	if ctx.Current().Negated {
		return evalContextNegated(ctx, iter)
	}

	ctx.traceEval(ctx.Current())

	// isRedo indicates if the expression's terms are defined at least once. If
	// any of the terms are undefined, then the closure below will not run (but
	// a Fail event still needs to be emitted).
	isRedo := false

	err := evalTerms(ctx, func(ctx *Context) error {
		isRedo = true

		// isTrue indicates if the expression is true and is used to determine
		// if a Fail event should be emitted below.
		isTrue := false

		err := evalExpr(ctx, func(ctx *Context) error {
			isTrue = true
			ctx = ctx.Step()
			return evalContext(ctx, iter)
		})

		if err != nil {
			return err
		}

		if !isTrue {
			ctx.traceFail(ctx.Current())
		}

		ctx.traceRedo(ctx.Current())

		return nil
	})

	if err != nil {
		return err
	}

	if !isRedo {
		ctx.traceFail(ctx.Current())
	}

	return nil
}

func evalContextNegated(ctx *Context, iter Iterator) error {

	negation := ast.NewBody(ctx.Current().Complement())
	child := ctx.Child(negation, ctx.Locals)

	ctx.traceEval(ctx.Current())

	isTrue := false

	err := Eval(child, func(*Context) error {
		isTrue = true
		return nil
	})

	if err != nil {
		return err
	}

	if !isTrue {
		return evalContext(ctx.Step(), iter)
	}

	ctx.traceFail(ctx.Current())

	return nil
}

func evalExpr(ctx *Context, iter Iterator) error {
	expr := PlugExpr(ctx.Current(), ctx.Binding)
	switch tt := expr.Terms.(type) {
	case []*ast.Term:
		builtin := builtinFunctions[tt[0].Value.(ast.Var)]
		if builtin == nil {
			// Operator validation is done at compile-time so we panic here because
			// this should never happen.
			panic(fmt.Sprintf("illegal built-in: %v", tt[0]))
		}
		return builtin(ctx, expr, iter)
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
				return iter(ctx)
			}
		}
		return nil
	default:
		panic(fmt.Sprintf("illegal argument: %v", tt))
	}
}

// evalRef evaluates the reference and invokes the iterator for each instance of
// the reference that is defined. The iterator is invoked with bindings for (1)
// all variables found in the reference and (2) the reference itself if that
// reference refers to a virtual document (ditto for nested references).
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
		return evalRefRec(ctx, path, iter)
	}

	head, tail := ref[0], ref[1:]
	n, ok := head.Value.(ast.Ref)
	if !ok {
		path = append(path, head)
		return evalRef(ctx, tail, path, iter)
	}

	return evalRef(ctx, n, ast.Ref{}, func(ctx *Context) error {

		var undo *Undo

		// Add a binding for the nested reference 'n' if one does not exist. If
		// 'n' referred to a virtual document the binding would already exist.
		// We bind nested references so that when the overall expression is
		// evaluated, it will not contain any nested references.
		if b := ctx.Binding(n); b == nil {
			var err error
			var v ast.Value
			switch p := PlugValue(n, ctx.Binding).(type) {
			case ast.Ref:
				v, err = lookupValue(ctx, p)
				if err != nil {
					return err
				}
			default:
				v = p
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

func evalRefRec(ctx *Context, ref ast.Ref, iter Iterator) error {

	// Obtain ground prefix of the reference.
	var plugged ast.Ref
	var prefix ast.Ref
	switch v := PlugValue(ref, ctx.Binding).(type) {
	case ast.Ref:
		plugged = v
		prefix = plugged.GroundPrefix()
	default:
		// Fast-path? TODO test case.
		return iter(ctx)
	}

	// Check if the prefix refers to a virtual document.
	var rules []*ast.Rule
	path := prefix
	for len(path) > 0 {
		if rules = ctx.Compiler.GetRulesExact(path); rules != nil {
			return evalRefRule(ctx, ref, path, rules, iter)
		}
		path = path[:len(path)-1]
	}

	if len(prefix) == len(ref) {
		return evalRefRecGround(ctx, ref, prefix, iter)
	}

	return evalRefRecNonGround(ctx, ref, prefix, iter)
}

// evalRefRecGround evaluates the ground reference prefix. The reference is
// processed to decide whether evaluation should continue. If the reference
// refers to one or more virtual documents, then all of the referenced documents
// (i.e., base and virtual documents) are merged and the ref is bound to the
// result before continuing.
func evalRefRecGround(ctx *Context, ref, prefix ast.Ref, iter Iterator) error {

	doc, readErr := ctx.Store.Read(ctx.txn, prefix)
	if readErr != nil {
		if !storage.IsNotFound(readErr) {
			return readErr
		}
	}

	node := ctx.Compiler.RuleTree
	for _, x := range prefix {
		node = node.Children[x.Value]
		if node == nil {
			break
		}
	}

	// If the reference does not refer to any virtual docs, evaluation continues
	// or stops depending on whether the reference is defined for some base doc.
	// The same logic is applied below after attempting to produce virtual
	// documents referred to by the reference.
	if node == nil || node.Size() == 0 {
		if storage.IsNotFound(readErr) {
			return nil
		}
		return iter(ctx)
	}

	vdoc, err := evalRefRecTree(ctx, prefix, node)
	if err != nil {
		return err
	}

	if vdoc == nil {
		if storage.IsNotFound(readErr) {
			return nil
		}
		return iter(ctx)
	}

	// The reference is defined for one or more virtual documents. Now merge the
	// virtual and base documents together (if they exist) and continue.
	result := vdoc
	if readErr == nil {

		v, err := ast.InterfaceToValue(doc)
		if err != nil {
			return err
		}

		// It should not be possible for the cast or merge to fail. The cast
		// cannot fail because by definition, the document must be an object, as
		// there are rules that have been evaluated and rules cannot be defined
		// inside arrays. The merge cannot fail either, because that would
		// indicate a conflict between a base document and a virtual document.
		//
		// TODO(tsandall): confirm that we have guards to prevent base and
		// virtual documents from conflicting with each other.
		result, _ = v.(ast.Object).Merge(result)
	}

	return Continue(ctx, ref, result, iter)
}

// evalRefRecTree evaluates the rules found in the leaves of the tree. For each
// non-leaf node in the tree, the results are merged together to form an object.
// The final result is the object representing the virtual document rooted at
// node.
func evalRefRecTree(ctx *Context, path ast.Ref, node *ast.RuleTreeNode) (ast.Object, error) {
	var v ast.Object

	for _, c := range node.Children {
		path = append(path, &ast.Term{Value: c.Key})
		if len(c.Rules) > 0 {
			var result ast.Value
			err := evalRefRule(ctx, path, path, c.Rules, func(ctx *Context) error {
				result = ctx.Binding(path)
				return nil
			})
			if err != nil {
				// TODO(tsandall): consider treating unbound globals as undefined.
				return nil, err
			}
			if result == nil {
				continue
			}
			key := path[len(path)-1]
			val := &ast.Term{Value: result}
			obj := ast.Object{ast.Item(key, val)}
			if v == nil {
				v = ast.Object{}
			}
			v, _ = v.Merge(obj)
		} else {
			result, err := evalRefRecTree(ctx, path, c)
			if err != nil {
				return nil, err
			}
			if result == nil {
				continue
			}
			key := &ast.Term{Value: c.Key}
			val := &ast.Term{Value: result}
			obj := ast.Object{ast.Item(key, val)}
			if v == nil {
				v = ast.Object{}
			}
			v, _ = v.Merge(obj)
		}
		path = path[:len(path)-1]
	}

	return v, nil
}

// evalRefRecNonGround processes the non-ground reference ref. The reference
// is processed by enumerating values that may be used as keys for the next
// (variable) term in the reference and then recursing on the reference.
func evalRefRecNonGround(ctx *Context, ref, prefix ast.Ref, iter Iterator) error {

	// Keep track of keys visited. The reference may refer to both virtual and
	// base documents or virtual documents produced by disjunctive rules. In
	// either case, we only want to visit each unique key once.
	visited := map[ast.Value]struct{}{}

	variable := ref[len(prefix)].Value

	doc, err := ctx.Store.Read(ctx.txn, prefix)
	if err != nil {
		if !storage.IsNotFound(err) {
			return err
		}
	}

	if err == nil {
		switch doc := doc.(type) {
		case map[string]interface{}:
			for k := range doc {
				key := ast.String(k)
				if _, ok := visited[key]; ok {
					continue
				}
				undo := ctx.Bind(variable, key, nil)
				err := evalRefRec(ctx, ref, iter)
				ctx.Unbind(undo)
				if err != nil {
					return err
				}
				visited[key] = struct{}{}
			}
		case []interface{}:
			for idx := range doc {
				undo := ctx.Bind(variable, ast.Number(idx), nil)
				err := evalRefRec(ctx, ref, iter)
				ctx.Unbind(undo)
				if err != nil {
					return err
				}
			}
			return nil
		default:
			return nil
		}
	}

	node := ctx.Compiler.ModuleTree
	for _, x := range prefix {
		node = node.Children[x.Value]
		if node == nil {
			return nil
		}
	}

	for _, mod := range node.Modules {
		for _, rule := range mod.Rules {
			key := ast.String(rule.Name)
			if _, ok := visited[key]; ok {
				continue
			}
			undo := ctx.Bind(variable, key, nil)
			err := evalRefRec(ctx, ref, iter)
			ctx.Unbind(undo)
			if err != nil {
				return err
			}
			visited[key] = struct{}{}
		}
	}

	for child := range node.Children {
		key := child.(ast.String)
		if _, ok := visited[key]; ok {
			continue
		}
		undo := ctx.Bind(variable, key, nil)
		err := evalRefRec(ctx, ref, iter)
		ctx.Unbind(undo)
		if err != nil {
			return err
		}
		visited[key] = struct{}{}
	}

	return nil
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
		for i, rule := range rules {
			err := evalRefRulePartialObjectDoc(ctx, ref, path, rule, i > 0, iter)
			if err != nil {
				return err
			}
		}

	case ast.PartialSetDoc:
		if len(suffix) == 0 {
			return fmt.Errorf("not implemented: full evaluation of virtual set documents: %v", ref)
		}
		for i, rule := range rules {
			err := evalRefRulePartialSetDoc(ctx, ref, path, rule, i > 0, iter)
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

	for i, rule := range rules {

		bindings := ast.NewValueMap()
		child := ctx.Child(rule.Body, bindings)
		if i == 0 {
			child.traceEnter(rule)
		} else {
			child.traceRedo(rule)
		}

		err := evalContext(child, func(child *Context) error {
			if result == nil {
				result = PlugValue(rule.Value.Value, child.Binding)
			} else {
				r := PlugValue(rule.Value.Value, child.Binding)
				if !result.Equal(r) {
					return conflictErr(ref, "complete documents", rule)
				}
			}
			child.traceExit(rule)
			child.traceRedo(rule)
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

func evalRefRulePartialObjectDoc(ctx *Context, ref ast.Ref, path ast.Ref, rule *ast.Rule, redo bool, iter Iterator) error {
	suffix := ref[len(path):]

	key := PlugValue(suffix[0].Value, ctx.Binding)

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
		child := ctx.Child(rule.Body, ast.NewValueMap())
		if redo {
			child.traceRedo(rule)
		} else {
			child.traceEnter(rule)
		}
		return evalContext(child, func(child *Context) error {

			key := PlugValue(rule.Key.Value, child.Binding)

			if r, ok := key.(ast.Ref); ok {
				var err error
				key, err = lookupValue(ctx, r)
				if err != nil {
					if storage.IsNotFound(err) {
						return nil
					}
					return err
				}
			}

			if !ast.IsScalar(key) {
				return typeErrObjectKey(rule, key)
			}

			value := PlugValue(rule.Value.Value, child.Binding)
			if !value.IsGround() {
				return fmt.Errorf("unbound variable: %v", value)
			}

			child.traceExit(rule)

			undo := ctx.Bind(suffix[0].Value.(ast.Var), key, nil)
			err := evalRefRuleResult(ctx, ref, ref[len(path)+1:], value, iter)
			ctx.Unbind(undo)
			child.traceRedo(rule)
			return err
		})
	}

	// Check if the rule has already been evaluated with this key. If it has,
	// proceed with the cached value. Otherwise, evaluate the rule and update
	// the cache.
	if docs, ok := ctx.cache.partialobjs[rule]; ok {
		if r, ok := key.(ast.Ref); ok {
			var err error
			key, err = lookupValue(ctx, r)
			if err != nil {
				if storage.IsNotFound(err) {
					return nil
				}
				return err
			}
		}
		if !ast.IsScalar(key) {
			return typeErrObjectKey(rule, key)
		}
		if doc, ok := docs[key]; ok {
			return evalRefRuleResult(ctx, ref, ref[len(path)+1:], doc, iter)
		}
	}

	child := ctx.Child(rule.Body, ast.NewValueMap())

	_, err := evalEqUnify(child, key, rule.Key.Value, nil, func(child *Context) error {

		if redo {
			child.traceRedo(rule)
		} else {
			child.traceEnter(rule)
		}

		return evalContext(child, func(child *Context) error {

			value := PlugValue(rule.Value.Value, child.Binding)
			if !value.IsGround() {
				return fmt.Errorf("unbound variable: %v", value)
			}

			cache, ok := ctx.cache.partialobjs[rule]
			if !ok {
				cache = map[ast.Value]ast.Value{}
				ctx.cache.partialobjs[rule] = cache
			}

			if r, ok := key.(ast.Ref); ok {
				var err error
				key, err = lookupValue(ctx, r)
				if err != nil {
					if storage.IsNotFound(err) {
						return nil
					}
					return err
				}
			}

			if !ast.IsScalar(key) {
				return typeErrObjectKey(rule, key)
			}

			cache[key] = value

			child.traceExit(rule)

			err := evalRefRuleResult(ctx, ref, ref[len(path)+1:], value.(ast.Value), iter)
			if err != nil {
				return err
			}

			child.traceRedo(rule)
			return nil
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

	for i, rule := range rules {

		bindings := ast.NewValueMap()
		child := ctx.Child(rule.Body, bindings)
		if i == 0 {
			child.traceEnter(rule)
		} else {
			child.traceRedo(rule)
		}

		err := evalContext(child, func(child *Context) error {
			key := PlugValue(rule.Key.Value, child.Binding)
			if r, ok := key.(ast.Ref); ok {
				var err error
				key, err = lookupValue(ctx, r)
				if err != nil {
					if storage.IsNotFound(err) {
						return nil
					}
					return err
				}
			}
			if !ast.IsScalar(key) {
				return typeErrObjectKey(rule, key)
			}
			if _, ok := keys.Get(key); ok {
				return conflictErr(ref, "object document keys", rule)
			}
			keys.Put(key, ast.Null{})
			value := PlugValue(rule.Value.Value, child.Binding)
			if !value.IsGround() {
				return fmt.Errorf("unbound variable: %v", value)
			}
			result = append(result, ast.Item(&ast.Term{Value: key}, &ast.Term{Value: value}))
			child.traceExit(rule)
			child.traceRedo(rule)
			return nil
		})

		if err != nil {
			return err
		}
	}

	return Continue(ctx, ref, result, iter)
}

func evalRefRulePartialSetDoc(ctx *Context, ref ast.Ref, path ast.Ref, rule *ast.Rule, redo bool, iter Iterator) error {

	suffix := ref[len(path):]
	key := PlugValue(suffix[0].Value, ctx.Binding)

	// See comment in evalRefRulePartialObjectDoc about the two branches below.
	if !key.IsGround() {
		child := ctx.Child(rule.Body, ast.NewValueMap())

		if redo {
			child.traceRedo(rule)
		} else {
			child.traceEnter(rule)
		}

		// TODO(tsandall): Currently this evaluates the child query without any
		// bindings from the current context. In cases where the key is partially
		// ground this may be quite inefficient. An optimization would be to unify
		// variables in the child query with ground values in the current context/query.
		// To do this, the unification may need to be improved to namespace variables
		// across contexts (otherwise we could end up with recursive bindings).
		return evalContext(child, func(child *Context) error {
			value := PlugValue(rule.Key.Value, child.Binding)
			if !value.IsGround() {
				return fmt.Errorf("unbound variable: %v", rule.Value)
			}
			child.traceExit(rule)
			undo, err := evalEqUnify(ctx, key, value, nil, func(child *Context) error {
				return Continue(ctx, ref[:len(path)+1], ast.Boolean(true), iter)
			})

			if err != nil {
				return err
			}
			ctx.Unbind(undo)
			child.traceRedo(rule)
			return nil
		})
	}

	child := ctx.Child(rule.Body, ast.NewValueMap())

	_, err := evalEqUnify(child, key, rule.Key.Value, nil, func(child *Context) error {
		if redo {
			child.traceRedo(rule)
		} else {
			child.traceEnter(rule)
		}
		return evalContext(child, func(child *Context) error {
			child.traceExit(rule)
			err := Continue(ctx, ref[:len(path)+1], ast.Boolean(true), iter)
			if err != nil {
				return err
			}
			child.traceRedo(rule)
			return nil
		})
	})

	return err
}

func evalRefRuleResult(ctx *Context, ref ast.Ref, suffix ast.Ref, result ast.Value, iter Iterator) error {

	s := make(ast.Ref, len(suffix))
	for i := range suffix {
		s[i] = PlugTerm(suffix[i], ctx.Binding)
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
	return evalRefRec(ctx, b, func(ctx *Context) error {
		return iter(ctx, PlugValue(b, ctx.Binding))
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
		t1 := PlugTerm(ts[1], ctx.Binding)
		t2 := PlugTerm(ts[2], ctx.Binding)
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
			r = append(r, PlugTerm(comp.Term, c.Binding))
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
		plugged := PlugTerm(nonIndexed, ctx.Binding)
		nonIndexedValue, err := ValueToInterface(plugged.Value, ctx)
		if err != nil {
			return err
		}

		// Iterate the bindings for the indexed term that when applied to the reference
		// would locate the non-indexed value obtained above.
		return ctx.Store.Index(ctx.txn, indexed, nonIndexedValue, func(bindings *ast.ValueMap) error {
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
	r := ctx.Compiler.GetRulesExact(tmp)
	if r != nil {
		return false, nil
	}

	for _, p := range ref[2:] {

		if !p.Value.IsGround() {
			break
		}

		tmp = append(tmp, p)
		r := ctx.Compiler.GetRulesExact(tmp)
		if r != nil {
			return false, nil
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

func lookupValue(ctx *Context, ref ast.Ref) (ast.Value, error) {
	r, err := ctx.Store.Read(ctx.txn, ref)
	if err != nil {
		return nil, err
	}
	return ast.InterfaceToValue(r)
}

func queryPartialSetDoc(params *QueryParams, rules []*ast.Rule) (interface{}, error) {
	result := []interface{}{}
	for _, rule := range rules {
		ctx := params.NewContext(rule.Body)
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
