package tester

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/util"
)

func getFakeTraceEvents() []*topdown.Event {
	return getFakeTraceEventsFor(ast.MustParseExpr("true = false"))
}

func getFakeTraceEventsFor(node ast.Node, modifiers ...func(e *topdown.Event)) []*topdown.Event {
	es := []*topdown.Event{
		{
			Op:       topdown.FailOp,
			Node:     node,
			Location: node.Loc(),
			QueryID:  0,
			ParentID: 0,
		},
	}

	for _, modifier := range modifiers {
		modifier(es[0])
	}

	return es
}

func TestPrettyReporterVerbose(t *testing.T) {
	var buf bytes.Buffer

	// supply fake trace events for each kind of event to ensure that only failures
	// report traces.
	ts := []*Result{
		{
			Package: "data.foo.bar",
			Name:    "test_baz",
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy1.rego",
				Row:  1,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_qux",
			Error:   errors.New("some err"),
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy1.rego",
				Row:  2,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_corge",
			Fail:    true,
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy2.rego",
				Row:  1,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "todo_test_qux",
			Skip:    true,
			Trace:   nil,
			Location: &ast.Location{
				File: "policy2.rego",
				Row:  2,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_contains_print",
			Output:  []byte("fake print output\n"),
			Location: &ast.Location{
				File: "policy3.rego",
				Row:  1,
			},
		},
		{
			Package: "data.foo.baz",
			Name:    "p.q.r.test_quz",
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy4.rego",
				Row:  1,
			},
		},
		{
			Package: "data.foo.qux",
			Name:    "test_cases",
			Trace:   getFakeTraceEvents(),
			Fail:    true,
			// Will be sorted to "bar", "baz", "foo" in output for stability
			SubResults: SubResultMap{
				"foo": {
					Name: "foo",
					Fail: false,
				},
				"bar": {
					Name: "bar",
					Fail: true,
				},
				"baz": {
					Name: "baz",
					Fail: false,
				},
			},
			Location: &ast.Location{
				File: "policy5.rego",
				Row:  1,
			},
		},
		{
			Package: "data.foo.qux",
			Name:    "test_cases_nested",
			Trace:   getFakeTraceEvents(),
			Fail:    true,
			SubResults: SubResultMap{
				"one": {
					Name: "one",
					Fail: false,
					SubResults: SubResultMap{
						"foo": {
							Name: "foo",
							Fail: false,
						},
						"bar": {
							Name: "bar",
							Fail: false,
						},
					},
				},
				"two": {
					Name: "two",
					Fail: true,
					SubResults: SubResultMap{
						"foo": {
							Name: "foo",
							Fail: true,
						},
						"bar": {
							Name: "bar",
							Fail: false,
						},
					},
				},
			},
			Location: &ast.Location{
				File: "policy5.rego",
				Row:  2,
			},
		},
	}

	r := PrettyReporter{
		Output:  &buf,
		Verbose: true,
	}

	ch := resultsChan(ts)
	if err := r.Report(ch); err != nil {
		t.Fatal(err)
	}

	exp := `FAILURES
--------------------------------------------------------------------------------
data.foo.bar.test_corge: FAIL (0s)

  query:1       | Fail true = false  

data.foo.qux.test_cases: FAIL (0s)

  query:1       | Fail true = false  

  bar: FAIL

data.foo.qux.test_cases_nested: FAIL (0s)

  query:1       | Fail true = false  

  two: FAIL
    foo: FAIL

SUMMARY
--------------------------------------------------------------------------------
policy1.rego:1:
data.foo.bar.test_baz: PASS (0s)
policy1.rego:2:
data.foo.bar.test_qux: ERROR (0s)
  some err

policy2.rego:1:
data.foo.bar.test_corge: FAIL (0s)
policy2.rego:2:
data.foo.bar.todo_test_qux: SKIPPED

policy3.rego:1:
data.foo.bar.test_contains_print: PASS (0s)

  fake print output


policy4.rego:1:
data.foo.baz.p.q.r.test_quz: PASS (0s)

policy5.rego:1:
data.foo.qux.test_cases: FAIL (0s)
  bar: FAIL
  baz: PASS
  foo: PASS
policy5.rego:2:
data.foo.qux.test_cases_nested: FAIL (0s)
  one: PASS
    bar: PASS
    foo: PASS
  two: FAIL
    bar: PASS
    foo: FAIL
--------------------------------------------------------------------------------
PASS: 8/13
FAIL: 3/13
SKIPPED: 1/13
ERROR: 1/13
`

	str := buf.String()

	if exp != str {
		t.Fatalf("Expected (%d bytes):\n\n%v\n\nGot (%d bytes):\n\n%v", len(exp), exp, len(str), str)
	}
}

