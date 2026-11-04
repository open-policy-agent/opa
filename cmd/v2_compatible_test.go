package cmd

import (
	"io"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util/test"

	"github.com/spf13/pflag"
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

func TestCompatibleFlag(t *testing.T) {
	cases := []struct {
		note   string
		args   []string
		exp    [3]bool // v0, v1, v2
		expErr bool
	}{
		{note: "unset"},
		{note: "v0", args: []string{"--compatible=v0"}, exp: [3]bool{true, false, false}},
		{note: "v1", args: []string{"--compatible=v1"}, exp: [3]bool{false, true, false}},
		{note: "v2", args: []string{"--compatible=v2"}, exp: [3]bool{false, false, true}},
		{note: "last value wins", args: []string{"--compatible=v1", "--compatible=v2"}, exp: [3]bool{false, false, true}},
		{note: "invalid", args: []string{"--compatible=v3"}, expErr: true},
		{note: "empty", args: []string{"--compatible="}, expErr: true},
		{note: "wrong case", args: []string{"--compatible=V1"}, expErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			var got [3]bool
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			fs.SetOutput(io.Discard)
			addCompatibleFlag(fs, &got[0], &got[1], &got[2])

			err := fs.Parse(tc.args)
			if tc.expErr != (err != nil) {
				t.Fatalf("expected error: %v, got: %v", tc.expErr, err)
			}
			if got != tc.exp {
				t.Errorf("expected %v, got %v", tc.exp, got)
			}
		})
	}
}

func TestCompatibleFlagRegistersLegacyFlags(t *testing.T) {
	cases := []struct {
		args []string
		exp  [3]bool // v0, v1, v2
	}{
		{args: []string{"--v0-compatible"}, exp: [3]bool{true, false, false}},
		{args: []string{"--v1-compatible"}, exp: [3]bool{false, true, false}},
	}

	for _, tc := range cases {
		t.Run(tc.args[0], func(t *testing.T) {
			var got [3]bool
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			addCompatibleFlag(fs, &got[0], &got[1], &got[2])

			if err := fs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			if got != tc.exp {
				t.Errorf("expected %v, got %v", tc.exp, got)
			}
		})
	}
}

func TestCompatibleRegoVersion(t *testing.T) {
	cases := []struct {
		note       string
		v0, v1, v2 bool
		exp        ast.RegoVersion
	}{
		{note: "unset", exp: ast.DefaultRegoVersion},
		{note: "v0", v0: true, exp: ast.RegoV0},
		{note: "v1", v1: true, exp: ast.RegoV1},
		{note: "v2", v2: true, exp: ast.RegoV2},
		{note: "v0 over v1", v0: true, v1: true, exp: ast.RegoV0},
		{note: "v0 over v2", v0: true, v2: true, exp: ast.RegoV0},
		{note: "v2 over v1", v1: true, v2: true, exp: ast.RegoV2},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			if act := compatibleRegoVersion(tc.v0, tc.v1, tc.v2); act != tc.exp {
				t.Errorf("expected %v, got %v", tc.exp, act)
			}
		})
	}
}
