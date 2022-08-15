// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package ast

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func BenchmarkObjectLookup(b *testing.B) {
	sizes := []int{5, 50, 500, 5000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			obj := NewObject()
			for i := 0; i < n; i++ {
				obj.Insert(StringTerm(fmt.Sprint(i)), IntNumberTerm(i))
			}
			key := StringTerm(fmt.Sprint(n - 1))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				value := obj.Get(key)
				if value == nil {
					b.Fatal("expected hit")
				}
			}
		})
	}
}

func BenchmarkObjectCreationAndLookup(b *testing.B) {
	sizes := []int{5, 50, 500, 5000, 50000, 500000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			obj := NewObject()
			for i := 0; i < n; i++ {
				obj.Insert(StringTerm(fmt.Sprint(i)), IntNumberTerm(i))
			}
			key := StringTerm(fmt.Sprint(n - 1))
			for i := 0; i < b.N; i++ {
				value := obj.Get(key)
				if value == nil {
					b.Fatal("expected hit")
				}
			}
		})
	}
}

func BenchmarkSetCreationAndLookup(b *testing.B) {
	sizes := []int{5, 50, 500, 5000, 50000, 500000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			set := NewSet()
			for i := 0; i < n; i++ {
				set.Add(StringTerm(fmt.Sprint(i)))
			}
			key := StringTerm(fmt.Sprint(n - 1))
			for i := 0; i < b.N; i++ {
				present := set.Contains(key)
				if !present {
					b.Fatal("expected hit")
				}
			}
		})
	}
}

func BenchmarkSetIntersection(b *testing.B) {
	sizes := []int{5, 50, 500, 5000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			setA := NewSet()
			setB := NewSet()
			for i := 0; i < n; i++ {
				setA.Add(IntNumberTerm(i))
				setB.Add(IntNumberTerm(i))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				setC := setA.Intersect(setB)
				if setC.Len() != setA.Len() || setC.Len() != setB.Len() {
					b.Fatal("expected equal")
				}
			}
		})
	}
}

func BenchmarkSetIntersectionDifferentSize(b *testing.B) {
	sizes := []int{4, 50, 500, 5000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			setA := NewSet()
			setB := NewSet()
			for i := 0; i < n; i++ {
				setA.Add(IntNumberTerm(i))
			}
			for i := 0; i < sizes[0]; i++ {
				setB.Add(IntNumberTerm(i))
			}
			setB.Add(IntNumberTerm(-1))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				setC := setA.Intersect(setB)
				if setC.Len() != sizes[0] {
					b.Fatal("expected size to be equal")
				}
			}
		})
	}
}

func BenchmarkSetMembership(b *testing.B) {
	sizes := []int{5, 50, 500, 5000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			setA := NewSet()
			for i := 0; i < n; i++ {
				setA.Add(IntNumberTerm(i))
			}
			key := IntNumberTerm(n - 1)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if !setA.Contains(key) {
					b.Fatal("expected hit")
				}
			}
		})
	}
}

func BenchmarkTermHashing(b *testing.B) {
	sizes := []int{10, 100, 1000}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s := String(strings.Repeat("a", n))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = s.Hash()
			}
		})
	}
}

var str string
var bs []byte

// BenchmarkObjectString generates several objects of different sizes, and
// marshals them to JSON via two ways:
//   map[string]int -> ast.Value -> .String()
// and
//   map[string]int -> json.Marshal()
//
// The difference between these two is relevant for feeding input into the
// wasm vm: when calling rego.New(...) with rego.Target("wasm"), it's up to
// the caller to provide the input in parsed form (ast.Value), or
// raw (interface{}).
func BenchmarkObjectString(b *testing.B) {
	var err error
	sizes := []int{5, 50, 500, 5000}

	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {

			obj := map[string]int{}
			for i := 0; i < n; i++ {
				obj[fmt.Sprint(i)] = i
			}
			val := MustInterfaceToValue(obj)

			b.Run("String()", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					str = val.String()
				}
			})
			b.Run("json.Marshal", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					bs, err = json.Marshal(obj)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// This benchmark works similarly to BenchmarkObjectString, but with a key
// difference: it benchmarks the String and MarshalJSON interface functions
// for the Objec, instead of the underlying data structure. This ensures
// that we catch the full performance properties of Object's implementation.
func BenchmarkObjectStringInterfaces(b *testing.B) {
	var err error
	sizes := []int{5, 50, 500, 5000, 50000}

	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {

			obj := map[string]int{}
			for i := 0; i < n; i++ {
				obj[fmt.Sprint(i)] = i
			}
			valString := MustInterfaceToValue(obj)
			valJSON := MustInterfaceToValue(obj)

			b.Run("String()", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					str = valString.String()
				}
			})
			b.Run("json.Marshal", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					bs, err = json.Marshal(valJSON)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

func BenchmarkObjectConstruction(b *testing.B) {
	sizes := []int{5, 50, 500, 5000, 50000, 500000}
	seed := time.Now().UnixNano()

	b.Run("shuffled keys", func(b *testing.B) {
		for _, n := range sizes {
			b.Run(fmt.Sprint(n), func(b *testing.B) {
				es := []struct{ k, v int }{}
				for i := 0; i < n; i++ {
					es = append(es, struct{ k, v int }{i, i})
				}
				rand.Seed(seed)
				rand.Shuffle(len(es), func(i, j int) { es[i], es[j] = es[j], es[i] })
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					obj := NewObject()
					for _, e := range es {
						obj.Insert(IntNumberTerm(e.k), IntNumberTerm(e.v))
					}
				}
			})
		}
	})
	b.Run("increasing keys", func(b *testing.B) {
		for _, n := range sizes {
			b.Run(fmt.Sprint(n), func(b *testing.B) {
				es := []struct{ k, v int }{}
				for v := 0; v < n; v++ {
					es = append(es, struct{ k, v int }{v, v})
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					obj := NewObject()
					for _, e := range es {
						obj.Insert(IntNumberTerm(e.k), IntNumberTerm(e.v))
					}
				}
			})
		}
	})
}

// BenchmarkArrayString compares the performance characteristics of
// (ast.Value).String() with the stdlib-native json.Marshal. See
// BenchmarkObjectString above for details.
func BenchmarkArrayString(b *testing.B) {
	var err error
	sizes := []int{5, 50, 500, 5000}

	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {

			obj := make([]string, n)
			for i := 0; i < n; i++ {
				obj[i] = fmt.Sprint(i)
			}
			val := MustInterfaceToValue(obj)

			b.Run("String()", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					str = val.String()
				}
			})
			b.Run("json.Marshal", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					bs, err = json.Marshal(obj)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

func BenchmarkSetString(b *testing.B) {
	sizes := []int{5, 50, 500, 5000, 50000}

	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			val := NewSet()
			for i := 0; i < n; i++ {
				val.Add(IntNumberTerm(i))
			}

			b.Run("String()", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					str = val.String()
				}
			})
		})
	}
}

func BenchmarkSetMarshalJSON(b *testing.B) {
	var err error
	sizes := []int{5, 50, 500, 5000, 50000}

	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			set := NewSet()
			for i := 0; i < n; i++ {
				set.Add(StringTerm(fmt.Sprint(i)))
			}

			b.Run("json.Marshal", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					bs, err = json.Marshal(set)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}

}