func TestPrettyReporterFailureLine(t *testing.T) {
	var buf bytes.Buffer

	// supply fake trace events to verify that traces are suppressed without verbose
	// flag.
	ts := []*Result{
		{
			Package: "data.foo.bar",
			Name:    "test_baz",
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy1.rego",
				Row:  1,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_qux",
			Error:   errors.New("some err"),
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy1.rego",
				Row:  2,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_corge",
			Fail:    true,
			Trace: getFakeTraceEventsFor(
				ast.MustParseExpr("x == y + z"),
				func(e *topdown.Event) {
					// QueryID == 0 is not pretty-printed, as this is the base query to eval the test rule; not the test rule itself.
					e.QueryID = 1
				},
				func(e *topdown.Event) {
					e.Location.File = "policy1.rego"
					e.Location.Row = 5
				},
				func(e *topdown.Event) {
					e.Locals = ast.NewValueMap()
					e.Locals.Put(ast.Var("x"), ast.Number("1"))
					e.Locals.Put(ast.Var("y"), ast.Number("2"))
					e.Locals.Put(ast.Var("z"), ast.Number("3"))
				},
				func(e *topdown.Event) {
					e.LocalMetadata = map[ast.Var]topdown.VarMetadata{
						"x": {Name: "x"},
						"y": {Name: "y"},
						"z": {Name: "z"},
					}
				}),
			Location: &ast.Location{
				File: "policy1.rego",
				Row:  3,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "todo_test_qux",
			Skip:    true,
			Trace:   nil,
			Location: &ast.Location{
				File: "policy1.rego",
				Row:  4,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_contains_print_pass",
			Output:  []byte("fake print output\n"),
			Location: &ast.Location{
				File: "policy1.rego",
				Row:  5,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_contains_print_fail",
			Fail:    true,
			Output:  []byte("fake print output2\n"),
			Location: &ast.Location{
				File: "policy2.rego",
				Row:  1,
			},
		},
		{
			Package: "data.foo.baz",
			Name:    "p.q.r.test_quz",
			Fail:    true,
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy3.rego",
				Row:  1,
			},
		},
		{
			Package: "data.foo.qux",
			Name:    "test_cases_nested",
			Trace:   getFakeTraceEvents(),
			Fail:    true,
			SubResults: SubResultMap{
				"one": {
					Name: "one",
					Fail: false,
					SubResults: SubResultMap{
						"foo": {
							Name: "foo",
							Fail: false,
						},
						"bar": {
							Name: "bar",
							Fail: false,
						},
					},
				},
				"two": {
					Name: "two",
					Fail: true,
					SubResults: SubResultMap{
						"foo": {
							Name: "foo",
							Fail: true,
							Trace: getFakeTraceEventsFor(
								ast.MustParseExpr("x == y + z"),
								func(e *topdown.Event) {
									// QueryID == 0 is not pretty-printed, as this is the base query to eval the test rule; not the test rule itself.
									e.QueryID = 1
								},
								func(e *topdown.Event) {
									e.Location.File = "policy5.rego"
									e.Location.Row = 5
								},
								func(e *topdown.Event) {
									e.Locals = ast.NewValueMap()
									e.Locals.Put(ast.Var("x"), ast.Number("1"))
									e.Locals.Put(ast.Var("y"), ast.Number("2"))
									e.Locals.Put(ast.Var("z"), ast.Number("3"))
								},
								func(e *topdown.Event) {
									e.LocalMetadata = map[ast.Var]topdown.VarMetadata{
										"x": {Name: "x"},
										"y": {Name: "y"},
										"z": {Name: "z"},
									}
								}),
						},
						"bar": {
							Name: "bar",
							Fail: false,
						},
					},
				},
			},
			Location: &ast.Location{
				File: "policy5.rego",
				Row:  1,
			},
		},
	}

	r := PrettyReporter{
		Output:      &buf,
		Verbose:     false,
		FailureLine: true,
		LocalVars:   true,
	}
	ch := resultsChan(ts)
	if err := r.Report(ch); err != nil {
		t.Fatal(err)
	}

	exp := `FAILURES
--------------------------------------------------------------------------------
data.foo.bar.test_corge: FAIL (0s)

  policy1.rego:5:
    x == y + z
    |    |   |
    |    |   3
    |    2
    1

data.foo.bar.test_contains_print_fail: FAIL (0s)


data.foo.baz.p.q.r.test_quz: FAIL (0s)


data.foo.qux.test_cases_nested: FAIL (0s)

  two: FAIL
    foo: FAIL
      
        policy5.rego:5:
          x == y + z
          |    |   |
          |    |   3
          |    2
          1      

SUMMARY
--------------------------------------------------------------------------------
policy1.rego:2:
data.foo.bar.test_qux: ERROR (0s)
  some err
policy1.rego:3:
data.foo.bar.test_corge: FAIL (0s)
policy1.rego:4:
data.foo.bar.todo_test_qux: SKIPPED

policy2.rego:1:
data.foo.bar.test_contains_print_fail: FAIL (0s)

  fake print output2


policy3.rego:1:
data.foo.baz.p.q.r.test_quz: FAIL (0s)

policy5.rego:1:
data.foo.qux.test_cases_nested: FAIL (0s)
  two: FAIL
    foo: FAIL
--------------------------------------------------------------------------------
PASS: 5/11
FAIL: 4/11
SKIPPED: 1/11
ERROR: 1/11
`

	if exp != buf.String() {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, buf.String())
	}
}

func TestPrettyReporter(t *testing.T) {
	var buf bytes.Buffer

	// supply fake trace events to verify that traces are suppressed without verbose
	// flag.
	ts := []*Result{
		{
			Package: "data.foo.bar",
			Name:    "test_baz",
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy1.rego",
				Row:  1,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_qux",
			Error:   errors.New("some err"),
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy1.rego",
				Row:  2,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_corge",
			Fail:    true,
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy1.rego",
				Row:  3,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "todo_test_qux",
			Skip:    true,
			Trace:   nil,
			Location: &ast.Location{
				File: "policy1.rego",
				Row:  4,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_contains_print_pass",
			Output:  []byte("fake print output\n"),
			Location: &ast.Location{
				File: "policy1.rego",
				Row:  5,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_contains_print_fail",
			Fail:    true,
			Output:  []byte("fake print output2\n"),
			Location: &ast.Location{
				File: "policy2.rego",
				Row:  1,
			},
		},
		{
			Package: "data.foo.baz",
			Name:    "p.q.r.test_quz",
			Fail:    true,
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy3.rego",
				Row:  1,
			},
		},
		{
			Package: "data.foo.qux",
			Name:    "test_cases",
			Trace:   getFakeTraceEvents(),
			Fail:    true,
			// Will be sorted to "bar", "baz", "foo" in output for stability
			SubResults: SubResultMap{
				"foo": {
					Name: "foo",
					Fail: false,
				},
				"bar": {
					Name: "bar",
					Fail: true,
				},
				"baz": {
					Name: "baz",
					Fail: false,
				},
			},
			Location: &ast.Location{
				File: "policy4.rego",
				Row:  1,
			},
		},
		{
			Package: "data.foo.qux",
			Name:    "test_cases_nested",
			Trace:   getFakeTraceEvents(),
			Fail:    true,
			SubResults: SubResultMap{
				"one": {
					Name: "one",
					Fail: false,
					SubResults: SubResultMap{
						"foo": {
							Name: "foo",
							Fail: false,
						},
						"bar": {
							Name: "bar",
							Fail: false,
						},
					},
				},
				"two": {
					Name: "two",
					Fail: true,
					SubResults: SubResultMap{
						"foo": {
							Name: "foo",
							Fail: true,
						},
						"bar": {
							Name: "bar",
							Fail: false,
						},
					},
				},
			},
			Location: &ast.Location{
				File: "policy4.rego",
				Row:  2,
			},
		},
	}

	r := PrettyReporter{
		Output:  &buf,
		Verbose: false,
	}
	ch := resultsChan(ts)
	if err := r.Report(ch); err != nil {
		t.Fatal(err)
	}

	exp := `policy1.rego:2:
data.foo.bar.test_qux: ERROR (0s)
  some err
policy1.rego:3:
data.foo.bar.test_corge: FAIL (0s)
policy1.rego:4:
data.foo.bar.todo_test_qux: SKIPPED

policy2.rego:1:
data.foo.bar.test_contains_print_fail: FAIL (0s)

  fake print output2


policy3.rego:1:
data.foo.baz.p.q.r.test_quz: FAIL (0s)

policy4.rego:1:
data.foo.qux.test_cases: FAIL (0s)
  bar: FAIL
policy4.rego:2:
data.foo.qux.test_cases_nested: FAIL (0s)
  two: FAIL
    foo: FAIL
--------------------------------------------------------------------------------
PASS: 7/14
FAIL: 5/14
SKIPPED: 1/14
ERROR: 1/14
`

	if exp != buf.String() {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, buf.String())
	}
}

func TestJSONReporter(t *testing.T) {
	tests := []struct {
		name     string
		sort     string
		expected string
	}{
		{
			name:     "default not sorted",
			expected: "JSONReporter.json",
		},
		{
			name:     "sorted",
			sort:     "duration",
			expected: "JSONReporter_sorted.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			ts := []*Result{
				{
					Package:  "data.foo.bar",
					Name:     "test_baz",
					Trace:    getFakeTraceEvents(),
					Duration: 7,
				},
				{
					Package:  "data.foo.bar",
					Name:     "test_qux",
					Error:    errors.New("some err"),
					Trace:    getFakeTraceEvents(),
					Duration: 6,
				},
				{
					Package:  "data.foo.bar",
					Name:     "test_corge",
					Fail:     true,
					Trace:    getFakeTraceEvents(),
					Duration: 5,
				},
				{
					Package:  "data.foo.bar",
					Name:     "todo_test_qux",
					Skip:     true,
					Trace:    nil,
					Duration: 4,
				},
				{
					Package:  "data.foo.bar",
					Name:     "test_contains_print",
					Output:   []byte("fake print output\n"),
					Duration: 3,
				},
				{
					Package:  "data.foo.baz",
					Name:     "p.q.r.test_quz",
					Duration: 2,
				},
				{
					Package: "data.foo.qux",
					Name:    "test_cases_nested",
					Trace:   getFakeTraceEvents(),
					Fail:    true,
					SubResults: SubResultMap{
						"one": {
							Name: "one",
							Fail: false,
							SubResults: SubResultMap{
								"foo": {
									Name: "foo",
									Fail: false,
								},
								"bar": {
									Name: "bar",
									Fail: false,
								},
							},
						},
						"two": {
							Name: "two",
							Fail: true,
							SubResults: SubResultMap{
								"foo": {
									Name: "foo",
									Fail: true,
								},
								"bar": {
									Name: "bar",
									Fail: false,
								},
							},
						},
					},
					Location: &ast.Location{
						File: "policy5.rego",
					},
					Duration: 1,
				},
			}

			r := JSONReporter{
				Output: &buf,
				Sort:   tc.sort,
			}

			ch := resultsChan(ts)

			if err := r.Report(ch); err != nil {
				t.Fatal(err)
			}

			testdata, err := os.ReadFile(filepath.Join("testdata", tc.expected))
			if err != nil {
				t.Fatal(err)
			}
			var actual, exp any
			if err := json.Unmarshal(testdata, &exp); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(buf.Bytes(), &actual); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(actual, exp) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, actual)
			}
		})
	}
}

