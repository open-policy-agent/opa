package compile

import (
	"bytes"
	"context"
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
			note: "entrypoint parse error",
			c:    New().WithEntrypoints("foo%bar"),
			want: fmt.Errorf("entrypoint foo%%bar not valid: use <package>/<rule>"),
		},
		{
			note: "entrypoint too short error",
			c:    New().WithEntrypoints("foo"),
			want: fmt.Errorf("entrypoint foo too short: use <package>/<rule>"),
		},
		{
			note: "optimizations require entrypoint",
			c:    New().WithOptimizationLevel(1),
			want: errors.New("bundle optimizations require at least one entrypoint"),
		},
		{
			note: "wasm compilation requires exactly one entrypoint",
			c:    New().WithTarget("wasm"),
			want: errors.New("wasm compilation requires exactly one entrypoint"),
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

		if !compiler.bundle.Equal(*exp) {
			t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, compiler.bundle)
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

func TestCompilerOptimization(t *testing.T) {
	files := map[string]string{
		"test.rego": `
			package test
			default p = false
			p { input.x = data.foo }`,
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

		exp := ast.MustParseModule(`
			package test

			p { input.x = 1}
			default p = false
		`)

		if len(compiler.bundle.Modules) != 1 || !compiler.bundle.Modules[0].Parsed.Equal(exp) {
			t.Fatalf("expected one module but got: %v", compiler.bundle.Modules)
		}
	})
}

func TestCompilerWasmTarget(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = true`,
	}

	test.WithTempFS(files, func(root string) {

		compiler := New().WithPaths(root).WithTarget("wasm").WithEntrypoints("test/p")
		err := compiler.Build(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if len(compiler.bundle.Wasm) == 0 {
			t.Fatal("expected to find compiled wasm module")
		}

	})
}

func TestCompilerWasmTargetLazyCompile(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p { input.x = q }
		q = "foo"`,
	}

	test.WithTempFS(files, func(root string) {

		compiler := New().WithPaths(root).WithTarget("wasm").WithEntrypoints("test/p").WithOptimizationLevel(1)
		err := compiler.Build(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if len(compiler.bundle.Wasm) == 0 {
			t.Fatal("expected to find compiled wasm module")
		}

		if _, exists := compiler.compiler.Modules["optimized/test.rego"]; !exists {
			t.Fatal("expected to find optimized module on compiler")
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
		result, err := bundle.NewReader(buf).Read()
		if err != nil {
			t.Fatal(err)
		}

		exp, err := loader.NewFileLoader().AsBundle(root)
		if err != nil {
			t.Fatal(err)
		}

		if !exp.Equal(result) {
			t.Fatalf("expected:\n\n%v\n\ngot:\n\n%v", *exp, result)
		}

		if !exp.Manifest.Equal(result.Manifest) {
			t.Fatalf("expected:\n\n%v\n\ngot:\n\n%v", exp.Manifest, result.Manifest)
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
			o := getOptimizer(tc.modules, "", tc.entrypoints)
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
					p { div(1, 0, x) }
				`,
			},
			wantErr: fmt.Errorf("test.rego:3: eval_builtin_error: div: divide by zero"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			o := getOptimizer(tc.modules, "", tc.entrypoints)
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

					p = true { 1 = input.x }
					p = true { 2 = input.x }

					default p = false
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
			entrypoints: []string{"data.test.p", "data.test.r"},
			modules: map[string]string{
				"test.rego": `
					package test

					p {
						q[input.x]
					}

					r {
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

					__not1_0__ = true { 1 = input.a; 1 = input.b }
					__not1_0__ = true { 1 = input.a; 2 = input.b }
					__not1_0__ = true { 1 = input.a; 3 = input.b }
					__not1_0__ = true { 2 = input.a; 1 = input.b }
					__not1_0__ = true { 2 = input.a; 2 = input.b }
					__not1_0__ = true { 2 = input.a; 3 = input.b }
					__not1_0__ = true { 3 = input.a; 1 = input.b }
					__not1_0__ = true { 3 = input.a; 2 = input.b }
					__not1_0__ = true { 3 = input.a; 3 = input.b }
				`,
				"optimized/test.rego": `
					package test

					p = __result__ { not data.partial.__not1_0__; __result__ = true }
				`,
				"test.rego": `
					package test

					q = true { k[input.a]; k[input.b] }
					k = {1, 2, 3} { true }
				`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			o := getOptimizer(tc.modules, tc.data, tc.entrypoints)
			original := o.bundle.Copy()
			err := o.Do(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			exp := &bundle.Bundle{
				Modules: getModuleFiles(tc.wantModules, false),
				Data:    original.Data, // data is not pruned at all today
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

func getOptimizer(modules map[string]string, data string, entries []string) *optimizer {

	b := &bundle.Bundle{
		Modules: getModuleFiles(modules, true),
	}

	if data != "" {
		b.Data = util.MustUnmarshalJSON([]byte(data)).(map[string]interface{})
	}

	b.Manifest.Init()
	b.Manifest.Revision = "DEADBEEF" // ensures that the manifest revision is getting reset
	entrypoints := make([]*ast.Term, len(entries))

	for i := range entrypoints {
		entrypoints[i] = ast.MustParseTerm(entries[i])
	}

	o := newOptimizer(b).
		WithEntrypoints(entrypoints)

	o.resultsymprefix = ""

	return o
}

func getModuleFiles(src map[string]string, includeRaw bool) []bundle.ModuleFile {

	var keys []string

	for k := range src {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	var modules []bundle.ModuleFile

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
