package compile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/format"
	"github.com/open-policy-agent/opa/internal/ref"
	"github.com/open-policy-agent/opa/ir"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

func TestOrderedStringSet(t *testing.T) {
	var ss orderedStringSet
	result := ss.Append("a", "b", "b", "a", "e", "c", "e")
	if !reflect.DeepEqual(result, orderedStringSet{"a", "b", "e", "c"}) {
		t.Fatal(result)
	}
}

func TestCompilerInitErrors(t *testing.T) {

	ctx := context.Background()

	tests := []struct {
		note string
		c    *Compiler
		want error
	}{
		{
			note: "bad target",
			c:    New().WithTarget("deadbeef"),
			want: fmt.Errorf("invalid target \"deadbeef\""),
		},
		{
			note: "optimizations require entrypoint",
			c:    New().WithOptimizationLevel(1),
			want: errors.New("bundle optimizations require at least one entrypoint"),
		},
		{
			note: "wasm compilation requires at least one entrypoint",
			c:    New().WithTarget("wasm"),
			want: errors.New("wasm compilation requires at least one entrypoint"),
		},
		{
			note: "plan compilation requires at least one entrypoint",
			c:    New().WithTarget("plan"),
			want: errors.New("plan compilation requires at least one entrypoint"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			err := tc.c.Build(ctx)
			if err == nil {
				t.Fatal("expected error")
			} else if err.Error() != tc.want.Error() {
				t.Fatalf("expected %v but got %v", tc.want, err)
			}
		})
	}

}

func TestCompilerLoadError(t *testing.T) {

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(nil, useMemoryFS, func(root string, fsys fs.FS) {

			err := New().
				WithFS(fsys).
				WithPaths(path.Join(root, "does-not-exist")).
				Build(context.Background())
			if err == nil {
				t.Fatal("expected failure")
			}
		})
	}
}

func TestCompilerLoadAsBundleSuccess(t *testing.T) {

	ctx := context.Background()

	files := map[string]string{
		"b1/.manifest": `{"roots": ["b1"]}`,
		"b1/test.rego": `
			package b1.test

			p = 1`,
		"b1/data.json": `
			{"b1": {"k": "v"}}`,
		"b2/.manifest": `{"roots": ["b2"]}`,
		"b2/data.json": `
			{"b2": {"k2": "v2"}}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			root1 := path.Join(root, "b1")
			root2 := path.Join(root, "b2")

			compiler := New().
				WithFS(fsys).
				WithPaths(root1, root2).
				WithAsBundle(true)

			err := compiler.Build(ctx)
			if err != nil {
				t.Fatal(err)
			}

			// Verify result is just merger of two bundles.
			a, err := loader.NewFileLoader().WithFS(fsys).AsBundle(root1)
			if err != nil {
				panic(err)
			}

			b, err := loader.NewFileLoader().WithFS(fsys).AsBundle(root2)
			if err != nil {
				panic(err)
			}

			exp, err := bundle.Merge([]*bundle.Bundle{a, b})
			if err != nil {
				panic(err)
			}

			err = exp.FormatModules(false)
			if err != nil {
				t.Fatal(err)
			}

			if !compiler.bundle.Equal(*exp) {
				t.Fatalf("expected %v but got %v", exp, compiler.bundle)
			}

			expRoots := []string{"b1", "b2"}
			expManifest := bundle.Manifest{
				Roots: &expRoots,
			}

			if !compiler.bundle.Manifest.Equal(expManifest) {
				t.Fatalf("expected %v but got %v", compiler.bundle.Manifest, expManifest)
			}
		})
	}
}

func TestCompilerLoadAsBundleMergeError(t *testing.T) {

	ctx := context.Background()

	// Omit manifests (defaulting to '') to trigger a merge error
	files := map[string]string{
		"b1/test.rego": `
			package b1.test

			p = 1`,
		"b1/data.json": `
			{"b1": {"k": "v"}}`,
		"b2/data.json": `
			{"b2": {"k2": "v2"}}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			root1 := path.Join(root, "b1")
			root2 := path.Join(root, "b2")

			compiler := New().
				WithFS(fsys).
				WithPaths(root1, root2).
				WithAsBundle(true)

			err := compiler.Build(ctx)
			if err == nil || err.Error() != "bundle merge failed: manifest has overlapped roots: '' and ''" {
				t.Fatal(err)
			}
		})
	}
}

func TestCompilerLoadFilesystem(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package b1.test

			p = 1`,
		"data.json": `
			{"b1": {"k": "v"}}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root)

			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			// Verify result is just bundle load.
			exp, err := loader.NewFileLoader().WithFS(fsys).AsBundle(root)
			if err != nil {
				panic(err)
			}

			err = exp.FormatModules(false)
			if err != nil {
				t.Fatal(err)
			}

			if !compiler.bundle.Equal(*exp) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", compiler.bundle, exp)
			}
		})
	}
}

func TestCompilerLoadFilesystemWithEnablePrintStatementsFalse(t *testing.T) {
	files := map[string]string{
		"test.rego": `
			package test

                        allow { print(1) }
		`,
		"data.json": `
			{"b1": {"k": "v"}}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("plan").WithEntrypoints("test/allow").
				WithEnablePrintStatements(false)

			if err := compiler.Build(context.Background()); err != nil {
				t.Fatal(err)
			}

			bundle := compiler.Bundle()

			if strings.Contains(string(bundle.PlanModules[0].Raw), "internal.print") {
				t.Fatalf("output different than expected:\n\ngot: %v\n\nfound: internal.print", string(bundle.PlanModules[0].Raw))
			}
		})
	}
}

func TestCompilerLoadFilesystemWithEnablePrintStatementsTrue(t *testing.T) {
	files := map[string]string{
		"test.rego": `
			package test

                        allow { print(1) }
		`,
		"data.json": `
			{"b1": {"k": "v"}}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("plan").
				WithEntrypoints("test/allow").
				WithEnablePrintStatements(true)

			if err := compiler.Build(context.Background()); err != nil {
				t.Fatal(err)
			}

			bundle := compiler.Bundle()

			if !strings.Contains(string(bundle.PlanModules[0].Raw), "internal.print") {
				t.Fatalf("output different than expected:\n\ngot: %v\n\nmissing: internal.print", string(bundle.PlanModules[0].Raw))
			}
		})
	}
}