func TestPrettyReporterVerboseBenchmark(t *testing.T) {
	var buf bytes.Buffer

	// supply fake trace events for each kind of event to ensure that only failures
	// report traces.
	ts := []*Result{
		{
			Package: "data.foo.bar",
			Name:    "test_baz",
			BenchmarkResult: &testing.BenchmarkResult{
				N:         1000,
				T:         123000,
				Bytes:     0,
				MemAllocs: 0,
				MemBytes:  0,
				Extra:     nil,
			},
		},
		{
			Package:         "data.foo.bar",
			Name:            "test_qux",
			Error:           errors.New("some err"),
			BenchmarkResult: nil,
		},
		{
			Package: "data.foo.bar",
			Name:    "test_corge",
			Trace:   getFakeTraceEvents(),
			Fail:    true,
			BenchmarkResult: &testing.BenchmarkResult{
				N:         100,
				T:         12300,
				Bytes:     0,
				MemAllocs: 567,
				MemBytes:  890,
				Extra:     nil,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_cases_fail",
			Fail:    true,
			Trace:   getFakeTraceEvents(),
			SubResults: SubResultMap{
				"one": {
					Name: "one",
					Fail: false,
				},
				"two": {
					Name: "two",
					Fail: true,
				},
			},
			BenchmarkResult: &testing.BenchmarkResult{
				N:         100,
				T:         12300,
				Bytes:     0,
				MemAllocs: 567,
				MemBytes:  890,
				Extra:     nil,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_cases_ok",
			Fail:    false,
			SubResults: SubResultMap{
				"one": {
					Name: "one",
					Fail: false,
				},
				"two": {
					Name: "two",
					Fail: false,
				},
			},
			BenchmarkResult: &testing.BenchmarkResult{
				N:         2000,
				T:         123000,
				Bytes:     0,
				MemAllocs: 567,
				MemBytes:  890,
				Extra:     nil,
			},
		},
	}

	r := PrettyReporter{
		Output:           &buf,
		Verbose:          true,
		BenchmarkResults: true,
	}

	ch := resultsChan(ts)
	if err := r.Report(ch); err != nil {
		t.Fatal(err)
	}

	exp := fixtureReporterVerboseBenchmark
	if exp != buf.String() {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, buf.String())
	}
}

func TestPrettyReporterVerboseBenchmarkShowAllocations(t *testing.T) {
	var buf bytes.Buffer

	// supply fake trace events for each kind of event to ensure that only failures
	// report traces.
	ts := []*Result{
		{
			Package: "data.foo.bar",
			Name:    "test_baz",
			BenchmarkResult: &testing.BenchmarkResult{
				N:         1000,
				T:         123000,
				Bytes:     0,
				MemAllocs: 678,
				MemBytes:  91011,
				Extra: map[string]float64{
					"timer_rego_query_eval_ns/op": 123,
				},
			},
		},
		{
			Package:         "data.foo.bar",
			Name:            "test_qux",
			Error:           errors.New("some err"),
			BenchmarkResult: nil,
		},
		{
			Package: "data.foo.bar",
			Name:    "test_corge",
			Fail:    true,
			BenchmarkResult: &testing.BenchmarkResult{
				N:         100,
				T:         12300,
				Bytes:     0,
				MemAllocs: 567,
				MemBytes:  890,
				Extra: map[string]float64{
					"timer_rego_query_eval_ns/op": 123,
				},
			},
		},
	}

	r := PrettyReporter{
		Output:                   &buf,
		Verbose:                  true,
		BenchmarkResults:         true,
		BenchMarkShowAllocations: true,
	}

	ch := resultsChan(ts)
	if err := r.Report(ch); err != nil {
		t.Fatal(err)
	}

	exp := fixtureReporterVerboseBenchmarkShowAllocations
	if exp != buf.String() {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, buf.String())
	}
}

