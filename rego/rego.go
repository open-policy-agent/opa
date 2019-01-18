// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package rego exposes high level APIs for evaluating Rego policies.
package rego

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/compiler/wasm"
	"github.com/open-policy-agent/opa/internal/ir"
	"github.com/open-policy-agent/opa/internal/planner"
	"github.com/open-policy-agent/opa/internal/wasm/encoding"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
)

const defaultPartialNamespace = "partial"

// CompileResult represents sthe result of compiling a Rego query, zero or more
// Rego modules, and arbitrary contextual data into an executable.
type CompileResult struct {
	Bytes []byte `json:"bytes"`
}

// PartialQueries contains the queries and support modules produced by partial
// evaluation.
type PartialQueries struct {
	Queries []ast.Body    `json:"queries,omitempty"`
	Support []*ast.Module `json:"modules,omitempty"`
}

// PartialResult represents the result of partial evaluation. The result can be
// used to generate a new query that can be run when inputs are known.
type PartialResult struct {
	compiler *ast.Compiler
	store    storage.Store
	body     ast.Body
}

// Rego returns an object that can be evaluated to produce a query result.
func (pr PartialResult) Rego(options ...func(*Rego)) *Rego {
	options = append(options, Compiler(pr.compiler), Store(pr.store), ParsedQuery(pr.body))
	return New(options...)
}

// Result defines the output of Rego evaluation.
type Result struct {
	Expressions []*ExpressionValue `json:"expressions"`
	Bindings    Vars               `json:"bindings,omitempty"`
}

func newResult() Result {
	return Result{
		Bindings: Vars{},
	}
}

// Location defines a position in a Rego query or module.
type Location struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// ExpressionValue defines the value of an expression in a Rego query.
type ExpressionValue struct {
	Value    interface{} `json:"value"`
	Text     string      `json:"text"`
	Location *Location   `json:"location"`
}

func newExpressionValue(expr *ast.Expr, value interface{}) *ExpressionValue {
	return &ExpressionValue{
		Value: value,
		Text:  string(expr.Location.Text),
		Location: &Location{
			Row: expr.Location.Row,
			Col: expr.Location.Col,
		},
	}
}

func (ev *ExpressionValue) String() string {
	return fmt.Sprint(ev.Value)
}

// ResultSet represents a collection of output from Rego evaluation. An empty
// result set represents an undefined query.
type ResultSet []Result

// Vars represents a collection of variable bindings. The keys are the variable
// names and the values are the binding values.
type Vars map[string]interface{}

// WithoutWildcards returns a copy of v with wildcard variables removed.
func (v Vars) WithoutWildcards() Vars {
	n := Vars{}
	for k, v := range v {
		if ast.Var(k).IsWildcard() || ast.Var(k).IsGenerated() {
			continue
		}
		n[k] = v
	}
	return n
}

// Errors represents a collection of errors returned when evaluating Rego.
type Errors []error

func (errs Errors) Error() string {
	if len(errs) == 0 {
		return "no error"
	}
	if len(errs) == 1 {
		return fmt.Sprintf("1 error occurred: %v", errs[0].Error())
	}
	buf := []string{fmt.Sprintf("%v errors occurred", len(errs))}
	for _, err := range errs {
		buf = append(buf, err.Error())
	}
	return strings.Join(buf, "\n")
}

// Rego constructs a query and can be evaluated to obtain results.
type Rego struct {
	query            string
	parsedQuery      ast.Body
	pkg              string
	parsedPackage    *ast.Package
	imports          []string
	parsedImports    []*ast.Import
	rawInput         *interface{}
	input            ast.Value
	unknowns         []string
	parsedUnknowns   []*ast.Term
	partialNamespace string
	modules          []rawModule
	compiler         *ast.Compiler
	store            storage.Store
	txn              storage.Transaction
	metrics          metrics.Metrics
	tracers          []topdown.Tracer
	tracebuf         *topdown.BufferTracer
	trace            bool
	instrumentation  *topdown.Instrumentation
	instrument       bool
	capture          map[*ast.Expr]ast.Var // map exprs to generated capture vars
	termVarID        int
	dump             io.Writer
	runtime          *ast.Term
}

