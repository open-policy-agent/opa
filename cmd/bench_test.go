// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

// Minimize the number of tests that *actually* run the benchmarks, they are pretty slow.
// Have one test that exercises the whole flow.
func TestRunBenchmark(t *testing.T) {
	params := testBenchParams()

	args := []string{"1 + 1"}
	var buf bytes.Buffer

	rc := benchMain(args, params, &buf, &goBenchRunner{})

	if rc != 0 {
		t.Fatalf("Unexpected return code %d, expected 0", rc)
	}

	// Expect a json serialized benchmark result with histogram fields
	var br testing.BenchmarkResult
	err := util.UnmarshalJSON(buf.Bytes(), &br)
	if err != nil {
		t.Fatalf("Unexpected error unmarshalling output: %s", err)
	}

	if br.N == 0 || br.T == 0 || br.MemAllocs == 0 || br.MemBytes == 0 {
		t.Fatalf("Expected benchmark results to be non-zero, got: %+v", br)
	}

	if _, ok := br.Extra["histogram_timer_rego_query_eval_ns_count"]; !ok {
		t.Fatalf("Expected benchmark results to contain histogram_timer_rego_query_eval_ns_count, got: %+v", br)
	}

	if float64(br.N) != br.Extra["histogram_timer_rego_query_eval_ns_count"] {
		t.Fatalf("Expected 'histogram_timer_rego_query_eval_ns_count' to be equal to N")
	}
}

func TestRunBenchmarkFailFast(t *testing.T) {
	params := testBenchParams()
	params.fail = true // configured to fail on undefined results

	args := []string{"a := 1; a > 2"}
	var buf bytes.Buffer

	rc := benchMain(args, params, &buf, &goBenchRunner{})

	if rc != 1 {
		t.Fatalf("Unexpected return code %d, expected 1", rc)
	}

	// Expect a json serialized benchmark result with histogram fields
	var pr presentation.Output
	err := util.UnmarshalJSON(buf.Bytes(), &pr)
	if err != nil {
		t.Fatalf("Unexpected error unmarshalling output: %s", err)
	}

	if len(pr.Errors) != 1 {
		t.Fatalf("Expected 1 error in result, got:\n\n%s\n", buf.String())
	}
}

// mockBenchRunner lets us test the bench CLI operations without having to wait ~10 seconds
// while the actual benchmark runner does its thing.
type mockBenchRunner struct {
	onRun func(ctx context.Context, ectx *evalContext, params benchmarkCommandParams, f func(context.Context, ...rego.EvalOption) error) (testing.BenchmarkResult, error)
}

func (r *mockBenchRunner) run(ctx context.Context, ectx *evalContext, params benchmarkCommandParams, f func(context.Context, ...rego.EvalOption) error) (testing.BenchmarkResult, error) {
	if r.onRun != nil {
		return r.onRun(ctx, ectx, params, f)
	}
	return testing.BenchmarkResult{}, nil
}

func TestBenchPartial(t *testing.T) {
	params := testBenchParams()
	params.partial = true
	params.fail = true
	args := []string{"input=1"}
	var buf bytes.Buffer

	rc := benchMain(args, params, &buf, &mockBenchRunner{})

	if rc != 0 {
		t.Fatalf("Unexpected return code %d, expected 0", rc)
	}
}

func TestBenchMainErrPreparing(t *testing.T) {
	params := testBenchParams()
	args := []string{"???"} // query compile error
	var buf bytes.Buffer

	rc := benchMain(args, params, &buf, &mockBenchRunner{})

	if rc != 1 {
		t.Fatalf("Unexpected return code %d, expected 1", rc)
	}
}

func TestBenchMainErrRunningBenchmark(t *testing.T) {
	params := testBenchParams()
	args := []string{"1+1"}
	var buf bytes.Buffer

	mockRunner := &mockBenchRunner{}
	mockRunner.onRun = func(ctx context.Context, ectx *evalContext, params benchmarkCommandParams, f func(context.Context, ...rego.EvalOption) error) (testing.BenchmarkResult, error) {
		return testing.BenchmarkResult{}, errors.New("error error error")
	}

	rc := benchMain(args, params, &buf, mockRunner)

	if rc != 1 {
		t.Fatalf("Unexpected return code %d, expected 1", rc)
	}
}

