// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package config

import (
	"slices"
	"testing"
)

const pluginPolicy = `package my.plugin.config

import data.opa.config.util

processed := object.union_n(array.concat([input.config], [patch | some patch in _patches]))

_patches contains {"interval": 10} if util.absent(["interval"])

errors contains "interval must be a positive number" if util.not_positive_number(["interval"])

warnings contains $"unknown option {key}" if {
	some key, _ in input.config
	not key in {"interval", "name"}
}
`

type pluginConfig struct {
	Interval int    `json:"interval"`
	Name     string `json:"name"`
}

func TestEvalPolicyInto(t *testing.T) {
	p, err := NewPolicy("my/plugin/validate.rego", pluginPolicy)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		raw      string
		want     pluginConfig
		warnings []string
		err      string
	}{
		"absent injects default":  {raw: ``, want: pluginConfig{Interval: 10}},
		"configured value wins":   {raw: `{"interval": 3, "name": "x"}`, want: pluginConfig{Interval: 3, Name: "x"}},
		"warnings are returned":   {raw: `{"intervall": 3}`, want: pluginConfig{Interval: 10}, warnings: []string{"unknown option intervall"}},
		"policy errors are fatal": {raw: `{"interval": -1}`, err: "interval must be a positive number"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var got pluginConfig
			warnings, err := EvalPolicyInto(t.Context(), p, []byte(tc.raw), &got)
			if tc.err != "" {
				if err == nil || err.Error() != tc.err {
					t.Fatalf("expected error %q, got %v", tc.err, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("want %+v, got %+v", tc.want, got)
			}
			if !slices.Equal(warnings, tc.warnings) {
				t.Errorf("want warnings %v, got %v", tc.warnings, warnings)
			}
		})
	}
}

func TestNewPolicyParseError(t *testing.T) {
	if _, err := NewPolicy("bad.rego", "this is not rego"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestEvalPolicyIntoCompileError(t *testing.T) {
	p, err := NewPolicy("bad.rego", "package bad\n\nprocessed := undefined_ref\n")
	if err != nil {
		t.Fatal(err)
	}
	var got pluginConfig
	if _, err := EvalPolicyInto(t.Context(), p, nil, &got); err == nil {
		t.Fatal("expected compile error")
	}
}