// Dump returns an argument that sets the writer to dump debugging information to.
func Dump(w io.Writer) func(r *Rego) {
	return func(r *Rego) {
		r.dump = w
	}
}

// Query returns an argument that sets the Rego query.
func Query(q string) func(r *Rego) {
	return func(r *Rego) {
		r.query = q
	}
}

// ParsedQuery returns an argument that sets the Rego query.
func ParsedQuery(q ast.Body) func(r *Rego) {
	return func(r *Rego) {
		r.parsedQuery = q
	}
}

// Package returns an argument that sets the Rego package on the query's
// context.
func Package(p string) func(r *Rego) {
	return func(r *Rego) {
		r.pkg = p
	}
}

// ParsedPackage returns an argument that sets the Rego package on the query's
// context.
func ParsedPackage(pkg *ast.Package) func(r *Rego) {
	return func(r *Rego) {
		r.parsedPackage = pkg
	}
}

// Imports returns an argument that adds a Rego import to the query's context.
func Imports(p []string) func(r *Rego) {
	return func(r *Rego) {
		r.imports = append(r.imports, p...)
	}
}

// ParsedImports returns an argument that adds Rego imports to the query's
// context.
func ParsedImports(imp []*ast.Import) func(r *Rego) {
	return func(r *Rego) {
		r.parsedImports = append(r.parsedImports, imp...)
	}
}

// Input returns an argument that sets the Rego input document. Input should be
// a native Go value representing the input document.
func Input(x interface{}) func(r *Rego) {
	return func(r *Rego) {
		r.rawInput = &x
	}
}

// ParsedInput returns an argument that set sthe Rego input document.
func ParsedInput(x ast.Value) func(r *Rego) {
	return func(r *Rego) {
		r.input = x
	}
}

// Unknowns returns an argument that sets the values to treat as unknown during
// partial evaluation.
func Unknowns(unknowns []string) func(r *Rego) {
	return func(r *Rego) {
		r.unknowns = unknowns
	}
}

// ParsedUnknowns returns an argument that sets the values to treat as unknown
// during partial evaluation.
func ParsedUnknowns(unknowns []*ast.Term) func(r *Rego) {
	return func(r *Rego) {
		r.parsedUnknowns = unknowns
	}
}

// PartialNamespace returns an argument that sets the namespace to use for
// partial evaluation results. The namespace must be a valid package path
// component.
func PartialNamespace(ns string) func(r *Rego) {
	return func(r *Rego) {
		r.partialNamespace = ns
	}
}

// Module returns an argument that adds a Rego module.
func Module(filename, input string) func(r *Rego) {
	return func(r *Rego) {
		r.modules = append(r.modules, rawModule{
			filename: filename,
			module:   input,
		})
	}
}

// Compiler returns an argument that sets the Rego compiler.
func Compiler(c *ast.Compiler) func(r *Rego) {
	return func(r *Rego) {
		r.compiler = c
	}
}

// Store returns an argument that sets the policy engine's data storage layer.
func Store(s storage.Store) func(r *Rego) {
	return func(r *Rego) {
		r.store = s
	}
}

// Transaction returns an argument that sets the transaction to use for storage
// layer operations.
func Transaction(txn storage.Transaction) func(r *Rego) {
	return func(r *Rego) {
		r.txn = txn
	}
}

// Metrics returns an argument that sets the metrics collection.
func Metrics(m metrics.Metrics) func(r *Rego) {
	return func(r *Rego) {
		r.metrics = m
	}
}

// Instrument returns an argument that enables instrumentation for diagnosing
// performance issues.
func Instrument(yes bool) func(r *Rego) {
	return func(r *Rego) {
		r.instrument = yes
	}
}

// Trace returns an argument that enables tracing on r.
func Trace(yes bool) func(r *Rego) {
	return func(r *Rego) {
		r.trace = yes
	}
}

// Tracer returns an argument that adds a query tracer to r.
func Tracer(t topdown.Tracer) func(r *Rego) {
	return func(r *Rego) {
		if t != nil {
			r.tracers = append(r.tracers, t)
		}
	}
}

// Runtime returns an argument that sets the runtime data to provide to the
// evaluation engine.
func Runtime(term *ast.Term) func(r *Rego) {
	return func(r *Rego) {
		r.runtime = term
	}
}

