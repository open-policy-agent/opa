// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tester_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/cover"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/tester"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/types"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestRun(t *testing.T) {
	testRun(t, testRunConfig{})
}

func TestRunBenchmark(t *testing.T) {
	testRun(t, testRunConfig{bench: true})
}

func TestRunWithCoverage(t *testing.T) {
	cov := cover.New()
	modules := testRun(t, testRunConfig{coverTracer: cov})
	report := cov.Report(modules)
	if len(report.Files) != len(modules) {
		t.Errorf("Expected %d files in coverage report, got %d", len(modules), len(report.Files))
	}
	if report.Coverage == 0 {
		t.Error("Expected test coverage")
	}
}

type expectedTestResult struct {
	wantErr  bool
	wantFail bool
	// nolint: structcheck // The test doesn't check this value, but should.
	wantSkip bool
}

type testRunConfig struct {
	bench       bool
	filter      string
	coverTracer topdown.QueryTracer
}

type expectedTestResults map[[2]string]expectedTestResult

func testRun(t *testing.T, conf testRunConfig) map[string]*ast.Module {
	files := map[string]string{
		"/a.rego": `package foo
			import rego.v1
			
			allow if { true }
			`,
		"/a_test.rego": `package foo
			import rego.v1
			
			test_pass if { allow }
			non_test if { true }
			test_fail if { not allow }
			test_fail_non_bool = 100
			test_err if { conflict }
			conflict = true
			conflict = false
			test_duplicate if { false }
			test_duplicate if { true }
			test_duplicate if { true }
			todo_test_skip if { true }
			`,
		"/b_test.rego": `package bar
			import rego.v1
			
			test_duplicate if { true }`,
		"/c_test.rego": `package baz
			import rego.v1

			a.b.test_duplicate if { false }
			a.b.test_duplicate if { true }
			a.b.test_duplicate if { true }`,
		// Regression test for issue #5496.
		"/d_test.rego": `package test
		import rego.v1

		a[0] := 1
		test_pass if { true }`,
	}

	tests := expectedTestResults{
		{"data.foo", "test_pass"}:                  {false, false, false},
		{"data.foo", "test_fail"}:                  {false, true, false},
		{"data.foo", "test_fail_non_bool"}:         {false, true, false},
		{"data.foo", "test_duplicate"}:             {false, true, false},
		{"data.foo", "test_duplicate#01"}:          {false, false, false},
		{"data.foo", "test_duplicate#02"}:          {false, false, false},
		{"data.foo", "test_err"}:                   {true, false, false},
		{"data.foo", "todo_test_skip"}:             {false, false, true},
		{"data.bar", "test_duplicate"}:             {false, false, false},
		{"data.baz", "a.b.test_duplicate"}:         {false, true, false},
		{"data.baz", "a.b[\"test_duplicate#01\"]"}: {false, false, false},
		{"data.baz", "a.b[\"test_duplicate#02\"]"}: {false, false, false},
		{"data.test", "test_pass"}:                 {false, false, false},
	}

	var modules map[string]*ast.Module
	test.WithTempFS(files, func(d string) {
		var rs []*tester.Result
		rs, modules = doTestRunWithTmpDir(t, d, conf)
		validateTestResults(t, tests, rs, conf)
	})
	return modules
}

func doTestRunWithTmpDir(t *testing.T, dir string, conf testRunConfig) ([]*tester.Result, map[string]*ast.Module) {
	t.Helper()

	ctx := context.Background()

	paths := []string{dir}
	modules, store, err := tester.Load(paths, nil)
	if err != nil {
		t.Fatal(err)
	}

	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	runner := tester.NewRunner().
		SetStore(store).
		SetModules(modules).
		Filter(conf.filter).
		SetTimeout(60 * time.Second).
		SetCoverageQueryTracer(conf.coverTracer)

	var ch chan *tester.Result
	if conf.bench {
		ch, err = runner.RunBenchmarks(ctx, txn, tester.BenchmarkOptions{})
	} else {
		ch, err = runner.RunTests(ctx, txn)
	}
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	var rs []*tester.Result
	for r := range ch {
		rs = append(rs, r)
	}

	return rs, modules
}

