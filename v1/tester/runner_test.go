// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tester_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"path/filepath"
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	cases    map[string]expectedTestResult
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
		"/e_test.rego": `package qux
			import rego.v1

			test_cases_pass[x] if { some x in ["foo", "bar"] }
			test_cases_fail[x] if { some x in ["foo", "bar"]; false }
			test_cases_partial_fail[x] if { some x in ["foo", "bar", "baz"]; x != "bar" }
			test_cases_nested[x][y] if { some x in ["foo", "bar"]; some y in ["do", "re", "mi"]; not f(x, y) }
			f(x, y) if { x == "foo"; y == "re" }`,
	}

	tests := expectedTestResults{
		{"data.foo", "test_pass"}:                  {false, false, false, nil},
		{"data.foo", "test_fail"}:                  {false, true, false, nil},
		{"data.foo", "test_fail_non_bool"}:         {false, true, false, nil},
		{"data.foo", "test_duplicate"}:             {false, true, false, nil},
		{"data.foo", "test_duplicate#01"}:          {false, false, false, nil},
		{"data.foo", "test_duplicate#02"}:          {false, false, false, nil},
		{"data.foo", "test_err"}:                   {true, false, false, nil},
		{"data.foo", "todo_test_skip"}:             {false, false, true, nil},
		{"data.bar", "test_duplicate"}:             {false, false, false, nil},
		{"data.baz", "a.b.test_duplicate"}:         {false, true, false, nil},
		{"data.baz", "a.b[\"test_duplicate#01\"]"}: {false, false, false, nil},
		{"data.baz", "a.b[\"test_duplicate#02\"]"}: {false, false, false, nil},
		{"data.test", "test_pass"}:                 {false, false, false, nil},
		{"data.qux", "test_cases_pass"}: {false, false, false, map[string]expectedTestResult{
			"foo": {false, false, false, nil},
			"bar": {false, false, false, nil},
		}},
		{"data.qux", "test_cases_fail"}: {false, true, false, map[string]expectedTestResult{
			"foo": {false, true, false, nil},
			"bar": {false, true, false, nil},
		}},
		{"data.qux", "test_cases_partial_fail"}: {false, true, false, map[string]expectedTestResult{
			"foo": {false, false, false, nil},
			"bar": {false, true, false, nil},
			"baz": {false, false, false, nil},
		}},
		{"data.qux", "test_cases_nested"}: {false, true, false, map[string]expectedTestResult{
			"foo": {false, true, false, map[string]expectedTestResult{
				"do": {false, false, false, nil},
				"re": {false, true, false, nil},
				"mi": {false, false, false, nil},
			}},
			"bar": {false, false, false, map[string]expectedTestResult{
				"do": {false, false, false, nil},
				"re": {false, false, false, nil},
				"mi": {false, false, false, nil},
			}},
		}},
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

	ctx := t.Context()

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
	for _, r := range rs {
		k := [2]string{r.Package, r.Name}
		seen[k] = struct{}{}
		exp, ok := tests[k]
		if !ok {
			t.Errorf("Unexpected result for %v", k)
			continue
		} else if exp.wantErr != (r.Error != nil) || exp.wantFail != r.Fail {
			t.Errorf("Expected %+v for %v but got: %v", exp, k, r)
		} else {
			// Test passed
			if conf.bench && r.BenchmarkResult == nil {
				t.Errorf("Expected BenchmarkResult for test %v, got nil", k)
			} else if !conf.bench && r.BenchmarkResult != nil {
				t.Errorf("Unexpected BenchmarkResult for test %v, expected nil", k)
			}
		}

		if exp.cases != nil {
			validateSubTestResults(t, exp.cases, r.SubResults)
		}
	}
	for k := range tests {
		if _, ok := seen[k]; !ok {
			t.Errorf("Expected result for %v", k)
		}
	}
}

