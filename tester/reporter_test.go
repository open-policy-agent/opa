package tester

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
)

func getFakeTraceEvents() []*topdown.Event {
	node := ast.MustParseExpr("true = false")
	return []*topdown.Event{
		{
			Op:       topdown.FailOp,
			Node:     node,
			Location: node.Loc(),
			QueryID:  0,
			ParentID: 0,
		},
	}
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
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_qux",
			Error:   fmt.Errorf("some err"),
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy1.rego",
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_corge",
			Fail:    true,
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy2.rego",
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "todo_test_qux",
			Skip:    true,
			Trace:   nil,
			Location: &ast.Location{
				File: "policy2.rego",
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_contains_print",
			Output:  []byte("fake print output\n"),
			Location: &ast.Location{
				File: "policy3.rego",
			},
		},
		{
			Package: "data.foo.baz",
			Name:    "p.q.r.test_quz",
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy4.rego",
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

  query:1     | Fail true = false

SUMMARY
--------------------------------------------------------------------------------
policy1.rego:
data.foo.bar.test_baz: PASS (0s)
data.foo.bar.test_qux: ERROR (0s)
  some err

policy2.rego:
data.foo.bar.test_corge: FAIL (0s)
data.foo.bar.todo_test_qux: SKIPPED

policy3.rego:
data.foo.bar.test_contains_print: PASS (0s)

  fake print output


policy4.rego:
data.foo.baz.p.q.r.test_quz: PASS (0s)
--------------------------------------------------------------------------------
PASS: 3/6
FAIL: 1/6
SKIPPED: 1/6
ERROR: 1/6
`

	str := buf.String()

	if exp != str {
		t.Fatalf("Expected (%d bytes):\n\n%v\n\nGot (%d bytes):\n\n%v", len(exp), exp, len(str), str)
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
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_qux",
			Error:   fmt.Errorf("some err"),
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy1.rego",
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_corge",
			Fail:    true,
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy1.rego",
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "todo_test_qux",
			Skip:    true,
			Trace:   nil,
			Location: &ast.Location{
				File: "policy1.rego",
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_contains_print_pass",
			Output:  []byte("fake print output\n"),
			Location: &ast.Location{
				File: "policy1.rego",
			},
		},
		{
			Package: "data.foo.bar",
			Name:    "test_contains_print_fail",
			Fail:    true,
			Output:  []byte("fake print output2\n"),
			Location: &ast.Location{
				File: "policy2.rego",
			},
		},
		{
			Package: "data.foo.baz",
			Name:    "p.q.r.test_quz",
			Fail:    true,
			Trace:   getFakeTraceEvents(),
			Location: &ast.Location{
				File: "policy3.rego",
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

	exp := `policy1.rego:
data.foo.bar.test_qux: ERROR (0s)
  some err
data.foo.bar.test_corge: FAIL (0s)
data.foo.bar.todo_test_qux: SKIPPED

policy2.rego:
data.foo.bar.test_contains_print_fail: FAIL (0s)

  fake print output2


policy3.rego:
data.foo.baz.p.q.r.test_quz: FAIL (0s)
--------------------------------------------------------------------------------
PASS: 2/7
FAIL: 3/7
SKIPPED: 1/7
ERROR: 1/7
`

	if exp != buf.String() {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, buf.String())
	}
}

func TestJSONReporter(t *testing.T) {
	var buf bytes.Buffer
	ts := []*Result{
		{
			Package: "data.foo.bar",
			Name:    "test_baz",
			Trace:   getFakeTraceEvents(),
		},
		{
			Package: "data.foo.bar",
			Name:    "test_qux",
			Error:   fmt.Errorf("some err"),
			Trace:   getFakeTraceEvents(),
		},
		{
			Package: "data.foo.bar",
			Name:    "test_corge",
			Fail:    true,
			Trace:   getFakeTraceEvents(),
		},
		{
			Package: "data.foo.bar",
			Name:    "todo_test_qux",
			Skip:    true,
			Trace:   nil,
		},
		{
			Package: "data.foo.bar",
			Name:    "test_contains_print",
			Output:  []byte("fake print output\n"),
		},
		{
			Package: "data.foo.baz",
			Name:    "p.q.r.test_quz",
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
    "trace": [
      {
        "Op": "Fail",
        "Node": {
          "index": 0,
          "terms": [
            {
              "type": "ref",
              "value": [
                {
                  "type": "var",
                  "value": "eq"
                }
              ]
            },
            {
              "type": "boolean",
              "value": true
            },
            {
              "type": "boolean",
              "value": false
            }
          ]
        },
        "Location": {
          "file": "",
          "row": 1,
          "col": 1
        },
        "QueryID": 0,
        "ParentID": 0,
        "Locals": null,
        "LocalMetadata": null,
        "Message": "",
        "Ref": null
      }
    ]
  },
  {
    "location": null,
    "package": "data.foo.bar",
    "name": "test_qux",
    "error": {},
    "duration": 0,
    "trace": [
      {
        "Op": "Fail",
        "Node": {
          "index": 0,
          "terms": [
            {
              "type": "ref",
              "value": [
                {
                  "type": "var",
                  "value": "eq"
                }
              ]
            },
            {
              "type": "boolean",
              "value": true
            },
            {
              "type": "boolean",
              "value": false
            }
          ]
        },
        "Location": {
          "file": "",
          "row": 1,
          "col": 1
        },
        "QueryID": 0,
        "ParentID": 0,
        "Locals": null,
        "LocalMetadata": null,
        "Message": "",
        "Ref": null
      }
    ]
  },
  {
    "location": null,
    "package": "data.foo.bar",
    "name": "test_corge",
    "fail": true,
    "duration": 0,
    "trace": [
      {
        "Op": "Fail",
        "Node": {
          "index": 0,
          "terms": [
            {
              "type": "ref",
              "value": [
                {
                  "type": "var",
                  "value": "eq"
                }
              ]
            },
            {
              "type": "boolean",
              "value": true
            },
            {
              "type": "boolean",
              "value": false
            }
          ]
        },
        "Location": {
          "file": "",
          "row": 1,
          "col": 1
        },
        "QueryID": 0,
        "ParentID": 0,
        "Locals": null,
        "LocalMetadata": null,
        "Message": "",
        "Ref": null
      }
    ]
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
	  "name": "test_contains_print",
	  "output": "ZmFrZSBwcmludCBvdXRwdXQK",
	  "duration": 0
  },
  {
	"location": null,
	"package": "data.foo.baz",
	"name": "p.q.r.test_quz",
	"duration": 0
}
]
`))

	result := util.MustUnmarshalJSON(buf.Bytes())

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, result)
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
			Error:           fmt.Errorf("some err"),
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
			Error:           fmt.Errorf("some err"),
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
			Error:           fmt.Errorf("some err"),
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
			Error:   fmt.Errorf("some err"),
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
    "location":null,
    "duration":0,
    "name":"todo_test_qux",
    "package":"data.foo.bar",
    "skip":true
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
