// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package tester contains utilities for executing Rego tests.
package tester

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/metrics"

	"github.com/open-policy-agent/opa/bundle"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
)

// TestPrefix declares the prefix for all test rules.
const TestPrefix = "test_"

// Run executes all test cases found under files in path.
func Run(ctx context.Context, paths ...string) ([]*Result, error) {
	return RunWithFilter(ctx, nil, paths...)
}

// RunWithFilter executes all test cases found under files in path. The filter
// will be applied to exclude files that should not be included.
func RunWithFilter(ctx context.Context, filter loader.Filter, paths ...string) ([]*Result, error) {
	modules, store, err := Load(paths, nil)
	if err != nil {
		return nil, err
	}
	ch, err := NewRunner().SetStore(store).Run(ctx, modules)
	if err != nil {
		return nil, err
	}
	result := []*Result{}
	for r := range ch {
		result = append(result, r)
	}
	return result, nil
}

// Result represents a single test case result.
type Result struct {
	Location        *ast.Location            `json:"location"`
	Package         string                   `json:"package"`
	Name            string                   `json:"name"`
	Fail            bool                     `json:"fail,omitempty"`
	Error           error                    `json:"error,omitempty"`
	Duration        time.Duration            `json:"duration"`
	Trace           []*topdown.Event         `json:"trace,omitempty"`
	FailedAt        *ast.Expr                `json:"failed_at,omitempty"`
	BenchmarkResult *testing.BenchmarkResult `json:"benchmark_result,omitempty"`
}

func newResult(loc *ast.Location, pkg, name string, duration time.Duration, trace []*topdown.Event) *Result {
	return &Result{
		Location: loc,
		Package:  pkg,
		Name:     name,
		Duration: duration,
		Trace:    trace,
	}
}

// Pass returns true if the test case passed.
func (r Result) Pass() bool {
	return !r.Fail && r.Error == nil
}

func (r *Result) String() string {
	return fmt.Sprintf("%v.%v: %v (%v)", r.Package, r.Name, r.outcome(), r.Duration)
}

func (r *Result) outcome() string {
	if r.Pass() {
		return "PASS"
	}
	if r.Fail {
		return "FAIL"
	}
	return "ERROR"
}

// BenchmarkOptions defines options specific to benchmarking tests
type BenchmarkOptions struct {
	ReportAllocations bool
}

// Runner implements simple test discovery and execution.
type Runner struct {
	compiler    *ast.Compiler
	store       storage.Store
	cover       topdown.Tracer
	trace       bool
	runtime     *ast.Term
	failureLine bool
	timeout     time.Duration
	modules     map[string]*ast.Module
	bundles     map[string]*bundle.Bundle
	filter      string
}

// NewRunner returns a new runner.
func NewRunner() *Runner {
	return &Runner{
		timeout: 5 * time.Second,
	}
}

// SetCompiler sets the compiler used by the runner.
func (r *Runner) SetCompiler(compiler *ast.Compiler) *Runner {
	r.compiler = compiler
	return r
}

// SetStore sets the store to execute tests over.
func (r *Runner) SetStore(store storage.Store) *Runner {
	r.store = store
	return r
}

// SetCoverageTracer sets the tracer to use to compute coverage.
func (r *Runner) SetCoverageTracer(tracer topdown.Tracer) *Runner {
	r.cover = tracer
	if r.cover != nil {
		r.trace = false
	}
	return r
}

// EnableTracing enables tracing of evaluation and includes traces in results.
// Tracing is currently mutually exclusive with coverage.
func (r *Runner) EnableTracing(yes bool) *Runner {
	r.trace = yes
	if r.trace {
		r.cover = nil
	}
	return r
}

// EnableFailureLine if set will provide the exact failure line
func (r *Runner) EnableFailureLine(yes bool) *Runner {
	r.failureLine = yes
	return r
}

