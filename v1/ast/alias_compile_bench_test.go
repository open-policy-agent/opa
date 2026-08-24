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
	}{
		{"aliased", "data.alias.path"},
		{"direct", "input.path"},
	} {
		for _, n := range []int{200, 2000} {
			b.Run(fmt.Sprintf("%s/%d", tc.name, n), func(b *testing.B) {
				var buf strings.Builder
				buf.WriteString("package test\n\n")
				for i := 1; i <= n; i++ {
					fmt.Fprintf(&buf, "p if %s == [\"ep%d\"]\n\n", tc.ref, i)
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