func validateTestResults(t *testing.T, tests expectedTestResults, rs []*tester.Result, conf testRunConfig) {
	t.Helper()
	seen := map[[2]string]struct{}{}
	for i := range rs {
		k := [2]string{rs[i].Package, rs[i].Name}
		seen[k] = struct{}{}
		exp, ok := tests[k]
		if !ok {
			t.Errorf("Unexpected result for %v", k)
		} else if exp.wantErr != (rs[i].Error != nil) || exp.wantFail != rs[i].Fail {
			t.Errorf("Expected %+v for %v but got: %v", exp, k, rs[i])
		} else {
			// Test passed
			if conf.bench && rs[i].BenchmarkResult == nil {
				t.Errorf("Expected BenchmarkResult for test %v, got nil", k)
			} else if !conf.bench && rs[i].BenchmarkResult != nil {
				t.Errorf("Unexpected BenchmarkResult for test %v, expected nil", k)
			}
		}
	}
	for k := range tests {
		if _, ok := seen[k]; !ok {
			t.Errorf("Expected result for %v", k)
		}
	}
}

func TestRunWithFilterRegex(t *testing.T) {
	files := map[string]string{
		"/a.rego": `package foo
			import rego.v1
			
			allow if { true }
			`,
		"/a_test.rego": `package foo
			import rego.v1
			
			test_pass if { allow }
			non_test if { true }
			test_fail if { not allow }
			test_fail_non_bool = 100
			test_err if { conflict }
			conflict = true
			conflict = false
			test_duplicate if { false }
			test_duplicate if { true }
			test_duplicate if { true }
			todo_test_skip if { true }
			todo_test_skip_too if { false }
			`,
		"/b_test.rego": `package bar
			import rego.v1
			
			test_duplicate if { true }`,
		"/c_test.rego": `package baz
			import rego.v1
			
			a.b.test_duplicate if { false }
			a.b.test_duplicate if { true }
			a.b.test_duplicate if { true }`,
	}

	cases := []struct {
		note  string
		regex string
		tests expectedTestResults
	}{
		{
			note:  "all tests match",
			regex: ".*",
			tests: expectedTestResults{
				{"data.foo", "test_pass"}:                  {false, false, false},
				{"data.foo", "test_fail"}:                  {false, true, false},
				{"data.foo", "test_fail_non_bool"}:         {false, true, false},
				{"data.foo", "test_duplicate"}:             {false, true, false},
				{"data.foo", "test_duplicate#01"}:          {false, false, false},
				{"data.foo", "test_duplicate#02"}:          {false, false, false},
				{"data.foo", "test_err"}:                   {true, false, false},
				{"data.foo", "todo_test_skip"}:             {false, false, true},
				{"data.foo", "todo_test_skip_too"}:         {false, false, true},
				{"data.bar", "test_duplicate"}:             {false, false, false},
				{"data.baz", "a.b.test_duplicate"}:         {false, true, false},
				{"data.baz", "a.b[\"test_duplicate#01\"]"}: {false, false, false},
				{"data.baz", "a.b[\"test_duplicate#02\"]"}: {false, false, false},
			},
		},
		{
			note:  "no filter",
			regex: "",
			tests: expectedTestResults{
				{"data.foo", "test_pass"}:                  {false, false, false},
				{"data.foo", "test_fail"}:                  {false, true, false},
				{"data.foo", "test_fail_non_bool"}:         {false, true, false},
				{"data.foo", "test_duplicate"}:             {false, true, false},
				{"data.foo", "test_duplicate#01"}:          {false, false, false},
				{"data.foo", "test_duplicate#02"}:          {false, false, false},
				{"data.foo", "test_err"}:                   {true, false, false},
				{"data.foo", "todo_test_skip"}:             {false, false, true},
				{"data.foo", "todo_test_skip_too"}:         {false, false, true},
				{"data.bar", "test_duplicate"}:             {false, false, false},
				{"data.baz", "a.b.test_duplicate"}:         {false, true, false},
				{"data.baz", "a.b[\"test_duplicate#01\"]"}: {false, false, false},
				{"data.baz", "a.b[\"test_duplicate#02\"]"}: {false, false, false},
			},
		},
		{
			note:  "no tests match",
			regex: "^$",
			tests: nil,
		},
		{
			note:  "single package name",
			regex: "bar",
			tests: expectedTestResults{
				{"data.bar", "test_duplicate"}: {false, false, false},
			},
		},
		{
			note:  "single package explicit",
			regex: "data.bar.test_duplicate",
			tests: expectedTestResults{
				{"data.bar", "test_duplicate"}: {false, false, false},
			},
		},
		{
			note:  "single test ",
			regex: "test_pass",
			tests: expectedTestResults{
				{"data.foo", "test_pass"}: {false, false, false},
			},
		},
		{
			note:  "single test explicit",
			regex: "data.foo.test_pass",
			tests: expectedTestResults{
				{"data.foo", "test_pass"}: {false, false, false},
			},
		},
		{
			note:  "single test skipped explicit",
			regex: "data.foo.todo_test_skip_too",
			tests: expectedTestResults{
				{"data.foo", "todo_test_skip_too"}: {false, false, true},
			},
		},
		{
			note:  "wildcards",
			regex: "^.*foo.*_fail.*$",
			tests: expectedTestResults{
				{"data.foo", "test_fail"}:          {false, true, false},
				{"data.foo", "test_fail_non_bool"}: {false, true, false},
			},
		},
		{
			note:  "mixed",
			regex: "(bar|data.foo.test_pass)",
			tests: expectedTestResults{
				{"data.foo", "test_pass"}:      {false, false, false},
				{"data.bar", "test_duplicate"}: {false, false, false},
			},
		},
		{
			note:  "case insensitive",
			regex: "(?i)DATA.BAR",
			tests: expectedTestResults{
				{"data.bar", "test_duplicate"}: {false, false, false},
			},
		},
		{
			note:  "matching ref rule halfways",
			regex: "data.baz.a",
			tests: expectedTestResults{
				{"data.baz", "a.b.test_duplicate"}:         {false, true, false},
				{"data.baz", "a.b[\"test_duplicate#01\"]"}: {false, false, false},
				{"data.baz", "a.b[\"test_duplicate#02\"]"}: {false, false, false},
			},
		},
	}

	test.WithTempFS(files, func(d string) {

		for _, tc := range cases {
			t.Run(tc.note, func(t *testing.T) {
				conf := testRunConfig{filter: tc.regex}
				rs, _ := doTestRunWithTmpDir(t, d, conf)
				validateTestResults(t, tc.tests, rs, conf)
			})
		}
	})
}

