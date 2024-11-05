package compile

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/util/test"
)

func TestCompilerLoadAsBundleSuccess(t *testing.T) {

	ctx := context.Background()
	rv := fmt.Sprintf("%d", ast.DefaultRegoVersion.Int())

	files := map[string]string{
		"b1/.manifest": `{"roots": ["b1"], "rego_version": ` + rv + `}`,
		"b1/test.rego": `
			package b1.test
			import rego.v1

			p = 1`,
		"b1/data.json": `
			{"b1": {"k": "v"}}`,
		"b2/.manifest": `{"roots": ["b2"], "rego_version": ` + rv + `}`,
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

			if !compiler.Bundle().Equal(*exp) {
				test.FatalMismatch(t, compiler.Bundle(), *exp)
			}

			expRoots := []string{"b1", "b2"}
			expManifest := bundle.Manifest{
				Roots: &expRoots,
			}
			expManifest.SetRegoVersion(ast.RegoV0)

			if !compiler.Bundle().Manifest.Equal(expManifest) {
				test.FatalMismatch(t, compiler.Bundle().Manifest, expManifest)
			}
		})
	}
}
