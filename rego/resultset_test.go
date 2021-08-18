package rego_test

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/rego"
)

func TestResultSetAllowed(t *testing.T) {
	tests := []struct {
		note     string
		module   string
		query    string
		expected bool
	}{
		{
			note: "simplest true",
			module: `package authz
allow { true }
`,
			query:    "data.authz.allow",
			expected: true,
		},
		{
			note: "simplest false",
			module: `package authz
default allow = false
`,
			query:    "data.authz.allow",
			expected: false,
		},
		{
			note: "true value + bindings",
			module: `package authz
allow { true }
`,
			query:    "data.authz.allow = x",
			expected: false,
		},
		{
			note: "object response, bound to var in query",
			module: `package authz
resp = { "allow": true } { true }
`,
			query:    "data.authz.resp = x",
			expected: false,
		},
		{
			note: "object response, treated as false",
			module: `package authz
resp = { "allow": true } { true }
`,
			query:    "data.authz.resp",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			r := rego.New(
				rego.Query(tc.query),
				rego.Module("", tc.module),
			)
			rs, err := r.Eval(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if exp, act := tc.expected, rs.Allowed(); exp != act {
				t.Errorf("expected %v, got %v", exp, act)
			}
		})
	}
}