func validateSubTestResults(t *testing.T, tests map[string]expectedTestResult, srs tester.SubResultMap) {
	t.Helper()
	seen := map[string]struct{}{}
	for k, exp := range tests {
		seen[k] = struct{}{}
		sr, ok := srs[k]
		if !ok {
			t.Errorf("Expected sub-result for %v", k)
			continue
		}

		if exp.wantFail != sr.Fail {
			t.Errorf("Expected %+v for %v but got: %v", exp, k, sr)
		}
	}
	for k, v := range srs {
		if _, ok := seen[k]; !ok {
			t.Errorf("Expected sub-result for %v", k)
		}

		if v.SubResults != nil {
			validateSubTestResults(t, tests[k].cases, v.SubResults)
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
			test_cases[x][y] if { x := "foo"; y := "bar" }
			test_duplicate.foo[y] if { x := "foo"; y := "bar" }
			test_duplicate[x][y] if { x := "foo"; y := "bar" }
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
				{"data.foo", "test_pass"}:                  {false, false, false, nil},
				{"data.foo", "test_fail"}:                  {false, true, false, nil},
				{"data.foo", "test_fail_non_bool"}:         {false, true, false, nil},
				{"data.foo", "test_duplicate"}:             {false, true, false, nil},
				{"data.foo", "test_duplicate#01"}:          {false, false, false, nil},
				{"data.foo", "test_duplicate#02"}:          {false, false, false, nil},
				{"data.foo", "test_err"}:                   {true, false, false, nil},
				{"data.foo", "todo_test_skip"}:             {false, false, true, nil},
				{"data.foo", "todo_test_skip_too"}:         {false, false, true, nil},
				{"data.foo", "test_cases"}:                 {false, false, false, nil},
				{"data.foo", "test_duplicate#03"}:          {false, false, false, nil},
				{"data.foo", "test_duplicate#04"}:          {false, false, false, nil},
				{"data.bar", "test_duplicate"}:             {false, false, false, nil},
				{"data.baz", "a.b.test_duplicate"}:         {false, true, false, nil},
				{"data.baz", "a.b[\"test_duplicate#01\"]"}: {false, false, false, nil},
				{"data.baz", "a.b[\"test_duplicate#02\"]"}: {false, false, false, nil},
			},
		},
		{
			note:  "no filter",
			regex: "",
			tests: expectedTestResults{
				{"data.foo", "test_pass"}:                  {false, false, false, nil},
				{"data.foo", "test_fail"}:                  {false, true, false, nil},
				{"data.foo", "test_fail_non_bool"}:         {false, true, false, nil},
				{"data.foo", "test_duplicate"}:             {false, true, false, nil},
				{"data.foo", "test_duplicate#01"}:          {false, false, false, nil},
				{"data.foo", "test_duplicate#02"}:          {false, false, false, nil},
				{"data.foo", "test_err"}:                   {true, false, false, nil},
				{"data.foo", "todo_test_skip"}:             {false, false, true, nil},
				{"data.foo", "todo_test_skip_too"}:         {false, false, true, nil},
				{"data.foo", "test_cases"}:                 {false, false, false, nil},
				{"data.foo", "test_duplicate#03"}:          {false, false, false, nil},
				{"data.foo", "test_duplicate#04"}:          {false, false, false, nil},
				{"data.bar", "test_duplicate"}:             {false, false, false, nil},
				{"data.baz", "a.b.test_duplicate"}:         {false, true, false, nil},
				{"data.baz", "a.b[\"test_duplicate#01\"]"}: {false, false, false, nil},
				{"data.baz", "a.b[\"test_duplicate#02\"]"}: {false, false, false, nil},
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
				{"data.bar", "test_duplicate"}: {false, false, false, nil},
			},
		},
		{
			note:  "single package explicit",
			regex: "data.bar.test_duplicate",
			tests: expectedTestResults{
				{"data.bar", "test_duplicate"}: {false, false, false, nil},
			},
		},
		{
			note:  "single test",
			regex: "test_pass",
			tests: expectedTestResults{
				{"data.foo", "test_pass"}: {false, false, false, nil},
			},
		},
		{
			note:  "single test explicit",
			regex: "data.foo.test_pass",
			tests: expectedTestResults{
				{"data.foo", "test_pass"}: {false, false, false, nil},
			},
		},
		{
			note:  "single test skipped explicit",
			regex: "data.foo.todo_test_skip_too",
			tests: expectedTestResults{
				{"data.foo", "todo_test_skip_too"}: {false, false, true, nil},
			},
		},
		{
			note:  "wildcards",
			regex: "^.*foo.*_fail.*$",
			tests: expectedTestResults{
				{"data.foo", "test_fail"}:          {false, true, false, nil},
				{"data.foo", "test_fail_non_bool"}: {false, true, false, nil},
			},
		},
		{
			note:  "mixed",
			regex: "(bar|data.foo.test_pass)",
			tests: expectedTestResults{
				{"data.foo", "test_pass"}:      {false, false, false, nil},
				{"data.bar", "test_duplicate"}: {false, false, false, nil},
			},
		},
		{
			note:  "case insensitive",
			regex: "(?i)DATA.BAR",
			tests: expectedTestResults{
				{"data.bar", "test_duplicate"}: {false, false, false, nil},
			},
		},
		{
			note:  "matching ref rule halfways",
			regex: "data.baz.a",
			tests: expectedTestResults{
				{"data.baz", "a.b.test_duplicate"}:         {false, true, false, nil},
				{"data.baz", "a.b[\"test_duplicate#01\"]"}: {false, false, false, nil},
				{"data.baz", "a.b[\"test_duplicate#02\"]"}: {false, false, false, nil},
			},
		},
		{
			note:  "matching sub-test rule",
			regex: "data.foo.test_cases",
			tests: expectedTestResults{
				{"data.foo", "test_cases"}: {false, false, false, nil},
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

	ctx, cancel := context.WithCancel(t.Context())

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

		if len(results) != 0 {
			t.Fatalf("Expected no tests to be run but, got: %d", len(results))
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

	ctx := t.Context()

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

		if bench {
			if !topdown.IsCancel(results[1].Error) {
				t.Fatalf("Expected cancel error for second test but got: %v", results[1].Error)
			}
		} else {
			if !topdown.IsCancel(results[1].Error) {
				t.Fatalf("Expected test to have timed out")
			}
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
		"/test2.rego": `package test
		import rego.v1

		test_d if { print("D") }
		test_e if { false; print("E") }
		test_f if { print("F"); false }
		p.q.r.test_g if { print("G") }`,
		"/test3.rego": `package test
		import rego.v1

		test_h if { print("H") }
		test_i if { false; print("I") }
		test_j if { print("J"); false }
		p.q.r.test_k if { print("K") }`,
	}

	ctx := t.Context()

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

		exp := map[string]map[string]string{
			"test.rego": {
				"test_a":       "A\n",
				"test_b":       "",
				"test_c":       "C\n",
				"p.q.r.test_d": "D\n",
			},
			"test2.rego": {
				"test_d":       "D\n",
				"test_e":       "",
				"test_f":       "F\n",
				"p.q.r.test_g": "G\n",
			},
			"test3.rego": {
				"test_h":       "H\n",
				"test_i":       "",
				"test_j":       "J\n",
				"p.q.r.test_k": "K\n",
			},
		}

		got := map[string]string{}
		var lastFile string
		for r := range ch {
			if lastFile == "" {
				lastFile = filepath.Base(r.Location.File)
			} else if lastFile != filepath.Base(r.Location.File) {
				// assert that all expected results for the file has been received
				// the individual files could be out of order, but it has to be grouped by file
				if !maps.Equal(exp[lastFile], got) {
					t.Fatal("expected:", exp, "got:", got)
				}

				// clear got for the next file
				got = map[string]string{}
				lastFile = filepath.Base(r.Location.File)
			}

			got[r.Name] = string(r.Output)
		}

		// check the last file
		if !maps.Equal(exp[lastFile], got) {
			t.Fatal("expected:", exp, "got:", got)
		}
	})
}

func registerSleepBuiltin() {
	ast.RegisterBuiltin(&ast.Builtin{
		Name: "test.sleep",
		Decl: types.NewFunction(
			types.Args(types.S),
			types.Nl,
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

	ctx := t.Context()

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

		results := make([]*tester.Result, 0, 10)

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

		if !maps.Equal(exp, got) {
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

	ctx := t.Context()

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
			ctx := t.Context()

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

func TestReporterFormatsWithExplicitParallel(t *testing.T) {
	tests := []struct {
		note     string
		parallel int
		r        func(writer io.Writer) tester.Reporter
		exp      func(string)
	}{
		{
			note:     "Pretty Format",
			parallel: 10,
			r: func(w io.Writer) tester.Reporter {
				return tester.PrettyReporter{
					Output: w,
				}
			},
			exp: func(output string) {
				exp := `PASS: 4/4
`
				if exp != output {
					t.Fatalf("Expected (%d bytes):\n\n%v\n\nGot (%d bytes):\n\n%v", len(exp), exp, len(output), output)
				}
			},
		},
		{
			note:     "JSON Format",
			parallel: 10,
			r: func(w io.Writer) tester.Reporter {
				return tester.JSONReporter{
					Output: w,
				}
			},
			exp: func(output string) {
				// the order of the tests and filepath and duration will be different each execution
				var r []*tester.Result
				if err := json.Unmarshal([]byte(output), &r); err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if len(r) != 4 {
					t.Fatalf("Expected exactly 4 results but got: %v", r)
				}
			},
		},
		{
			note:     "Go Bench Format",
			parallel: 10,
			r: func(w io.Writer) tester.Reporter {
				return tester.PrettyReporter{
					Output:                 w,
					BenchMarkGoBenchFormat: true,
				}
			},
			exp: func(output string) {
				exp := `PASS: 4/4
`
				if exp != output {
					t.Fatalf("Expected (%d bytes):\n\n%v\n\nGot (%d bytes):\n\n%v", len(exp), exp, len(output), output)
				}
			},
		},
	}

	files := map[string]string{
		"/test.rego": `package test
		import rego.v1

		test_a if { print("A") }
		test_a if { print("A") }
		test_a if { print("A") }
		test_a if { print("A") }`,
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := t.Context()

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

				var buf bytes.Buffer

				r := tc.r(&buf)

				if err := r.Report(ch); err != nil {
					t.Fatal(err)
				}

				str := buf.String()

				tc.exp(str)
			})
		})
	}
}
