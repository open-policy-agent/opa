package rego_test

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
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
allow if true
`,
			query:    "data.authz.allow",
			expected: true,
		},
		{
			note: "simplest false",
			module: `package authz
default allow := false
`,
			query:    "data.authz.allow",
			expected: false,
		},
		{
			note: "true value + bindings",
			module: `package authz
allow if true
`,
			query:    "data.authz.allow = x",
			expected: false,
		},
		{
			note: "object response, bound to var in query",
			module: `package authz
resp := {"allow": true}
`,
			query:    "data.authz.resp = x",
			expected: false,
		},
		{
			note: "object response, treated as false",
			module: `package authz
resp := {"allow": true}
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
			rs, err := r.Eval(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if exp, act := tc.expected, rs.Allowed(); exp != act {
				t.Errorf("expected %v, got %v", exp, act)
			}
		})
	}
}

func TestResultValue(t *testing.T) {
	t.Run("bool", func(t *testing.T) {
		tests := []struct {
			note          string
			module        string
			query         string
			expectedValue bool
			expectedOk    bool
		}{
			{
				note: "true value",
				module: `package authz
allow if true
`,
				query:         "data.authz.allow",
				expectedValue: true,
				expectedOk:    true,
			},
			{
				note: "false value",
				module: `package authz
default allow := false
`,
				query:         "data.authz.allow",
				expectedValue: false,
				expectedOk:    true,
			},
			{
				note: "value with bindings",
				module: `package authz
allow if true
`,
				query:         "data.authz.allow = x",
				expectedValue: false,
				expectedOk:    false,
			},
		}

		for _, tc := range tests {
			t.Run(tc.note, func(t *testing.T) {
				r := rego.New(
					rego.Query(tc.query),
					rego.Module("", tc.module),
				)
				rs, err := r.Eval(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				val, ok := rego.ResultValue[bool](rs)
				if exp, act := tc.expectedOk, ok; exp != act {
					t.Errorf("expected ok=%v, got ok=%v", exp, act)
				}
				if ok && tc.expectedOk {
					if exp, act := tc.expectedValue, val; exp != act {
						t.Errorf("expected value=%v, got value=%v", exp, act)
					}
				}
			})
		}
	})

	t.Run("string", func(t *testing.T) {
		tests := []struct {
			note          string
			module        string
			query         string
			expectedValue string
			expectedOk    bool
		}{
			{
				note: "string value",
				module: `package authz
message := "hello world"
`,
				query:         "data.authz.message",
				expectedValue: "hello world",
				expectedOk:    true,
			},
			{
				note: "empty string",
				module: `package authz
message := ""
`,
				query:         "data.authz.message",
				expectedValue: "",
				expectedOk:    true,
			},
		}

		for _, tc := range tests {
			t.Run(tc.note, func(t *testing.T) {
				r := rego.New(
					rego.Query(tc.query),
					rego.Module("", tc.module),
				)
				rs, err := r.Eval(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				val, ok := rego.ResultValue[string](rs)
				if exp, act := tc.expectedOk, ok; exp != act {
					t.Errorf("expected ok=%v, got ok=%v", exp, act)
				}
				if ok && tc.expectedOk {
					if exp, act := tc.expectedValue, val; exp != act {
						t.Errorf("expected value=%q, got value=%q", exp, act)
					}
				}
			})
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		module := `package authz
message := "hello world"
`
		r := rego.New(
			rego.Query("data.authz.message"),
			rego.Module("", module),
		)
		rs, err := r.Eval(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		val, ok := rego.ResultValue[int](rs)
		if ok {
			t.Errorf("expected ok=false for wrong type conversion, got ok=true with value=%v", val)
		}
	})

	t.Run("object value", func(t *testing.T) {
		module := `package authz
resp := {"allow": true, "user": "alice"}
`
		r := rego.New(
			rego.Query("data.authz.resp"),
			rego.Module("", module),
		)
		rs, err := r.Eval(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		val, ok := rego.ResultValue[map[string]any](rs)
		if !ok {
			t.Fatal("expected ok=true for map type")
		}
		if exp, act := true, val["allow"]; exp != act {
			t.Errorf("expected allow=%v, got allow=%v", exp, act)
		}
		if exp, act := "alice", val["user"]; exp != act {
			t.Errorf("expected user=%v, got user=%v", exp, act)
		}
	})
}