func TestPrettyReporterVerboseBenchmarkShowAllocationsGoBenchFormat(t *testing.T) {
	var buf bytes.Buffer

	// supply fake trace events for each kind of event to ensure that only failures
	// report traces.
	ts := []*Result{
		{
			Package: "data.foo.bar",
			Name:    "test_baz",
			BenchmarkResult: &testing.BenchmarkResult{
				N:         1000,
				T:         123000,
				Bytes:     0,
				MemAllocs: 678,
				MemBytes:  91011,
				Extra: map[string]float64{
					"timer_rego_query_eval_ns/op": 123,
				},
			},
		},
		{
			Package:         "data.foo.bar",
			Name:            "test_qux",
			Error:           errors.New("some err"),
			BenchmarkResult: nil,
		},
		{
			Package: "data.foo.bar",
			Name:    "test_corge",
			Fail:    true,
			BenchmarkResult: &testing.BenchmarkResult{
				N:         100,
				T:         12300,
				Bytes:     0,
				MemAllocs: 567,
				MemBytes:  890,
				Extra: map[string]float64{
					"timer_rego_query_eval_ns/op": 123,
				},
			},
		},
	}

	r := PrettyReporter{
		Output:                   &buf,
		Verbose:                  true,
		BenchmarkResults:         true,
		BenchMarkShowAllocations: true,
		BenchMarkGoBenchFormat:   true,
	}

	ch := resultsChan(ts)
	if err := r.Report(ch); err != nil {
		t.Fatal(err)
	}

	exp := fixtureReporterVerboseBenchmarkShowAllocationsGoBenchFormat
	if exp != buf.String() {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, buf.String())
	}
}

