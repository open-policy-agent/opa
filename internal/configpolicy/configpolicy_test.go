// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package configpolicy

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const evalModule = `package test.eval

processed := input.config

warnings contains "a warning" if input.config.warn == true

errors contains msg if {
	some msg in object.get(input.config, "errs", [])
}
`

func newEvalPolicy() *Policy {
	return New("test/eval.rego", evalModule, "data.test.eval = x")
}

func TestPolicyEval(t *testing.T) {
	p := newEvalPolicy()

	processed, warnings, err := p.Eval(t.Context(), map[string]any{"config": map[string]any{"x": "one"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(processed, map[string]any{"x": "one"}) {
		t.Fatalf("unexpected processed: %v", processed)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestPolicyEvalWarnings(t *testing.T) {
	p := newEvalPolicy()
	_, warnings, err := p.Eval(t.Context(), map[string]any{"config": map[string]any{"warn": true}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(warnings, []string{"a warning"}) {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestPolicyEvalErrorsSortedAndJoined(t *testing.T) {
	p := newEvalPolicy()
	_, _, err := p.Eval(t.Context(), map[string]any{"config": map[string]any{"errs": []any{"zeta", "alpha"}}})
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "alpha; zeta"; got != want {
		t.Fatalf("expected sorted, joined errors %q, got %q", want, got)
	}
}

func TestPolicyEvalMissingProcessed(t *testing.T) {
	p := New("test/noproc.rego", "package test.noproc\n\nwarnings contains \"w\"\n", "data.test.noproc = x")
	if _, _, err := p.Eval(t.Context(), map[string]any{"config": map[string]any{}}); err == nil {
		t.Fatal("expected error when policy produces no processed configuration")
	}
}

func TestPolicyCompileError(t *testing.T) {
	p := New("test/bad.rego", "package test.bad\n\nthis is not valid rego\n", "data.test.bad = x")
	if _, err := p.Compiler(); err == nil {
		t.Fatal("expected compile error")
	}
	// Eval surfaces the same error.
	if _, _, err := p.Eval(t.Context(), map[string]any{"config": map[string]any{}}); err == nil {
		t.Fatal("expected error from Eval on uncompilable policy")
	}
}

func TestPolicyCompilerCompilesOnce(t *testing.T) {
	p := newEvalPolicy()
	c1, err := p.Compiler()
	if err != nil {
		t.Fatal(err)
	}
	c2, err := p.Compiler()
	if err != nil {
		t.Fatal(err)
	}
	if c1 != c2 {
		t.Fatal("expected Compiler to return the same compiled result on repeated calls")
	}
}

const helpersModule = `package test.helpers

import data.opa.config.util

processed := object.union_n(array.concat([input.config], [patch | some patch in _patches]))

_patches contains {"size": 10} if util.absent(["size"])

errors contains "size must be a positive number" if util.not_positive_number(["size"])
`

// The shared helpers are compiled into every policy, so a policy can use them
// without embedding its own copy.
func TestPolicySharedHelpers(t *testing.T) {
	p := New("test/helpers.rego", helpersModule, "data.test.helpers = x")

	processed, _, err := p.Eval(t.Context(), map[string]any{"config": map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(processed, map[string]any{"size": json.Number("10")}) {
		t.Fatalf("expected the helper-guarded default to be injected, got %v", processed)
	}

	_, _, err = p.Eval(t.Context(), map[string]any{"config": map[string]any{"size": -1}})
	if err == nil || err.Error() != "size must be a positive number" {
		t.Fatalf("expected the helper to reject a non-positive value, got %v", err)
	}
}

const intoModule = `package test.into

processed := object.union({"name": "default"}, input.config)
`

type intoConfig struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestEvalConfigInto(t *testing.T) {
	p := New("test/into.rego", intoModule, "data.test.into = x")

	tests := map[string]struct {
		raw  []byte
		want intoConfig
	}{
		"nil injects default":       {raw: nil, want: intoConfig{Name: "default"}},
		"empty injects default":     {raw: []byte(``), want: intoConfig{Name: "default"}},
		"null injects default":      {raw: []byte(`null`), want: intoConfig{Name: "default"}},
		"configured value wins":     {raw: []byte(`{"name":"custom","count":5}`), want: intoConfig{Name: "custom", Count: 5}},
		"default fills absent only": {raw: []byte(`{"count":9}`), want: intoConfig{Name: "default", Count: 9}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var got intoConfig
			if _, err := EvalConfigInto(t.Context(), p, tc.raw, &got); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %+v, got %+v", tc.want, got)
			}
		})
	}
}

func TestEvalConfigIntoTypeMismatch(t *testing.T) {
	// The policy passes the value through; the typed unmarshal into the struct
	// is what rejects a mistyped option.
	p := New("test/into.rego", intoModule, "data.test.into = x")
	var got intoConfig
	if _, err := EvalConfigInto(t.Context(), p, []byte(`{"count":"not-a-number"}`), &got); err == nil {
		t.Fatal("expected error for mistyped option")
	}
}

func TestUnmarshalRawConfig(t *testing.T) {
	empty := map[string]any{}
	tests := map[string]struct {
		raw  []byte
		want any
	}{
		"nil":    {raw: nil, want: empty},
		"empty":  {raw: []byte(``), want: empty},
		"null":   {raw: []byte(`null`), want: empty},
		"object": {raw: []byte(`{"a":true}`), want: map[string]any{"a": true}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := unmarshalRawConfig(tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, got)
			}
		})
	}

	if _, err := unmarshalRawConfig([]byte(`{invalid`)); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestStringSet(t *testing.T) {
	tests := map[string]struct {
		in   any
		want []string
	}{
		"nil":         {in: nil, want: nil},
		"empty slice": {in: []any{}, want: nil},
		"strings":     {in: []any{"a", "b"}, want: []string{"a", "b"}},
		"mixed":       {in: []any{"a", 1, "b"}, want: []string{"a", "b"}},
		"no strings":  {in: []any{1, 2}, want: nil},
		"not a slice": {in: "x", want: nil},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := StringSet(tc.in); !slices.Equal(got, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, got)
			}
		})
	}
}

func TestPolicyCheck(t *testing.T) {
	modules := map[string]string{
		"check.rego": `package test.check

import data.test.limits

warnings contains "big" if input.config.size > limits.warn

errors contains "too big" if input.config.size > limits.max
`,
		"limits.rego": `package test.limits

warn := 10

max := 100
`,
	}
	p := NewWithModules("test/check", modules, "data.test.check = x")

	tests := map[string]struct {
		size     int
		warnings []string
		err      string
	}{
		"nothing reported": {size: 1},
		"warning":          {size: 50, warnings: []string{"big"}},
		"error":            {size: 500, err: "too big"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			warnings, err := p.Check(t.Context(), map[string]any{"config": map[string]any{"size": tc.size}})
			if tc.err != "" {
				if err == nil || err.Error() != tc.err {
					t.Fatalf("expected error %q, got %v", tc.err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(warnings, tc.warnings) {
				t.Fatalf("want warnings %v, got %v", tc.warnings, warnings)
			}
		})
	}
}

func TestPolicyCheckUndefined(t *testing.T) {
	p := New("test/other.rego", "package test.other\n\nx := 1\n", "data.test.undefined = x")
	warnings, err := p.Check(t.Context(), map[string]any{"config": map[string]any{}})
	if err != nil || warnings != nil {
		t.Fatalf("expected nothing reported for an undefined document, got %v, %v", warnings, err)
	}
}

func TestPolicyCheckRejectsMalformedResults(t *testing.T) {
	tests := map[string]struct {
		rule string
		err  string
	}{
		"object errors":    {rule: `errors contains {"msg": "denied"}`, err: "errors must be a set of strings"},
		"string errors":    {rule: `errors := "denied"`, err: "errors must be a set of strings"},
		"mixed errors":     {rule: "errors contains \"denied\"\n\nerrors contains 1", err: "errors must be a set of strings"},
		"object warnings":  {rule: `warnings contains {"msg": "careful"}`, err: "warnings must be a set of strings"},
		"boolean warnings": {rule: `warnings := true`, err: "warnings must be a set of strings"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			p := New("test/malformed.rego", "package test.malformed\n\n"+tc.rule+"\n", "data.test.malformed = x")
			_, err := p.Check(t.Context(), map[string]any{"config": map[string]any{}})
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("expected error containing %q, got %v", tc.err, err)
			}
		})
	}
}
