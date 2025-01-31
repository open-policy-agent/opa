package dependencies

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func BenchmarkBase(b *testing.B) {
	ruleCounts := []int{10, 20, 50}
	for _, ruleCount := range ruleCounts {
		b.Run(strconv.Itoa(ruleCount), func(b *testing.B) {
			policy := makePolicy(ruleCount)
			module := ast.MustParseModule(policy)
			compiler := ast.NewCompiler()
			if compiler.Compile(map[string]*ast.Module{"test": module}); compiler.Failed() {
				b.Fatalf("Failed to compile policy: %v", compiler.Errors)
			}

			ref := ast.MustParseRef("data.test.main")

			b.ResetTimer()

			_, err := Base(compiler, ref)
			if err != nil {
				b.Fatalf("Failed to compute base doc deps: %v", err)
			}
		})
	}
}

func BenchmarkVirtual(b *testing.B) {
	ruleCounts := []int{10, 20, 50}
	for _, ruleCount := range ruleCounts {
		b.Run(strconv.Itoa(ruleCount), func(b *testing.B) {
			policy := makePolicy(ruleCount)
			module := ast.MustParseModule(policy)
			compiler := ast.NewCompiler()
			if compiler.Compile(map[string]*ast.Module{"test": module}); compiler.Failed() {
				b.Fatalf("Failed to compile policy: %v", compiler.Errors)
			}

			ref := ast.MustParseRef("data.test.main")

			b.ResetTimer()

			_, err := Virtual(compiler, ref)
			if err != nil {
				b.Fatalf("Failed to compute virtual doc deps: %v", err)
			}
		})
	}
}

// makePolicy constructs a policy with ruleCount number of rules.
// Each rule will depend on as many other rules as possible without creating circular dependencies.
func makePolicy(ruleCount int) string {
	var b strings.Builder
	b.WriteString("package test\n\n")

	b.WriteString("main if {\n")
	for i := range ruleCount {
		b.WriteString(fmt.Sprintf("  p_%d\n", i))
	}
	b.WriteString("}\n\n")

	for i := range ruleCount {
		b.WriteString(fmt.Sprintf("p_%d if {\n", i))
		for j := i + 1; j < ruleCount; j++ {
			b.WriteString(fmt.Sprintf("  p_%d\n", j))
		}
		b.WriteString("  input.x == 1\n")
		b.WriteString("  input.y == 2\n")
		b.WriteString("  input.z == 3\n")
		b.WriteString("}\n")
	}
	return b.String()
}
