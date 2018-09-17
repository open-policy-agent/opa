package tester

import (
	"bytes"
	"fmt"
	"testing"
)

func TestPrettyReporter(t *testing.T) {

	ts := []*Result{
		{nil, "data.foo.bar", "test_baz", false, nil, 0},
		{nil, "data.foo.bar", "test_qux", false, fmt.Errorf("some err"), 0},
		{nil, "data.foo.bar", "test_corge", true, nil, 0},
	}

	var buf bytes.Buffer

	r := PrettyReporter{
		Output:  &buf,
		Verbose: true,
	}

	ch := make(chan *Result)
	go func() {
		for _, tr := range ts {
			ch <- tr
		}
		close(ch)
	}()

	if err := r.Report(ch); err != nil {
		t.Fatal(err)
	}

	exp := `data.foo.bar.test_baz: PASS (0s)
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