func TestRunnerCancel(t *testing.T) {
	testCancel(t, false)
}

func TestRunnerCancelBenchmark(t *testing.T) {
	testCancel(t, true)
}

func testCancel(t *testing.T, bench bool) {

	registerSleepBuiltin()

	ctx, cancel := context.WithCancel(context.Background())

	module := `package foo
	import rego.v1

	test_1 if { test.sleep("100ms") }
	test_2 if { true }`

	files := map[string]string{
		"/a_test.rego": module,
	}

	test.WithTempFS(files, func(d string) {
		paths := []string{d}
		modules, store, err := tester.Load(paths, nil)
		if err != nil {
			t.Fatal(err)
		}

		txn := storage.NewTransactionOrDie(ctx, store)
		runner := tester.NewRunner().SetStore(store).SetModules(modules)

		// Everything below uses a canceled context..
		cancel()

		var ch chan *tester.Result
		if bench {
			ch, err = runner.RunBenchmarks(ctx, txn, tester.BenchmarkOptions{})
		} else {
			ch, err = runner.RunTests(ctx, txn)
		}
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		var results []*tester.Result
		for r := range ch {
			results = append(results, r)
		}

		if len(results) != 1 {
			t.Fatalf("Expected only a single test result, got: %d", len(results))
		}

		if !topdown.IsCancel(results[0].Error) {
			t.Fatalf("Expected cancel error for first test but got: %v", results[0].Error)
		}

		if !errors.Is(results[0].Error, context.Canceled) {
			t.Fatalf("Expected error to be of type context.Canceled but got: %v", results[0].Error)
		}
	})
}

func TestRunnerTimeout(t *testing.T) {
	test.Skip(t)
	testTimeout(t, false)
}

func TestRunnerTimeoutBenchmark(t *testing.T) {
	testTimeout(t, true)
}

func testTimeout(t *testing.T, bench bool) {
	registerSleepBuiltin()

	ctx := context.Background()

	files := map[string]string{
		"/a_test.rego": `package foo
		import rego.v1

		test_1 if { test.sleep("100ms") }

		# 1ms is low enough for a single test to pass,
		# but long enough for benchmark to timeout
		test_2 if { test.sleep("1ms") }`,
	}

	test.WithTempFS(files, func(d string) {
		paths := []string{d}
		modules, store, err := tester.Load(paths, nil)
		if err != nil {
			t.Fatal(err)
		}
		duration, err := time.ParseDuration("15ms")
		if err != nil {
			t.Fatal(err)
		}

		txn := storage.NewTransactionOrDie(ctx, store)
		runner := tester.NewRunner().SetTimeout(duration).SetStore(store).SetModules(modules)

		var ch chan *tester.Result
		if bench {
			ch, err = runner.RunBenchmarks(ctx, txn, tester.BenchmarkOptions{})
		} else {
			ch, err = runner.RunTests(ctx, txn)
		}
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		var results []*tester.Result
		for r := range ch {
			results = append(results, r)
		}
		if !topdown.IsCancel(results[0].Error) {
			t.Fatalf("Expected cancel error for first test but got: %v", results[0].Error)
		}

		if bench {
			if !topdown.IsCancel(results[1].Error) {
				t.Fatalf("Expected cancel error for second test but got: %v", results[1].Error)
			}
		} else {
			if topdown.IsCancel(results[1].Error) {
				t.Fatalf("Expected no error for second test, but it timed out")
			}
		}

		if !errors.Is(results[0].Error, context.DeadlineExceeded) {
			t.Fatalf("Expected error to be of type context.DeadlineExceeded but got: %v", results[0].Error)
		}
	})
}

