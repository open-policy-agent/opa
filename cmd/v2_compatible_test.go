package cmd

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestCheckV2Compatible(t *testing.T) {
	policy := `package test

p if { true and false }
`

	cases := []struct {
		note         string
		v0Compatible bool
		v1Compatible bool
		v2Compatible bool
		expErr       bool
	}{
		{note: "default", expErr: true},
		{note: "v1-compatible", v1Compatible: true, expErr: true},
		{note: "v2-compatible", v2Compatible: true},
		{note: "v2-compatible takes precedence over v1-compatible", v1Compatible: true, v2Compatible: true},
		{note: "v0-compatible takes precedence over v2-compatible", v0Compatible: true, v2Compatible: true, expErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(map[string]string{"test.rego": policy}, func(root string) {
				params := newCheckParams()
				params.v0Compatible = tc.v0Compatible
				params.v1Compatible = tc.v1Compatible
				params.v2Compatible = tc.v2Compatible

				err := checkModules(params, []string{root})
				if tc.expErr && err == nil {
					t.Fatal("expected error but got none")
				}
				if !tc.expErr && err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		})
	}
}

func TestV2CompatibleRegoVersion(t *testing.T) {
	cases := []struct {
		note string
		got  func(v0, v1, v2 bool) ast.RegoVersion
	}{
		{"build", func(v0, v1, v2 bool) ast.RegoVersion {
			return (&buildParams{v0Compatible: v0, v1Compatible: v1, v2Compatible: v2}).regoVersion()
		}},
		{"deps", func(v0, v1, v2 bool) ast.RegoVersion {
			return (&depsCommandParams{v0Compatible: v0, v1Compatible: v1, v2Compatible: v2}).regoVersion()
		}},
		{"eval", func(v0, v1, v2 bool) ast.RegoVersion {
			return (&evalCommandParams{v0Compatible: v0, v1Compatible: v1, v2Compatible: v2}).regoVersion()
		}},
		{"fmt", func(v0, v1, v2 bool) ast.RegoVersion {
			return (&fmtCommandParams{v0Compatible: v0, v1Compatible: v1, v2Compatible: v2}).regoVersion()
		}},
		{"inspect", func(v0, v1, v2 bool) ast.RegoVersion {
			return (&inspectCommandParams{v0Compatible: v0, v1Compatible: v1, v2Compatible: v2}).regoVersion()
		}},
		{"oracle", func(v0, v1, v2 bool) ast.RegoVersion {
			return (&findDefinitionParams{v0Compatible: v0, v1Compatible: v1, v2Compatible: v2}).regoVersion()
		}},
		{"parse", func(v0, v1, v2 bool) ast.RegoVersion {
			return (&parseParams{v0Compatible: v0, v1Compatible: v1, v2Compatible: v2}).regoVersion()
		}},
		{"refactor", func(v0, v1, v2 bool) ast.RegoVersion {
			return (&moveCommandParams{v0Compatible: v0, v1Compatible: v1, v2Compatible: v2}).regoVersion()
		}},
		{"test", func(v0, v1, v2 bool) ast.RegoVersion {
			return (&testCommandParams{v0Compatible: v0, v1Compatible: v1, v2Compatible: v2}).RegoVersion()
		}},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			if v := tc.got(false, false, true); v != ast.RegoV2 {
				t.Errorf("v2-compatible: expected %v, got %v", ast.RegoV2, v)
			}
			if v := tc.got(false, true, true); v != ast.RegoV2 {
				t.Errorf("v1+v2-compatible: expected %v, got %v", ast.RegoV2, v)
			}
			if v := tc.got(true, false, true); v != ast.RegoV0 {
				t.Errorf("v0+v2-compatible: expected %v, got %v", ast.RegoV0, v)
			}
		})
	}
}