func TestCompilerLoadHonorsFilter(t *testing.T) {
	files := map[string]string{
		"test.rego": `
			package b1.test

			p = 1`,
		"data.json": `
			{"b1": {"k": "v"}}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithFilter(func(abspath string, _ os.FileInfo, _ int) bool {
					return strings.HasSuffix(abspath, ".json")
				})

			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.Data) > 0 {
				t.Fatal("expected no data to be loaded")
			}
		})
	}
}

func TestCompilerInputBundle(t *testing.T) {

	b := &bundle.Bundle{
		Modules: []bundle.ModuleFile{
			{
				URL:    "/foo.rego",
				Path:   "/foo.rego",
				Raw:    []byte("package test\np = 7"),
				Parsed: ast.MustParseModule("package test\np = 7"),
			},
		},
	}

	compiler := New().WithBundle(b)

	if err := compiler.Build(context.Background()); err != nil {
		t.Fatal(err)
	}

	exp := "package test\n\np = 7\n"

	if exp != string(compiler.Bundle().Modules[0].Raw) {
		t.Fatalf("expected module to have been formatted (output different than expected):\n\ngot: %v\n\nwant: %v", string(compiler.Bundle().Modules[0].Raw), exp)
	}
}

func TestCompilerInputInvalidBundle(t *testing.T) {

	b := &bundle.Bundle{
		Modules: []bundle.ModuleFile{
			{
				URL:    "/url",
				Path:   "/foo.rego",
				Raw:    []byte("package test\np = 0"),
				Parsed: ast.MustParseModule("package test\np = 0"),
			},
			{
				URL:    "/url",
				Path:   "/bar.rego",
				Raw:    []byte("package test\nq = 1"),
				Parsed: ast.MustParseModule("package test\nq = 1"),
			},
		},
	}

	compiler := New().WithBundle(b)

	if err := compiler.Build(context.Background()); err == nil {
		t.Fatal("duplicate module URL not detected")
	} else if err.Error() != "duplicate module URL: /url" {
		t.Fatal(err)
	}
}

func TestCompilerError(t *testing.T) {
	files := map[string]string{
		"test.rego": `
			package test
			default p = false
			p { p }`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root)

			err := compiler.Build(context.Background())
			if err == nil {
				t.Fatal("expected error")
			}

			astErr, ok := err.(ast.Errors)
			if !ok || len(astErr) != 1 || astErr[0].Code != ast.RecursionErr {
				t.Fatal("unexpected error:", err)
			}
		})
	}
}

func TestCompilerOptimizationL1(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test
			default p = false
			p { q }
			q { input.x = data.foo }`,
		"data.json": `
			{"foo": 1}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithOptimizationLevel(1).
				WithEntrypoints("test/p")

			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			optimizedExp := ast.MustParseModule(`
			package test

			default p = false
			p { data.test.q = X; X }
			q { input.x = 1 }
		`)

			// NOTE(tsandall): PE generates vars with wildcard prefix. Instead of
			// constructing the AST manually, just rewrite to the expected value
			// here. If this becomes a common pattern, we could refactor (e.g.,
			// allow caller to control var prefix, split into a reusable function,
			// etc.)
			_, err = ast.TransformVars(optimizedExp, func(x ast.Var) (ast.Value, error) {
				if x == ast.Var("X") {
					return ast.Var("$_term_1_01"), nil
				}
				return x, nil
			})
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.Modules) != 1 {
				t.Fatalf("expected 1 module but got: %v", compiler.bundle.Modules)
			}

			if !compiler.bundle.Modules[0].Parsed.Equal(optimizedExp) {
				t.Fatalf("expected optimized module to be:\n\n%v\n\ngot:\n\n%v", optimizedExp, compiler.bundle.Modules[0])
			}
		})
	}
}

func TestCompilerOptimizationL2(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test
			default p = false
			p { q }
			q { input.x = data.foo }`,
		"data.json": `
			{"foo": 1}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithOptimizationLevel(2).
				WithEntrypoints("test/p")

			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			prunedExp := ast.MustParseModule(`
			package test

			q { input.x = data.foo }`)

			optimizedExp := ast.MustParseModule(`
			package test

			default p = false
			p { input.x = 1 }
      `)

			if len(compiler.bundle.Modules) != 2 {
				t.Fatalf("expected two modules but got: %v", compiler.bundle.Modules)
			}

			// Note: L2 optimized ModuleFile ordering in a bundle is non-deterministic...
			if !compiler.bundle.Modules[0].Parsed.Equal(prunedExp) {
				if !compiler.bundle.Modules[0].Parsed.Equal(optimizedExp) {
					t.Fatalf("expected optimized module to be:\n\n%v\n\ngot:\n\n%v", optimizedExp, compiler.bundle.Modules[0])
				}
				if !compiler.bundle.Modules[1].Parsed.Equal(prunedExp) {
					t.Fatalf("expected pruned module to be:\n\n%v\n\ngot:\n\n%v", prunedExp, compiler.bundle.Modules[1])
				}
			} else {
				if !compiler.bundle.Modules[1].Parsed.Equal(optimizedExp) {
					t.Fatalf("expected optimized module to be:\n\n%v\n\ngot:\n\n%v", optimizedExp, compiler.bundle.Modules[1])
				}
			}
		})
	}
}

func TestCompilerOptimizationWithConfiguredNamespace(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test

			p { not q }
			q { k[input.a]; k[input.b] }  # generate a product that is not inlined
			k = {1,2,3}
		`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithOptimizationLevel(1).
				WithEntrypoints("test/p").
				WithPartialNamespace("custom")

			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.Modules) != 2 {
				t.Fatalf("expected two modules but got: %v", len(compiler.bundle.Modules))
			}

			optimizedExp := ast.MustParseModule(`package custom

			__not1_0_2__ = true { data.test.q = _; _ }`)

			if optimizedExp.String() != compiler.bundle.Modules[0].Parsed.String() {
				t.Fatalf("expected optimized module to be:\n\n%v\n\ngot:\n\n%v", optimizedExp, compiler.bundle.Modules[0])
			}

			expected := ast.MustParseModule(`package test
k = {1, 2, 3} { true }
p = true { not data.custom.__not1_0_2__ }
q = true { __local0__3 = input.a; data.test.k[__local0__3] = _; _; __local1__3 = input.b; data.test.k[__local1__3] = _; _ }`)

			if expected.String() != compiler.bundle.Modules[1].Parsed.String() {
				t.Fatalf("expected module to be:\n\n%v\n\ngot:\n\n%v", expected, compiler.bundle.Modules[1])
			}
		})
	}
}

