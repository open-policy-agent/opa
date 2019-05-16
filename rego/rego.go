// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package rego exposes high level APIs for evaluating Rego policies.
package rego

import (
	"bytes"
	"context"
	"errors"
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

// CompileResult represents the result of compiling a Rego query, zero or more
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
// If rego.Rego#Prepare was used to create the PartialResult this may lose
// the pre-parsed/compiled parts of the original Rego object. In those cases
// using rego.PartialResult#Eval is likely to be more performant.
func (pr PartialResult) Rego(options ...func(*Rego)) *Rego {
	options = append(options, Compiler(pr.compiler), Store(pr.store), ParsedQuery(pr.body))
	return New(options...)
}

// preparedQuery is a wrapper around a Rego object which has pre-processed
// state stored on it. Once prepared there are a more limited number of actions
// that can be taken with it. It will, however, be able to evaluate faster since
// it will not have to re-parse or compile as much.
type preparedQuery struct {
	r   *Rego
	cfg *PrepareConfig
}

// EvalContext defines the set of options allowed to be set at evaluation
// time. Any other options will need to be set on a new Rego object.
type EvalContext struct {
	hasInput         bool
	rawInput         *interface{}
	parsedInput      ast.Value
	metrics          metrics.Metrics
	txn              storage.Transaction
	instrument       bool
	instrumentation  *topdown.Instrumentation
	partialNamespace string
	tracers          []topdown.Tracer
	compiledQuery    compiledQuery
	unknowns         []string
	parsedUnknowns   []*ast.Term
}

// EvalOption defines a function to set an option on an EvalConfig
type EvalOption func(*EvalContext)

// EvalInput configures the input for a Prepared Query's evaluation
func EvalInput(input interface{}) EvalOption {
	return func(e *EvalContext) {
		e.rawInput = &input
		e.hasInput = true
	}
}

// EvalParsedInput configures the input for a Prepared Query's evaluation
func EvalParsedInput(input ast.Value) EvalOption {
	return func(e *EvalContext) {
		e.parsedInput = input
		e.hasInput = true
	}
}

// EvalMetrics configures the metrics for a Prepared Query's evaluation
func EvalMetrics(metric metrics.Metrics) EvalOption {
	return func(e *EvalContext) {
		e.metrics = metric
	}
}

// EvalTransaction configures the Transaction for a Prepared Query's evaluation
func EvalTransaction(txn storage.Transaction) EvalOption {
	return func(e *EvalContext) {
		e.txn = txn
	}
}

// EvalInstrument enables or disables instrumenting for a Prepared Query's evaluation
func EvalInstrument(instrument bool) EvalOption {
	return func(e *EvalContext) {
		e.instrument = instrument
	}
}

// EvalTracer configures a tracer for a Prepared Query's evaluation
func EvalTracer(tracer topdown.Tracer) EvalOption {
	return func(e *EvalContext) {
		if tracer != nil {
			e.tracers = append(e.tracers, tracer)
		}
	}
}

// EvalPartialNamespace returns an argument that sets the namespace to use for
// partial evaluation results. The namespace must be a valid package path
// component.
func EvalPartialNamespace(ns string) EvalOption {
	return func(e *EvalContext) {
		e.partialNamespace = ns
	}
}

// EvalUnknowns returns an argument that sets the values to treat as
// unknown during partial evaluation.
func EvalUnknowns(unknowns []string) EvalOption {
	return func(e *EvalContext) {
		e.unknowns = unknowns
	}
}

// EvalParsedUnknowns returns an argument that sets the values to treat
// as unknown during partial evaluation.
func EvalParsedUnknowns(unknowns []*ast.Term) EvalOption {
	return func(e *EvalContext) {
		e.parsedUnknowns = unknowns
	}
}

func (pq preparedQuery) newEvalContext(ctx context.Context, options []EvalOption) (*EvalContext, error) {
	ectx := &EvalContext{
		hasInput:         false,
		rawInput:         nil,
		parsedInput:      nil,
		metrics:          pq.r.metrics,
		txn:              pq.r.txn,
		instrument:       pq.r.instrument,
		instrumentation:  pq.r.instrumentation,
		partialNamespace: pq.r.partialNamespace,
		tracers:          pq.r.tracers,
		unknowns:         pq.r.unknowns,
		parsedUnknowns:   pq.r.parsedUnknowns,
		compiledQuery:    compiledQuery{},
	}

	for _, o := range options {
		o(ectx)
	}

	if ectx.instrument {
		ectx.instrumentation = topdown.NewInstrumentation(ectx.metrics)
	}

	var err error
	if ectx.txn == nil {
		ectx.txn, err = pq.r.store.NewTransaction(ctx)
		if err != nil {
			return nil, err
		}
		defer pq.r.store.Abort(ctx, ectx.txn)
	}

	// If we didn't get an input specified in the Eval options
	// then fall back to the Rego object's input fields.
	if !ectx.hasInput {
		ectx.rawInput = pq.r.rawInput
		ectx.parsedInput = pq.r.parsedInput
	}

	if ectx.parsedInput == nil {
		if ectx.rawInput == nil {
			// Fall back to the original Rego objects input if none was specified
			// Note that it could still be nil
			ectx.rawInput = pq.r.rawInput
		}
		ectx.parsedInput, err = pq.r.parseRawInput(ectx.rawInput, ectx.metrics)
		if err != nil {
			return nil, err
		}
	}

	return ectx, nil
}

// PreparedEvalQuery holds the prepared Rego state that has been pre-processed
// for subsequent evaluations.
type PreparedEvalQuery struct {
	preparedQuery
}

// Eval evaluates this PartialResult's Rego object with additional eval options
// and returns a ResultSet.
// If options are provided they will override the original Rego options respective value.
func (pq PreparedEvalQuery) Eval(ctx context.Context, options ...EvalOption) (ResultSet, error) {
	ectx, err := pq.newEvalContext(ctx, options)
	if err != nil {
		return nil, err
	}

	ectx.compiledQuery = pq.r.compiledQueries[evalQueryType]

	return pq.r.eval(ctx, ectx)
}

// PreparedPartialQuery holds the prepared Rego state that has been pre-processed
// for partial evaluations.
type PreparedPartialQuery struct {
	preparedQuery
}

// Partial runs partial evaluation on the prepared query and returns the result.
func (pq PreparedPartialQuery) Partial(ctx context.Context, options ...EvalOption) (*PartialQueries, error) {
	ectx, err := pq.newEvalContext(ctx, options)
	if err != nil {
		return nil, err
	}

	ectx.compiledQuery = pq.r.compiledQueries[partialQueryType]

	return pq.r.partial(ctx, ectx)
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

type compiledQuery struct {
	query    ast.Body
	compiler ast.QueryCompiler
}

type queryType int

// Define a query type for each of the top level Rego
// API's that compile queries differently.
const (
	evalQueryType          queryType = iota
	partialResultQueryType queryType = iota
	partialQueryType       queryType = iota
	compileQueryType       queryType = iota
)

// Rego constructs a query and can be evaluated to obtain results.
type Rego struct {
	query            string
	parsedQuery      ast.Body
	compiledQueries  map[queryType]compiledQuery
	pkg              string
	parsedPackage    *ast.Package
	imports          []string
	parsedImports    []*ast.Import
	rawInput         *interface{}
	parsedInput      ast.Value
	unknowns         []string
	parsedUnknowns   []*ast.Term
	partialNamespace string
	modules          []rawModule
	parsedModules    map[string]*ast.Module
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

// ParsedInput returns an argument that sets the Rego input document.
func ParsedInput(x ast.Value) func(r *Rego) {
	return func(r *Rego) {
		r.parsedInput = x
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

// PrintTrace is a helper function to write a human-readable version of the
// trace to the writer w.
func PrintTrace(w io.Writer, r *Rego) {
	if r == nil || r.tracebuf == nil {
		return
	}
	topdown.PrettyTrace(w, *r.tracebuf)
}

// New returns a new Rego object.
func New(options ...func(r *Rego)) *Rego {

	r := &Rego{
		capture:         map[*ast.Expr]ast.Var{},
		compiledQueries: map[queryType]compiledQuery{},
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
		r.compiler.WithMetrics(r.metrics)
	}

	if r.trace {
		r.tracebuf = topdown.NewBufferTracer()
		r.tracers = append(r.tracers, r.tracebuf)
	}

	if r.partialNamespace == "" {
		r.partialNamespace = defaultPartialNamespace
	}

	return r
}

// Eval evaluates this Rego object and returns a ResultSet.
func (r *Rego) Eval(ctx context.Context) (ResultSet, error) {
	var err error
	if r.txn == nil {
		r.txn, err = r.store.NewTransaction(ctx)
		if err != nil {
			return nil, err
		}
		defer r.store.Abort(ctx, r.txn)
	}

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		return nil, err
	}

	return pq.Eval(ctx)
}

// PartialEval has been deprecated and renamed to PartialResult.
func (r *Rego) PartialEval(ctx context.Context) (PartialResult, error) {
	return r.PartialResult(ctx)
}

// PartialResult partially evaluates this Rego object and returns a PartialResult.
func (r *Rego) PartialResult(ctx context.Context) (PartialResult, error) {
	var err error
	if r.txn == nil {
		r.txn, err = r.store.NewTransaction(ctx)
		if err != nil {
			return PartialResult{}, err
		}
		defer r.store.Abort(ctx, r.txn)
	}

	pq, err := r.PrepareForEval(ctx, WithPartialEval())
	if err != nil {
		return PartialResult{}, err
	}

	pr := PartialResult{
		compiler: pq.r.compiler,
		store:    pq.r.store,
		body:     pq.r.parsedQuery,
	}

	return pr, nil
}

// Partial runs partial evaluation on r and returns the result.
func (r *Rego) Partial(ctx context.Context) (*PartialQueries, error) {
	var err error
	if r.txn == nil {
		r.txn, err = r.store.NewTransaction(ctx)
		if err != nil {
			return nil, err
		}
		defer r.store.Abort(ctx, r.txn)
	}

	pq, err := r.PrepareForPartial(ctx)
	if err != nil {
		return nil, err
	}

	return pq.Partial(ctx)
}

// CompileOption defines a function to set options on Compile calls.
type CompileOption func(*CompileContext)

// CompileContext contains options for Compile calls.
type CompileContext struct {
	partial bool
}

// CompilePartial defines an option to control whether partial evaluation is run
// before the query is planned and compiled.
func CompilePartial(yes bool) CompileOption {
	return func(cfg *CompileContext) {
		cfg.partial = yes
	}
}

// Compile returns a compiled policy query.
func (r *Rego) Compile(ctx context.Context, opts ...CompileOption) (*CompileResult, error) {

	var cfg CompileContext

	for _, opt := range opts {
		opt(&cfg)
	}

	var queries []ast.Body
	var modules []*ast.Module

	if cfg.partial {

		pq, err := r.Partial(ctx)
		if err != nil {
			return nil, err
		}
		if r.dump != nil {
			if len(pq.Queries) != 0 {
				msg := fmt.Sprintf("QUERIES (%d total):", len(pq.Queries))
				fmt.Fprintln(r.dump, msg)
				fmt.Fprintln(r.dump, strings.Repeat("-", len(msg)))
				for i := range pq.Queries {
					fmt.Println(pq.Queries[i])
				}
				fmt.Fprintln(r.dump)
			}
			if len(pq.Support) != 0 {
				msg := fmt.Sprintf("SUPPORT (%d total):", len(pq.Support))
				fmt.Fprintln(r.dump, msg)
				fmt.Fprintln(r.dump, strings.Repeat("-", len(msg)))
				for i := range pq.Support {
					fmt.Println(pq.Support[i])
				}
				fmt.Fprintln(r.dump)
			}
		}

		queries = pq.Queries
		modules = pq.Support

		for _, module := range r.compiler.Modules {
			modules = append(modules, module)
		}
	} else {

		// execute block inside a closure so that transaction is closed before
		// planner runs.
		//
		// TODO(tsandall): in future, planner could make use of store, in which
		// case this will need to change.
		err := func() error {
			var err error
			if r.txn == nil {
				r.txn, err = r.store.NewTransaction(ctx)
				if err != nil {
					return err
				}
				defer r.store.Abort(ctx, r.txn)
			}

			if err := r.prepare(ctx, r.txn, compileQueryType, nil); err != nil {
				return err
			}

			for _, module := range r.compiler.Modules {
				modules = append(modules, module)
			}

			queries = []ast.Body{r.compiledQueries[compileQueryType].query}
			return nil
		}()

		if err != nil {
			return nil, err
		}
	}

	policy, err := planner.New().WithQueries(queries).WithModules(modules).Plan()
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

// PrepareOption defines a function to set an option to control
// the behavior of the Prepare call.
type PrepareOption func(*PrepareConfig)

// PrepareConfig holds settings to control the behavior of the
// Prepare call.
type PrepareConfig struct {
	doPartialEval bool
}

// WithPartialEval configures an option for PrepareForEval
// which will have it perform partial evaluation while preparing
// the query (similar to rego.Rego#PartialResult)
func WithPartialEval() PrepareOption {
	return func(p *PrepareConfig) {
		p.doPartialEval = true
	}
}

// PrepareForEval will parse inputs, modules, and query arguments in preparation
// of evaluating them.
func (r *Rego) PrepareForEval(ctx context.Context, opts ...PrepareOption) (PreparedEvalQuery, error) {
	if !r.hasQuery() {
		return PreparedEvalQuery{}, fmt.Errorf("cannot evaluate empty query")
	}

	pCfg := &PrepareConfig{}
	for _, o := range opts {
		o(pCfg)
	}

	txn := r.txn

	var err error
	if txn == nil {
		txn, err = r.store.NewTransaction(ctx)
		if err != nil {
			return PreparedEvalQuery{}, err
		}
		defer r.store.Abort(ctx, txn)
	}

	// If the caller wanted to do partial evaluation as part of preparation
	// do it now and use the new Rego object.
	if pCfg.doPartialEval {
		err := r.prepare(ctx, txn, partialResultQueryType, []extraStage{
			{
				after: "ResolveRefs",
				stage: ast.QueryCompilerStageDefinition{
					Name:       "RewriteForPartialEval",
					MetricName: "query_compile_stage_rewrite_for_partial_eval",
					Stage:      r.rewriteQueryForPartialEval,
				},
			},
		})
		if err != nil {
			return PreparedEvalQuery{}, err
		}

		ectx := &EvalContext{
			parsedInput:      r.parsedInput,
			metrics:          r.metrics,
			txn:              txn,
			partialNamespace: r.partialNamespace,
			tracers:          r.tracers,
			compiledQuery:    r.compiledQueries[partialResultQueryType],
		}

		pr, err := r.partialResult(ctx, ectx, ast.Wildcard)
		if err != nil {
			return PreparedEvalQuery{}, err
		}
		// Prepare the new query
		return pr.Rego().PrepareForEval(ctx)
	}

	err = r.prepare(ctx, txn, evalQueryType, []extraStage{
		{
			after: "ResolveRefs",
			stage: ast.QueryCompilerStageDefinition{
				Name:       "RewriteToCaptureValue",
				MetricName: "query_compile_stage_rewrite_to_capture_value",
				Stage:      r.rewriteQueryToCaptureValue,
			},
		},
	})
	if err != nil {
		return PreparedEvalQuery{}, err
	}

	return PreparedEvalQuery{preparedQuery{r, pCfg}}, err
}

// PrepareForPartial will parse inputs, modules, and query arguments in preparation
// of partially evaluating them.
func (r *Rego) PrepareForPartial(ctx context.Context, opts ...PrepareOption) (PreparedPartialQuery, error) {
	if !r.hasQuery() {
		return PreparedPartialQuery{}, fmt.Errorf("cannot evaluate empty query")
	}

	pCfg := &PrepareConfig{}
	for _, o := range opts {
		o(pCfg)
	}

	txn := r.txn

	var err error
	if txn == nil {
		txn, err = r.store.NewTransaction(ctx)
		if err != nil {
			return PreparedPartialQuery{}, err
		}
		defer r.store.Abort(ctx, txn)
	}

	err = r.prepare(ctx, txn, partialQueryType, []extraStage{
		{
			after: "CheckSafety",
			stage: ast.QueryCompilerStageDefinition{
				Name:       "RewriteEquals",
				MetricName: "query_compile_stage_rewrite_equals",
				Stage:      r.rewriteEqualsForPartialQueryCompile,
			},
		},
	})

	if err != nil {
		return PreparedPartialQuery{}, err
	}

	return PreparedPartialQuery{preparedQuery{r, pCfg}}, err
}

func (r *Rego) prepare(ctx context.Context, txn storage.Transaction, qType queryType, extras []extraStage) error {
	var err error
	r.parsedInput, err = r.parseInput()
	if err != nil {
		return err
	}

	r.parsedModules, err = r.parseModules(r.metrics)
	if err != nil {
		return err
	}

	// Compile the modules *before* the query, else functions
	// defined in the module won't be found...
	err = r.compileModules(ctx, txn, r.parsedModules, r.metrics)
	if err != nil {
		return err
	}

	r.parsedQuery, err = r.parseQuery(r.metrics)
	if err != nil {
		return err
	}

	return r.compileAndCacheQuery(qType, r.parsedQuery, r.metrics, extras)
}

func (r *Rego) parseModules(m metrics.Metrics) (map[string]*ast.Module, error) {
	m.Timer(metrics.RegoModuleParse).Start()
	defer m.Timer(metrics.RegoModuleParse).Stop()
	var errs Errors
	parsed := map[string]*ast.Module{}
	if r.parsedModules != nil {
		parsed = r.parsedModules
	} else {
		for _, module := range r.modules {
			p, err := module.Parse()
			if err != nil {
				errs = append(errs, err)
			}
			parsed[module.filename] = p
		}
		if len(errs) > 0 {
			return nil, errors.New(errs.Error())
		}
	}
	return parsed, nil
}

func (r *Rego) parseInput() (ast.Value, error) {
	if r.parsedInput != nil {
		return r.parsedInput, nil
	}
	return r.parseRawInput(r.rawInput, r.metrics)
}

func (r *Rego) parseRawInput(rawInput *interface{}, m metrics.Metrics) (ast.Value, error) {
	m.Timer(metrics.RegoInputParse).Start()
	defer m.Timer(metrics.RegoInputParse).Stop()
	var input ast.Value
	if rawInput != nil {
		rawPtr := util.Reference(rawInput)
		// roundtrip through json: this turns slices (e.g. []string, []bool) into
		// []interface{}, the only array type ast.InterfaceToValue can work with
		if err := util.RoundTrip(rawPtr); err != nil {
			return nil, err
		}
		val, err := ast.InterfaceToValue(*rawPtr)
		if err != nil {
			return nil, err
		}
		input = val
	}
	return input, nil
}

func (r *Rego) parseQuery(m metrics.Metrics) (ast.Body, error) {
	m.Timer(metrics.RegoQueryParse).Start()
	defer m.Timer(metrics.RegoQueryParse).Stop()

	var query ast.Body

	if r.parsedQuery != nil {
		query = r.parsedQuery
	} else {
		var err error
		query, err = ast.ParseBody(r.query)
		if err != nil {
			return nil, err
		}
	}

	return query, nil
}

func (r *Rego) compileModules(ctx context.Context, txn storage.Transaction, modules map[string]*ast.Module, m metrics.Metrics) error {

	m.Timer(metrics.RegoModuleCompile).Start()
	defer m.Timer(metrics.RegoModuleCompile).Stop()

	if len(modules) > 0 {
		r.compiler.WithPathConflictsCheck(storage.NonEmpty(ctx, r.store, txn)).Compile(modules)
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

func (r *Rego) compileAndCacheQuery(qType queryType, query ast.Body, m metrics.Metrics, extras []extraStage) error {
	m.Timer(metrics.RegoQueryCompile).Start()
	defer m.Timer(metrics.RegoQueryCompile).Stop()

	cachedQuery, ok := r.compiledQueries[qType]
	if ok && cachedQuery.query != nil && cachedQuery.compiler != nil {
		return nil
	}

	qc, compiled, err := r.compileQuery(query, m, extras)
	if err != nil {
		return err
	}

	// cache the query for future use
	r.compiledQueries[qType] = compiledQuery{
		query:    compiled,
		compiler: qc,
	}
	return nil
}

func (r *Rego) compileQuery(query ast.Body, m metrics.Metrics, extras []extraStage) (ast.QueryCompiler, ast.Body, error) {
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

	qctx := ast.NewQueryContext().
		WithPackage(pkg).
		WithImports(imports)

	qc := r.compiler.QueryCompiler().WithContext(qctx)

	for _, extra := range extras {
		qc = qc.WithStageAfter(extra.after, extra.stage)
	}

	compiled, err := qc.Compile(query)

	return qc, compiled, err

}

func (r *Rego) evalContext(input ast.Value, txn storage.Transaction) EvalContext {
	return EvalContext{
		rawInput:        r.rawInput,
		parsedInput:     input,
		metrics:         r.metrics,
		txn:             txn,
		instrument:      r.instrument,
		instrumentation: r.instrumentation,
		tracers:         r.tracers,
	}
}

func (r *Rego) eval(ctx context.Context, ectx *EvalContext) (ResultSet, error) {

	q := topdown.NewQuery(ectx.compiledQuery.query).
		WithCompiler(r.compiler).
		WithStore(r.store).
		WithTransaction(ectx.txn).
		WithMetrics(ectx.metrics).
		WithInstrumentation(ectx.instrumentation).
		WithRuntime(r.runtime)

	for i := range ectx.tracers {
		q = q.WithTracer(ectx.tracers[i])
	}

	if ectx.parsedInput != nil {
		q = q.WithInput(ast.NewTerm(ectx.parsedInput))
	}

	// Cancel query if context is cancelled or deadline is reached.
	c := topdown.NewCancel()
	q = q.WithCancel(c)
	exit := make(chan struct{})
	defer close(exit)
	go waitForDone(ctx, exit, func() {
		c.Cancel()
	})

	rewritten := ectx.compiledQuery.compiler.RewrittenVars()
	var rs ResultSet
	err := q.Iter(ctx, func(qr topdown.QueryResult) error {
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
		for _, expr := range ectx.compiledQuery.query {
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

func (r *Rego) partialResult(ctx context.Context, ectx *EvalContext, output *ast.Term) (PartialResult, error) {

	pq, err := r.partial(ctx, ectx)
	if err != nil {
		return PartialResult{}, err
	}

	// Construct module for queries.
	module := ast.MustParseModule("package " + ectx.partialNamespace)
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
		body:     ast.MustParseBody(fmt.Sprintf("data.%v.__result__", ectx.partialNamespace)),
	}

	return result, nil
}

func (r *Rego) partial(ctx context.Context, ectx *EvalContext) (*PartialQueries, error) {

	var unknowns []*ast.Term

	if ectx.parsedUnknowns != nil {
		unknowns = ectx.parsedUnknowns
	} else if ectx.unknowns != nil {
		unknowns = make([]*ast.Term, len(ectx.unknowns))
		for i := range ectx.unknowns {
			var err error
			unknowns[i], err = ast.ParseTerm(ectx.unknowns[i])
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Use input document as unknown if caller has not specified any.
		unknowns = []*ast.Term{ast.NewTerm(ast.InputRootRef)}
	}

	// Check partial namespace to ensure it's valid.
	if term, err := ast.ParseTerm(ectx.partialNamespace); err != nil {
		return nil, err
	} else if _, ok := term.Value.(ast.Var); !ok {
		return nil, fmt.Errorf("bad partial namespace")
	}

	q := topdown.NewQuery(ectx.compiledQuery.query).
		WithCompiler(r.compiler).
		WithStore(r.store).
		WithTransaction(ectx.txn).
		WithMetrics(ectx.metrics).
		WithInstrumentation(ectx.instrumentation).
		WithUnknowns(unknowns).
		WithRuntime(r.runtime)

	for i := range ectx.tracers {
		q = q.WithTracer(r.tracers[i])
	}

	if ectx.parsedInput != nil {
		q = q.WithInput(ast.NewTerm(ectx.parsedInput))
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

// rewriteEqualsForPartialQueryCompile will rewrite == to = in queries. Normally
// this wouldn't be done, except for handling queries with the `Partial` API
// where rewriting them can substantially simplify the result, and it is unlikely
// that the caller would need expression values.
func (r *Rego) rewriteEqualsForPartialQueryCompile(_ ast.QueryCompiler, query ast.Body) (ast.Body, error) {
	doubleEq := ast.Equal.Ref()
	unifyOp := ast.Equality.Ref()
	ast.WalkExprs(query, func(x *ast.Expr) bool {
		if x.IsCall() {
			operator := x.Operator()
			if operator.Equal(doubleEq) && len(x.Operands()) == 2 {
				x.SetOperator(ast.NewTerm(unifyOp))
			}
		}
		return false
	})
	return query, nil
}

func (r *Rego) generateTermVar() *ast.Term {
	r.termVarID++
	return ast.VarTerm(ast.WildcardPrefix + fmt.Sprintf("term%v", r.termVarID))
}

func (r Rego) hasQuery() bool {
	return len(r.query) != 0 || len(r.parsedQuery) != 0
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
	stage ast.QueryCompilerStageDefinition
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