// PrintTrace is a helper fnuction to write a human-readable version of the
// trace to the writer w.
func PrintTrace(w io.Writer, r *Rego) {
	if r == nil || r.tracebuf == nil {
		return
	}
	topdown.PrettyTrace(w, *r.tracebuf)
}

// New returns a new Rego object.
func New(options ...func(*Rego)) *Rego {

	r := &Rego{
		capture: map[*ast.Expr]ast.Var{},
	}

	for _, option := range options {
		option(r)
	}

	if r.compiler == nil {
		r.compiler = ast.NewCompiler()
	}

	if r.store == nil {
		r.store = inmem.New()
	}

	if r.metrics == nil {
		r.metrics = metrics.New()
	}

	if r.instrument {
		r.instrumentation = topdown.NewInstrumentation(r.metrics)
	}

	if r.trace {
		r.tracebuf = topdown.NewBufferTracer()
		r.tracers = append(r.tracers, r.tracebuf)
	}

	return r
}

// Eval evaluates this Rego object and returns a ResultSet.
func (r *Rego) Eval(ctx context.Context) (ResultSet, error) {

	if len(r.query) == 0 && len(r.parsedQuery) == 0 {
		return nil, fmt.Errorf("cannot evaluate empty query")
	}

	parsed, query, err := r.parse()
	if err != nil {
		return nil, err
	}

	err = r.compileModules(parsed)
	if err != nil {
		return nil, err
	}

	qc, compiled, err := r.compileQuery([]extraStage{
		{
			after: "ResolveRefs",
			stage: r.rewriteQueryToCaptureValue,
		},
	}, query)

	if err != nil {
		return nil, err
	}

	txn := r.txn

	if txn == nil {
		txn, err = r.store.NewTransaction(ctx)
		if err != nil {
			return nil, err
		}
		defer r.store.Abort(ctx, txn)
	}

	return r.eval(ctx, qc, compiled, txn)
}

// PartialEval has been deprecated and renamed to PartialResult.
func (r *Rego) PartialEval(ctx context.Context) (PartialResult, error) {
	return r.PartialResult(ctx)
}

// PartialResult partially evaluates this Rego object and returns a PartialResult.
func (r *Rego) PartialResult(ctx context.Context) (PartialResult, error) {

	if len(r.query) == 0 && len(r.parsedQuery) == 0 {
		return PartialResult{}, fmt.Errorf("cannot evaluate empty query")
	}

	parsed, query, err := r.parse()
	if err != nil {
		return PartialResult{}, err
	}

	err = r.compileModules(parsed)
	if err != nil {
		return PartialResult{}, err
	}

	_, compiled, err := r.compileQuery([]extraStage{
		{
			after: "ResolveRefs",
			stage: r.rewriteQueryForPartialEval,
		},
	}, query)

	if err != nil {
		return PartialResult{}, err
	}

	txn := r.txn

	if txn == nil {
		txn, err = r.store.NewTransaction(ctx)
		if err != nil {
			return PartialResult{}, err
		}
		defer r.store.Abort(ctx, txn)
	}

	partialNamespace := r.partialNamespace
	if partialNamespace == "" {
		partialNamespace = defaultPartialNamespace
	}

	return r.partialResult(ctx, compiled, txn, partialNamespace, ast.Wildcard)
}

// Partial runs partial evaluation on r and returns the result.
func (r *Rego) Partial(ctx context.Context) (*PartialQueries, error) {

	if len(r.query) == 0 && len(r.parsedQuery) == 0 {
		return nil, fmt.Errorf("cannot evaluate empty query")
	}

	parsed, query, err := r.parse()
	if err != nil {
		return nil, err
	}

	err = r.compileModules(parsed)
	if err != nil {
		return nil, err
	}

	_, compiled, err := r.compileQuery(nil, query)
	if err != nil {
		return nil, err
	}

	txn := r.txn

	if txn == nil {
		txn, err = r.store.NewTransaction(ctx)
		if err != nil {
			return nil, err
		}
		defer r.store.Abort(ctx, txn)
	}

	partialNamespace := r.partialNamespace
	if partialNamespace == "" {
		partialNamespace = defaultPartialNamespace
	}

	return r.partial(ctx, compiled, txn, partialNamespace)
}