func TestCompilerOptimizationWithGeneralRefs(t *testing.T) {
	tests := []struct {
		note       string
		entrypoint string
		files      map[string]string
		expected   []string
	}{
		{
			note: "special characters in ref term",
			files: map[string]string{
				"base.rego": `package base
allow["entity/slash"].action {
	action := "action"
	input.principal == input.entity
}`,
				"query.rego": `package query
main {
	data.base.allow[input.entity.type][input.action]
}`,
			},
			entrypoint: "query/main",
			expected: []string{
				`package base

allow["entity/slash"].action {
	input.principal = input.entity
}
`,
				`package query

main {
	__local1__1 = input.entity.type
	__local2__1 = input.action
	"entity/slash" = __local1__1
	"action" = __local2__1
	data.base.allow[__local1__1][__local2__1] = _term_1_21
	_term_1_21
}
`,
			},
		},
		{
			note:       "single rule (one key), no ref, no unknowns",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test
p[r] {
	r := ["do", "re"][_]
}`,
			},
			expected: []string{
				`package test

p = __result__ {
	__result__ = {"do", "re"}
}
`,
			},
		},
		{
			note:       "single rule (one key), no ref, unknown in body",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test
p[r] {
	r := ["do", "re"][_]
	input.x
}`,
			},
			expected: []string{
				`package test

p["do"] {
	input.x = _term_1_21
	_term_1_21
}

p["re"] {
	input.x = _term_1_21
	_term_1_21
}
`,
			},
		},
		{
			note:       "single rule (one key), no unknowns",
			entrypoint: "test/p/q",
			files: map[string]string{
				"test.rego": `package test
p.q[r] {
	r := ["do", "re"][_]
	input.x
}`,
			},
			expected: []string{
				`package test.p.q

do {
	input.x = _term_1_21
	_term_1_21
}

re {
	input.x = _term_1_21
	_term_1_21
}
`,
			},
		},
		{
			note:       "single rule, no unknowns",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test
p[q][r] {
	q := ["foo", "bar"][_]
	r := ["do", "re"][_]
}`,
			},
			expected: []string{
				`package test

p = __result__ {
	__result__ = {"foo": {"do": true, "re": true}, "bar": {"do": true, "re": true}}
}
`,
			},
		},
		{
			note:       "single rule, unknown value",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test
p[q][r] := s {
	q := ["foo", "bar"][_]
	r := ["do", "re"][_]
	s := input.x
}`,
			},
			expected: []string{
				`package test.p.foo

do = __local2__1 {
	__local2__1 = input.x
}

re = __local2__1 {
	__local2__1 = input.x
}
`,
				`package test.p.bar

do = __local2__1 {
	__local2__1 = input.x
}

re = __local2__1 {
	__local2__1 = input.x
}
`,
			},
		},
		{
			note:       "single rule, unknown key (first)",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test
p[q][r] {
	q := input.x[_]
	r := ["do", "re"][_]
}`,
			},
			expected: []string{
				`package test

p[__local0__1].do {
	__local0__1 = input.x[_01]
}

p[__local0__1].re {
	__local0__1 = input.x[_01]
}
`,
			},
		},
		{
			note:       "single rule, unknown key (second)",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test
p[q][r] {
	q := ["foo", "bar"][_]
	r := input.x[_]
}`,
			},
			expected: []string{
				`package test.p

bar[__local1__1] = true {
	__local1__1 = input.x[_11]
}

foo[__local1__1] = true {
	__local1__1 = input.x[_11]
}
`,
			},
		},
		{
			note:       "regression test for #6338",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test

p { 
	q[input.x][input.y] 
}

q["foo/bar"][x] {
	x := "baz"
	input.x == 1
}`,
			},
			expected: []string{
				`package test

p {
	__local1__1 = input.x
	__local2__1 = input.y
	"foo/bar" = __local1__1
	data.test.q[__local1__1][__local2__1] = _term_1_21
	_term_1_21
}

q["foo/bar"].baz {
	input.x = 1
}
`,
			},
		},
		{
			note:       "regression test for #6339",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test

import future.keywords.in

p {
	q[input.x][input.y]
}

q[entity][action] {
	some action in ["show", "update"]
	some entity in ["pay", "roll"]
	input.z == 1
}`,
			},
			expected: []string{
				`package test

p {
	__local8__1 = input.x
	__local9__1 = input.y
	data.test.q[__local8__1][__local9__1] = _term_1_21
	_term_1_21
}
`,
				`package test.q.pay

show {
	input.z = 1
}

update {
	input.z = 1
}
`,
				`package test.q.roll

show {
	input.z = 1
}

update {
	input.z = 1
}
`,
			},
		},
		{
			note:       "not",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test

import future.keywords.in

p {
	not q[input.x][input.y]
}

q[entity][action] {
	some action in ["show", "update"]
	some entity in ["pay", "roll"]
	input.z == 1
}`,
			},
			expected: []string{
				`package partial

__not1_2_2__(__local8__1, __local9__1) {
	data.test.q[__local8__1][__local9__1] = _term_2_01
	_term_2_01
}
`,
				`package test

p {
	__local8__1 = input.x
	__local9__1 = input.y
	not data.partial.__not1_2_2__(__local8__1, __local9__1)
}
`,
				`package test.q.pay

show {
	input.z = 1
}

update {
	input.z = 1
}
`,
				`package test.q.roll

show {
	input.z = 1
}

update {
	input.z = 1
}
`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			for _, useMemoryFS := range []bool{false, true} {
				test.WithTestFS(tc.files, useMemoryFS, func(root string, fsys fs.FS) {

					compiler := New().
						WithFS(fsys).
						WithPaths(root).
						WithOptimizationLevel(1).
						WithEntrypoints(tc.entrypoint)

					err := compiler.Build(context.Background())
					if err != nil {
						t.Fatal(err)
					}

					if len(compiler.bundle.Modules) != len(tc.expected) {
						t.Fatalf("expected %v modules but got: %v:\n\n%v",
							len(tc.expected), len(compiler.bundle.Modules), modulesToString(compiler.bundle.Modules))
					}

					actual := make(map[string]struct{})
					for _, m := range compiler.bundle.Modules {
						actual[string(m.Raw)] = struct{}{}
					}

					for _, e := range tc.expected {
						if _, ok := actual[e]; !ok {
							t.Fatalf("expected to find module:\n\n%v\n\nin bundle but got:\n\n%v",
								e, modulesToString(compiler.bundle.Modules))
						}
					}
				})
			}
		})
	}
}

func modulesToString(modules []bundle.ModuleFile) string {
	var buf bytes.Buffer
	//result := make([]string, len(modules))
	for i, m := range modules {
		//result[i] = m.Parsed.String()
		buf.WriteString(strconv.Itoa(i))
		buf.WriteString(":\n")
		buf.WriteString(string(m.Raw))
		buf.WriteString("\n\n")
	}
	return buf.String()
}

// NOTE(sr): we override this to not depend on build tags in tests
func wasmABIVersions(vs ...ast.WasmABIVersion) *ast.Capabilities {
	caps := ast.CapabilitiesForThisVersion()
	caps.WasmABIVersions = vs
	return caps
}

func TestCompilerWasmTarget(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = 7
		q = p+1`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("wasm").
				WithEntrypoints("test/p", "test/q").
				WithCapabilities(wasmABIVersions(ast.WasmABIVersion{Version: 1}))
			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.WasmModules) == 0 {
				t.Fatal("expected to find compiled wasm module")
			}

			if len(compiler.bundle.Wasm) != 0 {
				t.Error("expected NOT to find deprecated bundle `Wasm` value")
			}

			ensureEntrypointRemoved(t, compiler.bundle, "test/p")
		})
	}
}

