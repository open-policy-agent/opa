// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/pkg/errors"
)

// Topdown stores the state of the evaluation process and contains context
// needed to evaluate queries.
type Topdown struct {
	Query    ast.Body
	Compiler *ast.Compiler
	Input    ast.Value
	Index    int
	Previous *Topdown
	Store    *storage.Storage
	Tracer   Tracer
	Context  context.Context

	txn    storage.Transaction
	locals *ast.ValueMap
	refs   *valueMapStack
	cache  *contextcache
	qid    uint64
	redos  *redoStack
}

// ResetQueryIDs resets the query ID generator. This is only for test purposes.
func ResetQueryIDs() {
	qidFactory.Reset()
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
	t   *Topdown
	evt *Event
}

// New returns a new Topdown object without any bindings.
func New(ctx context.Context, query ast.Body, compiler *ast.Compiler, store *storage.Storage, txn storage.Transaction) *Topdown {
	return &Topdown{
		Context:  ctx,
		Query:    query,
		Compiler: compiler,
		Store:    store,
		refs:     newValueMapStack(),
		txn:      txn,
		cache:    newContextCache(),
		qid:      qidFactory.Next(),
		redos:    &redoStack{},
	}
}

// Vars represents a set of var bindings.
type Vars map[ast.Var]ast.Value

// Diff returns the var bindings in vs that are not in other.
func (vs Vars) Diff(other Vars) Vars {
	result := Vars{}
	for k := range vs {
		if _, ok := other[k]; !ok {
			result[k] = vs[k]
		}
	}
	return result
}

// Equal returns true if vs is equal to other.
func (vs Vars) Equal(other Vars) bool {
	return len(vs.Diff(other)) == 0 && len(other.Diff(vs)) == 0
}

// Vars returns bindings for the vars in the current query
func (t *Topdown) Vars() map[ast.Var]ast.Value {
	result := map[ast.Var]ast.Value{}
	t.locals.Iter(func(k, v ast.Value) bool {
		if k, ok := k.(ast.Var); ok {
			result[k] = v
		}
		return false
	})
	return result
}

// Binding returns the value bound to the given key.
func (t *Topdown) Binding(k ast.Value) ast.Value {
	if _, ok := k.(ast.Ref); ok {
		return t.refs.Binding(k)
	}
	return t.locals.Get(k)
}

// Undo represents a binding that can be undone.
type Undo struct {
	Key   ast.Value
	Value ast.Value
	Prev  *Undo
}

// Bind updates t to include a binding from the key to the value. The return
// value is used to return t to the state before the binding was added.
func (t *Topdown) Bind(key ast.Value, value ast.Value, prev *Undo) *Undo {
	if _, ok := key.(ast.Ref); ok {
		return t.refs.Bind(key, value, prev)
	}
	o := t.locals.Get(key)
	if t.locals == nil {
		t.locals = ast.NewValueMap()
	}
	t.locals.Put(key, value)
	return &Undo{key, o, prev}
}

// Unbind updates t by removing the binding represented by the undo.
func (t *Topdown) Unbind(undo *Undo) {
	if undo == nil {
		return
	}
	if _, ok := undo.Key.(ast.Ref); ok {
		for u := undo; u != nil; u = u.Prev {
			t.refs.Unbind(u)
		}
	} else {
		for u := undo; u != nil; u = u.Prev {
			if u.Value != nil {
				t.locals.Put(u.Key, u.Value)
			} else {
				t.locals.Delete(u.Key)
			}
		}
	}
}

// Closure returns a new Topdown object to evaluate query with bindings from t.
func (t *Topdown) Closure(query ast.Body) *Topdown {
	cpy := *t
	cpy.Query = query
	cpy.Previous = t
	cpy.Index = 0
	cpy.qid = qidFactory.Next()
	return &cpy
}

// Child returns a new Topdown object to evaluate query without bindings from t.
func (t *Topdown) Child(query ast.Body) *Topdown {
	cpy := t.Closure(query)
	cpy.locals = nil
	cpy.refs = newValueMapStack()
	return cpy
}

// Current returns the current expression to evaluate.
func (t *Topdown) Current() *ast.Expr {
	return t.Query[t.Index]
}

// Resolve returns the native Go value referred to by the ref.
func (t *Topdown) Resolve(ref ast.Ref) (interface{}, error) {

	if ref.IsNested() {
		cpy := make(ast.Ref, len(ref))
		for i := range ref {
			switch v := ref[i].Value.(type) {
			case ast.Ref:
				r, err := lookupValue(t, v)
				if err != nil {
					return nil, err
				}
				cpy[i] = ast.NewTerm(r)
			default:
				cpy[i] = ref[i]
			}
		}
		ref = cpy
	}

	path, err := storage.NewPathForRef(ref)
	if err != nil {
		return nil, err
	}

	return t.Store.Read(t.Context, t.txn, path)
}

// Step returns a new Topdown object to evaluate the next expression.
func (t *Topdown) Step() *Topdown {
	cpy := *t
	cpy.Index++
	return &cpy
}

// WithInput returns a new Topdown object that has the input document set.
func (t *Topdown) WithInput(input ast.Value) *Topdown {
	cpy := *t
	cpy.Input = input
	return &cpy
}

// WithTracer returns a new Topdown object that has a tracer set.
func (t *Topdown) WithTracer(tracer Tracer) *Topdown {
	cpy := *t
	cpy.Tracer = tracer
	return &cpy
}

func (t *Topdown) traceEnter(node interface{}) {
	if t.tracingEnabled() {
		evt := t.makeEvent(EnterOp, node)
		t.flushRedos(evt)
		t.Tracer.Trace(t, evt)
	}
}

func (t *Topdown) traceExit(node interface{}) {
	if t.tracingEnabled() {
		evt := t.makeEvent(ExitOp, node)
		t.flushRedos(evt)
		t.Tracer.Trace(t, evt)
	}
}

func (t *Topdown) traceEval(node interface{}) {
	if t.tracingEnabled() {
		evt := t.makeEvent(EvalOp, node)
		t.flushRedos(evt)
		t.Tracer.Trace(t, evt)
	}
}

func (t *Topdown) traceRedo(node interface{}) {
	if t.tracingEnabled() {
		evt := t.makeEvent(RedoOp, node)
		t.saveRedo(evt)
	}
}

func (t *Topdown) traceFail(node interface{}) {
	if t.tracingEnabled() {
		evt := t.makeEvent(FailOp, node)
		t.flushRedos(evt)
		t.Tracer.Trace(t, evt)
	}
}