// SetRuntime sets runtime information to expose to the evaluation engine.
func (r *Runner) SetRuntime(term *ast.Term) *Runner {
	r.runtime = term
	return r
}

// SetTimeout sets the timeout for the individual test cases
func (r *Runner) SetTimeout(timout time.Duration) *Runner {
	r.timeout = timout
	return r
}

// SetModules will add modules to the Runner which will be compiled then used
// for discovering and evaluating tests.
func (r *Runner) SetModules(modules map[string]*ast.Module) *Runner {
	r.modules = modules
	return r
}

// SetBundles will add bundles to the Runner which will be compiled then used
// for discovering and evaluating tests.
func (r *Runner) SetBundles(bundles map[string]*bundle.Bundle) *Runner {
	r.bundles = bundles
	return r
}

// Filter will set a test name regex filter for the test runner. Only test
// cases which match the filter will be run.
func (r *Runner) Filter(regex string) *Runner {
	r.filter = regex
	return r
}

func getFailedAtFromTrace(bufFailureLineTracer *topdown.BufferTracer) *ast.Expr {
	events := *bufFailureLineTracer
	const SecondToLast = 2
	eventsLen := len(events)
	for i, opFail := eventsLen-1, 0; i >= 0; i-- {
		if events[i].Op == topdown.FailOp {
			opFail++
		}
		if opFail == SecondToLast {
			return events[i].Node.(*ast.Expr)
		}
	}
	return nil
}

// Run executes all tests contained in supplied modules.
// Deprecated: Use RunTests and the Runner#SetModules or Runner#SetBundles
// helpers instead. This will NOT use the modules or bundles set on the Runner.
func (r *Runner) Run(ctx context.Context, modules map[string]*ast.Module) (ch chan *Result, err error) {
	return r.SetModules(modules).RunTests(ctx, nil)
}

// RunTests executes tests found in either modules or bundles loaded on the runner.
func (r *Runner) RunTests(ctx context.Context, txn storage.Transaction) (ch chan *Result, err error) {
	return r.runTests(ctx, txn, r.runTest)
}

// RunBenchmarks executes tests similar to tester.Runner#RunTests but will repeat
// a number of times to get stable performance metrics.
func (r *Runner) RunBenchmarks(ctx context.Context, txn storage.Transaction, options BenchmarkOptions) (ch chan *Result, err error) {
	return r.runTests(ctx, txn, func(ctx context.Context, txn storage.Transaction, module *ast.Module, rule *ast.Rule) (result *Result, b bool) {
		return r.runBenchmark(ctx, txn, module, rule, options)
	})
}

func (r *Runner) runTests(ctx context.Context, txn storage.Transaction, runFunc func(context.Context, storage.Transaction, *ast.Module, *ast.Rule) (*Result, bool)) (ch chan *Result, err error) {
	var testRegex *regexp.Regexp
	if r.filter != "" {
		var err error
		testRegex, err = regexp.Compile(r.filter)
		if err != nil {
			return nil, err
		}
	}

	if r.compiler == nil {
		r.compiler = ast.NewCompiler()
	}

	// rewrite duplicate test_* rule names as we compile modules
	r.compiler.WithStageAfter("ResolveRefs", ast.CompilerStageDefinition{
		Name:       "RewriteDuplicateTestNames",
		MetricName: "rewrite_duplicate_test_names",
		Stage:      rewriteDuplicateTestNames,
	})

	if r.store == nil {
		r.store = inmem.New()
	}

	if r.bundles != nil && len(r.bundles) > 0 {
		if txn == nil {
			return nil, fmt.Errorf("unable to activate bundles: storage transaction is nil")
		}

		// Activate the bundle(s) to get their info and policies into the store
		// the actual compiled policies will overwritten later..
		opts := &bundle.ActivateOpts{
			Ctx:      ctx,
			Store:    r.store,
			Txn:      txn,
			Compiler: r.compiler,
			Metrics:  metrics.New(),
			Bundles:  r.bundles,
		}
		err = bundle.Activate(opts)
		if err != nil {
			return nil, err
		}

		// Aggregate the bundle modules with other ones provided
		if r.modules == nil {
			r.modules = map[string]*ast.Module{}
		}
		for path, b := range r.bundles {
			for name, mod := range b.ParsedModules(path) {
				r.modules[name] = mod
			}
		}
	}

	if r.modules != nil && len(r.modules) > 0 {
		if r.compiler.Compile(r.modules); r.compiler.Failed() {
			return nil, r.compiler.Errors
		}
	}

	filenames := make([]string, 0, len(r.compiler.Modules))
	for name := range r.compiler.Modules {
		filenames = append(filenames, name)
	}

	sort.Strings(filenames)

	ch = make(chan *Result)

	go func() {
		defer close(ch)
		for _, name := range filenames {
			module := r.compiler.Modules[name]
			for _, rule := range module.Rules {
				if !r.shouldRun(rule, testRegex) {
					continue
				}
				tr, stop := func() (*Result, bool) {
					runCtx, cancel := context.WithTimeout(ctx, r.timeout)
					defer cancel()
					return runFunc(runCtx, txn, module, rule)
				}()
				ch <- tr
				if stop {
					return
				}
			}
		}
	}()

	return ch, nil
}

