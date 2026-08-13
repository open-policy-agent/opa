package ast

import (
	"strings"
	"testing"
)

// Compile-level probes around issue #8302: safe bodies that must still compile,
// the #8302 shape itself, and genuinely unsafe bodies that must still be rejected.
func TestIssue8302RegressionProbe(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		wantErr bool
		errSub  string
	}{
		{
			note: "safe/simple equality",
			module: `package t
p if { x = 1; y = x }`,
		},
		{
			note: "safe/ordered comprehension",
			module: `package t
f(x) := x
p := z if { x = 2; y = f([v | v = x][0]); z = f(y) }`,
		},
		{
			note: "safe/issue-8302 shape must compile",
			module: `package t
f(x) := x
p := z if { y = f([v | v = x][0]); z = f(y); x = 2 }`,
		},
		{
			note: "safe/double indirection through comprehension",
			module: `package t
f(x) := x
p := w if {
	y = f([v | v = x][0])
	z = f([v | v = y][0])
	w = f(z)
	x = 2
}`,
		},
		{
			note: "safe/with modifier",
			module: `package t
f(x) := x
p := z if { y = f([v | v = x][0]) with input as {}; z = f(y); x = 2 }`,
		},
		{
			note: "safe/every after grounding",
			module: `package t
p if { xs = [1]; every y in xs { y == 1 } }`,
		},
		{
			note: "safe/issue-8302 with +1 function",
			module: `package t
f(x) := x + 1
p := z if { y = f([v | v = x][0]); z = f(y); x = 2 }`,
		},
		{
			note: "unsafe/unbound var",
			module: `package t
p if { y = x }`,
			wantErr: true,
			errSub:  "unsafe",
		},
		{
			note: "unsafe/circular unify",
			module: `package t
p if { x = y; y = x }`,
			wantErr: true,
			errSub:  "unsafe",
		},
		{
			note: "unsafe/unbound comprehension var",
			module: `package t
p if { xs = [v | v = x] }`,
			wantErr: true,
			errSub:  "unsafe",
		},
		{
			note: "unsafe/negation of unbound",
			module: `package t
p if { not x }`,
			wantErr: true,
			errSub:  "unsafe",
		},
		{
			note: "unsafe/:= cannot reorder through comprehension (directional)",
			module: `package t
f(x) := x
p := z if { y := f([v | v = x][0]); z := f(y); x := 2 }`,
			wantErr: true,
			errSub:  "unsafe",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler().WithEnablePrintStatements(true)
			c.Compile(map[string]*Module{
				"t": MustParseModule(tc.module),
			})
			failed := c.Failed()
			if tc.wantErr && !failed {
				t.Fatalf("expected compile error, got success")
			}
			if !tc.wantErr && failed {
				t.Fatalf("unexpected compile errors: %v", c.Errors)
			}
			if tc.wantErr && tc.errSub != "" {
				joined := c.Errors.Error()
				if !strings.Contains(joined, tc.errSub) {
					t.Fatalf("expected error containing %q, got %v", tc.errSub, c.Errors)
				}
			}
		})
	}
}
