// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tester

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util/test"
)

func TestLoad_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		expErrs []string
	}{
		{
			note: "v0 module", // default rego-version
			module: `package test

p[x] { 
	x = "a" 
}

test_p {
	p["a"]
}`,
		},
		{
			note: "import rego.v1",
			module: `package test
import rego.v1

p contains x if { 
	x := "a" 
}

test_p if {
	"a" in p
}`,
		},
		{
			note: "v1 module", // NOT default rego-version
			module: `package test

p contains x if { 
	x := "a" 
}

test_p if {
	"a" in p
}`,
			expErrs: []string{
				"test.rego:8: rego_parse_error: unexpected identifier token",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.rego": tc.module,
			}

			test.WithTempFS(files, func(root string) {
				modules, store, err := Load([]string{root}, nil)
				if len(tc.expErrs) > 0 {
					if err == nil {
						t.Fatalf("Expected error but got nil")
					}

					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("Expected error to contain:\n\n%q\n\nbut got:\n\n%v", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}

					if modules == nil {
						t.Fatalf("Expected modules to be non-nil")
					}

					if store == nil {
						t.Fatalf("Expected store to be non-nil")
					}
				}
			})
		})
	}
}

// TestRun_DefaultRegoVersion asserts that the internal compiler instantiated by the runner has the correct default rego-version.
func TestRun_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note   string
		module ast.Module
	}{
		{
			note: "no v1 violations",
			module: ast.Module{
				Package: ast.MustParsePackage(`package test`),
				Rules: []*ast.Rule{
					ast.MustParseRule(`p[x] { x = "a" }`),
					ast.MustParseRule(`test_p { p["a"] }`),
				},
			},
		},
		{
			note: "v1 violations",
			module: ast.Module{
				Package: ast.MustParsePackage(`package test`),
				Imports: ast.MustParseImports(`
					import data.foo
					import data.bar as foo
				`),
				Rules: []*ast.Rule{
					ast.MustParseRule(`p[x] { x = "a" }`),
					ast.MustParseRule(`test_p { p["a"] }`),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := context.Background()

			modules := map[string]*ast.Module{
				"test": &tc.module,
			}

			store := inmem.New()
			txn := storage.NewTransactionOrDie(ctx, store)
			defer store.Abort(ctx, txn)

			runner := NewRunner().
				SetStore(store).
				SetModules(modules).
				SetTimeout(10 * time.Second)

			ch, err := runner.RunTests(ctx, txn)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var rs []*Result
			for r := range ch {
				rs = append(rs, r)
			}

			if len(rs) != 1 {
				t.Fatalf("Expected exactly one result but got: %v", rs)
			}

			if rs[0].Fail {
				t.Fatalf("Expected test to pass but it failed")
			}
		})
	}
}
