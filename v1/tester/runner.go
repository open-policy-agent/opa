// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package tester contains utilities for executing Rego tests.
package tester

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	wasm_errors "github.com/open-policy-agent/opa/internal/wasm/sdk/opa/errors"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/loader"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/util"
)

// TestPrefix declares the prefix for all test rules.
const TestPrefix = "test_"

// SkipTestPrefix declares the prefix for tests that should be skipped.
const SkipTestPrefix = "todo_test_"

// Run executes all test cases found under files in path.
func Run(ctx context.Context, paths ...string) ([]*Result, error) {
	return RunWithFilter(ctx, nil, paths...)
}

// RunWithFilter executes all test cases found under files in path. The filter
// will be applied to exclude files that should not be included.
func RunWithFilter(ctx context.Context, _ loader.Filter, paths ...string) ([]*Result, error) {
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

type SubResult struct {
	Name       string           `json:"name,omitempty"`
	Fail       bool             `json:"fail,omitempty"`
	Trace      []*topdown.Event `json:"-"`
	SubResults SubResultMap     `json:"sub_results,omitempty"`
}

type SubResultMap map[string]*SubResult

func (srm SubResultMap) Update(path ast.Array, trace []*topdown.Event) bool {
	strPath := make([]string, path.Len())
	for i := range path.Len() {
		strPath[i] = termToString(path.Elem(i))
	}
	return srm.update(strPath, 0, trace)
}

func (srm SubResultMap) update(path []string, i int, trace []*topdown.Event) bool {
	if i >= len(path) {
		return true
	}

	k := path[i]
	entry, ok := srm[k]
	if !ok {
		entry = &SubResult{
			Name:       path[i],
			Fail:       true,
			SubResults: SubResultMap{},
		}
		srm[k] = entry
	}

	if i == len(path)-1 {
		entry.Trace = trace
		return entry.Fail
	}

	fail := entry.SubResults.update(path, i+1, trace)

	if fail {
		entry.Fail = true
	}

	return fail
}

type unknownResolver struct{}

func (unknownResolver) Resolve(_ ast.Ref) (interface{}, error) {
	return "UNKNOWN", nil
}

func termToString(t *ast.Term) string {
	ti, err := ast.ValueToInterface(t.Value, unknownResolver{})
	if err != nil {
		return "INVALID"
	}
	var str string
	var ok bool
	if str, ok = ti.(string); !ok {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(ti); err != nil {
			return "INVALID"
		}
		str = strings.TrimSpace(buf.String())
	}

	return str
}

// Result represents a single test case result.
type Result struct {
	Location        *ast.Location            `json:"location"`
	Package         string                   `json:"package"`
	Name            string                   `json:"name"`
	Fail            bool                     `json:"fail,omitempty"`
	Error           error                    `json:"error,omitempty"`
	Skip            bool                     `json:"skip,omitempty"`
	Duration        time.Duration            `json:"duration"`
	Trace           []*topdown.Event         `json:"trace,omitempty"`
	Output          []byte                   `json:"output,omitempty"`
	FailedAt        *ast.Expr                `json:"failed_at,omitempty"`
	BenchmarkResult *testing.BenchmarkResult `json:"benchmark_result,omitempty"`
	SubResults      SubResultMap             `json:"sub_results,omitempty"`
}

func newResult(loc *ast.Location, pkg, name string, duration time.Duration, trace []*topdown.Event, output []byte) *Result {
	return &Result{
		Location:   loc,
		Package:    pkg,
		Name:       name,
		Duration:   duration,
		Trace:      trace,
		Output:     output,
		SubResults: SubResultMap{},
	}
}

// Pass returns true if the test case passed.
func (r *Result) Pass() bool {
	return !r.Fail && !r.Skip && r.Error == nil
}

func (r *Result) String() string {
	return r.string(true)
}

func (r *Result) string(subResults bool) string {
	if r.Skip {
		return fmt.Sprintf("%v.%v: %v", r.Package, r.Name, r.outcome())
	}
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("%v.%v: %v (%v)", r.Package, r.Name, r.outcome(), r.Duration))

	if subResults {
		buf.WriteString("\n")
		buf.WriteString(r.SubResults.String())
	}

	return buf.String()
}