func TestRunnerPrintOutput(t *testing.T) {

	files := map[string]string{
		"/test.rego": `package test
		import rego.v1

		test_a if { print("A") }
		test_b if { false; print("B") }
		test_c if { print("C"); false }
		p.q.r.test_d if { print("D") }`,
	}

	ctx := context.Background()

	test.WithTempFS(files, func(d string) {
		paths := []string{d}
		modules, store, err := tester.Load(paths, nil)
		if err != nil {
			t.Fatal(err)
		}

		txn := storage.NewTransactionOrDie(ctx, store)
		runner := tester.NewRunner().SetStore(store).SetModules(modules).CapturePrintOutput(true)
		ch, err := runner.RunTests(ctx, txn)
		if err != nil {
			t.Fatal(err)
		}

		var results []*tester.Result
		for r := range ch {
			results = append(results, r)
		}

		exp := map[string]string{
			"test_a":       "A\n",
			"test_b":       "",
			"test_c":       "C\n",
			"p.q.r.test_d": "D\n",
		}

		got := map[string]string{}

		for _, tr := range results {
			got[tr.Name] = string(tr.Output)
		}

		if !reflect.DeepEqual(exp, got) {
			t.Fatal("expected:", exp, "got:", got)
		}
	})
}

func registerSleepBuiltin() {
	ast.RegisterBuiltin(&ast.Builtin{
		Name: "test.sleep",
		Decl: types.NewFunction(
			types.Args(types.S),
			types.NewNull(),
		),
	})

	topdown.RegisterBuiltinFunc("test.sleep", func(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
		d, _ := time.ParseDuration(string(operands[0].Value.(ast.String)))
		time.Sleep(d)
		return iter(ast.NullTerm())
	})
}

func TestRunnerWithCustomBuiltin(t *testing.T) {

	var myBuiltinDecl = &ast.Builtin{
		Name: "my_sum",
		Decl: types.NewFunction(
			types.Args(
				types.N,
				types.N,
			),
			types.N,
		),
	}

	var myBuiltin = &tester.Builtin{
		Decl: myBuiltinDecl,
		Func: rego.Function2(
			&rego.Function{
				Name: myBuiltinDecl.Name,
				Decl: myBuiltinDecl.Decl,
			},
			func(_ rego.BuiltinContext, a, b *ast.Term) (*ast.Term, error) {
				var num1, num2 int
				if err := ast.As(a.Value, &num1); err != nil {
					return nil, err
				}
				if err := ast.As(b.Value, &num2); err != nil {
					return nil, err
				}
				return ast.IntNumberTerm(num1 + num2), nil
			},
		),
	}

	files := map[string]string{
		"/test.rego": `package test
		import rego.v1

		test_a if { my_sum(2,3) == 5 }
		test_b if { my_sum(5,4) == 1 }
		test_c if { my_sum(4,1.0) == 5 }`,
	}

	ctx := context.Background()

	test.WithTempFS(files, func(d string) {
		paths := []string{d}
		modules, store, err := tester.Load(paths, nil)
		if err != nil {
			t.Fatal(err)
		}
		txn := storage.NewTransactionOrDie(ctx, store)
		runner := tester.NewRunner().SetStore(store).SetModules(modules).AddCustomBuiltins([]*tester.Builtin{myBuiltin})
		ch, err := runner.RunTests(ctx, txn)
		if err != nil {
			t.Fatal(err)
		}

		var results []*tester.Result

		for r := range ch {
			results = append(results, r)
		}

		exp := map[string]bool{
			"test_a": true,
			"test_b": false,
			"test_c": false,
		}

		got := map[string]bool{}

		for _, tr := range results {
			got[tr.Name] = tr.Pass()
		}

		if !reflect.DeepEqual(exp, got) {
			t.Fatal("expected:", exp, "got:", got)
		}
	})
}

