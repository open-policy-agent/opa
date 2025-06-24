package ast_test

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

var (
	val = ast.String("open-policy-agent")
	obj = map[string]ast.Value{"open-policy-agent": val}
)

//go:noinline
func getPackageVarValue() ast.Value {
	return val
}

//go:noinline
func getObjectValue() ast.Value {
	return obj["open-policy-agent"]
}

//go:noinline
func getInternedValue() ast.Value {
	return ast.InternedTerm("open-policy-agent").Value
}

//go:noinline
func getNewValue() ast.Value {
	return ast.String("open-policy-agent")
}

// Benchmark experiment to compare the performance of accessing values in different ways.
//
// BenchmarkInterningAccessValue/package_var_value-12         100000000      10.95 ns/op     16 B/op    1 allocs/op
// BenchmarkInterningAccessValue/interned_value-12            175498335       6.81 ns/op      0 B/op    0 allocs/op
// BenchmarkInterningAccessValue/object_value-12              247139934       4.78 ns/op      0 B/op    0 allocs/op
// BenchmarkInterningAccessValue/new_value-12                 1000000000      0.70 ns/op      0 B/op    0 allocs/op
func BenchmarkInterningAccessValue(b *testing.B) {
	ast.InternStringTerm("open-policy-agent")

	b.Run("package var value", func(b *testing.B) {
		for range b.N {
			_ = getPackageVarValue()
		}
	})

	b.Run("interned value", func(b *testing.B) {
		for range b.N {
			_ = getInternedValue()
		}
	})

	b.Run("object value", func(b *testing.B) {
		for range b.N {
			_ = getObjectValue()
		}
	})

	b.Run("new value", func(b *testing.B) {
		for range b.N {
			_ = getNewValue()
		}
	})
}
