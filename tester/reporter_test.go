package tester

import (
	"bytes"
	"fmt"
	"testing"
)

func TestPrettyReporterVerbose(t *testing.T) {
	var buf bytes.Buffer
	ts := []*Result{
		{nil, "data.foo.bar", "test_baz", false, nil, 0, "trace test_baz"},
		{nil, "data.foo.bar", "test_qux", false, fmt.Errorf("some err"), 0, "trace test_qux"},
		{nil, "data.foo.bar", "test_corge", true, nil, 0, "trace test_corge"},
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

trace test_corge

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
	ts := []*Result{
		{nil, "data.foo.bar", "test_baz", false, nil, 0, "trace test_baz"},
		{nil, "data.foo.bar", "test_qux", false, fmt.Errorf("some err"), 0, "trace test_qux"},
		{nil, "data.foo.bar", "test_corge", true, nil, 0, "trace test_corge"},
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
		{nil, "data.foo.bar", "test_baz", false, nil, 0, "trace test_baz"},
		{nil, "data.foo.bar", "test_qux", false, fmt.Errorf("some err"), 0, "trace test_qux"},
		{nil, "data.foo.bar", "test_corge", true, nil, 0, "trace test_corge"},
	}

	r := JSONReporter{
		Output: &buf,
	}
	ch := resultsChan(ts)
	if err := r.Report(ch); err != nil {
		t.Fatal(err)
	}

	exp := `[
  {
    "location": null,
    "package": "data.foo.bar",
    "name": "test_baz",
    "duration": 0,
    "trace": "trace test_baz"
  },
  {
    "location": null,
    "package": "data.foo.bar",
    "name": "test_qux",
    "error": {},
    "duration": 0,
    "trace": "trace test_qux"
  },
  {
    "location": null,
    "package": "data.foo.bar",
    "name": "test_corge",
    "fail": true,
    "duration": 0,
    "trace": "trace test_corge"
  }
]
`

	if exp != buf.String() {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, buf.String())
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
