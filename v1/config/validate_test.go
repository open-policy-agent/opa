// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package config

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/topdown"
)

func TestParseConfigWarnsOnUnknownOption(t *testing.T) {
	// The motivating example from issue #2745.
	conf, err := ParseConfig([]byte(`{"decision_log": {"console": true}}`), "id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `unknown configuration option "decision_log" encountered`
	if !slices.Contains(conf.Warnings, want) {
		t.Fatalf("expected warning %q, got %v", want, conf.Warnings)
	}
}

func TestParseConfigNoWarningsForValidConfig(t *testing.T) {
	conf, err := ParseConfig([]byte(`{"decision_logs": {"console": true}}`), "id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conf.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", conf.Warnings)
	}
}

func TestParseConfigEmptyInjectsDefaults(t *testing.T) {
	// The SDK and other callers parse an absent configuration (nil/empty bytes);
	// defaults must still be injected and no error returned.
	for name, raw := range map[string][]byte{
		"nil":        nil,
		"empty":      []byte(``),
		"empty-obj":  []byte(`{}`),
		"null":       []byte(`null`),
		"whitespace": []byte(`  `),
	} {
		t.Run(name, func(t *testing.T) {
			conf, err := ParseConfig(raw, "id")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if conf.DefaultDecision == nil || *conf.DefaultDecision != "/system/main" {
				t.Fatalf("expected default decision to be injected, got %v", conf.DefaultDecision)
			}
			if conf.Labels["id"] != "id" {
				t.Fatalf("expected id label to be injected, got %v", conf.Labels)
			}
		})
	}
}

func TestParseConfigNullDecisionDefaults(t *testing.T) {
	// A field explicitly set to null must fall back to the default (not error),
	// matching the pre-Rego behavior where a nil pointer was treated as unset.
	conf, err := ParseConfig([]byte(`{"default_decision": null}`), "id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conf.DefaultDecision == nil || *conf.DefaultDecision != "/system/main" {
		t.Fatalf("expected default decision to be injected, got %v", conf.DefaultDecision)
	}
}

func TestParseConfigNonStringDecisionErrors(t *testing.T) {
	// A present, non-null, non-string value is still a fatal error.
	if _, err := ParseConfig([]byte(`{"default_decision": 42}`), "id"); err == nil {
		t.Fatal("expected error for non-string default_decision, got nil")
	}
}

// TestCoreValidationRootSpecMatchesConfigStruct is a drift guard: the set of
// top-level keys known to the core validation policy must exactly match the
// JSON-tagged fields of the Config struct. If a field is added to Config without
// updating validate.rego (or vice versa), this test fails.
func TestCoreValidationRootSpecMatchesConfigStruct(t *testing.T) {
	structKeys := map[string]struct{}{}
	objType := reflect.TypeFor[Config]()
	for field := range objType.Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		structKeys[name] = struct{}{}
	}

	policyKeys := rootSpecKeys(t)

	for k := range structKeys {
		if _, ok := policyKeys[k]; !ok {
			t.Errorf("config key %q is present in Config struct but missing from validate.rego root spec", k)
		}
	}
	for k := range policyKeys {
		if _, ok := structKeys[k]; !ok {
			t.Errorf("config key %q is present in validate.rego root spec but not in Config struct", k)
		}
	}
}

// rootSpecKeys evaluates the core policy and returns the key set of the spec
// whose pattern is empty (i.e. the top-level configuration object).
func rootSpecKeys(t *testing.T) map[string]struct{} {
	t.Helper()

	ctx := context.Background()
	compiler, err := compileValidationPolicy()
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	store := inmem.New()
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		t.Fatalf("txn: %v", err)
	}
	defer store.Abort(ctx, txn)

	qrs, err := topdown.NewQuery(ast.MustParseBody("data.opa.config._specs = x")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		Run(ctx)
	if err != nil {
		t.Fatalf("eval _specs: %v", err)
	}
	if len(qrs) != 1 {
		t.Fatalf("unexpected result set: %v", qrs)
	}

	value, err := ast.JSON(qrs[0][ast.Var("x")].Value)
	if err != nil {
		t.Fatalf("convert _specs: %v", err)
	}
	specs, ok := value.([]any)
	if !ok {
		t.Fatalf("unexpected _specs type %T", value)
	}

	for _, s := range specs {
		spec := s.(map[string]any)
		pattern := spec["pattern"].([]any)
		if len(pattern) != 0 {
			continue
		}
		keys := map[string]struct{}{}
		for _, k := range spec["keys"].([]any) {
			keys[k.(string)] = struct{}{}
		}
		return keys
	}

	t.Fatal("no root spec (empty pattern) found in validate.rego")
	return nil
}