// Compile returnss a compiled policy query.
func (r *Rego) Compile(ctx context.Context) (*CompileResult, error) {

	pq, err := r.Partial(ctx)
	if err != nil {
		return nil, err
	}

	if len(pq.Support) > 0 {
		return nil, fmt.Errorf("modules not supported")
	}

	policy, err := planner.New().WithQueries(pq.Queries).Plan()
	if err != nil {
		return nil, err
	}

	if r.dump != nil {
		fmt.Fprintln(r.dump, "PLAN:")
		fmt.Fprintln(r.dump, "-----")
		ir.Pretty(r.dump, policy)
		fmt.Fprintln(r.dump)
	}

	m, err := wasm.New().WithPolicy(policy).Compile()
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer

	if err := encoding.WriteModule(&out, m); err != nil {
		return nil, err
	}

	result := &CompileResult{
		Bytes: out.Bytes(),
	}

	return result, nil
}

func (r *Rego) parse() (map[string]*ast.Module, ast.Body, error) {

	r.metrics.Timer(metrics.RegoModuleParse).Start()

	var errs Errors
	parsed := map[string]*ast.Module{}

	for _, module := range r.modules {
		p, err := module.Parse()
		if err != nil {
			errs = append(errs, err)
		}
		parsed[module.filename] = p
	}

	r.metrics.Timer(metrics.RegoModuleParse).Stop()
	r.metrics.Timer(metrics.RegoQueryParse).Start()
	defer r.metrics.Timer(metrics.RegoQueryParse).Stop()

	var query ast.Body

	if r.parsedQuery != nil {
		query = r.parsedQuery
	} else {
		var err error
		query, err = ast.ParseBody(r.query)
		if err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			return nil, nil, errs
		}
	}

	return parsed, query, nil
}

func (r *Rego) compileModules(modules map[string]*ast.Module) error {

	r.metrics.Timer(metrics.RegoModuleCompile).Start()
	defer r.metrics.Timer(metrics.RegoModuleCompile).Stop()

	if len(modules) > 0 {
		r.compiler.Compile(modules)
		if r.compiler.Failed() {
			var errs Errors
			for _, err := range r.compiler.Errors {
				errs = append(errs, err)
			}
			return errs
		}
	}

	return nil
}

func (r *Rego) compileQuery(extras []extraStage, query ast.Body) (ast.QueryCompiler, ast.Body, error) {

	r.metrics.Timer(metrics.RegoQueryCompile).Start()
	defer r.metrics.Timer(metrics.RegoQueryCompile).Stop()

	var pkg *ast.Package

	if r.pkg != "" {
		var err error
		pkg, err = ast.ParsePackage(fmt.Sprintf("package %v", r.pkg))
		if err != nil {
			return nil, nil, err
		}
	} else {
		pkg = r.parsedPackage
	}

	imports := r.parsedImports

	if len(r.imports) > 0 {
		s := make([]string, len(r.imports))
		for i := range r.imports {
			s[i] = fmt.Sprintf("import %v", r.imports[i])
		}
		parsed, err := ast.ParseImports(strings.Join(s, "\n"))
		if err != nil {
			return nil, nil, err
		}
		imports = append(imports, parsed...)
	}

	var input ast.Value

	if r.rawInput != nil {
		rawPtr := util.Reference(r.rawInput)
		// roundtrip through json: this turns slices (e.g. []string, []bool) into
		// []interface{}, the only array type ast.InterfaceToValue can work with
		if err := util.RoundTrip(rawPtr); err != nil {
			return nil, nil, err
		}
		val, err := ast.InterfaceToValue(*rawPtr)
		if err != nil {
			return nil, nil, err
		}
		input = val
		r.input = val
	} else {
		input = r.input
	}

	qctx := ast.NewQueryContext().
		WithPackage(pkg).
		WithImports(imports).
		WithInput(input)

	qc := r.compiler.QueryCompiler().WithContext(qctx)

	for _, extra := range extras {
		qc = qc.WithStageAfter(extra.after, extra.stage)
	}

	compiled, err := qc.Compile(query)
	return qc, compiled, err

}