func TestBenchMainWithCount(t *testing.T) {
	params := testBenchParams()
	args := []string{"1+1"}
	var buf bytes.Buffer

	mockRunner := &mockBenchRunner{}

	params.count = 25
	actualCount := 0
	mockRunner.onRun = func(ctx context.Context, ectx *evalContext, params benchmarkCommandParams, f func(context.Context, ...rego.EvalOption) error) (testing.BenchmarkResult, error) {
		actualCount++
		return testing.BenchmarkResult{}, nil
	}

	rc := benchMain(args, params, &buf, mockRunner)

	if rc != 0 {
		t.Fatalf("Unexpected return code %d, expected 0", rc)
	}

	if actualCount != params.count {
		t.Fatalf("Expected benchmark to be run %d times, only ran %d", params.count, actualCount)
	}
}

func TestBenchMainWithNegativeCount(t *testing.T) {
	params := testBenchParams()
	args := []string{"1+1"}
	var buf bytes.Buffer

	mockRunner := &mockBenchRunner{}

	params.count = -1
	actualCount := 0
	mockRunner.onRun = func(ctx context.Context, ectx *evalContext, params benchmarkCommandParams, f func(context.Context, ...rego.EvalOption) error) (testing.BenchmarkResult, error) {
		actualCount++
		return testing.BenchmarkResult{}, nil
	}

	rc := benchMain(args, params, &buf, mockRunner)

	if rc != 0 {
		t.Fatalf("Unexpected return code %d, expected 0", rc)
	}

	if actualCount != 0 {
		t.Fatalf("Expected benchmark to not be run, instead ran %d times", actualCount)
	}
}

func validateBenchMainPrep(t *testing.T, args []string, params benchmarkCommandParams) {

	var buf bytes.Buffer

	mockRunner := &mockBenchRunner{}

	mockRunner.onRun = func(ctx context.Context, ectx *evalContext, params benchmarkCommandParams, f func(context.Context, ...rego.EvalOption) error) (testing.BenchmarkResult, error) {

		// cheat and use the ectx to evalute the query to ensure the input setup on it was valid
		pq, err := ectx.r.PrepareForEval(ctx)
		if err != nil {
			return testing.BenchmarkResult{}, err
		}

		rs, err := pq.Eval(ctx, ectx.evalArgs...)
		if err != nil {
			return testing.BenchmarkResult{}, err
		}

		if len(rs) != 1 || rs[0].Expressions[0].Value != true {
			t.Errorf("Unexpected results: %+v", rs)
		}

		return testing.BenchmarkResult{}, nil
	}

	rc := benchMain(args, params, &buf, mockRunner)
	if rc != 0 {
		t.Fatalf("Unexpected return code %d, expected 0", rc)
	}
}

func TestBenchMainWithJSONInputFile(t *testing.T) {
	params := testBenchParams()
	files := map[string]string{
		"/input.json": `{"x": 42}`,
	}
	args := []string{"input.x == 42"}
	test.WithTempFS(files, func(path string) {
		params.inputPath = filepath.Join(path, "input.json")

		validateBenchMainPrep(t, args, params)
	})
}

func TestBenchMainWithYAMLInputFile(t *testing.T) {
	params := testBenchParams()
	files := map[string]string{
		"/input.yaml": `x: 42`,
	}
	args := []string{"input.x == 42"}
	test.WithTempFS(files, func(path string) {
		params.inputPath = filepath.Join(path, "input.yaml")

		validateBenchMainPrep(t, args, params)
	})
}

func TestBenchMainInvalidInputFile(t *testing.T) {
	params := testBenchParams()
	files := map[string]string{
		"/input.yaml": `x: 42`,
	}
	args := []string{"1+1"}
	test.WithTempFS(files, func(path string) {
		params.inputPath = filepath.Join(path, "definitely/not/input.yaml")

		var buf bytes.Buffer

		rc := benchMain(args, params, &buf, &mockBenchRunner{})
		if rc != 1 {
			t.Fatalf("Unexpected return code %d, expected 1", rc)
		}
	})
}

func TestBenchMainWithBundleData(t *testing.T) {
	params := testBenchParams()

	mod := `package a.b

	x {
	   data.a.b.c == 42
	}
	`

	b := bundle.Bundle{
		Manifest: bundle.Manifest{},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": 42,
				},
			},
		},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/a/b/policy.rego",
				Raw:    []byte(mod),
				Parsed: ast.MustParseModule(mod),
			},
		},
	}

	files := map[string]string{
		"bundle.tar.gz": "",
	}

	test.WithTempFS(files, func(path string) {
		bundlePath := filepath.Join(path, "bundle.tar.gz")
		f, err := os.OpenFile(bundlePath, os.O_WRONLY, os.ModePerm)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		err = bundle.Write(f, b)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		params.bundlePaths.Set(bundlePath)

		args := []string{"data.a.b.x"}

		validateBenchMainPrep(t, args, params)
	})

}