func (r *Result) outcome() string {
	if r.Pass() {
		return "PASS"
	}
	if r.Fail {
		return "FAIL"
	}
	if r.Skip {
		return "SKIPPED"
	}
	return "ERROR"
}

func (sr *SubResult) String() string {
	return fmt.Sprintf("%v: %v", sr.Name, sr.outcome())
}

func (sr *SubResult) outcome() string {
	if sr.Fail {
		return "FAIL"
	}
	return "PASS"
}

// Iter is a depth-first iterator over all sub-results.
func (srm SubResultMap) Iter(yield func([]string, *SubResult) bool) {
	srm.iter(nil, yield)
}

func (srm SubResultMap) iter(namePrefix []string, yield func([]string, *SubResult) bool) {
	for _, k := range util.KeysSorted(srm) {
		sr := srm[k]

		fullName := make([]string, len(namePrefix)+1)
		copy(fullName, namePrefix)
		fullName[len(fullName)-1] = k

		if !yield(fullName, sr) {
			return
		}
		sr.SubResults.iter(fullName, yield)
	}
}

func (srm SubResultMap) String() string {
	return srm.string("  ")
}

func (srm SubResultMap) string(indent string) string {
	var buf bytes.Buffer
	for fullName, sr := range srm.Iter {
		buf.WriteString(fmt.Sprintf("%s%s\n",
			strings.Repeat(indent, len(fullName)-1),
			sr.String(),
		))
	}
	return buf.String()
}

// BenchmarkOptions defines options specific to benchmarking tests
type BenchmarkOptions struct {
	ReportAllocations bool
}

// Runner implements simple test discovery and execution.
type Runner struct {
	compiler              *ast.Compiler
	store                 storage.Store
	cover                 topdown.QueryTracer
	trace                 bool
	enablePrintStatements bool
	raiseBuiltinErrors    bool
	runtime               *ast.Term
	timeout               time.Duration
	modules               map[string]*ast.Module
	bundles               map[string]*bundle.Bundle
	filter                string
	target                string // target type (wasm, rego, etc.)
	customBuiltins        []*Builtin
	defaultRegoVersion    ast.RegoVersion
}

// NewRunner returns a new runner.
func NewRunner() *Runner {
	return &Runner{
		timeout:            5 * time.Second,
		defaultRegoVersion: ast.DefaultRegoVersion,
	}
}

// SetDefaultRegoVersion sets the default Rego version to use when compiling modules.
// Not applicable if a custom [ast.Compiler] is set via [SetCompiler].
func (r *Runner) SetDefaultRegoVersion(v ast.RegoVersion) *Runner {
	r.defaultRegoVersion = v
	return r
}

// SetCompiler sets the compiler used by the runner.
func (r *Runner) SetCompiler(compiler *ast.Compiler) *Runner {
	r.compiler = compiler
	return r
}

// RaiseBuiltinErrors sets the runner to raise errors encountered by builtins
// such as parsing input.
func (r *Runner) RaiseBuiltinErrors(enabled bool) *Runner {
	r.raiseBuiltinErrors = enabled
	return r
}

type Builtin struct {
	Decl *ast.Builtin
	Func func(*rego.Rego)
}

func (r *Runner) AddCustomBuiltins(builtinsList []*Builtin) *Runner {
	r.customBuiltins = builtinsList
	return r
}

// SetStore sets the store to execute tests over.
func (r *Runner) SetStore(store storage.Store) *Runner {
	r.store = store
	return r
}

// SetCoverageTracer sets the tracer to use to compute coverage.
// Deprecated: Use SetCoverageQueryTracer instead.
func (r *Runner) SetCoverageTracer(tracer topdown.Tracer) *Runner {
	if tracer == nil {
		return r
	}
	if qt, ok := tracer.(topdown.QueryTracer); ok {
		r.cover = qt
	} else {
		r.cover = topdown.WrapLegacyTracer(tracer)
	}
	r.trace = false
	return r
}