func (r *Rego) eval(ctx context.Context, qc ast.QueryCompiler, compiled ast.Body, txn storage.Transaction) (rs ResultSet, err error) {

	q := topdown.NewQuery(compiled).
		WithCompiler(r.compiler).
		WithStore(r.store).
		WithTransaction(txn).
		WithMetrics(r.metrics).
		WithInstrumentation(r.instrumentation).
		WithRuntime(r.runtime)

	for i := range r.tracers {
		q = q.WithTracer(r.tracers[i])
	}

	if r.input != nil {
		q = q.WithInput(ast.NewTerm(r.input))
	}

	// Cancel query if context is cancelled or deadline is reached.
	c := topdown.NewCancel()
	q = q.WithCancel(c)
	exit := make(chan struct{})
	defer close(exit)
	go waitForDone(ctx, exit, func() {
		c.Cancel()
	})

	rewritten := qc.RewrittenVars()

	err = q.Iter(ctx, func(qr topdown.QueryResult) error {
		result := newResult()
		for k := range qr {
			v, err := ast.JSON(qr[k].Value)
			if err != nil {
				return err
			}
			if rw, ok := rewritten[k]; ok {
				k = rw
			}
			if isTermVar(k) || k.IsGenerated() || k.IsWildcard() {
				continue
			}
			result.Bindings[string(k)] = v
		}
		for _, expr := range compiled {
			if expr.Generated {
				continue
			}
			if k, ok := r.capture[expr]; ok {
				v, err := ast.JSON(qr[k].Value)
				if err != nil {
					return err
				}
				result.Expressions = append(result.Expressions, newExpressionValue(expr, v))
			} else {
				result.Expressions = append(result.Expressions, newExpressionValue(expr, true))
			}
		}
		rs = append(rs, result)
		return nil
	})

	if err != nil {
		return nil, err
	}

	if len(rs) == 0 {
		return nil, nil
	}

	return rs, nil
}

func (r *Rego) partialResult(ctx context.Context, compiled ast.Body, txn storage.Transaction, partialNamespace string, output *ast.Term) (PartialResult, error) {

	pq, err := r.partial(ctx, compiled, txn, partialNamespace)
	if err != nil {
		return PartialResult{}, err
	}

	// Construct module for queries.
	module := ast.MustParseModule("package " + partialNamespace)
	module.Rules = make([]*ast.Rule, len(pq.Queries))
	for i, body := range pq.Queries {
		module.Rules[i] = &ast.Rule{
			Head:   ast.NewHead(ast.Var("__result__"), nil, output),
			Body:   body,
			Module: module,
		}
	}

	// Update compiler with partial evaluation output.
	r.compiler.Modules["__partialresult__"] = module
	for i, module := range pq.Support {
		r.compiler.Modules[fmt.Sprintf("__partialsupport%d__", i)] = module
	}

	r.compiler.Compile(r.compiler.Modules)
	if r.compiler.Failed() {
		return PartialResult{}, r.compiler.Errors
	}

	result := PartialResult{
		compiler: r.compiler,
		store:    r.store,
		body:     ast.MustParseBody(fmt.Sprintf("data.%v.__result__", partialNamespace)),
	}

	return result, nil
}