func TestRenderBenchmarkResultJSONOutput(t *testing.T) {
	params := testBenchParams()
	params.outputFormat.Set(evalJSONOutput)

	br := fakeBenchResults()

	var buf bytes.Buffer
	renderBenchmarkResult(params, br, &buf)

	actual := buf.String()

	expected := `{
  "N": 134844,
  "T": 1088294120,
  "Bytes": 0,
  "MemAllocs": 8360721,
  "MemBytes": 449906736,
  "Extra": {
    "histogram_timer_rego_query_eval_ns_75%": 4953.75,
    "histogram_timer_rego_query_eval_ns_90%": 6309.6,
    "histogram_timer_rego_query_eval_ns_95%": 7872.55,
    "histogram_timer_rego_query_eval_ns_99%": 14947.34000000001,
    "histogram_timer_rego_query_eval_ns_99.9%": 174377.08200000023,
    "histogram_timer_rego_query_eval_ns_99.99%": 176301,
    "histogram_timer_rego_query_eval_ns_count": 134844,
    "histogram_timer_rego_query_eval_ns_max": 176301,
    "histogram_timer_rego_query_eval_ns_mean": 5118.3706225680935,
    "histogram_timer_rego_query_eval_ns_median": 4312,
    "histogram_timer_rego_query_eval_ns_min": 3553,
    "histogram_timer_rego_query_eval_ns_stddev": 6587.830963916497
  }
}
`
	if actual != expected {
		t.Fatalf("\nExpected:\n%s\n\nGot:\n%s\n", expected, actual)
	}
}

func TestRenderBenchmarkResultPrettyOutput(t *testing.T) {
	params := testBenchParams()
	params.benchMem = false
	params.outputFormat.Set(evalPrettyOutput)

	br := fakeBenchResults()

	var buf bytes.Buffer
	renderBenchmarkResult(params, br, &buf)

	actual := buf.String()

	expected := `+-------------------------------------------+------------+
| samples                                   |     134844 |
| ns/op                                     |       8071 |
| histogram_timer_rego_query_eval_ns_75%    |       4954 |
| histogram_timer_rego_query_eval_ns_90%    |       6310 |
| histogram_timer_rego_query_eval_ns_95%    |       7873 |
| histogram_timer_rego_query_eval_ns_99%    |      14947 |
| histogram_timer_rego_query_eval_ns_99.9%  |     174377 |
| histogram_timer_rego_query_eval_ns_99.99% |     176301 |
| histogram_timer_rego_query_eval_ns_count  |     134844 |
| histogram_timer_rego_query_eval_ns_max    |     176301 |
| histogram_timer_rego_query_eval_ns_mean   |       5118 |
| histogram_timer_rego_query_eval_ns_median |       4312 |
| histogram_timer_rego_query_eval_ns_min    |       3553 |
| histogram_timer_rego_query_eval_ns_stddev |       6588 |
+-------------------------------------------+------------+
`
	if actual != expected {
		t.Fatalf("\nExpected:\n%s\n\nGot:\n%s\n", expected, actual)
	}
}

func TestRenderBenchmarkResultPrettyOutputShowAllocs(t *testing.T) {
	params := testBenchParams()
	params.benchMem = true
	params.outputFormat.Set(evalPrettyOutput)

	br := fakeBenchResults()

	var buf bytes.Buffer
	renderBenchmarkResult(params, br, &buf)

	actual := buf.String()

	expected := `+-------------------------------------------+------------+
| samples                                   |     134844 |
| ns/op                                     |       8071 |
| B/op                                      |       3336 |
| allocs/op                                 |         62 |
| histogram_timer_rego_query_eval_ns_75%    |       4954 |
| histogram_timer_rego_query_eval_ns_90%    |       6310 |
| histogram_timer_rego_query_eval_ns_95%    |       7873 |
| histogram_timer_rego_query_eval_ns_99%    |      14947 |
| histogram_timer_rego_query_eval_ns_99.9%  |     174377 |
| histogram_timer_rego_query_eval_ns_99.99% |     176301 |
| histogram_timer_rego_query_eval_ns_count  |     134844 |
| histogram_timer_rego_query_eval_ns_max    |     176301 |
| histogram_timer_rego_query_eval_ns_mean   |       5118 |
| histogram_timer_rego_query_eval_ns_median |       4312 |
| histogram_timer_rego_query_eval_ns_min    |       3553 |
| histogram_timer_rego_query_eval_ns_stddev |       6588 |
+-------------------------------------------+------------+
`
	if actual != expected {
		t.Fatalf("\nExpected:\n%s\n\nGot:\n%s\n", expected, actual)
	}
}

