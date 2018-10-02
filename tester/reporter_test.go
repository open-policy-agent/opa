package tester

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
)

func getFakeTraceEvents() []*topdown.Event {
	return []*topdown.Event{
		{
			Op:       topdown.FailOp,
			Node:     ast.MustParseExpr("true = false"),
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
		{nil, "data.foo.bar", "test_baz", false, nil, 0, getFakeTraceEvents()},
		{nil, "data.foo.bar", "test_qux", false, fmt.Errorf("some err"), 0, getFakeTraceEvents()},
		{nil, "data.foo.bar", "test_corge", true, nil, 0, getFakeTraceEvents()},
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

  | Fail true = false

SUMMARY
--------------------------------------------------------------------------------
data.foo.bar.test_baz: PASS (0s)
data.foo.bar.test_qux: ERROR (0s)
  some err
data.foo.bar.test_corge: FAIL (0s)
--------------------------------------------------------------------------------
PASS: 1/3
FAIL: 1/3
ERROR: 1/3
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
		{nil, "data.foo.bar", "test_baz", false, nil, 0, getFakeTraceEvents()},
		{nil, "data.foo.bar", "test_qux", false, fmt.Errorf("some err"), 0, getFakeTraceEvents()},
		{nil, "data.foo.bar", "test_corge", true, nil, 0, getFakeTraceEvents()},
	}

	r := PrettyReporter{
		Output:  &buf,
		Verbose: false,
	}
	ch := resultsChan(ts)
	if err := r.Report(ch); err != nil {
		t.Fatal(err)
	}

	exp := `data.foo.bar.test_qux: ERROR (0s)
  some err
data.foo.bar.test_corge: FAIL (0s)
--------------------------------------------------------------------------------
PASS: 1/3
FAIL: 1/3
ERROR: 1/3
`

	if exp != buf.String() {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, buf.String())
	}
}

func TestJSONReporter(t *testing.T) {
	var buf bytes.Buffer
	ts := []*Result{
		{nil, "data.foo.bar", "test_baz", false, nil, 0, getFakeTraceEvents()},
		{nil, "data.foo.bar", "test_qux", false, fmt.Errorf("some err"), 0, getFakeTraceEvents()},
		{nil, "data.foo.bar", "test_corge", true, nil, 0, getFakeTraceEvents()},
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
		  "QueryID": 0,
		  "ParentID": 0,
		  "Locals": null,
		  "Message": ""
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
		  "QueryID": 0,
		  "ParentID": 0,
		  "Locals": null,
		  "Message": ""
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
		  "QueryID": 0,
		  "ParentID": 0,
		  "Locals": null,
		  "Message": ""
		}
	  ]  }
]
`))

	result := util.MustUnmarshalJSON(buf.Bytes())

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, result)
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
