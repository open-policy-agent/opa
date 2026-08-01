package ast_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

// TestIssue8302EvalRegressionProbe checks the evaluation-level symptom of #8302:
// a body with an indirect dependency through a comprehension compiles cleanly but
// used to evaluate to an empty result set because reorderBodyForSafety scheduled
// expressions before their inputs were genuinely grounded.
//
// Policy shape taken from https://github.com/open-policy-agent/opa/issues/8302.
func TestIssue8302EvalRegressionProbe(t *testing.T) {
	mod := `package test

f(x) := x + 1

a := z if {
	y = f(x)
	z = f(y)
	x = 2
}

b := z if {
	y = f([v | v = x][0])
	z = f(y)
	x = 2
}

c := z if {
	x = 2
	y = f([v | v = x][0])
	z = f(y)
}
`

	cases := []struct {
		name string
		q    string
		want string
	}{
		{"a", "data.test.a", "4"},
		{"b", "data.test.b", "4"},
		{"c", "data.test.c", "4"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := rego.New(rego.Query(tc.q), rego.Module("test.rego", mod))
			rs, err := r.Eval(context.Background())
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			if len(rs) == 0 {
				t.Fatalf("expected a result, got empty ResultSet (issue #8302 silent-drop symptom)")
			}
			got := fmt.Sprint(rs[0].Expressions[0].Value)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