func (r *Rego) partial(ctx context.Context, compiled ast.Body, txn storage.Transaction, partialNamespace string) (*PartialQueries, error) {

	var unknowns []*ast.Term

	if r.parsedUnknowns != nil {
		unknowns = r.parsedUnknowns
	} else if r.unknowns != nil {
		unknowns = make([]*ast.Term, len(r.unknowns))
		for i := range r.unknowns {
			var err error
			unknowns[i], err = ast.ParseTerm(r.unknowns[i])
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Use input document as unknown if caller has not specified any.
		unknowns = []*ast.Term{ast.NewTerm(ast.InputRootRef)}
	}

	// Check partial namespace to ensure it's valid.
	if term, err := ast.ParseTerm(partialNamespace); err != nil {
		return nil, err
	} else if _, ok := term.Value.(ast.Var); !ok {
		return nil, fmt.Errorf("bad partial namespace")
	}

	q := topdown.NewQuery(compiled).
		WithCompiler(r.compiler).
		WithStore(r.store).
		WithTransaction(txn).
		WithMetrics(r.metrics).
		WithInstrumentation(r.instrumentation).
		WithUnknowns(unknowns).
		WithRuntime(r.runtime)

	for i := range r.tracers {
		q = q.WithTracer(r.tracers[i])
	}

	if r.input != nil {
		q = q.WithInput(ast.NewTerm(r.input))
	}

	// Cancel query if context is cancelled or deadline is reached.
	c := topdown.NewCancel()
	q = q.WithCancel(c)
	exit := make(chan struct{})
	defer close(exit)
	go waitForDone(ctx, exit, func() {
		c.Cancel()
	})

	queries, support, err := q.PartialRun(ctx)
	if err != nil {
		return nil, err
	}

	pq := &PartialQueries{
		Queries: queries,
		Support: support,
	}

	return pq, nil
}

func (r *Rego) rewriteQueryToCaptureValue(qc ast.QueryCompiler, query ast.Body) (ast.Body, error) {

	checkCapture := iteration(query) || len(query) > 1

	for _, expr := range query {

		if expr.Negated {
			continue
		}

		if expr.IsAssignment() || expr.IsEquality() {
			continue
		}

		var capture *ast.Term

		// If the expression can be evaluated as a function, rewrite it to
		// capture the return value. E.g., neq(1,2) becomes neq(1,2,x) but
		// plus(1,2,x) does not get rewritten.
		switch terms := expr.Terms.(type) {
		case *ast.Term:
			capture = r.generateTermVar()
			expr.Terms = ast.Equality.Expr(terms, capture).Terms
			r.capture[expr] = capture.Value.(ast.Var)
		case []*ast.Term:
			if r.compiler.GetArity(expr.Operator()) == len(terms)-1 {
				capture = r.generateTermVar()
				expr.Terms = append(terms, capture)
				r.capture[expr] = capture.Value.(ast.Var)
			}
		}

		if capture != nil && checkCapture {
			cpy := expr.Copy()
			cpy.Terms = capture
			cpy.Generated = true
			cpy.With = nil
			query.Append(cpy)
		}
	}

	return query, nil
}

func (r *Rego) rewriteQueryForPartialEval(_ ast.QueryCompiler, query ast.Body) (ast.Body, error) {
	if len(query) != 1 {
		return nil, fmt.Errorf("partial evaluation requires single ref (not multiple expressions)")
	}

	term, ok := query[0].Terms.(*ast.Term)
	if !ok {
		return nil, fmt.Errorf("partial evaluation requires ref (not expression)")
	}

	ref, ok := term.Value.(ast.Ref)
	if !ok {
		return nil, fmt.Errorf("partial evaluation requires ref (not %v)", ast.TypeName(term.Value))
	}

	if !ref.IsGround() {
		return nil, fmt.Errorf("partial evaluation requires ground ref")
	}

	return ast.NewBody(ast.Equality.Expr(ast.Wildcard, term)), nil
}

func (r *Rego) generateTermVar() *ast.Term {
	r.termVarID++
	return ast.VarTerm(ast.WildcardPrefix + fmt.Sprintf("term%v", r.termVarID))
}

func isTermVar(v ast.Var) bool {
	return strings.HasPrefix(string(v), ast.WildcardPrefix+"term")
}

func waitForDone(ctx context.Context, exit chan struct{}, f func()) {
	select {
	case <-exit:
		return
	case <-ctx.Done():
		f()
		return
	}
}

type rawModule struct {
	filename string
	module   string
}

func (m rawModule) Parse() (*ast.Module, error) {
	return ast.ParseModule(m.filename, m.module)
}

type extraStage struct {
	after string
	stage ast.QueryCompilerStage
}

func iteration(x interface{}) bool {

	var stopped bool

	vis := ast.NewGenericVisitor(func(x interface{}) bool {
		switch x := x.(type) {
		case *ast.Term:
			if ast.IsComprehension(x.Value) {
				return true
			}
		case ast.Ref:
			if !stopped {
				if bi := ast.BuiltinMap[x.String()]; bi != nil {
					if bi.Relation {
						stopped = true
						return stopped
					}
				}
				for i := 1; i < len(x); i++ {
					if _, ok := x[i].Value.(ast.Var); ok {
						stopped = true
						return stopped
					}
				}
			}
			return stopped
		}
		return stopped
	})

	ast.Walk(vis, x)

	return stopped
}