func TestRunnerWithBuiltinErrors(t *testing.T) {
	const ruleTemplate = `package test
	import rego.v1
	
	test_json_parsing if {
      x := json.unmarshal("%s")
	  x.test == 123
	}`

	testCases := []struct {
		desc          string
		json          string
		builtinErrors bool
		wantErr       bool
	}{
		{
			desc:          "Valid JSON with flag enabled does not raise an error",
			json:          `{\"test\": 123}`,
			builtinErrors: true,
		},
		{
			desc:          "Invalid JSON with flag enabled raises an error",
			json:          `test: 123`,
			builtinErrors: true,
			wantErr:       true,
		},
		{
			desc: "Invalid JSON with flag disabled does not raise an error",
			json: `test: 123`,
		},
	}

	ctx := context.Background()

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			files := map[string]string{
				"builtin_error_test.rego": fmt.Sprintf(ruleTemplate, tc.json),
			}

			test.WithTempFS(files, func(d string) {
				paths := []string{d}
				modules, store, err := tester.Load(paths, nil)
				if err != nil {
					t.Fatal(err)
				}
				txn := storage.NewTransactionOrDie(ctx, store)
				runner := tester.
					NewRunner().
					SetStore(store).
					SetModules(modules).
					RaiseBuiltinErrors(tc.builtinErrors)

				ch, err := runner.RunTests(ctx, txn)
				if err != nil {
					t.Fatal(err)
				}
				for result := range ch {
					if gotErr := result.Error != nil; gotErr != tc.wantErr {
						t.Errorf("wantErr = %v, gotErr = %v", tc.wantErr, gotErr)
					}
				}
			})
		})
	}
}

func TestLoad_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		expErrs []string
	}{
		{
			note: "v0 module", // NOT default rego-version
			module: `package test

p[x] { 
	x = "a" 
}

test_p {
	p["a"]
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:3: rego_parse_error: `contains` keyword is required for partial set rules",
				"test.rego:7: rego_parse_error: `if` keyword is required before rule body",
			},
		},
		{
			note: "import rego.v1",
			module: `package test
import rego.v1

p contains x if { 
	x := "a" 
}

test_p if {
	"a" in p
}`,
		},
		{
			note: "v1 module", // default rego-version
			module: `package test

p contains x if { 
	x := "a" 
}

test_p if {
	"a" in p
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.rego": tc.module,
			}

			test.WithTempFS(files, func(root string) {
				modules, store, err := tester.Load([]string{root}, nil)
				if len(tc.expErrs) > 0 {
					if err == nil {
						t.Fatalf("Expected error but got nil")
					}

					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("Expected error to contain:\n\n%q\n\nbut got:\n\n%v", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}

					if modules == nil {
						t.Fatalf("Expected modules to be non-nil")
					}

					if store == nil {
						t.Fatalf("Expected store to be non-nil")
					}
				}
			})
		})
	}
}

func TestRun_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		module  ast.Module
		expErrs []string
	}{
		{
			note: "no v1 violations",
			module: ast.Module{
				Package: ast.MustParsePackage(`package test`),
				Rules: []*ast.Rule{
					ast.MustParseRule(`p[x] { x = "a" }`),
					ast.MustParseRule(`test_p { p["a"] }`),
				},
			},
		},
		{
			note: "v1 violations",
			module: ast.Module{
				Package: ast.MustParsePackage(`package test`),
				Imports: ast.MustParseImports(`
					import data.foo
					import data.bar as foo
				`),
				Rules: []*ast.Rule{
					ast.MustParseRule(`p[x] { x = "a" }`),
					ast.MustParseRule(`test_p { p["a"] }`),
				},
			},
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := context.Background()

			modules := map[string]*ast.Module{
				"test": &tc.module,
			}

			store := inmem.New()
			txn := storage.NewTransactionOrDie(ctx, store)
			defer store.Abort(ctx, txn)

			runner := tester.NewRunner().
				SetStore(store).
				SetModules(modules).
				SetTimeout(10 * time.Second)

			ch, err := runner.RunTests(ctx, txn)

			if len(tc.expErrs) > 0 {
				if err == nil {
					t.Fatalf("Expected error but got nil")
				}

				for _, expErr := range tc.expErrs {
					if !strings.Contains(err.Error(), expErr) {
						t.Fatalf("Expected error to contain:\n\n%q\n\nbut got:\n\n%v", expErr, err)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				var rs []*tester.Result
				for r := range ch {
					rs = append(rs, r)
				}

				if len(rs) != 1 {
					t.Fatalf("Expected exactly one result but got: %v", rs)
				}

				if rs[0].Fail {
					t.Fatalf("Expected test to pass but it failed")
				}
			}
		})
	}
}