// If we're building a wasm bundle, and the `opa` binary we use to do that
// does not support wasm _itself_, then it shouldn't bother.
func TestCompilerWasmTargetWithCapabilitiesUnset(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = 7
		q = p+1`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("wasm").
				WithEntrypoints("test/p", "test/q")
			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestCompilerWasmTargetWithCapabilitiesMismatch(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = 7
		q = p+1`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			for note, wabis := range map[string][]ast.WasmABIVersion{
				"none":     {},
				"mismatch": {{Version: 0}, {Version: 1, Minor: 2000}},
			} {
				t.Run(note, func(t *testing.T) {
					caps := ast.CapabilitiesForThisVersion()
					caps.WasmABIVersions = wabis
					compiler := New().
						WithFS(fsys).
						WithPaths(root).
						WithTarget("wasm").
						WithEntrypoints("test/p", "test/q").
						WithCapabilities(caps)
					err := compiler.Build(context.Background())
					if err == nil {
						t.Fatal("expected err, got nil")
					}
				})
			}
		})
	}
}

func TestCompilerWasmTargetMultipleEntrypoints(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = true`,
		"policy.rego": `package policy

		authz = true`,
		"mask.rego": `package system.log

		mask["/input/password"]`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("wasm").
				WithEntrypoints("test/p", "policy/authz").
				WithCapabilities(wasmABIVersions(ast.WasmABIVersion{Version: 1}))
			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.WasmModules) != 1 {
				t.Fatalf("expected 1 Wasm modules, got: %d", len(compiler.bundle.WasmModules))
			}

			expManifest := bundle.Manifest{}
			expManifest.Init()
			expManifest.WasmResolvers = []bundle.WasmResolver{
				{
					Entrypoint: "test/p",
					Module:     "/policy.wasm",
				},
				{
					Entrypoint: "policy/authz",
					Module:     "/policy.wasm",
				},
			}

			if !compiler.bundle.Manifest.Equal(expManifest) {
				t.Fatalf("\nExpected manifest: %+v\nGot: %+v\n", expManifest, compiler.bundle.Manifest)
			}

			ensureEntrypointRemoved(t, compiler.bundle, "test/p")
			ensureEntrypointRemoved(t, compiler.bundle, "policy/authz")
		})
	}
}

func TestCompilerWasmTargetAnnotations(t *testing.T) {
	files := map[string]string{
		"test.rego": `
# METADATA
# title: My test package
package test

# METADATA
# title: My P rule
# entrypoint: true
p = true`,
		"policy.rego": `
package policy

# METADATA
# title: All my Q rules
# scope: document

# METADATA
# title: My Q rule
q = true`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("wasm").
				WithEntrypoints("test", "policy/q").
				WithRegoAnnotationEntrypoints(true)

			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.WasmModules) != 1 {
				t.Fatalf("expected 1 Wasm modules, got: %d", len(compiler.bundle.WasmModules))
			}

			expWasmResolvers := []bundle.WasmResolver{
				{
					Entrypoint: "test",
					Module:     "/policy.wasm",
				},
				{
					Entrypoint: "policy/q",
					Module:     "/policy.wasm",
					Annotations: []*ast.Annotations{
						{
							Title: "All my Q rules",
							Scope: "document",
						},
						{
							Title: "My Q rule",
							Scope: "rule",
						},
					},
				},
				{
					Entrypoint: "test/p",
					Module:     "/policy.wasm",
					Annotations: []*ast.Annotations{
						{
							Title:      "My P rule",
							Scope:      "rule",
							Entrypoint: true,
						},
					},
				},
			}

			if len(expWasmResolvers) != len(compiler.bundle.Manifest.WasmResolvers) {
				t.Fatalf("\nExpected WasmResolvers:\n  %+v\nGot:\n  %+v\n", expWasmResolvers, compiler.bundle.Manifest.WasmResolvers)
			}

			for i, expWasmResolver := range expWasmResolvers {
				if !expWasmResolver.Equal(&compiler.bundle.Manifest.WasmResolvers[i]) {
					t.Fatalf("WasmResolver at index %v mismatch\n\nExpected WasmResolvers:\n  %+v\nGot:\n  %+v\n",
						i, expWasmResolvers, compiler.bundle.Manifest.WasmResolvers)
				}
			}
		})
	}
}

func TestCompilerWasmTargetEntrypointDependents(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p { q }
		q { r }
		r = 1
		s = 2
		z { r }`}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("wasm").
				WithEntrypoints("test/r", "test/z").
				WithCapabilities(wasmABIVersions(ast.WasmABIVersion{Version: 1}))
			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.WasmModules) != 1 {
				t.Fatalf("expected 1 Wasm modules, got: %d", len(compiler.bundle.WasmModules))
			}

			expManifest := bundle.Manifest{}
			expManifest.Init()
			expManifest.WasmResolvers = []bundle.WasmResolver{
				{
					Entrypoint: "test/r",
					Module:     "/policy.wasm",
				},
				{
					Entrypoint: "test/z",
					Module:     "/policy.wasm",
				},
				{
					Entrypoint: "test/p",
					Module:     "/policy.wasm",
				},
				{
					Entrypoint: "test/q",
					Module:     "/policy.wasm",
				},
			}

			if !compiler.bundle.Manifest.Equal(expManifest) {
				t.Fatalf("\nExpected manifest: %+v\nGot: %+v\n", expManifest, compiler.bundle.Manifest)
			}

			ensureEntrypointRemoved(t, compiler.bundle, "test/p")
			ensureEntrypointRemoved(t, compiler.bundle, "test/q")
			ensureEntrypointRemoved(t, compiler.bundle, "test/r")
		})
	}
}