// SetCoverageQueryTracer sets the tracer to use to compute coverage.
func (r *Runner) SetCoverageQueryTracer(tracer topdown.QueryTracer) *Runner {
	if tracer == nil {
		return r
	}
	r.cover = tracer
	r.trace = false
	return r
}

// CapturePrintOutput captures print() call outputs during evaluation and
// includes the output in test results.
func (r *Runner) CapturePrintOutput(yes bool) *Runner {
	r.enablePrintStatements = yes
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

// Target sets the output target type to use.
func (r *Runner) Target(target string) *Runner {
	r.target = target
	return r
}

// Run executes all tests contained in supplied modules.
// Deprecated: Use RunTests and the Runner#SetModules or Runner#SetBundles
// helpers instead. This will NOT use the modules or bundles set on the Runner.
func (r *Runner) Run(ctx context.Context, modules map[string]*ast.Module) (chan *Result, error) {
	return r.SetModules(modules).RunTests(ctx, nil)
}

// RunTests executes tests found in either modules or bundles loaded on the runner.
func (r *Runner) RunTests(ctx context.Context, txn storage.Transaction) (chan *Result, error) {
	return r.runTests(ctx, txn, true, r.runTest)
}

// RunBenchmarks executes tests similar to tester.Runner#RunTests but will repeat
// a number of times to get stable performance metrics.
func (r *Runner) RunBenchmarks(ctx context.Context, txn storage.Transaction, options BenchmarkOptions) (chan *Result, error) {
	return r.runTests(ctx, txn, false, func(ctx context.Context, txn storage.Transaction, module *ast.Module, rule *ast.Rule) (result *Result, b bool) {
		return r.runBenchmark(ctx, txn, module, rule, options)
	})
}

type run func(context.Context, storage.Transaction, *ast.Module, *ast.Rule) (*Result, bool)

func (r *Runner) runTests(ctx context.Context, txn storage.Transaction, enablePrintStatements bool, runFunc run) (chan *Result, error) {
	var testRegex *regexp.Regexp
	var err error

	if r.filter != "" {
		testRegex, err = regexp.Compile(r.filter)
		if err != nil {
			return nil, err
		}
	}

	if r.compiler == nil {
		capabilities := ast.CapabilitiesForThisVersion()

		// Add custom builtins declarations to compiler
		for _, builtin := range r.customBuiltins {
			capabilities.Builtins = append(capabilities.Builtins, builtin.Decl)
		}

		r.compiler = ast.NewCompiler().
			WithCapabilities(capabilities).
			WithEnablePrintStatements(enablePrintStatements).
			WithDefaultRegoVersion(r.defaultRegoVersion)
	}

	// rewrite duplicate test_* rule names as we compile modules
	r.compiler.WithStageAfter("RewriteRuleHeadRefs", ast.CompilerStageDefinition{
		Name:       "RewriteDuplicateTestNames",
		MetricName: "rewrite_duplicate_test_names",
		Stage:      rewriteDuplicateTestNames,
	})

	r.compiler.WithStageAfter("RewriteLocalVars", ast.CompilerStageDefinition{
		Name:       "InjectTestCaseFunc",
		MetricName: "inject_test_case_func",
		Stage:      injectTestCaseFunc,
	})

	if r.store == nil {
		r.store = inmem.NewWithOpts(inmem.OptRoundTripOnWrite(false))
	}

	if len(r.bundles) > 0 {
		if txn == nil {
			return nil, errors.New("unable to activate bundles: storage transaction is nil")
		}

		// Activate the bundle(s) to get their info and policies into the store
		// the actual compiled policies will overwritten later..
		opts := &bundle.ActivateOpts{
			Ctx:           ctx,
			Store:         r.store,
			Txn:           txn,
			Compiler:      r.compiler,
			Metrics:       metrics.New(),
			Bundles:       r.bundles,
			ParserOptions: ast.ParserOptions{RegoVersion: r.defaultRegoVersion},
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

	if len(r.modules) > 0 {
		if r.compiler.Compile(r.modules); r.compiler.Failed() {
			return nil, r.compiler.Errors
		}
	}

	filenames := util.KeysSorted(r.compiler.Modules)
	ch := make(chan *Result)

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

func (*Runner) shouldRun(rule *ast.Rule, testRegex *regexp.Regexp) bool {
	var ref ast.Ref

	for _, term := range rule.Head.Ref().GroundPrefix() {
		ref = ref.Append(term)

		var n string
		switch v := term.Value.(type) {
		case ast.Var:
			n = string(v)
		case ast.String:
			n = string(v)
		default:
			n = ""
		}

		if strings.HasPrefix(n, TestPrefix) || strings.HasPrefix(n, SkipTestPrefix) {
			// Even with the prefix it needs to pass the regex (if applicable)
			fullName := rule.Module.Package.Path.Extend(ref).String()
			if testRegex != nil && !testRegex.MatchString(fullName) {
				return false
			}

			return true
		}
	}

	return false
}

// rewriteDuplicateTestNames will rewrite duplicate test names to have a numbered suffix.
// This uses a global "count" of each to ensure compiling more than once as new modules
// are added can't introduce duplicates again.
func rewriteDuplicateTestNames(compiler *ast.Compiler) *ast.Error {
	count := map[string]int{}
	for _, mod := range compiler.Modules {
		for _, rule := range mod.Rules {
			name, ref := ruleName(rule.Head)
			if !strings.HasPrefix(name, TestPrefix) {
				continue
			}

			key := mod.Package.Path.Extend(ref).String()
			if k, ok := count[key]; ok {
				dynamicSuffix := rule.Head.Ref()[len(ref):]
				newName := fmt.Sprintf("%s#%02d", name, k)
				if len(ref) == 1 {
					ref[0] = ast.VarTerm(newName)
				} else {
					ref[len(ref)-1] = ast.StringTerm(newName)
				}
				rule.Head.SetRef(append(ref, dynamicSuffix...))
			}
			count[key]++
		}
	}
	return nil
}

var testCaseFuncRef = ast.InternalTestCase.Ref()

// injectTestCaseFunc will inject a call to the 'internal.test_case' function into partial-object test rules.
// We attempt to find the earliest point in the rule body where we can inject the call, to ensure that the test-case
// function is called as early as possible so that we capture as many failed test cases as possible.
// This may require us to move generated assignment expressions up the body.
// We do not attempt to move non-generated expressions, as that could contradict author intent.
//
// Consider the test rule:
//
//	test_concat[tc.note] if {
//		some tc in [{
//			"note": "empty + empty",
//			"a": [],
//			"b": [],
//			"exp": [],
//		}]
//		act := array.concat(tc.a, tc.b)
//		act == tc.exp
//	}
//
// The compiler will rewrite this rule to (mid-stage @ 'RewriteLocalVars'):
//
//	test_concat[__local0__] := true if {
//		__local3__ = [{"a": [], "b": [], "exp": [], "note": "empty + empty"}][__local2__]
//		__local4__ = array.concat(__local3__.a, __local3__.b)
//		__local4__ == __local3__.exp
//		__local0__ = __local3__.note # generated var
//	}
//
// We move the generated var assignment as far up the body as possible, and inject the test-case function below it:
//
//	test_concat[__local0__] := true if {
//		__local3__ = [{"a": [], "b": [], "exp": [], "note": "empty + empty"}][__local2__]
//		__local0__ = __local3__.note                          # moved up
//		internal.test_case([__local0__])                      # injected
//		__local4__ = array.concat(__local3__.a, __local3__.b) # this and below expressions can now fail eval and we will still have captured the test-case
//		__local4__ == __local3__.exp
//	}
func injectTestCaseFunc(compiler *ast.Compiler) *ast.Error {
	for _, mod := range compiler.Modules {
		for _, rule := range mod.Rules {
			// Only apply to test rules
			rName, rRef := ruleName(rule.Head)
			if !strings.HasPrefix(rName, TestPrefix) {
				continue
			}

			// Only apply to rules that doesn't have manual use of the test-case function
			manualCall := false
			ast.WalkExprs(rule.Body, func(expr *ast.Expr) bool {
				if expr.IsCall() && expr.Operator().Equal(testCaseFuncRef) {
					manualCall = true
					return true
				}
				return false
			})

			if manualCall {
				continue
			}

			// Construct test-case name
			ref := rule.Head.Ref()
			if len(ref) <= len(rRef) {
				// We only inject the test-case function if there is a rule ref "tail" behind the rule name
				continue
			}
			argsRef := ref[len(rRef):]
			args := ast.NewArray(argsRef...)

			//
			// Pass 1: Move generated assignment expressions up the body
			//

			for _, term := range argsRef {
				// We expect to find generated expressions - if any - at the tail of the body, so we start from the end
				for i := len(rule.Body) - 1; i >= 0; {
					expr := rule.Body[i]
					moved := false

					// If the expression is a generated assignment of a var in the head ref, we attempt to move it as far
					// up the body as possible.
					// This is a shallow move, we don't attempt to detect multiple levels of indirection and don't move such expressions; in such case, we move the assigning expression up to the first reference.
					// Once done for all vars in the head ref, we can inject the test case function below the last (possibly moved) such expr.
					// Note: We don't move non-generated expressions, as that could contradict author intent.
					if expr.Generated && (expr.IsEquality() || expr.IsAssignment()) && expr.Operand(0).Equal(term) {
						// Based on the vars in the rhs of the expr, see if we can move it up the rule body
						// FIXME: Can we get away with just placing it under the lowes first occurrence of any referenced var?
						vars := ast.NewVarSet()
						ast.WalkVars(expr.Operand(1), func(v ast.Var) bool {
							// We only care about local vars
							if isLocalVar(v) {
								vars.Add(v)
							}
							return false
						})

						if len(vars) == 0 {
							// No local vars referenced, can be moved to top of body
							rule.Body, moved = moveExpr(rule.Body, i, 0)
						} else {
							// Find the lowest (highest up the body) individual index of each var referenced in the rhs,
							// and select the highest (lowest down the body) of those

							// TODO: Use TypedValueMap once synced with main
							lowest := ast.NewValueMap()

							for j := i - 1; j >= 0; j-- {
								expr := rule.Body[j]
								ast.WalkVars(expr, func(v ast.Var) bool {
									if vars.Contains(v) {
										// We override the value for each var, so we get the lowest index (line highest up the body) for each
										lowest.Put(v, ast.Number(strconv.Itoa(j)))
										return true
									}
									return false
								})
							}

							highest := 0
							lowest.Iter(func(k, v ast.Value) bool {
								if n, err := strconv.Atoi(string(v.(ast.Number))); err == nil {
									if n > highest {
										highest = n
									}
								}
								return false
							})

							if highest < i {
								// The expression is lower in the body than the lowes line of any expression that might contribute to its assignment
								// Move the expression to just after the lowest line
								moveTo := highest + 1
								rule.Body, moved = moveExpr(rule.Body, i, moveTo)
							}
						}
					}

					// If the expression was moved, we need to re-evaluate the current index, as it contains a new expression
					if !moved {
						i--
					}
				}
			}

			//
			// Pass 2: Inject the test-case function below the lowest first occurrence of any referenced var
			//

			injectBelowMap := ast.NewValueMap()
			for _, term := range argsRef {
				for i := len(rule.Body) - 1; i >= 0; i-- {
					expr := rule.Body[i]

					ast.WalkVars(expr, func(v ast.Var) bool {
						if term.Value.Compare(v) == 0 {
							injectBelowMap.Put(v, ast.Number(strconv.Itoa(i)))
						}
						return false
					})
				}
			}

			// Find the earliest point where the test case function can be injected
			injectBelow := -1
			injectBelowMap.Iter(func(k, v ast.Value) bool {
				if n, err := strconv.Atoi(string(v.(ast.Number))); err == nil {
					if n > injectBelow {
						injectBelow = n
					}
				}
				return false
			})

			testCaseFuncExpr := ast.NewExpr([]*ast.Term{
				ast.NewTerm(ast.InternalTestCase.Ref()),
				ast.NewTerm(args),
			})

			rule.Body = insertExpr(rule.Body, testCaseFuncExpr, injectBelow+1)
		}
	}
	return nil
}

func isLocalVar(v ast.Value) bool {
	if v, ok := v.(ast.Var); ok {
		if strings.HasPrefix(string(v), ast.LocalVarPrefix) {
			return true
		}
	}
	return false
}

func insertExpr(body ast.Body, expr *ast.Expr, index int) ast.Body {
	if index <= 0 {
		return append(ast.Body{expr}, body...)
	}

	if index >= len(body) {
		return append(body, expr)
	}

	return append(body[:index], append(ast.Body{expr}, body[index:]...)...)
}

func moveExpr(body ast.Body, from int, to int) (ast.Body, bool) {
	if from == to {
		return body, false
	}

	expr := body[from]                                                // Save the expression to move
	body = append(body[:from], body[from+1:]...)                      // Remove the expression from the body
	body = append(body[:to], append(ast.Body{expr}, body[to:]...)...) // Insert the expression at the new position
	return body, true
}

// ruleName is a helper to be used when checking if a function
// (a) is a test, or
// (b) needs to be skipped
// -- it'll resolve `p.q.r` to `r`. For representing results, we'll
// use rule.Head.Ref()
func ruleName(h *ast.Head) (string, ast.Ref) {
	var n string
	var ref ast.Ref

	for _, term := range h.Ref().GroundPrefix() {
		ref = ref.Append(term)
		switch v := term.Value.(type) {
		case ast.Var:
			n = string(v)
		case ast.String:
			n = string(v)
		default:
			n = ""
		}

		if strings.HasPrefix(n, TestPrefix) || strings.HasPrefix(n, SkipTestPrefix) {
			break
		}
	}

	return n, ref
}

func (r *Runner) runTest(ctx context.Context, txn storage.Transaction, mod *ast.Module, rule *ast.Rule) (*Result, bool) {
	ruleName, ruleRef := ruleName(rule.Head)
	if strings.HasPrefix(ruleName, SkipTestPrefix) { // TODO(sr): add test
		tr := newResult(rule.Loc(), mod.Package.Path.String(), ruleRef.String(), 0*time.Second, nil, nil)
		tr.Skip = true
		return tr, false
	}

	var bufferTracer *topdown.BufferTracer
	var tracers []topdown.QueryTracer

	if r.cover != nil {
		t := NewTestQueryTracer()
		tracers = append(tracers, r.cover, t)
		bufferTracer = &t.BufferTracer
	} else if r.trace {
		bufferTracer = topdown.NewBufferTracer()
		tracers = append(tracers, bufferTracer)
	} else {
		t := NewTestQueryTracer()
		tracers = append(tracers, t)
		bufferTracer = &t.BufferTracer
	}

	printbuf := bytes.NewBuffer(nil)
	var builtinErrors []topdown.Error
	queryPath := rule.Module.Package.Path.Extend(ruleRef)

	opts := []func(*rego.Rego){
		rego.Store(r.store),
		rego.Transaction(txn),
		rego.Compiler(r.compiler),
		rego.Query(queryPath.String()),
		rego.Runtime(r.runtime),
		rego.Target(r.target),
		rego.PrintHook(topdown.NewPrintHook(printbuf)),
		rego.BuiltinErrorList(&builtinErrors),
	}

	for _, t := range tracers {
		opts = append(opts, rego.QueryTracer(t))
	}

	rg := rego.New(opts...)

	// Register custom builtins on rego instance
	for _, v := range r.customBuiltins {
		v.Func(rg)
	}

	t0 := time.Now()
	rs, err := rg.Eval(ctx)
	dt := time.Since(t0)

	var trace []*topdown.Event
	if bufferTracer != nil {
		trace = *bufferTracer
	}

	tr := newResult(rule.Loc(), mod.Package.Path.String(), ruleRef.String(), dt, trace, printbuf.Bytes())

	// If there was an error other than errors from builtins, prefer that error.
	if err != nil {
		tr.Error = err
	} else if r.raiseBuiltinErrors && len(builtinErrors) > 0 {
		if len(builtinErrors) == 1 {
			tr.Error = &builtinErrors[0]
		} else {
			tr.Error = fmt.Errorf("%v", builtinErrors)
		}
	}

	var stop bool
	if err != nil {
		if topdown.IsCancel(err) || wasm_errors.IsCancel(err) {
			stop = ctx.Err() != context.DeadlineExceeded
		}
	} else if len(rs) == 0 {
		tr.Fail = true
	} else if rule.Head.DocKind() == ast.PartialObjectDoc {
		tr.Fail, tr.SubResults = subResults(rs[0].Expressions[0].Value, trace)
	} else if b, ok := rs[0].Expressions[0].Value.(bool); !ok || !b {
		tr.Fail = true
	}

	return tr, stop
}

func subResults(v any, trace []*topdown.Event) (bool, map[string]*SubResult) {
	if v == nil {
		return true, map[string]*SubResult{}
	}

	var fail bool
	result := SubResultMap{}

	switch x := v.(type) {
	case map[string]any:
		for k, v := range x {
			sr := subResult(k, v)
			result[k] = sr
			if sr.Fail {
				fail = true
			}
		}
	}

	// Create failed sub-results and apply per-test-case traces.
	// For each test-case event, we capture the trace from first event up until the next test-case event.
	var testEvent *topdown.Event
	for i, e := range trace {
		if e.Op == topdown.TestCaseOp {
			if testEvent != nil {
				if p, ok := testCaseTerms(testEvent); ok {
					if f := result.Update(*p, trace[:i]); f {
						fail = true
					}
				}
			}

			testEvent = e
		}
	}
	if testEvent != nil {
		if p, ok := testCaseTerms(testEvent); ok {
			if f := result.Update(*p, trace); f {
				fail = true
			}
		}
	}

	return fail, result
}

func testCaseTerms(e *topdown.Event) (*ast.Array, bool) {
	if e == nil {
		return nil, false
	}

	if expr, ok := e.Node.(*ast.Expr); ok {
		if arr, ok := expr.Operand(0).Value.(*ast.Array); ok {
			return arr, true
		}
	}

	return nil, false
}

func subResult(n string, v any) *SubResult {
	if v == nil {
		return &SubResult{}
	}

	switch x := v.(type) {
	case map[string]any:
		fail, srs := subResults(x, nil)
		return &SubResult{
			Name:       n,
			Fail:       fail,
			SubResults: srs,
		}
	case bool:
		return &SubResult{
			Name: n,
			Fail: !x,
		}
	default:
		return &SubResult{
			Name: n,
			Fail: true,
		}
	}
}

func (r *Runner) runBenchmark(ctx context.Context, txn storage.Transaction, mod *ast.Module, rule *ast.Rule, options BenchmarkOptions) (*Result, bool) {
	_, rf := ruleName(rule.Head)

	tr := &Result{
		Location: rule.Loc(),
		Package:  mod.Package.Path.String(),
		Name:     rf.String(), // TODO(sr): test
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
			rego.Target(r.target),
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

		for range b.N {
			opts := []rego.EvalOption{
				rego.EvalTransaction(txn),
				rego.EvalMetrics(m),
			}

			var tracer *TestQueryTracer
			if rule.Head.DocKind() == ast.PartialObjectDoc {
				tracer = NewTestQueryTracer()
				opts = append(opts, rego.EvalQueryTracer(tracer))
			}

			// Start the timer (might already be started, but that's ok)
			b.StartTimer()

			rs, err := pq.Eval(
				ctx,
				opts...,
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
			} else if rule.Head.DocKind() == ast.PartialObjectDoc {
				tr.Fail, tr.SubResults = subResults(rs[0].Expressions[0].Value, tracer.Events())
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
	return LoadWithRegoVersion(args, filter, ast.DefaultRegoVersion)
}

// LoadWithRegoVersion returns modules and an in-memory store for running tests.
// Modules are parsed in accordance with the given RegoVersion.
func LoadWithRegoVersion(args []string, filter loader.Filter, regoVersion ast.RegoVersion) (map[string]*ast.Module, storage.Store, error) {
	if regoVersion == ast.RegoUndefined {
		regoVersion = ast.DefaultRegoVersion
	}

	loaded, err := loader.NewFileLoader().
		WithRegoVersion(regoVersion).
		WithProcessAnnotation(true).
		Filtered(args, filter)
	if err != nil {
		return nil, nil, err
	}
	store := inmem.NewFromObject(loaded.Documents)
	modules := make(map[string]*ast.Module, len(loaded.Modules))
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

// LoadWithParserOptions returns modules and an in-memory store for running tests.
// Modules are parsed in accordance with the given [ast.ParserOptions].
func LoadWithParserOptions(args []string, filter loader.Filter, popts ast.ParserOptions) (map[string]*ast.Module, storage.Store, error) {
	loaded, err := loader.NewFileLoader().
		WithRegoVersion(popts.RegoVersion).
		WithCapabilities(popts.Capabilities).
		WithProcessAnnotation(popts.ProcessAnnotation).
		Filtered(args, filter)
	if err != nil {
		return nil, nil, err
	}
	store := inmem.NewFromObject(loaded.Documents)
	modules := make(map[string]*ast.Module, len(loaded.Modules))
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
	return LoadBundlesWithRegoVersion(args, filter, ast.RegoV0)
}

// LoadBundlesWithRegoVersion will load the given args as bundles, either tarball or directory is OK.
// Bundles are parsed in accordance with the given RegoVersion.
func LoadBundlesWithRegoVersion(args []string, filter loader.Filter, regoVersion ast.RegoVersion) (map[string]*bundle.Bundle, error) {
	if regoVersion == ast.RegoUndefined {
		regoVersion = ast.DefaultRegoVersion
	}

	bundles := make(map[string]*bundle.Bundle, len(args))
	for _, bundleDir := range args {
		b, err := loader.NewFileLoader().
			WithRegoVersion(regoVersion).
			WithProcessAnnotation(true).
			WithSkipBundleVerification(true).
			WithFilter(filter).
			AsBundle(bundleDir)
		if err != nil {
			return nil, fmt.Errorf("unable to load bundle %s: %s", bundleDir, err)
		}
		bundles[bundleDir] = b
	}

	return bundles, nil
}

// LoadBundlesWithParserOptions will load the given args as bundles, either tarball or directory is OK.
// Bundles are parsed in accordance with the given [ast.ParserOptions].
func LoadBundlesWithParserOptions(args []string, filter loader.Filter, popts ast.ParserOptions) (map[string]*bundle.Bundle, error) {
	if popts.RegoVersion == ast.RegoUndefined {
		popts.RegoVersion = ast.DefaultRegoVersion
	}

	bundles := make(map[string]*bundle.Bundle, len(args))
	for _, bundleDir := range args {
		b, err := loader.NewFileLoader().
			WithRegoVersion(popts.RegoVersion).
			WithCapabilities(popts.Capabilities).
			WithProcessAnnotation(popts.ProcessAnnotation).
			WithSkipBundleVerification(true).
			WithFilter(filter).
			AsBundle(bundleDir)
		if err != nil {
			return nil, fmt.Errorf("unable to load bundle %s: %s", bundleDir, err)
		}
		bundles[bundleDir] = b
	}

	return bundles, nil
}
