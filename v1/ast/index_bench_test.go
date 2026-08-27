package ast

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

// Linear growth in index build for a single rule + ref where only the value differs
func BenchmarkBuildEqIndex(b *testing.B) {
	for i := range 5 {
		n := math.Pow(10, float64(i))
		b.Run(strconv.Itoa(int(n)), func(b *testing.B) {
			rules := eqIndexRules(int(n))

			for b.Loop() {
				index := newBaseDocEqIndex(isVirtual)
				if !index.Build(rules) {
					b.Fatal("failed to build index")
				}
			}
		})
	}
}

// 162.0 ns/op	      16 B/op	       1 allocs/op
func BenchmarkLookupEqIndex(b *testing.B) {
	rules := eqIndexRules(1000)
	index := newBaseDocEqIndex(isVirtual)
	if !index.Build(rules) {
		b.Fatal("failed to build index")
	}
	input := inputResolver{input: MustParseTerm(`{"foo": {"bar": 999}}`).Value}

	for b.Loop() {
		res, err := index.Lookup(input)
		if err != nil {
			b.Fatal(err)
		} else if len(res.Rules) != 1 {
			b.Fatalf("expected 1 rule, got %d", len(res.Rules))
		}
		IndexResultPool.Put(res)
	}
}

// Exponential growth in index build for a single rule where each ref is unique
// Cost reduced by two thirds by lazy init of scalars map
func BenchmarkBuildNakedRefIndex(b *testing.B) {
	// Exp capped at 4 here because at 5 this allocates 12.8GB of memory,
	// which our CI runner won't be happy about.
	for i := range 4 {
		n := math.Pow(10, float64(i))
		b.Run(strconv.Itoa(int(n)), func(b *testing.B) {
			rules := nakedRefRules(int(n))

			for b.Loop() {
				index := newBaseDocEqIndex(isVirtual)
				if !index.Build(rules) {
					b.Fatal("failed to build index")
				}
			}
		})
	}
}

// 54912 ns/op	      16 B/op	       1 allocs/op
func BenchmarkLookupNakedRefIndex(b *testing.B) {
	rules := nakedRefRules(1000)
	input := inputResolver{input: MustParseTerm(`{"foo": {"500": 500}}`).Value}
	index := newBaseDocEqIndex(isVirtual)
	if !index.Build(rules) {
		b.Fatal("failed to build index")
	}

	for b.Loop() {
		res, err := index.Lookup(input)
		if err != nil {
			b.Fatal(err)
		} else if len(res.Rules) != 1 {
			b.Fatalf("expected 1 rule, got %d", len(res.Rules))
		}
		IndexResultPool.Put(res)
	}
}

// Exponential growth in non-sensical index build where each ref+value is unique
//
// BenchmarkBuildNonsensicalIndex/n_=_10-16           137611        8766 ns/op	      20984 B/op         223 allocs/op
// BenchmarkBuildNonsensicalIndex/n_=_100-16            2427      502340 ns/op	    1360788 B/op       11125 allocs/op
// BenchmarkBuildNonsensicalIndex/n_=_1000-16             24    46709530 ns/op	  128900337 B/op     1011048 allocs/op
// BenchmarkBuildNonsensicalIndex/n_=_10000-16             1  5349418417 ns/op	12808545832 B/op   100110165 allocs/op
func BenchmarkBuildNonsensicalIndex(b *testing.B) {
	// Exp capped at 4 here because at 5 this allocates 12.8GB of memory,
	// which our CI runner won't be happy about.
	for i := range 4 {
		n := math.Pow(10, float64(i))
		b.Run(strconv.Itoa(int(n)), func(b *testing.B) {
			rules := nonsensicalRules(int(n))

			for b.Loop() {
				index := newBaseDocEqIndex(isVirtual)
				if !index.Build(rules) {
					b.Fatal("failed to build index")
				}
			}
		})
	}
}

// Index build for a single rule whose membership collection holds n values.
// Recording collection values without the per-value duplicate scan took this
// from quadratic to linear: at n = 8000, the build was 226ms before.
//
// BenchmarkBuildMembershipIndex/1000-16        272750 ns/op
// BenchmarkBuildMembershipIndex/8000-16       2157209 ns/op
// BenchmarkBuildMembershipIndex/100000-16    12888042 ns/op
func BenchmarkBuildMembershipIndex(b *testing.B) {
	for _, n := range []int{1000, 8000, 100000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			rules := membershipRules(b, n)

			for b.Loop() {
				index := newBaseDocEqIndex(isVirtual)
				if !index.Build(rules) {
					b.Fatal("failed to build index")
				}
			}
		})
	}
}

// membershipRules returns one compiled `allow if input.subject in {...}` rule
// over a set of n values.
func membershipRules(b *testing.B, n int) []*Rule {
	b.Helper()

	var sb strings.Builder
	sb.WriteString("package p\n\nallow if input.subject in {")
	for i := range n {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`"u`)
		sb.WriteString(strconv.Itoa(i))
		sb.WriteByte('"')
	}
	sb.WriteString("}\n")

	c := NewCompiler()
	c.Compile(map[string]*Module{"p.rego": MustParseModule(sb.String())})
	if c.Failed() {
		b.Fatal(c.Errors)
	}

	return c.Modules["p.rego"].Rules
}

type inputResolver struct {
	input Value
}

func (v inputResolver) Resolve(r Ref) (Value, error) {
	value, _ := v.input.Find(r[1:])
	return value, nil
}

func isVirtual(Ref) bool {
	return false
}

func eqIndexRules(n int) []*Rule {
	var sb strings.Builder
	sb.WriteString("package p\n\n")
	for i := range n {
		sb.WriteString("allow if input.foo.bar == ")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteByte('\n')
	}
	return MustParseModule(sb.String()).Rules
}

func nakedRefRules(n int) []*Rule {
	var sb strings.Builder
	sb.WriteString("package p\n\n")
	for i := range n {
		sb.WriteString(`allow if input.foo["`)
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(`"]`)
		sb.WriteByte('\n')
	}
	return MustParseModule(sb.String()).Rules
}

func nonsensicalRules(n int) []*Rule {
	var sb strings.Builder
	sb.WriteString("package p\n\n")
	for i := range n {
		si := strconv.Itoa(i)
		sb.WriteString(`allow if input["`)
		sb.WriteString(si)
		sb.WriteString(`"] = `)
		sb.WriteString(si)
		sb.WriteByte('\n')
	}
	return MustParseModule(sb.String()).Rules
}