func TestCompilerWasmTargetLazyCompile(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p { input.x = q }
		q = "foo"`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("wasm").
				WithEntrypoints("test/p").
				WithOptimizationLevel(1).
				WithCapabilities(wasmABIVersions(ast.WasmABIVersion{Version: 1}))
			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.WasmModules) == 0 {
				t.Fatal("expected to find compiled wasm module")
			}

			if _, exists := compiler.compiler.Modules["optimized/test.rego"]; !exists {
				t.Fatal("expected to find optimized module on compiler")
			}

			ensureEntrypointRemoved(t, compiler.bundle, "test/p")
		})
	}
}

func ensureEntrypointRemoved(t *testing.T, b *bundle.Bundle, e string) {
	t.Helper()
	r, err := ref.ParseDataPath(e)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	for _, mf := range b.Modules {
		for _, rule := range mf.Parsed.Rules {
			if rule.Path().Equal(r) {
				t.Errorf("expected entrypoint to be removed from rego all modules in bundle, found rule: %s in %s", rule.Path(), mf.Path)
			}
		}
	}
}

func TestCompilerPlanTarget(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = 7
		q = p+1`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("plan").
				WithEntrypoints("test/p", "test/q")
			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.PlanModules) == 0 {
				t.Fatal("expected to find compiled plan module")
			}
		})
	}
}

func TestCompilerPlanTargetPruneUnused(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test
		p[1]
		f(x) { p[x] }`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("plan").
				WithEntrypoints("test").
				WithPruneUnused(true)
			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.PlanModules) == 0 {
				t.Fatal("expected to find compiled plan module")
			}

			plan := compiler.bundle.PlanModules[0].Raw
			var policy ir.Policy

			if err := json.Unmarshal(plan, &policy); err != nil {
				t.Fatal(err)
			}
			if exp, act := 1, len(policy.Funcs.Funcs); act != exp {
				t.Fatalf("expected %d funcs, got %d", exp, act)
			}
			f := policy.Funcs.Funcs[0]
			if exp, act := "g0.data.test.p", f.Name; act != exp {
				t.Fatalf("expected func named %v, got %v", exp, act)
			}
		})
	}
}

func TestCompilerPlanTargetUnmatchedEntrypoints(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p := 7
		q := p + 1`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("plan").
				WithEntrypoints("test/p", "test/q", "test/no")
			err := compiler.Build(context.Background())
			if err == nil {
				t.Error("expected error from unmatched entrypoint")
			}
			expectError := "entrypoint \"test/no\" does not refer to a rule or policy decision"
			if err.Error() != expectError {
				t.Errorf("expected error %s, got: %s", expectError, err.Error())
			}
		})
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("plan").
				WithEntrypoints("foo", "foo.bar", "test/no")
			err := compiler.Build(context.Background())
			if err == nil {
				t.Error("expected error from unmatched entrypoints")
			}
			expectError := "entrypoint \"foo\" does not refer to a rule or policy decision"
			if err.Error() != expectError {
				t.Errorf("expected error %s, got: %s", expectError, err.Error())
			}
		})
	}
}

func TestCompilerRegoEntrypointAnnotations(t *testing.T) {
	tests := []struct {
		note            string
		entrypoints     []string
		modules         map[string]string
		data            string
		roots           []string
		wantEntrypoints map[string]struct{}
	}{
		{
			note:        "rule annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
package test

# METADATA
# entrypoint: true
p {
	q[input.x]
}

q[1]
q[2]
q[3]
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/p": {},
			},
		},
		{
			note:        "package annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
# METADATA
# entrypoint: true
package test

p {
	q[input.x]
}

q[1]
q[2]
q[3]
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test": {},
			},
		},
		{
			note:        "nested rule annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
package test

import data.test.nested

p {
	q[input.x]
	nested.p
}

q[1]
q[2]
q[3]
				`,
				"test/nested.rego": `
package test.nested

# METADATA
# entrypoint: true
p {
	q[input.x]
}

q[1]
q[2]
q[3]
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/nested/p": {},
			},
		},
		{
			note:        "nested package annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
package test

import data.test.nested

p {
	q[input.x]
	nested.p
}

q[1]
q[2]
q[3]
				`,
				"test/nested.rego": `
# METADATA
# entrypoint: true
package test.nested

p {
	q[input.x]
}

q[1]
q[2]
q[3]
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/nested": {},
			},
		},
		{
			note:        "mixed manual entrypoints + annotation entrypoints",
			entrypoints: []string{"test/p"},
			modules: map[string]string{
				"test.rego": `
package test

import data.test.nested

p {
	q[input.x]
	nested.p
}

q[1]
q[2]
q[3]
				`,
				"test/nested.rego": `
# METADATA
# entrypoint: true
package test.nested

p {
	q[input.x]
}

q[1]
q[2]
q[3]
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/nested": {},
				"test/p":      {},
			},
		},
		{
			note:        "ref head rule annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
package test

# METADATA
# entrypoint: true
a.b.c.p {
	q[input.x]
}

q[1]
q[2]
q[3]
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/a/b/c/p": {},
			},
		},
		{
			note:        "mixed ref head rule/package annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
# METADATA
# entrypoint: true
package test.a.b.c

# METADATA
# entrypoint: true
d.e.f.g {
	q[input.x]
}

q[1]
q[2]
q[3]
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/a/b/c":         {},
				"test/a/b/c/d/e/f/g": {},
			},
		},
		{
			note:        "numbers in refs annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
package test

# METADATA
# entrypoint: true
a.b[1.0] {
	q[input.x]
}

q[1]
q[2]
q[3]
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/a/b/1.0": {},
			},
		},
		{
			note:        "string path with brackets annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
package test

# METADATA
# entrypoint: true
a.b["1.0.0"].foo {
	q[input.x]
}

q[1]
q[2]
q[3]
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/a/b/1.0.0/foo": {},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			for _, useMemoryFS := range []bool{false, true} {
				test.WithTestFS(tc.modules, useMemoryFS, func(root string, fsys fs.FS) {
					compiler := New().
						WithFS(fsys).
						WithPaths(root).
						WithTarget("plan").
						WithEntrypoints(tc.entrypoints...).
						WithRegoAnnotationEntrypoints(true).
						WithPruneUnused(true)
					err := compiler.Build(context.Background())
					if err != nil {
						t.Fatal(err)
					}

					// Ensure we have the right number of entrypoints.
					if len(compiler.entrypoints) != len(tc.wantEntrypoints) {
						t.Fatalf("Wrong number of entrypoints. Expected %v, got %v.", tc.wantEntrypoints, compiler.entrypoints)
					}

					// Ensure those entrypoints match the ones we expect.
					for _, entrypoint := range compiler.entrypoints {
						if _, found := tc.wantEntrypoints[entrypoint]; !found {
							t.Fatalf("Unexpected entrypoint '%s'", entrypoint)
						}
					}
				})
			}
		})
	}
}

func TestCompilerSetRevision(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = true`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithRevision("deadbeef")
			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if compiler.bundle.Manifest.Revision != "deadbeef" {
				t.Fatal("expected revision to be set but got:", compiler.bundle.Manifest)
			}
		})
	}
}

