package ast

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkCompileAliasedRefs(b *testing.B) {
	aliasModule := `package alias

path := input.path`

	for _, tc := range []struct {
		name string
		ref  string
		with bool
	}{
		{"aliased", "data.alias.path", false},
		{"aliased_with_input", "data.alias.path", true},
		{"direct", "input.path", false},
	} {
		for _, n := range []int{200, 2000} {
			b.Run(fmt.Sprintf("%s/%d", tc.name, n), func(b *testing.B) {
				var buf strings.Builder
				buf.WriteString("package test\n\n")
				with := ""
				if tc.with {
					with = ` with input as {"path": ["ep%d"]}`
				}
				for i := 1; i <= n; i++ {
					if tc.with {
						fmt.Fprintf(&buf, "p if %s == [\"ep%d\"]"+with+"\n\n", tc.ref, i, i)
					} else {
						fmt.Fprintf(&buf, "p if %s == [\"ep%d\"]\n\n", tc.ref, i)
					}
				}
				baseAlias := MustParseModule(aliasModule)
				baseTest := MustParseModule(buf.String())

				b.ReportAllocs()
				b.ResetTimer()

				for b.Loop() {
					b.StopTimer()
					mods := map[string]*Module{
						"alias.rego": baseAlias.Copy(),
						"test.rego":  baseTest.Copy(),
					}
					b.StartTimer()

					c := NewCompiler()
					if c.Compile(mods); c.Failed() {
						b.Fatal(c.Errors)
					}
				}
			})
		}
	}
}
