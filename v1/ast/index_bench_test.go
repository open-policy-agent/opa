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

// Index build for rules that each read a ref of their own.
//
// Cost reduced by two thirds by lazy init of scalars map. Then linear in memory
// rather than quadratic, once a rule path stopped being padded out to the full
// depth with a node per level it does not constrain (see insertPath):
//
//	          before                       after
//	 1000    46.3ms  128651465 B/op      10.7ms    779466 B/op
//	10000        --   12.8GB (est.)     976.0ms   7345864 B/op
//
// n=10000 used to be out of reach -- the comment here read "capped at 4 because at
// 5 this allocates 12.8GB of memory, which our CI runner won't be happy about" --
// and now costs 7MB. Build time is still quadratic, since a path is walked as far
// as its last constrained level, so this stays the largest size worth running; the
// 10000 row is a single iteration, the rest are the default benchtime.
func BenchmarkBuildNakedRefIndex(b *testing.B) {
	for i := range 5 {
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

// Non-sensical index build where each ref+value is unique.
//
//	             before                              after
//	   10       8766 ns      20984 B/op        13875 ns      9496 B/op
//	  100     502340 ns    1360788 B/op       141875 ns     93608 B/op
//	 1000   46709530 ns  128900337 B/op      8614041 ns   1027464 B/op
//	10000 5349418417 ns 12808545832 B/op    833599250 ns   9825864 B/op
//
// The before column is what this recorded while every rule path was padded out to
// the full depth; n=10000 was left out of the run because 12.8GB of it would not
// fit on a CI runner. See insertPath.
func BenchmarkBuildNonsensicalIndex(b *testing.B) {
	for i := range 5 {
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

// Lookup where every rule constrains a ref of its own and every one of them is
// satisfied, so traversal descends into all of their branches. That is what makes
// the trie's shape decide the cost: padding each rule's path out to the full depth
// gave every one of them a private copy of the tail, and a walk that reaches all
// of those copies grew with n^2.
//
// The two neighbouring lookup benchmarks miss this on purpose-built inputs:
// BenchmarkLookupEqIndex has every rule on one ref, so there is one level to walk,
// and BenchmarkLookupNakedRefIndex leaves all but one ref absent, so traversal
// takes a single branch per level. Both are linear whatever the trie looks like.
func BenchmarkLookupDistinctRefIndex(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			index := newBaseDocEqIndex(isVirtual)
			if !index.Build(distinctRefRules(n)) {
				b.Fatal("failed to build index")
			}
			input := inputResolver{input: distinctRefInput(n)}

			// Nothing here discriminates -- every rule reads a ref of its own and
			// all of them hold -- so the whole ruleset is a candidate. The index
			// cannot narrow this; the point is what it costs to find that out.
			b.ResetTimer()
			for b.Loop() {
				res, err := index.Lookup(input)
				if err != nil {
					b.Fatal(err)
				} else if len(res.Rules) != n {
					b.Fatalf("expected %d rules, got %d", n, len(res.Rules))
				}
				IndexResultPool.Put(res)
			}
		})
	}
}

// distinctRefRules returns n rules that each compare a ref of their own.
func distinctRefRules(n int) []*Rule {
	var sb strings.Builder
	sb.WriteString("package p\n\n")
	for i := range n {
		sb.WriteString(`allow if input.f`)
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(` == "x"`)
		sb.WriteByte('\n')
	}
	return MustParseModule(sb.String()).Rules
}

// distinctRefInput defines every ref the rules read, and satisfies all of them,
// so traversal descends into every rule's branch rather than stopping early.
func distinctRefInput(n int) Value {
	var sb strings.Builder
	sb.WriteByte('{')
	for i := range n {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteByte('"')
		sb.WriteString("f" + strconv.Itoa(i))
		sb.WriteString(`": `)
		sb.WriteString(`"x"`)
	}
	sb.WriteByte('}')
	return MustParseTerm(sb.String()).Value
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