func TestCompilerSetMetadata(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = true`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			metadata := map[string]interface{}{"OPA version": "0.36.1"}
			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithMetadata(&metadata)

			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if compiler.bundle.Manifest.Metadata["OPA version"] != "0.36.1" {
				t.Fatal("expected metadata to be set but got:", compiler.bundle.Manifest)
			}
		})
	}
}

func TestCompilerSetRoots(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		import data.common

		x = true`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithRoots("test")

			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if len(*compiler.bundle.Manifest.Roots) != 1 || (*compiler.bundle.Manifest.Roots)[0] != "test" {
				t.Fatal("expected roots to be set to ['test'] but got:", compiler.bundle.Manifest.Roots)
			}
		})
	}
}

func TestCompilerOutput(t *testing.T) {
	// NOTE(tsandall): must use format package here because the compiler formats.
	files := map[string]string{
		"test.rego": string(format.MustAst(ast.MustParseModule(`package test

		p { input.x = data.foo }`))),
		"data.json": `{"foo": 1}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			buf := bytes.NewBuffer(nil)
			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithOutput(buf)
			err := compiler.Build(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			// Check that the written bundle is expected.
			result, err := bundle.NewReader(buf).Read()
			if err != nil {
				t.Fatal(err)
			}

			exp, err := loader.NewFileLoader().WithFS(fsys).AsBundle(root)
			if err != nil {
				t.Fatal(err)
			}

			if !exp.Equal(result) {
				t.Fatalf("expected-1:\n\n%+v\n\ngot-1:\n\n%+v", *exp, result)
			}

			if !exp.Manifest.Equal(result.Manifest) {
				t.Fatalf("expected-2:\n\n%+v\n\ngot-2:\n\n%+v", exp.Manifest, result.Manifest)
			}

			// Check that the returned bundle is the expected.
			compiled := compiler.Bundle()

			if !exp.Equal(*compiled) {
				t.Fatalf("expected-3:\n\n%v\n\ngot-3:\n\n%v", *exp, *compiled)
			}

			if !exp.Manifest.Equal(compiled.Manifest) {
				t.Fatalf("expected-4:\n\n%v\n\ngot-4:\n\n%v", exp.Manifest, compiled.Manifest)
			}
		})
	}
}

func TestOptimizerNoops(t *testing.T) {
	tests := []struct {
		note        string
		entrypoints []string
		modules     map[string]string
	}{
		{
			note:        "recursive result",
			entrypoints: []string{"data.test.foo"},
			modules: map[string]string{
				"test.rego": `
					package test.foo.bar

					p { input.x = 1 }
				`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			o := getOptimizer(tc.modules, "", tc.entrypoints, nil, "")
			cpy := o.bundle.Copy()
			err := o.Do(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !o.bundle.Equal(cpy) {
				t.Fatalf("Expected no change:\n\n%v\n\nGot:\n\n%v", prettyBundle{cpy}, prettyBundle{*o.bundle})
			}
		})
	}
}

func TestOptimizerErrors(t *testing.T) {
	tests := []struct {
		note        string
		entrypoints []string
		modules     map[string]string
		wantErr     error
	}{
		{
			note:        "undefined entrypoint",
			entrypoints: []string{"data.test.p"},
			wantErr:     fmt.Errorf("undefined entrypoint data.test.p"),
		},
		{
			note:        "compile error",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test
					p { data.test.p }
				`,
			},
			wantErr: fmt.Errorf("1 error occurred: test.rego:3: rego_recursion_error: rule data.test.p is recursive: data.test.p -> data.test.p"),
		},
		{
			note:        "partial eval error",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test
					p { {k: v | k = ["a", "a"][_]; v = [0, 1][_] } }
				`,
			},
			wantErr: fmt.Errorf("test.rego:3: eval_conflict_error: object keys must be unique"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			o := getOptimizer(tc.modules, "", tc.entrypoints, nil, "")
			cpy := o.bundle.Copy()
			got := o.Do(context.Background())
			if got == nil || got.Error() != tc.wantErr.Error() {
				t.Fatalf("expected error to be %v but got %v", tc.wantErr, got)
			}
			if !o.bundle.Equal(cpy) {
				t.Fatalf("Expected no change:\n\n%v\n\nGot:\n\n%v", prettyBundle{cpy}, prettyBundle{*o.bundle})
			}
		})
	}
}

func TestOptimizerOutput(t *testing.T) {
	tests := []struct {
		note        string
		entrypoints []string
		modules     map[string]string
		data        string
		roots       []string
		namespace   string
		wantModules map[string]string
	}{
		{
			note:        "rule pruning",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p {
						q[input.x]
					}

					q[1]
					q[2]
					q[3]
				`,
			},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					p = __result__ { 1 = input.x; __result__ = true }
					p = __result__ { 2 = input.x; __result__ = true }
					p = __result__ { 3 = input.x; __result__ = true }
				`,
				"test.rego": `
					package test

					q[1]
					q[2]
					q[3]
				`,
			},
		},
		{
			note:        "support rules",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					default p = false

					p { q[input.x] }

					q[1]
					q[2]`,
			},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					default p = false

					p = true { 1 = input.x }
					p = true { 2 = input.x }

				`,
				"test.rego": `
					package test

					q[1]
					q[2]
				`,
			},
		},
		{
			note:        "support rules, ref heads",
			entrypoints: []string{"data.test.p.q.r"},
			modules: map[string]string{
				"test.rego": `
					package test

					default p.q.r = false
					p.q.r { q[input.x] }

					q[1]
					q[2]`,
			},
			wantModules: map[string]string{
				"optimized/test/p/q.rego": `
					package test.p.q

					default r = false
					r = true { 1 = input.x }
					r = true { 2 = input.x }

				`,
				"test.rego": `
					package test

					q[1]
					q[2]
				`,
			},
		},
		{
			note:        "multiple entrypoints",
			entrypoints: []string{"data.test.p", "data.test.r", "data.test.s"},
			modules: map[string]string{
				"test.rego": `
					package test

					p {
						q[input.x]
					}

					r {
						q[input.x]
					}

					s {
						q[input.x]
					}

					q[1]
				`,
			},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					p = __result__ { 1 = input.x; __result__ = true }
				`,
				"optimized/test.1.rego": `
					package test

					r = __result__ { 1 = input.x; __result__ = true }
				`,
				"optimized/test.2.rego": `
					package test

					s = __result__ { 1 = input.x; __result__ = true }
				`,
				"test.rego": `
					package test

					q[1] { true }
				`,
			},
		},
		{
			note:        "package pruning",
			entrypoints: []string{"data.test.foo"},
			modules: map[string]string{
				"test.rego": `
					package test.foo.bar

					p = true
				`,
			},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					foo = __result__ { __result__ = {"bar": {"p": true}} }`,
			},
		},
		{
			note:        "entrypoint dependent integrity",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p { q[input.x] }

					q[x] {
						s[x]
					}

					s[1]
					s[2]

					t {
						p
					}

					r { t with q as {3} }
				`,
			},
			wantModules: map[string]string{
				"optimized/test.1.rego": `
					package test

					p = __result__ { data.test.q[input.x]; __result__ = true }
				`,
				"optimized/test.rego": `
					package test

					q[1] { true }
					q[2] { true }
				`,
				"test.rego": `
					package test

					s[1] { true }
					s[2] { true }
					t { p }
					r = true { t with q as {3} }
				`,
			},
		},
		{
			note:        "output filename safety",
			entrypoints: []string{`data.test["foo bar"].p`},
			modules: map[string]string{
				"x.rego": `
					package test["foo bar"]  # package does not match safe pattern so use alt. format
					p { q[input.x] }
					q[1]
					q[2]
				`,
			},
			wantModules: map[string]string{
				"optimized/partial/0/0.rego": `
					package test["foo bar"]
					p = __result__ { 1 = input.x; __result__ = true }
					p = __result__ { 2 = input.x; __result__ = true }
				`,
				"x.rego": `
					package test["foo bar"]

					q[1]
					q[2]
				`,
			},
		},
		{
			note:        "generated package namespace",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p { not q }
					q { k[input.a]; k[input.b] }  # generate a product that is not inlined
					k = {1,2,3}
				`,
			},
			wantModules: map[string]string{
				"optimized/partial.rego": `
					package partial

					__not1_0_2__ = true { 1 = input.a; 1 = input.b }
					__not1_0_2__ = true { 1 = input.a; 2 = input.b }
					__not1_0_2__ = true { 1 = input.a; 3 = input.b }
					__not1_0_2__ = true { 2 = input.a; 1 = input.b }
					__not1_0_2__ = true { 2 = input.a; 2 = input.b }
					__not1_0_2__ = true { 2 = input.a; 3 = input.b }
					__not1_0_2__ = true { 3 = input.a; 1 = input.b }
					__not1_0_2__ = true { 3 = input.a; 2 = input.b }
					__not1_0_2__ = true { 3 = input.a; 3 = input.b }
				`,
				"optimized/test.rego": `
					package test

					p = __result__ { not data.partial.__not1_0_2__; __result__ = true }
				`,
				"test.rego": `
					package test

					q = true { k[input.a]; k[input.b] }
					k = {1, 2, 3} { true }
				`,
			},
		},
		{
			note:        "configured package namespace",
			entrypoints: []string{"data.test.p"},
			namespace:   "custom",
			modules: map[string]string{
				"test.rego": `
					package test

					p { not q }
					q { k[input.a]; k[input.b] }  # generate a product that is not inlined
					k = {1,2,3}
				`,
			},
			wantModules: map[string]string{
				"optimized/custom.rego": `
					package custom

					__not1_0_2__ = true { 1 = input.a; 1 = input.b }
					__not1_0_2__ = true { 1 = input.a; 2 = input.b }
					__not1_0_2__ = true { 1 = input.a; 3 = input.b }
					__not1_0_2__ = true { 2 = input.a; 1 = input.b }
					__not1_0_2__ = true { 2 = input.a; 2 = input.b }
					__not1_0_2__ = true { 2 = input.a; 3 = input.b }
					__not1_0_2__ = true { 3 = input.a; 1 = input.b }
					__not1_0_2__ = true { 3 = input.a; 2 = input.b }
					__not1_0_2__ = true { 3 = input.a; 3 = input.b }
				`,
				"optimized/test.rego": `
					package test

					p = __result__ { not data.custom.__not1_0_2__; __result__ = true }
				`,
				"test.rego": `
					package test

					q = true { k[input.a]; k[input.b] }
					k = {1, 2, 3} { true }
				`,
			},
		},
		{
			note:        "infer unknowns from roots",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p {
						q[x]
						data.external.users[x] == input.user
					}

					q["foo"]
					q["bar"]
				`,
			},
			roots: []string{"test"},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					p = __result__ { data.external.users.bar = input.user; __result__ = true }
					p = __result__ { data.external.users.foo = input.user; __result__ = true }
				`,
				"test.rego": `
					package test

					q["foo"]
					q["bar"]
				`,
			},
		},
		{
			note:        "generate rules with type violations: complete doc",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p {
						x := split(input.a, ":")
						f(x[0])
					}

					f(x) { x == null }
					f(x) { startswith(x, "foo") }
				`,
			},
			roots: []string{"test"},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					p = __result__ { split(input.a, ":", __local3__1); startswith(__local3__1[0], "foo"); __result__ = true }
				`,
				"test.rego": `
					package test

					f(x) { x == null }
					f(x) { startswith(x, "foo") }
				`,
			},
		},
		{
			note:        "generate rules with type violations: partial set",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p[msg] {
						x := split(input.a, ":")
						f(x[0])
						msg := "test string"
					}

					f(x) { x == null }
					f(x) { startswith(x, "foo") }
				`,
			},
			roots: []string{"test"},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					p["test string"] { split(input.a, ":", __local4__1); startswith(__local4__1[0], "foo") }
				`,
				"test.rego": `
					package test

					f(x) { x == null }
					f(x) { startswith(x, "foo") }
				`,
			},
		},
		{
			note:        "generate rules with type violations: partial object",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p[k] = value  {
						x := split(input.a, ":")
						f(x[0])
						k := "a"
						value := 1
					}

					f(x) { x == null }
					f(x) { startswith(x, "foo") }
				`,
			},
			roots: []string{"test"},
			wantModules: map[string]string{
				"optimized/test/p.rego": `
					package test.p

					a = 1 { split(input.a, ":", __local5__1); startswith(__local5__1[0], "foo") }
				`,
				"test.rego": `
					package test

					f(x) { x == null }
					f(x) { startswith(x, "foo") }
				`,
			},
		},
		{
			note:        "generate rules with type violations: negation",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p  { not q }
					q {
						x := split(input.a, ":")
						f(x[0])
					}
					f(x) { x == null }
					f(x) { startswith(x, "foo") }
				`,
			},
			roots: []string{"test"},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					p = __result__ { not data.partial.__not1_0_2__; __result__ = true }
				`,
				"test.rego": `
					package test
					q = true { assign(x, split(input.a, ":")); f(x[0]) }
					f(x) { x == null }
					f(x) { startswith(x, "foo") }
				`,
				"optimized/partial.rego": `
					package partial
            		__not1_0_2__ = true { split(input.a, ":", __local3__3); startswith(__local3__3[0], "foo") }
				`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			o := getOptimizer(tc.modules, tc.data, tc.entrypoints, tc.roots, tc.namespace)
			original := o.bundle.Copy()
			err := o.Do(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			exp := &bundle.Bundle{
				Modules: getModuleFiles(tc.wantModules, false),
				Data:    original.Data, // data is not pruned at all today
			}

			if len(tc.roots) > 0 {
				exp.Manifest.Roots = &tc.roots

				// optimizer will add the manifest root in the optimized bundle automatically
				if tc.namespace != "" {
					exp.Manifest.AddRoot(tc.namespace)
				} else {
					exp.Manifest.AddRoot("partial")
				}
			}

			exp.Manifest.Revision = "" // optimizations must reset the revision.

			if !exp.Equal(*o.bundle) {
				t.Errorf("Expected:\n\n%v\n\nGot:\n\n%v", prettyBundle{*exp}, prettyBundle{*o.bundle})
			}

			if !o.bundle.Manifest.Equal(exp.Manifest) {
				t.Errorf("Expected manifest: %v\n\nGot manifest: %v", exp.Manifest, o.bundle.Manifest)
			}
		})
	}
}