func (r *Runner) shouldRun(rule *ast.Rule, testRegex *regexp.Regexp) bool {
	ruleName := string(rule.Head.Name)

	// All tests must have the right prefix
	if !strings.HasPrefix(ruleName, TestPrefix) {
		return false
	}

	// Even with the prefix it needs to pass the regex (if applicable)
	fullName := fmt.Sprintf("%s.%s", rule.Module.Package.Path.String(), ruleName)
	if testRegex != nil && !testRegex.MatchString(fullName) {
		return false
	}

	return true
}

// rewriteDuplicateTestNames will rewrite duplicate test names to have a numbered suffix.
// This uses a global "count" of each to ensure compiling more than once as new modules
// are added can't introduce duplicates again.
func rewriteDuplicateTestNames(compiler *ast.Compiler) *ast.Error {
	count := map[string]int{}
	for _, mod := range compiler.Modules {
		for _, rule := range mod.Rules {
			name := rule.Head.Name.String()
			if !strings.HasPrefix(name, TestPrefix) {
				continue
			}
			key := rule.Path().String()
			if k, ok := count[key]; ok {
				rule.Head.Name = ast.Var(fmt.Sprintf("%s#%02d", name, k))
			}
			count[key]++
		}
	}
	return nil
}

func (r *Runner) runTest(ctx context.Context, txn storage.Transaction, mod *ast.Module, rule *ast.Rule) (*Result, bool) {

	var bufferTracer *topdown.BufferTracer
	var bufFailureLineTracer *topdown.BufferTracer
	var tracer topdown.Tracer

	if r.cover != nil {
		tracer = r.cover
	} else if r.trace {
		bufferTracer = topdown.NewBufferTracer()
		tracer = bufferTracer
	} else if r.failureLine {
		bufFailureLineTracer = topdown.NewBufferTracer()
		tracer = bufFailureLineTracer
	}

	rego := rego.New(
		rego.Store(r.store),
		rego.Transaction(txn),
		rego.Compiler(r.compiler),
		rego.Query(rule.Path().String()),
		rego.Tracer(tracer),
		rego.Runtime(r.runtime),
	)

	t0 := time.Now()
	rs, err := rego.Eval(ctx)
	dt := time.Since(t0)

	var trace []*topdown.Event

	if bufferTracer != nil {
		trace = *bufferTracer
	}

	tr := newResult(rule.Loc(), mod.Package.Path.String(), string(rule.Head.Name), dt, trace)
	var stop bool

	if err != nil {
		tr.Error = err
		if topdown.IsCancel(err) && !(ctx.Err() == context.DeadlineExceeded) {
			stop = true
		}
	} else if len(rs) == 0 {
		tr.Fail = true
		if bufFailureLineTracer != nil {
			tr.FailedAt = getFailedAtFromTrace(bufFailureLineTracer)
		}
	} else if b, ok := rs[0].Expressions[0].Value.(bool); !ok || !b {
		tr.Fail = true
	}

	return tr, stop
}