func (t *Topdown) tracingEnabled() bool {
	return t.Tracer != nil && t.Tracer.Enabled()
}

func (t *Topdown) saveRedo(evt *Event) {

	buf := &redoStackElement{
		t:   t,
		evt: evt,
	}

	// Search stack for redo that this (redo) event should follow.
	for len(t.redos.events) > 0 {
		idx := len(t.redos.events) - 1
		top := t.redos.events[idx]

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
		t.redos.events = t.redos.events[:idx]
	}

	t.redos.events = append(t.redos.events, buf)
}

func (t *Topdown) flushRedos(evt *Event) {

	idx := len(t.redos.events) - 1

	if idx != -1 {
		top := t.redos.events[idx]

		if top.evt.QueryID == evt.QueryID {
			for _, buf := range t.redos.events {
				t.Tracer.Trace(buf.t, buf.evt)
			}
		}

		t.redos.events = nil
	}

}

func (t *Topdown) makeEvent(op Op, node interface{}) *Event {
	evt := Event{
		Op:      op,
		Node:    node,
		QueryID: t.qid,
		Locals:  t.locals.Copy(),
	}
	if t.Previous != nil {
		evt.ParentID = t.Previous.qid
	}
	return &evt
}

// contextcache stores the result of rule evaluation for a query. The
// contextcache is inherited by child contexts. The contextcache is consulted
// when virtual document references are evaluated. If a miss occurs, the virtual
// document is generated and the contextcache is updated.
type contextcache struct {
	partialobjs partialObjDocCache
	complete    completeDocCache
}

type partialObjDocCache map[*ast.Rule]map[ast.Value]ast.Value
type completeDocCache map[*ast.Rule]ast.Value

func newContextCache() *contextcache {
	return &contextcache{
		partialobjs: partialObjDocCache{},
		complete:    completeDocCache{},
	}
}

func (c *contextcache) Invalidate() {
	c.partialobjs = partialObjDocCache{}
	c.complete = completeDocCache{}
}

// Iterator is the interface for processing evaluation results.
type Iterator func(*Topdown) error

// Continue binds key to value in t and calls the iterator. This is a helper
// function for simple cases where a single value (e.g., a variable) needs to be
// bound to a value in order for the evaluation the proceed.
func Continue(t *Topdown, key, value ast.Value, iter Iterator) error {
	undo := t.Bind(key, value, nil)
	err := iter(t)
	t.Unbind(undo)
	return err
}

// ContinueN binds N keys to N values. The key/value pairs are passed in as
// alternating pairs, e.g., key-1, value-1, key-2, value-2, ..., key-N, value-N.
func ContinueN(t *Topdown, iter Iterator, x ...ast.Value) error {
	var prev *Undo
	for i := 0; i < len(x)/2; i++ {
		offset := i * 2
		prev = t.Bind(x[offset], x[offset+1], prev)
	}
	err := iter(t)
	t.Unbind(prev)
	return err
}