func TestRefSet(t *testing.T) {
	rs := newRefSet(ast.MustParseRef("input"), ast.MustParseRef("data.foo.bar"))

	expFound := []string{
		"input",
		"input.foo",
		"data.foo.bar",
		"data.foo.bar.baz",
		"data.foo.bar[1]",
	}

	for _, exp := range expFound {
		if !rs.ContainsPrefix(ast.MustParseRef(exp)) {
			t.Fatal("expected to find:", exp)
		}
	}

	expNotFound := []string{
		"x.bar",
		"data",
		"data.bar",
		"data.foo",
	}

	for _, exp := range expNotFound {
		if rs.ContainsPrefix(ast.MustParseRef(exp)) {
			t.Fatal("expected not to find:", exp)
		}
	}

	rs.AddPrefix(ast.MustParseRef("data.foo"))

	if !rs.ContainsPrefix(ast.MustParseRef("data.foo")) {
		t.Fatal("expected to find data.foo after adding to set")
	}

	sorted := rs.Sorted()

	if len(sorted) != 2 || !sorted[0].Equal(ast.MustParseTerm("data.foo")) || !sorted[1].Equal(ast.MustParseTerm("input")) {
		t.Fatal("expected 2 prefixes (data.foo and input) but got:", sorted)
	}

	// The prefixes should not be affected (because data.foo already exists).
	rs.AddPrefix(ast.MustParseRef("data.foo.qux"))
	sorted = rs.Sorted()

	if len(sorted) != 2 || !sorted[0].Equal(ast.MustParseTerm("data.foo")) || !sorted[1].Equal(ast.MustParseTerm("input")) {
		t.Fatal("expected 2 prefixes (data.foo and input) but got:", sorted)
	}

}