func TestJSONReporterBenchmark(t *testing.T) {
	var buf bytes.Buffer
	ts := []*Result{
		{
			Package: "data.foo.bar",
			Name:    "test_baz",
			BenchmarkResult: &testing.BenchmarkResult{
				N:         1000,
				T:         123000,
				Bytes:     0,
				MemAllocs: 678,
				MemBytes:  91011,
				Extra: map[string]float64{
					"timer_rego_query_eval_ns/op": 123,
				},
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_qux",
			Error:   errors.New("some err"),
		},
		{
			Package: "data.foo.bar",
			Name:    "test_corge",
			Fail:    true,
		},
		{
			Package: "data.foo.bar",
			Name:    "todo_test_qux",
			Skip:    true,
		},
		{
			Package: "data.foo.bar",
			Name:    "test_cases_fail",
			Fail:    true,
			SubResults: SubResultMap{
				"one": {
					Name: "one",
					Fail: false,
				},
				"two": {
					Name: "two",
					Fail: true,
				},
			},
			BenchmarkResult: &testing.BenchmarkResult{
				N:         100,
				T:         12300,
				Bytes:     0,
				MemAllocs: 567,
				MemBytes:  890,
				Extra:     nil,
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_cases_ok",
			Fail:    false,
			SubResults: SubResultMap{
				"one": {
					Name: "one",
					Fail: false,
				},
				"two": {
					Name: "two",
					Fail: true,
				},
			},
			BenchmarkResult: &testing.BenchmarkResult{
				N:         2000,
				T:         123000,
				Bytes:     0,
				MemAllocs: 567,
				MemBytes:  890,
				Extra:     nil,
			},
		},
	}

	r := JSONReporter{
		Output: &buf,
	}

	ch := resultsChan(ts)

	if err := r.Report(ch); err != nil {
		t.Fatal(err)
	}

	exp := util.MustUnmarshalJSON([]byte(`[
  {
    "location": null,
    "package": "data.foo.bar",
    "name": "test_baz",
    "duration": 0,
    "benchmark_result": {
      "N": 1000,
      "T": 123000,
      "Bytes": 0,
      "MemAllocs": 678,
      "MemBytes": 91011,
      "Extra": {
        "timer_rego_query_eval_ns/op": 123
      }
    }
  },
  {
    "location": null,
    "package": "data.foo.bar",
    "name": "test_qux",
    "error": {},
    "duration": 0
  },
  {
    "location": null,
    "package": "data.foo.bar",
    "name": "test_corge",
    "fail": true,
    "duration": 0
  },
  {
    "location": null,
    "package": "data.foo.bar",
    "name": "todo_test_qux",
    "skip": true,
    "duration": 0
  },
  {
    "location": null,
    "package": "data.foo.bar",
    "name": "test_cases_fail",
    "fail": true,
    "duration": 0,
    "benchmark_result": {
      "N": 100,
      "T": 12300,
      "Bytes": 0,
      "MemAllocs": 567,
      "MemBytes": 890,
      "Extra": null
    },
    "sub_results": {
      "one": {
        "name": "one"
      },
      "two": {
        "name": "two",
        "fail": true
      }
    }
  },
  {
    "location": null,
    "package": "data.foo.bar",
    "name": "test_cases_ok",
    "duration": 0,
    "benchmark_result": {
      "N": 2000,
      "T": 123000,
      "Bytes": 0,
      "MemAllocs": 567,
      "MemBytes": 890,
      "Extra": null
    },
    "sub_results": {
      "one": {
        "name": "one"
      },
      "two": {
        "name": "two",
        "fail": true
      }
    }
  }
]
`))

	result := util.MustUnmarshalJSON(buf.Bytes())

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", string(util.MustMarshalJSON(exp)), string(util.MustMarshalJSON(result)))
	}
}

func TestPrettyReporterFmtBenchmark(t *testing.T) {
	benchResult := &testing.BenchmarkResult{
		N:         1000,
		T:         1230000,
		Bytes:     0,
		MemAllocs: 10000,
		MemBytes:  123456,
		Extra: map[string]float64{
			"extra1": 99887766,
			"extra2": 11223344,
		},
	}
	cases := []struct {
		note            string
		tr              *Result
		goBenchFmt      bool
		showAllocations bool
		expectedName    string
	}{
		{
			note: "base",
			tr: &Result{
				Package:         "data.foo.bar",
				Name:            "test_baz",
				BenchmarkResult: benchResult,
			},
			expectedName: "data.foo.bar.test_baz",
		},
		{
			note: "with memory",
			tr: &Result{
				Package:         "data.foo.bar",
				Name:            "test_baz",
				BenchmarkResult: benchResult,
			},
			expectedName:    "data.foo.bar.test_baz",
			showAllocations: true,
		},
		{
			note: "gobench format",
			tr: &Result{
				Package:         "data.foo.bar",
				Name:            "test_baz",
				BenchmarkResult: benchResult,
			},
			expectedName: "BenchmarkDataFooBarTestBaz",
			goBenchFmt:   true,
		},
		{
			note: "gobench format with memory",
			tr: &Result{
				Package:         "data.foo.bar",
				Name:            "test_baz",
				BenchmarkResult: benchResult,
			},
			expectedName:    "BenchmarkDataFooBarTestBaz",
			goBenchFmt:      true,
			showAllocations: true,
		},
		{
			note: "gobench format extra underscores",
			tr: &Result{
				Package:         "data.foo.bar",
				Name:            "_test___baz__",
				BenchmarkResult: benchResult,
			},
			expectedName: "BenchmarkDataFooBarTestBaz",
			goBenchFmt:   true,
		},
		{
			note: "gobench format already camelcase",
			tr: &Result{
				Package:         "data.foo.bar",
				Name:            "test_fooBar",
				BenchmarkResult: benchResult,
			},
			expectedName: "BenchmarkDataFooBarTestFooBar",
			goBenchFmt:   true,
		},

		{
			note: "gobench format underscore in path",
			tr: &Result{
				Package:         "data.foo_bar.test_thing__",
				Name:            "test_fooBar",
				BenchmarkResult: benchResult,
			},
			expectedName: "BenchmarkDataFooBarTestThingTestFooBar",
			goBenchFmt:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			r := PrettyReporter{
				BenchmarkResults:         true,
				BenchMarkShowAllocations: tc.showAllocations,
				BenchMarkGoBenchFormat:   tc.goBenchFmt,
			}

			actual := r.fmtBenchmark(tc.tr)

			fields := strings.Fields(actual)

			// Expect the first field to be the name
			name := fields[0]
			if name != tc.expectedName {
				t.Fatalf("Expected first field of formatted result to be %s, got %s\n\n\t Full Result: %s", tc.expectedName, name, actual)
			}

			// The next field should be the count (N)
			n, err := strconv.Atoi(fields[1])
			if err != nil {
				t.Fatalf("Unexpected error parsing count (N): %s", err)
			}
			if n != tc.tr.BenchmarkResult.N {
				t.Fatalf("Expected N == %d, got %d", tc.tr.BenchmarkResult.N, n)
			}

			// Every field after this is optional, and the order doesn't really matter. Expect pairs of fields
			// with the first being the value and second being the name
			results := map[string]float64{}
			for i := 2; i < len(fields); i += 2 {
				v, err := strconv.ParseFloat(fields[i], 64)
				if err != nil {
					t.Fatalf("Unexpected error parsing value '%s' for key '%s': %s", fields[i], fields[i+1], err)
				}
				results[fields[i+1]] = v
			}

			requiredKeys := []string{
				"ns/op",
			}

			for k := range tc.tr.BenchmarkResult.Extra {
				requiredKeys = append(requiredKeys, k)
			}

			if tc.showAllocations {
				requiredKeys = append(requiredKeys, "B/op", "allocs/op")
			}

			for _, k := range requiredKeys {
				_, ok := results[k]
				if !ok {
					t.Errorf("Missing expected key %s in results, got %+v", k, results)
				}
			}
		})
	}
}

func resultsChan(ts []*Result) chan *Result {
	ch := make(chan *Result)
	go func() {
		for _, tr := range ts {
			ch <- tr
		}
		close(ch)
	}()
	return ch
}