// Eval evaluates the query in t and calls iter once for each set of bindings
// that satisfy all of the expressions in the query.
func Eval(t *Topdown, iter Iterator) error {
	t.traceEnter(t.Query)
	return eval(t, func(t *Topdown) error {
		t.traceExit(t.Query)
		if err := iter(t); err != nil {
			return err
		}
		t.traceRedo(t.Query)
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
		plugged := *term
		plugged.Value = PlugValue(v, binding)
		return &plugged

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

	case *ast.Set:
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
			return PlugValue(b, binding)
		}
		buf := make(ast.Ref, len(v))
		buf[0] = v[0]
		for i, p := range v[1:] {
			buf[i+1] = PlugTerm(p, binding)
		}
		if b := binding(buf); b != nil {
			return PlugValue(b, binding)
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

	case *ast.Set:
		buf := &ast.Set{}
		for _, e := range *v {
			buf.Add(PlugTerm(e, binding))
		}
		return buf

	case nil:
		return nil

	default:
		if !v.IsGround() {
			panic(fmt.Sprintf("illegal value: %v", v))
		}
		return v
	}
}

// QueryParams defines input parameters for the query interface.
type QueryParams struct {
	Context     context.Context
	Compiler    *ast.Compiler
	Store       *storage.Storage
	Transaction storage.Transaction
	Input       ast.Value
	Tracer      Tracer
	Path        ast.Ref
}

// NewQueryParams returns a new QueryParams.
func NewQueryParams(ctx context.Context, compiler *ast.Compiler, store *storage.Storage, txn storage.Transaction, input ast.Value, path ast.Ref) *QueryParams {
	return &QueryParams{
		Context:     ctx,
		Compiler:    compiler,
		Store:       store,
		Transaction: txn,
		Input:       input,
		Path:        path,
	}
}

// NewTopdown returns a new Topdown object.
//
// This function will not propagate optional values such as the tracer, input
// document, etc. Those must be set by the caller.
func (q *QueryParams) NewTopdown(query ast.Body) *Topdown {
	return New(q.Context, query, q.Compiler, q.Store, q.Transaction)
}

// QueryResult represents a single query result.
type QueryResult struct {
	Result   interface{}            // Result contains the document referred to by the params Path.
	Bindings map[string]interface{} // Bindings contains values for variables in the params Input.
}

func (qr *QueryResult) String() string {
	return fmt.Sprintf("[%v %v]", qr.Result, qr.Bindings)
}

// QueryResultSet represents a collection of query results.
type QueryResultSet []*QueryResult

// Undefined returns true if the query did not find any results.
func (qrs QueryResultSet) Undefined() bool {
	return len(qrs) == 0
}

// Add inserts a result into the query result set.
func (qrs *QueryResultSet) Add(qr *QueryResult) {
	*qrs = append(*qrs, qr)
}

// Query returns the value of the document referred to by the params' path. If
// the params' input contains non-ground terms, there may be multiple query
// results.
func Query(params *QueryParams) (QueryResultSet, error) {

	t, resultVar, requestVars, err := makeTopdown(params)
	if err != nil {
		return nil, err
	}

	qrs := QueryResultSet{}

	err = Eval(t, func(t *Topdown) error {

		// Gather bindings for vars from the request.
		bindings := map[string]interface{}{}
		for v := range requestVars {
			binding, err := ValueToInterface(PlugValue(v, t.Binding), t)
			if err != nil {
				return err
			}
			bindings[v.String()] = binding
		}

		// Gather binding for result var.
		val, err := ValueToInterface(PlugValue(resultVar, t.Binding), t)
		if err != nil {
			return err
		}

		// Aggregate results.
		qrs.Add(&QueryResult{val, bindings})
		return nil
	})

	return qrs, err
}

func makeTopdown(params *QueryParams) (*Topdown, ast.Var, ast.VarSet, error) {

	inputVar := ast.VarTerm(ast.WildcardPrefix + "0")
	pathVar := ast.VarTerm(ast.WildcardPrefix + "1")

	var query ast.Body

	if params.Input == nil {
		query = ast.NewBody(ast.Equality.Expr(ast.NewTerm(params.Path), pathVar))
	} else {
		// <input> = $0,
		// <path> = $1 with input as $0
		inputExpr := ast.Equality.Expr(ast.NewTerm(params.Input), inputVar)
		pathExpr := ast.Equality.Expr(ast.NewTerm(params.Path), pathVar).
			IncludeWith(ast.NewTerm(ast.InputRootRef), inputVar)
		query = ast.NewBody(inputExpr, pathExpr)
	}

	compiled, err := params.Compiler.QueryCompiler().Compile(query)
	if err != nil {
		return nil, "", nil, err
	}

	vis := ast.NewVarVisitor().WithParams(ast.VarVisitorParams{
		SkipRefHead:  true,
		SkipClosures: true,
	})

	ast.Walk(vis, params.Input)

	t := params.NewTopdown(compiled).
		WithTracer(params.Tracer)

	return t, pathVar.Value.(ast.Var), vis.Vars(), nil
}

// Resolver defines the interface for resolving references to base documents to
// native Go values. The native Go value types map to JSON types.
type Resolver interface {
	Resolve(ref ast.Ref) (value interface{}, err error)
}

type resolver struct {
	context context.Context
	store   *storage.Storage
	txn     storage.Transaction
}

func (r resolver) Resolve(ref ast.Ref) (interface{}, error) {
	path, err := storage.NewPathForRef(ref)
	if err != nil {
		return nil, err
	}
	return r.store.Read(r.context, r.txn, path)
}

// ResolveRefs returns the AST value obtained by resolving references to base
// documents.
func ResolveRefs(v ast.Value, t *Topdown) (ast.Value, error) {
	result, err := ast.TransformRefs(v, func(r ast.Ref) (ast.Value, error) {
		return lookupValue(t, r)
	})
	if err != nil {
		return nil, err
	}
	return result.(ast.Value), nil
}

// ValueToInterface returns the underlying Go value associated with an AST value.
// If the value is a reference, the reference is fetched from storage. Composite
// AST values such as objects and arrays are converted recursively.
func ValueToInterface(v ast.Value, resolver Resolver) (interface{}, error) {
	switch v := v.(type) {
	case ast.Null:
		return nil, nil
	case ast.Boolean:
		return bool(v), nil
	case ast.Number:
		return json.Number(v), nil
	case ast.String:
		return string(v), nil
	case ast.Array:
		buf := []interface{}{}
		for _, x := range v {
			x1, err := ValueToInterface(x.Value, resolver)
			if err != nil {
				return nil, err
			}
			buf = append(buf, x1)
		}
		return buf, nil
	case ast.Object:
		buf := map[string]interface{}{}
		for _, x := range v {
			k, err := ValueToInterface(x[0].Value, resolver)
			if err != nil {
				return nil, err
			}
			asStr, stringKey := k.(string)
			if !stringKey {
				return nil, fmt.Errorf("object key type %T", k)
			}
			v, err := ValueToInterface(x[1].Value, resolver)
			if err != nil {
				return nil, err
			}
			buf[asStr] = v
		}
		return buf, nil
	case *ast.Set:
		buf := []interface{}{}
		for _, x := range *v {
			x1, err := ValueToInterface(x.Value, resolver)
			if err != nil {
				return nil, err
			}
			buf = append(buf, x1)
		}
		return buf, nil
	case ast.Ref:
		return resolver.Resolve(v)
	default:
		return nil, fmt.Errorf("unbound value: %v", v)
	}
}

func eval(t *Topdown, iter Iterator) error {

	if t.Index >= len(t.Query) {
		return iter(t)
	}

	if len(t.Current().With) > 0 {
		return evalWith(t, iter)
	}

	return evalStep(t, func(t *Topdown) error {
		t = t.Step()
		return eval(t, iter)
	})
}

func evalStep(t *Topdown, iter Iterator) error {

	if t.Current().Negated {
		return evalNot(t, iter)
	}

	t.traceEval(t.Current())

	// isRedo indicates if the expression's terms are defined at least once. If
	// any of the terms are undefined, then the closure below will not run (but
	// a Fail event still needs to be emitted).
	isRedo := false

	err := evalTerms(t, func(t *Topdown) error {
		isRedo = true

		// isTrue indicates if the expression is true and is used to determine
		// if a Fail event should be emitted below.
		isTrue := false

		err := evalExpr(t, func(t *Topdown) error {
			isTrue = true
			return iter(t)
		})

		if err != nil {
			return err
		}

		if !isTrue {
			t.traceFail(t.Current())
		}

		t.traceRedo(t.Current())

		return nil
	})

	if err != nil {
		return err
	}

	if !isRedo {
		t.traceFail(t.Current())
	}

	return nil
}

func evalNot(t *Topdown, iter Iterator) error {

	negation := ast.NewBody(t.Current().Complement().NoWith())
	child := t.Closure(negation)

	t.traceEval(t.Current())

	isTrue := false

	err := Eval(child, func(*Topdown) error {
		isTrue = true
		return nil
	})

	if err != nil {
		return err
	}

	if !isTrue {
		return iter(t)
	}

	t.traceFail(t.Current())

	return nil
}

func evalWith(t *Topdown, iter Iterator) error {

	curr := t.Current()
	pairs := make([][2]*ast.Term, len(curr.With))

	for i := range curr.With {
		plugged := PlugTerm(curr.With[i].Value, t.Binding)
		pairs[i] = [...]*ast.Term{curr.With[i].Target, plugged}
	}

	input, err := MakeInput(pairs)
	if err != nil {
		return &Error{
			Code:     ConflictErr,
			Location: curr.Location,
			Message:  err.Error(),
		}
	}

	cpy := t.WithInput(input)

	// All ref bindings added during evaluation of this expression must be
	// discarded before moving to the next expression. Push a new binding map
	// onto the stack that will be popped below before continuing.
	cpy.refs.Push(ast.NewValueMap())

	return evalStep(cpy, func(next *Topdown) error {
		next.refs.Pop()
		// TODO(tsandall): invalidation could be smarter, e.g., only invalidate
		// caches for rules that were evaluated during this expr.
		next.cache.Invalidate()
		next = next.Step()
		return eval(next, iter)
	})
}

func evalExpr(t *Topdown, iter Iterator) error {
	expr := PlugExpr(t.Current(), t.Binding)
	switch tt := expr.Terms.(type) {
	case []*ast.Term:
		builtin, ok := builtinFunctions[tt[0].Value.(ast.Var)]
		if !ok {
			return unsupportedBuiltinErr(expr.Location)
		}
		return builtin(t, expr, iter)
	case *ast.Term:
		v := tt.Value
		if r, ok := v.(ast.Ref); ok {
			var err error
			v, err = lookupValue(t, r)
			if err != nil {
				return err
			}
		}
		if !v.Equal(ast.Boolean(false)) {
			if v.IsGround() {
				return iter(t)
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
func evalRef(t *Topdown, ref, path ast.Ref, iter Iterator) error {

	if len(ref) == 0 {

		if path.HasPrefix(ast.DefaultRootRef) {
			return evalRefRec(t, path, iter)
		}

		if path.HasPrefix(ast.InputRootRef) {
			// If no input was supplied, then any references to the input
			// are undefined.
			if t.Input == nil {
				return nil
			}
			return evalRefRuleResult(t, path, path[1:], t.Input, iter)
		}

		if v := t.Binding(path[0].Value); v != nil {
			return evalRefRuleResult(t, path, path[1:], v, iter)
		}

		// This should not be reachable.
		return fmt.Errorf("unbound ref head: %v", path)
	}

	head, tail := ref[0], ref[1:]
	n, ok := head.Value.(ast.Ref)
	if !ok {
		path = append(path, head)
		return evalRef(t, tail, path, iter)
	}

	return evalRef(t, n, ast.Ref{}, func(t *Topdown) error {

		var undo *Undo

		// Add a binding for the nested reference 'n' if one does not exist. If
		// 'n' referred to a virtual document the binding would already exist.
		// We bind nested references so that when the overall expression is
		// evaluated, it will not contain any nested references.
		if b := t.Binding(n); b == nil {
			var err error
			var v ast.Value
			switch p := PlugValue(n, t.Binding).(type) {
			case ast.Ref:
				v, err = lookupValue(t, p)
				if err != nil {
					return err
				}
			default:
				v = p
			}
			undo = t.Bind(n, v, nil)
		}

		tmp := append(path, head)
		err := evalRef(t, tail, tmp, iter)

		if undo != nil {
			t.Unbind(undo)
		}

		return err
	})
}

func evalRefRec(t *Topdown, ref ast.Ref, iter Iterator) error {

	// Obtain ground prefix of the reference.
	var prefix ast.Ref

	switch v := PlugValue(ref, t.Binding).(type) {
	case ast.Ref:
		// https://github.com/open-policy-agent/opa/issues/238
		// We must set ref to the plugged value here otherwise the ref
		// evaluation doesn't have consistent values for prefix and ref.
		ref = v
		prefix = v.GroundPrefix()
	default:
		// Fast-path? TODO test case.
		return iter(t)
	}

	// Check if the prefix refers to a virtual document.
	var rules []*ast.Rule
	path := prefix
	for len(path) > 0 {
		if rules = t.Compiler.GetRulesExact(path); rules != nil {
			return evalRefRule(t, ref, path, rules, iter)
		}
		path = path[:len(path)-1]
	}

	if len(prefix) == len(ref) {
		return evalRefRecGround(t, ref, prefix, iter)
	}

	return evalRefRecNonGround(t, ref, prefix, iter)
}

// evalRefRecGround evaluates the ground reference prefix. The reference is
// processed to decide whether evaluation should continue. If the reference
// refers to one or more virtual documents, then all of the referenced documents
// (i.e., base and virtual documents) are merged and the ref is bound to the
// result before continuing.
func evalRefRecGround(t *Topdown, ref, prefix ast.Ref, iter Iterator) error {

	doc, readErr := t.Resolve(prefix)
	if readErr != nil {
		if !storage.IsNotFound(readErr) {
			return readErr
		}
	}

	node := t.Compiler.RuleTree
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
		return iter(t)
	}

	vdoc, err := evalRefRecTree(t, prefix, node)
	if err != nil {
		return err
	}

	if vdoc == nil {
		if storage.IsNotFound(readErr) {
			return nil
		}
		return iter(t)
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

	return Continue(t, ref, result, iter)
}

// evalRefRecTree evaluates the rules found in the leaves of the tree. For each
// non-leaf node in the tree, the results are merged together to form an object.
// The final result is the object representing the virtual document rooted at
// node.
func evalRefRecTree(t *Topdown, path ast.Ref, node *ast.RuleTreeNode) (ast.Object, error) {
	var v ast.Object

	for _, c := range node.Children {
		path = append(path, &ast.Term{Value: c.Key})
		if len(c.Rules) > 0 {
			var result ast.Value
			err := evalRefRule(t, path, path, c.Rules, func(t *Topdown) error {
				result = t.Binding(path)
				return nil
			})
			if err != nil {
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
			result, err := evalRefRecTree(t, path, c)
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
func evalRefRecNonGround(t *Topdown, ref, prefix ast.Ref, iter Iterator) error {

	// Keep track of keys visited. The reference may refer to both virtual and
	// base documents or virtual documents produced by disjunctive rules. In
	// either case, we only want to visit each unique key once.
	visited := map[ast.Value]struct{}{}

	variable := ref[len(prefix)].Value

	doc, err := t.Resolve(prefix)
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
				undo := t.Bind(variable, key, nil)
				err := evalRefRec(t, ref, iter)
				t.Unbind(undo)
				if err != nil {
					return err
				}
				visited[key] = struct{}{}
			}
		case []interface{}:
			for idx := range doc {
				undo := t.Bind(variable, ast.IntNumberTerm(idx).Value, nil)
				err := evalRefRec(t, ref, iter)
				t.Unbind(undo)
				if err != nil {
					return err
				}
			}
			return nil
		default:
			return nil
		}
	}

	node := t.Compiler.ModuleTree
	for _, x := range prefix {
		node = node.Children[x.Value]
		if node == nil {
			return nil
		}
	}

	for _, mod := range node.Modules {
		for _, rule := range mod.Rules {
			key := ast.String(rule.Head.Name)
			if _, ok := visited[key]; ok {
				continue
			}
			undo := t.Bind(variable, key, nil)
			err := evalRefRec(t, ref, iter)
			t.Unbind(undo)
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
		undo := t.Bind(variable, key, nil)
		err := evalRefRec(t, ref, iter)
		t.Unbind(undo)
		if err != nil {
			return err
		}
		visited[key] = struct{}{}
	}

	return nil
}

func evalRefRule(t *Topdown, ref ast.Ref, path ast.Ref, rules []*ast.Rule, iter Iterator) error {

	suffix := ref[len(path):]

	switch rules[0].Head.DocKind() {

	case ast.CompleteDoc:
		return evalRefRuleCompleteDoc(t, ref, suffix, rules, iter)

	case ast.PartialObjectDoc:
		if len(suffix) == 0 {
			return evalRefRulePartialObjectDocFull(t, ref, rules, iter)
		}
		for i, rule := range rules {
			err := evalRefRulePartialObjectDoc(t, ref, path, rule, i > 0, iter)
			if err != nil {
				return err
			}
		}

	case ast.PartialSetDoc:
		if len(suffix) == 0 {
			return evalRefRulePartialSetDocFull(t, ref, rules, iter)
		}
		if len(suffix) != 1 {
			return setDereferenceTypeErr(t.Current().Location)
		}
		for i, rule := range rules {
			err := evalRefRulePartialSetDoc(t, ref, path, rule, i > 0, iter)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func evalRefRuleCompleteDoc(t *Topdown, ref ast.Ref, suffix ast.Ref, rules []*ast.Rule, iter Iterator) error {

	// Check if we have cached the result of evaluating this rule set already.
	for _, rule := range rules {
		if doc, ok := t.cache.complete[rule]; ok {
			return evalRefRuleResult(t, ref, suffix, doc, iter)
		}
	}

	var result ast.Value
	var defaultRule *ast.Rule
	var redo bool

	for _, rule := range rules {
		if rule.Default {
			// Compiler guarantees that there is only one default rule per shared name.
			defaultRule = rule
			continue
		}
		next, err := evalRefRuleCompleteDocSingle(t, rule, redo, result)
		if err != nil {
			return err
		}
		if next != nil {
			result = next
		}
		redo = true
	}

	if result == nil && defaultRule != nil {
		var err error
		result, err = evalRefRuleCompleteDocSingle(t, defaultRule, redo, nil)
		if err != nil {
			return err
		}
	}

	if result != nil {
		// Add the result to the cache. All of the rules have either produced the same value
		// or only one of them has produced a value. As such, we can cache the result on any
		// of them.
		t.cache.complete[rules[0]] = result
		return evalRefRuleResult(t, ref, suffix, result, iter)
	}

	return nil
}

func evalRefRuleCompleteDocSingle(t *Topdown, rule *ast.Rule, redo bool, last ast.Value) (ast.Value, error) {

	child := t.Child(rule.Body)

	if !redo {
		child.traceEnter(rule)
	} else {
		child.traceRedo(rule)
	}

	var result ast.Value

	err := eval(child, func(child *Topdown) error {

		result = PlugValue(rule.Head.Value.Value, child.Binding)

		// If document is already defined, check for conflict.
		if last != nil {
			if !last.Equal(result) {
				return completeDocConflictErr(t.Current().Location)
			}
		} else {
			last = result
		}

		child.traceExit(rule)
		child.traceRedo(rule)
		return nil
	})

	return result, err
}

func evalRefRulePartialObjectDoc(t *Topdown, ref ast.Ref, path ast.Ref, rule *ast.Rule, redo bool, iter Iterator) error {
	suffix := ref[len(path):]

	key := PlugValue(suffix[0].Value, t.Binding)

	// There are two cases being handled below. The first deals with non-ground
	// keys. If the key is not ground, we evaluate the child query and copy the
	// key binding from the child into t. The second deals with ground keys. In
	// that case, we initialize the child with a key binding and evaluate the
	// child query. This reduces the amount of processing the child query has to
	// do.
	//
	// In the first case, we do not unify the keys because the unification does
	// not namespace variables within their context. As a result, we could end
	// up with a recursive binding if we unified "key" with "rule.Head.Key.Value". If
	// unification is improved to handle namespacing, this can be revisited.
	if !key.IsGround() {
		child := t.Child(rule.Body)
		if redo {
			child.traceRedo(rule)
		} else {
			child.traceEnter(rule)
		}
		return eval(child, func(child *Topdown) error {

			key := PlugValue(rule.Head.Key.Value, child.Binding)

			if r, ok := key.(ast.Ref); ok {
				var err error
				key, err = lookupValue(t, r)
				if err != nil {
					if storage.IsNotFound(err) {
						return nil
					}
					return err
				}
			}

			if !ast.IsScalar(key) {
				return objectDocKeyTypeErr(t.Current().Location)
			}

			value := PlugValue(rule.Head.Value.Value, child.Binding)
			if !value.IsGround() {
				return fmt.Errorf("unbound variable: %v", value)
			}

			child.traceExit(rule)

			undo := t.Bind(suffix[0].Value.(ast.Var), key, nil)
			err := evalRefRuleResult(t, ref, ref[len(path)+1:], value, iter)
			t.Unbind(undo)
			child.traceRedo(rule)
			return err
		})
	}

	// Check if the rule has already been evaluated with this key. If it has,
	// proceed with the cached value. Otherwise, evaluate the rule and update
	// the cache.
	if docs, ok := t.cache.partialobjs[rule]; ok {
		if r, ok := key.(ast.Ref); ok {
			var err error
			key, err = lookupValue(t, r)
			if err != nil {
				if storage.IsNotFound(err) {
					return nil
				}
				return err
			}
		}
		if !ast.IsScalar(key) {
			return objectDocKeyTypeErr(t.Current().Location)
		}
		if doc, ok := docs[key]; ok {
			return evalRefRuleResult(t, ref, ref[len(path)+1:], doc, iter)
		}
	}

	child := t.Child(rule.Body)

	_, err := evalEqUnify(child, key, rule.Head.Key.Value, nil, func(child *Topdown) error {

		if redo {
			child.traceRedo(rule)
		} else {
			child.traceEnter(rule)
		}

		return eval(child, func(child *Topdown) error {

			value := PlugValue(rule.Head.Value.Value, child.Binding)
			if !value.IsGround() {
				return fmt.Errorf("unbound variable: %v", value)
			}

			cache, ok := t.cache.partialobjs[rule]
			if !ok {
				cache = map[ast.Value]ast.Value{}
				t.cache.partialobjs[rule] = cache
			}

			if r, ok := key.(ast.Ref); ok {
				var err error
				key, err = lookupValue(t, r)
				if err != nil {
					if storage.IsNotFound(err) {
						return nil
					}
					return err
				}
			}

			if !ast.IsScalar(key) {
				return objectDocKeyTypeErr(t.Current().Location)
			}

			cache[key] = value

			child.traceExit(rule)

			err := evalRefRuleResult(t, ref, ref[len(path)+1:], value.(ast.Value), iter)
			if err != nil {
				return err
			}

			child.traceRedo(rule)
			return nil
		})
	})

	return err

}

func evalRefRulePartialObjectDocFull(t *Topdown, ref ast.Ref, rules []*ast.Rule, iter Iterator) error {

	var result ast.Object
	keys := ast.NewValueMap()

	for i, rule := range rules {

		child := t.Child(rule.Body)

		if i == 0 {
			child.traceEnter(rule)
		} else {
			child.traceRedo(rule)
		}

		err := eval(child, func(child *Topdown) error {

			key := PlugValue(rule.Head.Key.Value, child.Binding)

			if r, ok := key.(ast.Ref); ok {
				var err error
				key, err = lookupValue(t, r)
				if err != nil {
					if storage.IsNotFound(err) {
						return nil
					}
					return err
				}
			}

			if !ast.IsScalar(key) {
				return objectDocKeyTypeErr(t.Current().Location)
			}

			value := PlugValue(rule.Head.Value.Value, child.Binding)

			if !value.IsGround() {
				return fmt.Errorf("unbound variable: %v", value)
			}

			if exist := keys.Get(key); exist != nil && !exist.Equal(value) {
				return objectDocKeyConflictErr(t.Current().Location)
			}

			keys.Put(key, value)

			result = append(result, ast.Item(&ast.Term{Value: key}, &ast.Term{Value: value}))
			child.traceExit(rule)
			child.traceRedo(rule)
			return nil
		})

		if err != nil {
			return err
		}
	}

	return Continue(t, ref, result, iter)
}

func evalRefRulePartialSetDoc(t *Topdown, ref ast.Ref, path ast.Ref, rule *ast.Rule, redo bool, iter Iterator) error {

	suffix := ref[len(path):]
	key := PlugValue(suffix[0].Value, t.Binding)

	// See comment in evalRefRulePartialObjectDoc about the two branches below.
	if !key.IsGround() {

		child := t.Child(rule.Body)

		if redo {
			child.traceRedo(rule)
		} else {
			child.traceEnter(rule)
		}

		// TODO(tsandall): Currently this evaluates the child query without any
		// bindings from t. In cases where the key is partially ground this may
		// be quite inefficient. An optimization would be to unify variables in
		// the child query with ground values in the current context/query. To
		// do this, the unification may need to be improved to namespace
		// variables across contexts (otherwise we could end up with recursive
		// bindings).
		return eval(child, func(child *Topdown) error {
			value := PlugValue(rule.Head.Key.Value, child.Binding)
			if !value.IsGround() {
				return fmt.Errorf("unbound variable: %v", rule.Head.Value)
			}
			child.traceExit(rule)
			undo, err := evalEqUnify(t, key, value, nil, func(child *Topdown) error {
				return Continue(t, ref[:len(path)+1], ast.Boolean(true), iter)
			})

			if err != nil {
				return err
			}
			t.Unbind(undo)
			child.traceRedo(rule)
			return nil
		})
	}

	child := t.Child(rule.Body)

	_, err := evalEqUnify(child, key, rule.Head.Key.Value, nil, func(child *Topdown) error {
		if redo {
			child.traceRedo(rule)
		} else {
			child.traceEnter(rule)
		}
		return eval(child, func(child *Topdown) error {
			child.traceExit(rule)
			err := Continue(t, ref[:len(path)+1], ast.Boolean(true), iter)
			if err != nil {
				return err
			}
			child.traceRedo(rule)
			return nil
		})
	})

	return err
}

func evalRefRulePartialSetDocFull(t *Topdown, ref ast.Ref, rules []*ast.Rule, iter Iterator) error {

	result := &ast.Set{}

	for i, rule := range rules {

		child := t.Child(rule.Body)

		if i == 0 {
			child.traceEnter(rule)
		} else {
			child.traceRedo(rule)
		}

		err := eval(child, func(child *Topdown) error {
			value := PlugValue(rule.Head.Key.Value, child.Binding)
			result.Add(&ast.Term{Value: value})
			child.traceExit(rule)
			child.traceRedo(rule)
			return nil
		})

		if err != nil {
			return err
		}
	}

	return Continue(t, ref, result, iter)
}

func evalRefRuleResult(t *Topdown, ref ast.Ref, suffix ast.Ref, result ast.Value, iter Iterator) error {

	s := make(ast.Ref, len(suffix))

	for i := range suffix {
		s[i] = PlugTerm(suffix[i], t.Binding)
	}

	return evalRefRuleResultRec(t, result, s, ast.Ref{}, func(t *Topdown, v ast.Value) error {
		// Must add binding with plugged value of ref in case ref contains
		// suffix with one or more vars.
		// Test case: "input: object dereference ground 2" exercises this.
		return Continue(t, PlugValue(ref, t.Binding), v, iter)
	})
}

func evalRefRuleResultRec(t *Topdown, v ast.Value, ref, path ast.Ref, iter func(*Topdown, ast.Value) error) error {

	if len(ref) == 0 {
		return iter(t, v)
	}

	switch v := v.(type) {
	case ast.Array:
		return evalRefRuleResultRecArray(t, v, ref, path, iter)
	case *ast.Set:
		return evalRefRuleResultRecSet(t, v, ref, path, iter)
	case ast.Object:
		return evalRefRuleResultRecObject(t, v, ref, path, iter)
	case ast.Ref:
		return evalRefRuleResultRecRef(t, v, ref, path, iter)
	}

	return nil
}

func evalRefRuleResultRecArray(t *Topdown, arr ast.Array, ref, path ast.Ref, iter func(*Topdown, ast.Value) error) error {
	head, tail := ref[0], ref[1:]
	switch n := head.Value.(type) {
	case ast.Number:
		idx, ok := n.Int()
		if !ok || idx < 0 {
			return nil
		}
		if idx >= len(arr) {
			return nil
		}
		el := arr[idx]
		path = append(path, head)
		return evalRefRuleResultRec(t, el.Value, tail, path, iter)
	case ast.Var:
		for i := range arr {
			idx := ast.IntNumberTerm(i)
			undo := t.Bind(n, idx.Value, nil)
			path = append(path, idx)
			if err := evalRefRuleResultRec(t, arr[i].Value, tail, path, iter); err != nil {
				return err
			}
			t.Unbind(undo)
			path = path[:len(path)-1]
		}
	}
	return nil
}

func evalRefRuleResultRecObject(t *Topdown, obj ast.Object, ref, path ast.Ref, iter func(*Topdown, ast.Value) error) error {
	head, tail := ref[0], ref[1:]
	switch k := head.Value.(type) {
	case ast.String:
		match := -1
		for idx, i := range obj {
			x := i[0].Value
			if r, ok := i[0].Value.(ast.Ref); ok {
				var err error
				if x, err = lookupValue(t, r); err != nil {
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
		return evalRefRuleResultRec(t, obj[match][1].Value, tail, path, iter)
	case ast.Var:
		for _, i := range obj {
			undo := t.Bind(k, i[0].Value, nil)
			path = append(path, i[0])
			if err := evalRefRuleResultRec(t, i[1].Value, tail, path, iter); err != nil {
				return err
			}
			t.Unbind(undo)
			path = path[:len(path)-1]
		}
	}
	return nil
}

func evalRefRuleResultRecRef(t *Topdown, v, ref, path ast.Ref, iter func(*Topdown, ast.Value) error) error {

	b := append(v, ref...)

	return evalRefRec(t, b, func(t *Topdown) error {
		return iter(t, PlugValue(b, t.Binding))
	})
}

func evalRefRuleResultRecSet(t *Topdown, set *ast.Set, ref, suffix ast.Ref, iter func(*Topdown, ast.Value) error) error {
	head, tail := ref[0], ref[1:]
	if len(tail) > 0 {
		return nil
	}
	switch k := head.Value.(type) {
	case ast.Var:
		for _, e := range *set {
			undo := t.Bind(k, e.Value, nil)
			err := iter(t, ast.Boolean(true))
			if err != nil {
				return err
			}
			t.Unbind(undo)
		}
		return nil
	default:
		// Set lookup requires that nested references be resolved to their
		// values. In some cases this will be expensive, so it may have to be
		// revisited.
		resolved, err := ResolveRefs(set, t)
		if err != nil {
			return err
		}

		rset := resolved.(*ast.Set)
		rval, err := ResolveRefs(k, t)
		if err != nil {
			return err
		}

		if rset.Contains(ast.NewTerm(rval)) {
			return iter(t, ast.Boolean(true))
		}
		return nil
	}
}

func evalTerms(t *Topdown, iter Iterator) error {

	expr := t.Current()

	// Attempt to evaluate the terms using indexing. Indexing can be used
	// if this is an equality expression where one side is a non-ground,
	// non-nested reference to a base document and the other side is a
	// reference or some ground term. If indexing is available for the terms,
	// the index is built lazily.
	if expr.IsEquality() {

		// The terms must be plugged otherwise the index may yield bindings
		// that would result in false positives.
		ts := expr.Terms.([]*ast.Term)
		t1 := PlugTerm(ts[1], t.Binding)
		t2 := PlugTerm(ts[2], t.Binding)
		r1, _ := t1.Value.(ast.Ref)
		r2, _ := t2.Value.(ast.Ref)

		if indexingAllowed(r1, t2) {
			ok, err := indexBuildLazy(t, r1)
			if err != nil {
				return errors.Wrapf(err, "index build failed on %v", r1)
			}
			if ok {
				return evalTermsIndexed(t, iter, r1, t2)
			}
		}

		if indexingAllowed(r2, t1) {
			ok, err := indexBuildLazy(t, r2)
			if err != nil {
				return errors.Wrapf(err, "index build failed on %v", r2)
			}
			if ok {
				return evalTermsIndexed(t, iter, r2, t1)
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

	return evalTermsRec(t, iter, ts)
}

func evalTermsComprehension(t *Topdown, comp ast.Value, iter Iterator) error {
	switch comp := comp.(type) {
	case *ast.ArrayComprehension:
		result := ast.Array{}
		child := t.Closure(comp.Body)

		err := Eval(child, func(child *Topdown) error {
			result = append(result, PlugTerm(comp.Term, child.Binding))
			return nil
		})

		if err != nil {
			return err
		}

		return Continue(t, comp, result, iter)

	default:
		panic(fmt.Sprintf("illegal argument: %v %v", t, comp))
	}
}

func evalTermsIndexed(t *Topdown, iter Iterator, indexed ast.Ref, nonIndexed *ast.Term) error {

	iterateIndex := func(t *Topdown) error {

		// Evaluate the non-indexed term.
		value, err := ValueToInterface(PlugValue(nonIndexed.Value, t.Binding), t)
		if err != nil {
			return err
		}

		// Iterate the bindings for the indexed term that when applied to the reference
		// would locate the non-indexed value obtained above.
		return t.Store.Index(t.txn, indexed, value, func(bindings *ast.ValueMap) error {
			var prev *Undo

			// We will skip these bindings if the non-indexed term contains a
			// different binding for the same variable. This can arise if output
			// variables in references on either side intersect (e.g., a[i] = g[i][j]).
			skip := bindings.Iter(func(k, v ast.Value) bool {
				if o := t.Binding(k); o != nil && !o.Equal(v) {
					return true
				}
				prev = t.Bind(k, v, prev)
				return false
			})

			var err error

			if !skip {
				err = iter(t)
			}

			t.Unbind(prev)
			return err
		})

	}

	return evalTermsRec(t, iterateIndex, []*ast.Term{nonIndexed})
}

func evalTermsRec(t *Topdown, iter Iterator, ts []*ast.Term) error {

	if len(ts) == 0 {
		return iter(t)
	}

	head := ts[0]
	tail := ts[1:]

	rec := func(t *Topdown) error {
		return evalTermsRec(t, iter, tail)
	}

	switch head := head.Value.(type) {
	case ast.Ref:
		return evalRef(t, head, ast.Ref{}, rec)
	case ast.Array:
		return evalTermsRecArray(t, head, 0, rec)
	case ast.Object:
		return evalTermsRecObject(t, head, 0, rec)
	case *ast.Set:
		return evalTermsRecSet(t, head, 0, rec)
	case *ast.ArrayComprehension:
		return evalTermsComprehension(t, head, rec)
	default:
		return evalTermsRec(t, iter, tail)
	}
}

func evalTermsRecArray(t *Topdown, arr ast.Array, idx int, iter Iterator) error {
	if idx >= len(arr) {
		return iter(t)
	}

	rec := func(t *Topdown) error {
		return evalTermsRecArray(t, arr, idx+1, iter)
	}

	switch v := arr[idx].Value.(type) {
	case ast.Ref:
		return evalRef(t, v, ast.Ref{}, rec)
	case ast.Array:
		return evalTermsRecArray(t, v, 0, rec)
	case ast.Object:
		return evalTermsRecObject(t, v, 0, rec)
	case *ast.Set:
		return evalTermsRecSet(t, v, 0, rec)
	case *ast.ArrayComprehension:
		return evalTermsComprehension(t, v, rec)
	default:
		return evalTermsRecArray(t, arr, idx+1, iter)
	}
}

func evalTermsRecObject(t *Topdown, obj ast.Object, idx int, iter Iterator) error {
	if idx >= len(obj) {
		return iter(t)
	}

	rec := func(t *Topdown) error {
		return evalTermsRecObject(t, obj, idx+1, iter)
	}

	switch k := obj[idx][0].Value.(type) {
	case ast.Ref:
		return evalRef(t, k, ast.Ref{}, func(t *Topdown) error {
			switch v := obj[idx][1].Value.(type) {
			case ast.Ref:
				return evalRef(t, v, ast.Ref{}, rec)
			case ast.Array:
				return evalTermsRecArray(t, v, 0, rec)
			case ast.Object:
				return evalTermsRecObject(t, v, 0, rec)
			case *ast.Set:
				return evalTermsRecSet(t, v, 0, rec)
			case *ast.ArrayComprehension:
				return evalTermsComprehension(t, v, rec)
			default:
				return evalTermsRecObject(t, obj, idx+1, iter)
			}
		})
	default:
		switch v := obj[idx][1].Value.(type) {
		case ast.Ref:
			return evalRef(t, v, ast.Ref{}, rec)
		case ast.Array:
			return evalTermsRecArray(t, v, 0, rec)
		case ast.Object:
			return evalTermsRecObject(t, v, 0, rec)
		case *ast.Set:
			return evalTermsRecSet(t, v, 0, rec)
		case *ast.ArrayComprehension:
			return evalTermsComprehension(t, v, rec)
		default:
			return evalTermsRecObject(t, obj, idx+1, iter)
		}
	}
}

func evalTermsRecSet(t *Topdown, set *ast.Set, idx int, iter Iterator) error {
	if idx >= len(*set) {
		return iter(t)
	}

	rec := func(t *Topdown) error {
		return evalTermsRecSet(t, set, idx+1, iter)
	}

	switch v := (*set)[idx].Value.(type) {
	case ast.Ref:
		return evalRef(t, v, ast.Ref{}, rec)
	case ast.Array:
		return evalTermsRecArray(t, v, 0, rec)
	case ast.Object:
		return evalTermsRecObject(t, v, 0, rec)
	case *ast.ArrayComprehension:
		return evalTermsComprehension(t, v, rec)
	default:
		return evalTermsRecSet(t, set, idx+1, iter)
	}
}

// indexBuildLazy returns true if there is an index built for this term. If there is no index
// currently built for the term, but the term is a candidate for indexing, ther index will be
// built on the fly.
func indexBuildLazy(t *Topdown, ref ast.Ref) (bool, error) {

	// Check if index was already built.
	if t.Store.IndexExists(ref) {
		return true, nil
	}

	// Ignore refs against variables.
	if !ref[0].Equal(ast.DefaultRootDocument) {
		return false, nil
	}

	// Ignore refs against virtual docs.
	tmp := ast.Ref{ref[0], ref[1]}
	r := t.Compiler.GetRulesExact(tmp)
	if r != nil {
		return false, nil
	}

	for _, p := range ref[2:] {

		if !p.Value.IsGround() {
			break
		}

		tmp = append(tmp, p)
		r := t.Compiler.GetRulesExact(tmp)
		if r != nil {
			return false, nil
		}
	}

	if err := t.Store.BuildIndex(t.Context, t.txn, ref); err != nil {
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

// indexingAllowed returns true if indexing can be used for the expression
// eq(ref, term).
func indexingAllowed(ref ast.Ref, term *ast.Term) bool {

	// Will not build indices for non-refs or refs that are ground as this would
	// be pointless. Also, storage does not support nested refs, so exclude
	// those.
	if ref == nil || ref.IsGround() || ref.IsNested() {
		return false
	}

	// Cannot perform index lookup for non-ground terms (except for refs which
	// will be evaluated in isolation).
	// TODO(tsandall): should be able to support non-ground terms that only
	// contain refs with output vars.
	if _, ok := term.Value.(ast.Ref); !ok && !term.IsGround() {
		return false
	}

	return true
}

func lookupValue(t *Topdown, ref ast.Ref) (ast.Value, error) {
	r, err := t.Resolve(ref)
	if err != nil {
		return nil, err
	}
	return ast.InterfaceToValue(r)
}

// valueMapStack is used to store a stack of bindings.
type valueMapStack struct {
	sl []*ast.ValueMap
}

func newValueMapStack() *valueMapStack {
	return &valueMapStack{}
}

func (s *valueMapStack) Push(vm *ast.ValueMap) {
	s.sl = append(s.sl, vm)
}

func (s *valueMapStack) Pop() *ast.ValueMap {
	idx := len(s.sl) - 1
	vm := s.sl[idx]
	s.sl = s.sl[:idx]
	return vm
}

func (s *valueMapStack) Peek() *ast.ValueMap {
	idx := len(s.sl) - 1
	if idx == -1 {
		return nil
	}
	return s.sl[idx]
}

func (s *valueMapStack) Binding(k ast.Value) ast.Value {
	for i := len(s.sl) - 1; i >= 0; i-- {
		if v := s.sl[i].Get(k); v != nil {
			return v
		}
	}
	return nil
}

func (s *valueMapStack) Bind(k, v ast.Value, prev *Undo) *Undo {
	if len(s.sl) == 0 {
		s.Push(ast.NewValueMap())
	}
	vm := s.Peek()
	orig := vm.Get(k)
	vm.Put(k, v)
	return &Undo{k, orig, prev}
}

func (s *valueMapStack) Unbind(u *Undo) {
	vm := s.Peek()
	if u.Value != nil {
		vm.Put(u.Key, u.Value)
	} else {
		vm.Delete(u.Key)
	}
}

func (s *valueMapStack) String() string {
	buf := make([]string, len(s.sl))
	for i := range s.sl {
		buf[i] = s.sl[i].String()
	}
	return "[" + strings.Join(buf, ", ") + "]"
}
