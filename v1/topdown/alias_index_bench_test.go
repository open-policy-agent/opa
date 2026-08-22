// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

func BenchmarkRuleIndexAliasedRefs(b *testing.B) {
	aliasModule := `package alias

path := input.path`

	for _, tc := range []struct {
		name string
		ref  string
	}{
		{"aliased", "data.alias.path"},
		{"direct", "input.path"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			var buf strings.Builder
			buf.WriteString("package test\n\n")
			for i := 1; i <= 200; i++ {
				fmt.Fprintf(&buf, "p if %s == [\"ep%d\"]\n\n", tc.ref, i)
			}

			compiler := ast.NewCompiler()
			compiler.Compile(map[string]*ast.Module{
				"alias.rego": ast.MustParseModule(aliasModule),
				"test.rego":  ast.MustParseModule(buf.String()),
			})
			if compiler.Failed() {
				b.Fatalf("compilation failed: %v", compiler.Errors)
			}

			store := inmem.New()
			input := ast.MustParseTerm(`{"path": ["ep200"]}`)
			ctx := context.Background()

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				q := NewQuery(ast.MustParseBody("data.test.p")).
					WithCompiler(compiler).
					WithStore(store).
					WithInput(input)

				rs, err := q.Run(ctx)
				if err != nil {
					b.Fatalf("query failed: %v", err)
				}
				if len(rs) != 1 {
					b.Fatalf("expected exactly one result, got %d", len(rs))
				}
			}
		})
	}
}