func getOptimizer(modules map[string]string, data string, entries []string, roots []string, ns string) *optimizer {

	b := &bundle.Bundle{
		Modules: getModuleFiles(modules, true),
	}

	if data != "" {
		b.Data = util.MustUnmarshalJSON([]byte(data)).(map[string]interface{})
	}

	if len(roots) > 0 {
		b.Manifest.Roots = &roots
	}

	b.Manifest.Init()
	b.Manifest.Revision = "DEADBEEF" // ensures that the manifest revision is getting reset
	entrypoints := make([]*ast.Term, len(entries))

	for i := range entrypoints {
		entrypoints[i] = ast.MustParseTerm(entries[i])
	}

	o := newOptimizer(ast.CapabilitiesForThisVersion(), b).
		WithEntrypoints(entrypoints)

	if ns != "" {
		o = o.WithPartialNamespace(ns)
	}

	o.resultsymprefix = ""

	return o
}

func getModuleFiles(src map[string]string, includeRaw bool) []bundle.ModuleFile {

	keys := make([]string, 0, len(src))
	for k := range src {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	modules := make([]bundle.ModuleFile, 0, len(keys))

	for _, k := range keys {
		module, err := ast.ParseModule(k, src[k])
		if err != nil {
			panic(err)
		}
		modules = append(modules, bundle.ModuleFile{
			Parsed: module,
			Path:   k,
			URL:    k,
		})
		if includeRaw {
			modules[len(modules)-1].Raw = []byte(src[k])
		}
	}

	return modules
}

type prettyBundle struct {
	bundle.Bundle
}

func (p prettyBundle) String() string {

	buf := []string{fmt.Sprintf("%d module(s) (hiding data):", len(p.Modules)), ""}

	for _, mf := range p.Modules {
		buf = append(buf, "#")
		buf = append(buf, fmt.Sprintf("# Module: %q", mf.Path))
		buf = append(buf, "#")
		buf = append(buf, mf.Parsed.String())
		buf = append(buf, "")
	}

	return strings.Join(buf, "\n")
}