func TestRenderBenchmarkResultGoBenchOutputShowAllocs(t *testing.T) {
	params := testBenchParams()
	params.benchMem = true
	params.outputFormat.Set(benchmarkGoBenchOutput)

	br := fakeBenchResults()

	var buf bytes.Buffer
	renderBenchmarkResult(params, br, &buf)

	actual := buf.String()

	if !strings.HasPrefix(actual, "Benchmark") {
		t.Fatalf("Expected line output to start with 'Benchmark', got: \n\n%s\n", actual)
	}

	if len(strings.Split(strings.TrimSpace(actual), "\n")) != 1 {
		t.Fatalf("Expected only a single line of output")
	}
}

func TestRenderBenchmarkErrorJSONOutput(t *testing.T) {
	params := testBenchParams()
	params.outputFormat.Set(evalJSONOutput)

	var buf bytes.Buffer

	_, err := ast.ParseBody("???")

	renderBenchmarkError(params, err, &buf)

	actual := buf.String()
	expected := `{
  "errors": [
    {
      "message": "illegal token",
      "code": "rego_parse_error",
      "location": {
        "file": "",
        "row": 1,
        "col": 1
      },
      "details": {
        "line": "???",
        "idx": 0
      }
    }
  ]
}
`

	if actual != expected {
		t.Fatalf("\nExpected:\n%s\n\nGot:\n%s\n", expected, actual)
	}
}

func TestRenderBenchmarkErrorPrettyOutput(t *testing.T) {
	params := testBenchParams()
	params.outputFormat.Set(evalPrettyOutput)

	testPrettyBenchmarkOutput(t, params)
}

func TestRenderBenchmarkErrorGoBenchOutput(t *testing.T) {
	params := testBenchParams()
	params.outputFormat.Set(benchmarkGoBenchOutput)

	testPrettyBenchmarkOutput(t, params)
}

func testPrettyBenchmarkOutput(t *testing.T, params benchmarkCommandParams) {
	var buf bytes.Buffer

	_, err := ast.ParseBody("???")

	renderBenchmarkError(params, err, &buf)

	actual := buf.String()
	expected := `1 error occurred: 1:1: rego_parse_error: illegal token
	???
	^
`
	if actual != expected {
		t.Fatalf("\nExpected:\n%s\n\nGot:\n%s\n", expected, actual)
	}
}

func testBenchParams() benchmarkCommandParams {
	params := newBenchmarkEvalParams()
	params.benchMem = true
	params.metrics = true
	params.outputFormat.Set(evalJSONOutput)
	params.count = 1
	return params
}

func fakeBenchResults() testing.BenchmarkResult {
	return testing.BenchmarkResult{
		N:         134844,
		T:         1088294120,
		Bytes:     0,
		MemAllocs: 8360721,
		MemBytes:  449906736,
		Extra: map[string]float64{
			"histogram_timer_rego_query_eval_ns_75%":    4953.75,
			"histogram_timer_rego_query_eval_ns_90%":    6309.6,
			"histogram_timer_rego_query_eval_ns_95%":    7872.55,
			"histogram_timer_rego_query_eval_ns_99%":    14947.34000000001,
			"histogram_timer_rego_query_eval_ns_99.9%":  174377.08200000023,
			"histogram_timer_rego_query_eval_ns_99.99%": 176301,
			"histogram_timer_rego_query_eval_ns_count":  134844,
			"histogram_timer_rego_query_eval_ns_max":    176301,
			"histogram_timer_rego_query_eval_ns_mean":   5118.3706225680935,
			"histogram_timer_rego_query_eval_ns_median": 4312,
			"histogram_timer_rego_query_eval_ns_min":    3553,
			"histogram_timer_rego_query_eval_ns_stddev": 6587.830963916497,
		},
	}
}
