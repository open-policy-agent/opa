// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tester_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/cover"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/tester"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util/test"
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
			allow { true }
			`,
		"/a_test.rego": `package foo
			test_pass { allow }
			non_test { true }
			test_fail { not allow }
			test_fail_non_bool = 100
			test_err { conflict }
			conflict = true
			conflict = false
			test_duplicate { false }
			test_duplicate { true }
			test_duplicate { true }
			todo_test_skip { true }
			`,
		"/b_test.rego": `package bar

		test_duplicate { true }`,
	}

	tests := expectedTestResults{
		{"data.foo", "test_pass"}:          {false, false, false},
		{"data.foo", "test_fail"}:          {false, true, false},
		{"data.foo", "test_fail_non_bool"}: {false, true, false},
		{"data.foo", "test_duplicate"}:     {false, true, false},
		{"data.foo", "test_duplicate#01"}:  {false, false, false},
		{"data.foo", "test_duplicate#02"}:  {false, false, false},
		{"data.foo", "test_err"}:           {true, false, false},
		{"data.foo", "todo_test_skip"}:     {false, false, true},
		{"data.bar", "test_duplicate"}:     {false, false, false},
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
			allow { true }
			`,
		"/a_test.rego": `package foo
			test_pass { allow }
			non_test { true }
			test_fail { not allow }
			test_fail_non_bool = 100
			test_err { conflict }
			conflict = true
			conflict = false
			test_duplicate { false }
			test_duplicate { true }
			test_duplicate { true }
			todo_test_skip { true }
			todo_test_skip_too { false }
			`,
		"/b_test.rego": `package bar

		test_duplicate { true }`,
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
				{"data.foo", "test_pass"}:          {false, false, false},
				{"data.foo", "test_fail"}:          {false, true, false},
				{"data.foo", "test_fail_non_bool"}: {false, true, false},
				{"data.foo", "test_duplicate"}:     {false, true, false},
				{"data.foo", "test_duplicate#01"}:  {false, false, false},
				{"data.foo", "test_duplicate#02"}:  {false, false, false},
				{"data.foo", "test_err"}:           {true, false, false},
				{"data.foo", "todo_test_skip"}:     {false, false, true},
				{"data.foo", "todo_test_skip_too"}: {false, false, true},
				{"data.bar", "test_duplicate"}:     {false, false, false},
			},
		},
		{
			note:  "no filter",
			regex: "",
			tests: expectedTestResults{
				{"data.foo", "test_pass"}:          {false, false, false},
				{"data.foo", "test_fail"}:          {false, true, false},
				{"data.foo", "test_fail_non_bool"}: {false, true, false},
				{"data.foo", "test_duplicate"}:     {false, true, false},
				{"data.foo", "test_duplicate#01"}:  {false, false, false},
				{"data.foo", "test_duplicate#02"}:  {false, false, false},
				{"data.foo", "test_err"}:           {true, false, false},
				{"data.foo", "todo_test_skip"}:     {false, false, true},
				{"data.foo", "todo_test_skip_too"}: {false, false, true},
				{"data.bar", "test_duplicate"}:     {false, false, false},
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

	test_1 { test.sleep("100ms") }
	test_2 { true }`

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

		test_1 { test.sleep("100ms") }

		# 1ms is low enough for a single test to pass,
		# but long enough for benchmark to timeout
		test_2 { test.sleep("1ms") }`,
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
	})
}

func TestRunnerPrintOutput(t *testing.T) {

	files := map[string]string{
		"/test.rego": `package test

		test_a { print("A") }
		test_b { false; print("B") }
		test_c { print("C"); false }`,
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
			"test_a": "A\n",
			"test_b": "",
			"test_c": "C\n",
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

	topdown.RegisterFunctionalBuiltin1("test.sleep", func(a ast.Value) (ast.Value, error) {
		d, _ := time.ParseDuration(string(a.(ast.String)))
		time.Sleep(d)
		return ast.Null{}, nil
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

		test_a { my_sum(2,3) == 5 }
		test_b { my_sum(5,4) == 1 }
		test_c { my_sum(4,1.0) == 5 }`,
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
