package compile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"sort"
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

	test.WithTempFS(nil, func(root string) {
		err := New().WithPaths(path.Join(root, "does-not-exist")).Build(context.Background())
		if err == nil {
			t.Fatal("expected failure")
		}
	})
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

	test.WithTempFS(files, func(root string) {

		root1 := path.Join(root, "b1")
		root2 := path.Join(root, "b2")

		compiler := New().
			WithPaths(root1, root2).
			WithAsBundle(true)

		err := compiler.Build(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Verify result is just merger of two bundles.
		a, err := loader.NewFileLoader().AsBundle(root1)
		if err != nil {
			panic(err)
		}

		b, err := loader.NewFileLoader().AsBundle(root2)
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

	test.WithTempFS(files, func(root string) {

		root1 := path.Join(root, "b1")
		root2 := path.Join(root, "b2")

		compiler := New().
			WithPaths(root1, root2).
			WithAsBundle(true)

		err := compiler.Build(ctx)
		if err == nil || err.Error() != "bundle merge failed: manifest has overlapped roots: '' and ''" {
			t.Fatal(err)
		}
	})
}

func TestCompilerLoadFilesystem(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package b1.test

			p = 1`,
		"data.json": `
			{"b1": {"k": "v"}}`,
	}

	test.WithTempFS(files, func(root string) {

		compiler := New().
			WithPaths(root)

		err := compiler.Build(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		// Verify result is just bundle load.
		exp, err := loader.NewFileLoader().AsBundle(root)
		if err != nil {
			panic(err)
		}

		err = exp.FormatModules(false)
		if err != nil {
			t.Fatal(err)
		}

		if !compiler.bundle.Equal(*exp) {
			t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, compiler.bundle)
		}
	})
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

	test.WithTempFS(files, func(root string) {
		compiler := New().
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

func TestCompilerLoadFilesystemWithEnablePrintStatementsTrue(t *testing.T) {
	files := map[string]string{
		"test.rego": `
			package test

                        allow { print(1) }
		`,
		"data.json": `
			{"b1": {"k": "v"}}`,
	}

	test.WithTempFS(files, func(root string) {
		compiler := New().
			WithPaths(root).
			WithTarget("plan").WithEntrypoints("test/allow").
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

func TestCompilerLoadHonorsFilter(t *testing.T) {
	files := map[string]string{
		"test.rego": `
			package b1.test

			p = 1`,
		"data.json": `
			{"b1": {"k": "v"}}`,
	}

	test.WithTempFS(files, func(root string) {

		compiler := New().
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

	test.WithTempFS(files, func(root string) {

		compiler := New().
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

	test.WithTempFS(files, func(root string) {

		compiler := New().
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

	test.WithTempFS(files, func(root string) {

		compiler := New().
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

		if !compiler.bundle.Modules[0].Parsed.Equal(prunedExp) {
			t.Fatalf("expected pruned module to be:\n\n%v\n\ngot:\n\n%v", prunedExp, compiler.bundle.Modules[0])
		}

		if !compiler.bundle.Modules[1].Parsed.Equal(optimizedExp) {
			t.Fatalf("expected optimized module to be:\n\n%v\n\ngot:\n\n%v", optimizedExp, compiler.bundle.Modules[1])
		}
	})
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

	test.WithTempFS(files, func(root string) {

		compiler := New().WithPaths(root).WithTarget("wasm").WithEntrypoints("test/p", "test/q").
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

// If we're building a wasm bundle, and the `opa` binary we use to do that
// does not support wasm _itself_, then it shouldn't bother.
func TestCompilerWasmTargetWithCapabilitiesUnset(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = 7
		q = p+1`,
	}

	test.WithTempFS(files, func(root string) {

		compiler := New().WithPaths(root).WithTarget("wasm").WithEntrypoints("test/p", "test/q")
		err := compiler.Build(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}

func TestCompilerWasmTargetWithCapabilitiesMismatch(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = 7
		q = p+1`,
	}

	test.WithTempFS(files, func(root string) {

		for note, wabis := range map[string][]ast.WasmABIVersion{
			"none":     {},
			"mismatch": {{Version: 0}, {Version: 1, Minor: 2000}},
		} {
			t.Run(note, func(t *testing.T) {
				caps := ast.CapabilitiesForThisVersion()
				caps.WasmABIVersions = wabis
				compiler := New().WithPaths(root).WithTarget("wasm").WithEntrypoints("test/p", "test/q").
					WithCapabilities(caps)
				err := compiler.Build(context.Background())
				if err == nil {
					t.Fatal("expected err, got nil")
				}
			})
		}
	})
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

	test.WithTempFS(files, func(root string) {

		compiler := New().WithPaths(root).WithTarget("wasm").WithEntrypoints("test/p", "policy/authz").
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

func TestCompilerWasmTargetEntrypointDependents(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p { q }
		q { r }
		r = 1
		s = 2
		z { r }`}

	test.WithTempFS(files, func(root string) {

		compiler := New().WithPaths(root).WithTarget("wasm").WithEntrypoints("test/r", "test/z").
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

func TestCompilerWasmTargetLazyCompile(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p { input.x = q }
		q = "foo"`,
	}

	test.WithTempFS(files, func(root string) {

		compiler := New().WithPaths(root).WithTarget("wasm").WithEntrypoints("test/p").WithOptimizationLevel(1).
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

	test.WithTempFS(files, func(root string) {

		compiler := New().WithPaths(root).WithTarget("plan").WithEntrypoints("test/p", "test/q")
		err := compiler.Build(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if len(compiler.bundle.PlanModules) == 0 {
			t.Fatal("expected to find compiled plan module")
		}
	})
}

func TestCompilerPlanTargetPruneUnused(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test
		p[1]
		f(x) { p[x] }`,
	}

	test.WithTempFS(files, func(root string) {

		compiler := New().
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

func TestCompilerPlanTargetUnmatchedEntrypoints(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p := 7
		q := p + 1`,
	}

	test.WithTempFS(files, func(root string) {

		compiler := New().WithPaths(root).WithTarget("plan").WithEntrypoints("test/p", "test/q", "test/no")
		err := compiler.Build(context.Background())
		if err == nil {
			t.Error("expected error from unmatched entrypoint")
		}
		expectError := "entrypoint \"test/no\" does not refer to a rule or policy decision"
		if err.Error() != expectError {
			t.Errorf("expected error %s, got: %s", expectError, err.Error())
		}
	})

	test.WithTempFS(files, func(root string) {

		compiler := New().WithPaths(root).WithTarget("plan").WithEntrypoints("foo", "foo.bar", "test/no")
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

func TestCompilerSetRevision(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = true`,
	}

	test.WithTempFS(files, func(root string) {

		compiler := New().WithPaths(root).WithRevision("deadbeef")
		err := compiler.Build(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if compiler.bundle.Manifest.Revision != "deadbeef" {
			t.Fatal("expected revision to be set but got:", compiler.bundle.Manifest)
		}
	})
}

func TestCompilerSetMetadata(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = true`,
	}

	test.WithTempFS(files, func(root string) {
		metadata := map[string]interface{}{"OPA version": "0.36.1"}
		compiler := New().WithPaths(root).WithMetadata(&metadata)

		err := compiler.Build(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if compiler.bundle.Manifest.Metadata["OPA version"] != "0.36.1" {
			t.Fatal("expected metadata to be set but got:", compiler.bundle.Manifest)
		}
	})
}

func TestCompilerOutput(t *testing.T) {
	// NOTE(tsandall): must use format package here because the compiler formats.
	files := map[string]string{
		"test.rego": string(format.MustAst(ast.MustParseModule(`package test

		p { input.x = data.foo }`))),
		"data.json": `{"foo": 1}`,
	}
	test.WithTempFS(files, func(root string) {

		buf := bytes.NewBuffer(nil)
		compiler := New().WithPaths(root).WithOutput(buf)
		err := compiler.Build(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		// Check that the written bundle is expected.
		result, err := bundle.NewReader(buf).Read()
		if err != nil {
			t.Fatal(err)
		}

		exp, err := loader.NewFileLoader().AsBundle(root)
		if err != nil {
			t.Fatal(err)
		}

		if !exp.Equal(result) {
			t.Fatalf("expected:\n\n%+v\n\ngot:\n\n%+v", *exp, result)
		}

		if !exp.Manifest.Equal(result.Manifest) {
			t.Fatalf("expected:\n\n%+v\n\ngot:\n\n%+v", exp.Manifest, result.Manifest)
		}

		// Check that the returned bundle is the expected.
		compiled := compiler.Bundle()

		if !exp.Equal(*compiled) {
			t.Fatalf("expected:\n\n%v\n\ngot:\n\n%v", *exp, *compiled)
		}

		if !exp.Manifest.Equal(compiled.Manifest) {
			t.Fatalf("expected:\n\n%v\n\ngot:\n\n%v", exp.Manifest, compiled.Manifest)
		}
	})
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
			o := getOptimizer(tc.modules, "", tc.entrypoints, nil)
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
			wantErr: fmt.Errorf("1 error occurred: test.rego:3: rego_recursion_error: rule p is recursive: p -> p"),
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
			o := getOptimizer(tc.modules, "", tc.entrypoints, nil)
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
				"optimized/test.rego": `
					package test

					p["a"] = 1 { split(input.a, ":", __local5__1); startswith(__local5__1[0], "foo") }
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

			o := getOptimizer(tc.modules, tc.data, tc.entrypoints, tc.roots)
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
				exp.Manifest.AddRoot("partial") // optimizer will add this automatically
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

func getOptimizer(modules map[string]string, data string, entries []string, roots []string) *optimizer {

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
