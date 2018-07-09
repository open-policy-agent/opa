// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package tester contains utilities for executing Rego tests.
package tester

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
)

// TestPrefix declares the prefix for all rules.
const TestPrefix = "test_"

// Undefined declares the constant used to identify undefined results in TestFile
const Undefined = math.MaxInt32

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
	Location *ast.Location `json:"location"`
	Package  string        `json:"package"`
	Name     string        `json:"name"`
	Fail     *interface{}  `json:"fail,omitempty"`
	Error    error         `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

func newResult(loc *ast.Location, pkg, name string, duration time.Duration) *Result {
	return &Result{
		Location: loc,
		Package:  pkg,
		Name:     name,
		Duration: duration,
	}
}

// Pass returns true if the test case passed.
func (r Result) Pass() bool {
	return r.Fail == nil && r.Error == nil
}

func (r *Result) String() string {
	return fmt.Sprintf("%v.%v: %v (%v)", r.Package, r.Name, r.outcome(), r.Duration/time.Microsecond)
}

func (r *Result) outcome() string {
	if r.Pass() {
		return "PASS"
	}
	if r.Fail != nil {
		return "FAIL"
	}
	return "ERROR"
}

func (r *Result) setFail(fail interface{}) {
	r.Fail = &fail
}

// Runner implements simple test discovery and execution.
type Runner struct {
	compiler *ast.Compiler
	store    storage.Store
	tracer   topdown.Tracer
}

// NewRunner returns a new runner.
func NewRunner() *Runner {
	return &Runner{}
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

// SetTracer sets the tracer to use during test execution.
func (r *Runner) SetTracer(tracer topdown.Tracer) *Runner {
	r.tracer = tracer
	return r
}

// Run executes all tests contained in supplied modules.
func (r *Runner) Run(ctx context.Context, modules map[string]*ast.Module) (ch chan *Result, err error) {

	if r.compiler == nil {
		r.compiler = ast.NewCompiler()
	}

	if r.store == nil {
		r.store = inmem.New()
	}

	filenames := make([]string, 0, len(modules))
	for name := range modules {
		filenames = append(filenames, name)
	}

	sort.Strings(filenames)

	if r.compiler.Compile(modules); r.compiler.Failed() {
		return nil, r.compiler.Errors
	}

	ch = make(chan *Result)

	go func() {
		defer close(ch)
		for _, name := range filenames {
			module := r.compiler.Modules[name]
			for _, rule := range module.Rules {
				if !strings.HasPrefix(string(rule.Head.Name), TestPrefix) {
					continue
				}
				tr, stop := r.runTest(ctx, module, rule)
				ch <- tr
				if stop {
					return
				}
			}
		}
	}()

	return ch, nil
}

func (r *Runner) runTest(ctx context.Context, mod *ast.Module, rule *ast.Rule) (*Result, bool) {

	rego := rego.New(
		rego.Store(r.store),
		rego.Compiler(r.compiler),
		rego.Query(rule.Path().String()),
		rego.Tracer(r.tracer),
	)

	t0 := time.Now()
	rs, err := rego.Eval(ctx)
	dt := time.Since(t0)

	tr := newResult(rule.Loc(), mod.Package.Path.String(), string(rule.Head.Name), dt)
	var stop bool

	if err != nil {
		tr.Error = err
		if topdown.IsCancel(err) {
			stop = true
		}
	} else if len(rs) == 0 {
		tr.setFail(false)
	} else if b, ok := rs[0].Expressions[0].Value.(bool); !ok || !b {
		tr.setFail(rs[0].Expressions[0].Value)
	}

	return tr, stop
}

// Load returns modules and an in-memory store for running tests.
func Load(args []string, filter loader.Filter) (map[string]*ast.Module, storage.Store, error) {
	loaded, err := loader.Filtered(args, filter)
	if err != nil {
		return nil, nil, err
	}
	store := inmem.NewFromObject(loaded.Documents)
	modules := map[string]*ast.Module{}
	for _, loadedModule := range loaded.Modules {
		modules[loadedModule.Name] = loadedModule.Parsed
	}
	return modules, store, nil
}

// TestFile loads all the data in paths, runs query with inputs and ctx and checks that the result equals expected.
// Returns err if this is not the case.
// The expected value can be of 3 types:
// 1) tracer.Undefined -- this will only match the result if the query returns undefined
// 2) any JSON type -- this will match the result if they are equivalent JSON objects
// 3) any error -- will match result if an error was produced during evaluation that contains the expected error as a substring
// If the expected value does not fall into one of these three types (a channel, for instance) this function will panic.
// This is for testing purposes only.
func TestFile(ctx context.Context, query string, expected interface{}, inputs map[string]interface{}, paths ...string) error {

	modules, store, err := Load(paths, nil)
	if err != nil {
		return isExpectedError(err, expected)
	}

	cmp := ast.NewCompiler()
	if cmp.Compile(modules); cmp.Failed() {
		return isExpectedError(err, expected)
	}

	if ctx == nil {
		ctx = context.Background()
	}

	rg := rego.New(
		rego.Query(query),
		rego.Compiler(cmp),
		rego.Store(store),
		rego.Input(inputs),
	)

	rs, err := rg.Eval(ctx)
	if err != nil {
		return isExpectedError(err, expected)
	}

	if expected == Undefined {
		if len(rs) != 0 {
			return fmt.Errorf("Expected: %v\nGot: %v", "undefined", rs)
		}
		return nil
	}

	if len(rs) == 0 {
		return fmt.Errorf("Expected: %v\nGot: %v", expected, "undefined")
	}

	// compare the two
	if len(rs[0].Expressions) == 0 {
		return fmt.Errorf("no expressions found upon evaluation")
	}

	result := rs[0].Expressions[0].Value
	if !util.AreEqualJSON(expected, result) {
		return fmt.Errorf("Expected: %v\nGot: %v", expected, result)
	}
	return nil
}

// returns nil if err was anticipated by expected, error otherwise.
// expects that err is non-nil
func isExpectedError(err error, expected interface{}) error {
	if exp, ok := expected.(error); ok {
		if !strings.Contains(err.Error(), exp.Error()) {
			return fmt.Errorf("expected error %v but got: %v", exp.Error(), err.Error())
		}
		return nil
	}
	return fmt.Errorf("unexpected error: %v", err.Error())
}