func (r *Runner) runBenchmark(ctx context.Context, txn storage.Transaction, mod *ast.Module, rule *ast.Rule, options BenchmarkOptions) (*Result, bool) {
	tr := &Result{
		Location: rule.Loc(),
		Package:  mod.Package.Path.String(),
		Name:     string(rule.Head.Name),
	}

	var stop bool

	t0 := time.Now()

	br := testing.Benchmark(func(b *testing.B) {

		pq, err := rego.New(
			rego.Store(r.store),
			rego.Transaction(txn),
			rego.Compiler(r.compiler),
			rego.Query(rule.Path().String()),
			rego.Runtime(r.runtime),
		).PrepareForEval(ctx)

		if err != nil {
			tr.Fail = true
			b.Fatalf("Unexpected error: %s", err)
		}

		m := metrics.New()

		// Track memory allocations
		if options.ReportAllocations {
			b.ReportAllocs()
		}

		// Don't count setup in the benchmark time, only evaluation time
		b.ResetTimer()

		for i := 0; i < b.N; i++ {

			// Start the timer (might already be started, but that's ok)
			b.StartTimer()

			rs, err := pq.Eval(
				ctx,
				rego.EvalTransaction(txn),
				rego.EvalMetrics(m),
			)

			// Stop the timer so we don't count any of the error handling time
			b.StopTimer()

			if err != nil {
				tr.Error = err
				if topdown.IsCancel(err) && !(ctx.Err() == context.DeadlineExceeded) {
					stop = true
				}
				b.Fatalf("Unexpected error: %s", err)
			} else if len(rs) == 0 {
				tr.Fail = true
				b.Fatal("Expected boolean result, got `undefined`")
			} else if pass, ok := rs[0].Expressions[0].Value.(bool); !ok || !pass {
				tr.Fail = true
				b.Fatal("Expected test to evaluate as true, got false")
			}
		}

		for k, v := range m.All() {
			fv := float64(v.(int64)) / float64(b.N)
			b.ReportMetric(fv, k+"/op")
		}
	})

	tr.Duration = time.Since(t0)
	tr.BenchmarkResult = &br

	return tr, stop
}

// Load returns modules and an in-memory store for running tests.
func Load(args []string, filter loader.Filter) (map[string]*ast.Module, storage.Store, error) {
	loaded, err := loader.NewFileLoader().Filtered(args, filter)
	if err != nil {
		return nil, nil, err
	}
	store := inmem.NewFromObject(loaded.Documents)
	modules := map[string]*ast.Module{}
	ctx := context.Background()
	err = storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		for _, loadedModule := range loaded.Modules {
			modules[loadedModule.Name] = loadedModule.Parsed

			// Add the policies to the store to ensure that any future bundle
			// activations will preserve them and re-compile the module with
			// the bundle modules.
			err := store.UpsertPolicy(ctx, txn, loadedModule.Name, loadedModule.Raw)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return modules, store, err
}

// LoadBundles will load the given args as bundles, either tarball or directory is OK.
func LoadBundles(args []string, filter loader.Filter) (map[string]*bundle.Bundle, error) {
	bundles := map[string]*bundle.Bundle{}
	for _, bundleDir := range args {
		b, err := loader.NewFileLoader().AsBundle(bundleDir)
		if err != nil {
			return nil, fmt.Errorf("unable to load bundle %s: %s", bundleDir, err)
		}
		bundles[bundleDir] = b
	}

	return bundles, nil
}
