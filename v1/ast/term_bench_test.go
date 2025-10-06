// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package ast

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/util"
)

var (
	str string
	bs  []byte
)

func BenchmarkObjectLookup(b *testing.B) {
	sizes := []int{5, 50, 500, 5000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			obj := NewObject()
			for i := range n {
				obj.Insert(StringTerm(strconv.Itoa(i)), InternedTerm(i))
			}
			key := StringTerm(strconv.Itoa(n - 1))
			b.ResetTimer()
			for range b.N {
				value := obj.Get(key)
				if value == nil {
					b.Fatal("expected hit")
				}
			}
		})
	}
}

func BenchmarkObjectFind(b *testing.B) {
	sizes := []int{5, 50, 500, 5000}
	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmt.Sprintf("%d_%d", n, m), func(b *testing.B) {
				obj := NewObject()
				for i := range n {
					arr := NewArray()
					for j := range m {
						arr = arr.Append(IntNumberTerm(j))
					}
					obj.Insert(StringTerm(strconv.Itoa(i)), NewTerm(arr))
				}
				key := Ref{StringTerm(strconv.Itoa(n - 1)), IntNumberTerm(m - 1)}
				b.ResetTimer()
				for range b.N {
					value, err := obj.Find(key)
					if err != nil {
						b.Fatal(err)
					}
					if value == nil {
						b.Fatal("expected hit")
					}
				}
			})
		}
	}
}

func BenchmarkObjectInsert(b *testing.B) {
	nums := make([]*Term, 0, 100)
	for i := range 100 {
		nums = append(nums, InternedTerm(i))
	}

	b.Run("existing key and value", func(b *testing.B) {
		obj := newobject(0)
		obj.Insert(nums[0], nums[0])

		for range b.N {
			for range nums {
				obj.Insert(nums[0], nums[0])
			}
		}
	})

	b.Run("existing key, new value", func(b *testing.B) {
		obj := newobject(0)
		obj.Insert(nums[0], StringTerm("foo"))

		for range b.N {
			for i := range nums {
				obj.Insert(nums[0], nums[i])
			}
		}
	})

	b.Run("new key", func(b *testing.B) {
		obj := newobject(0)

		for range b.N {
			for i := range nums {
				obj.Insert(nums[i], nums[0])
				if i >= len(nums)-1 {
					reset(obj)
				}
			}
		}
	})

	b.Run("new key, new value", func(b *testing.B) {
		obj := newobject(0)

		for range b.N {
			for i := range nums {
				obj.Insert(nums[i], nums[i])
				if i >= len(nums)-1 {
					reset(obj)
				}
			}
		}
	})
}

func BenchmarkObjectCreationAndLookup(b *testing.B) {
	sizes := []int{5, 50, 500, 5000, 50000, 500000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			obj := NewObject()
			for i := range n {
				obj.Insert(StringTerm(strconv.Itoa(i)), IntNumberTerm(i))
			}
			key := StringTerm(strconv.Itoa(n - 1))
			for range b.N {
				value := obj.Get(key)
				if value == nil {
					b.Fatal("expected hit")
				}
			}
		})
	}
}

func BenchmarkLazyObjectLookup(b *testing.B) {
	sizes := []int{5, 50, 500, 5000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			data := make(map[string]any, n)
			for i := range n {
				data[strconv.Itoa(i)] = i
			}
			obj := LazyObject(data)
			key := StringTerm(strconv.Itoa(n - 1))
			b.ResetTimer()
			for range b.N {
				value := obj.Get(key)
				if value == nil {
					b.Fatal("expected hit")
				}
			}
		})
	}
}

func BenchmarkLazyObjectFind(b *testing.B) {
	sizes := []int{5, 50, 500, 5000}
	for _, n := range sizes {
		for _, m := range sizes {
			b.Run(fmt.Sprintf("%d_%d", n, m), func(b *testing.B) {
				data := make(map[string]any, n)
				for i := range n {
					arr := make([]string, 0, m)
					for j := range m {
						arr = append(arr, strconv.Itoa(j))
					}
					data[strconv.Itoa(i)] = arr
				}
				obj := LazyObject(data)
				key := Ref{StringTerm(strconv.Itoa(n - 1)), IntNumberTerm(m - 1)}
				b.ResetTimer()
				for range b.N {
					value, err := obj.Find(key)
					if err != nil {
						b.Fatal(err)
					}
					if value == nil {
						b.Fatal("expected hit")
					}
				}
			})
		}
	}
}

func BenchmarkSetCreationAndLookup(b *testing.B) {
	sizes := []int{5, 50, 500, 5000, 50000, 500000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			set := NewSet()
			for i := range n {
				set.Add(StringTerm(strconv.Itoa(i)))
			}
			key := StringTerm(strconv.Itoa(n - 1))
			for range b.N {
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
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			setA := NewSet()
			setB := NewSet()
			for i := range n {
				setA.Add(IntNumberTerm(i))
				setB.Add(IntNumberTerm(i))
			}
			b.ResetTimer()
			for range b.N {
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
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			setA := NewSet()
			setB := NewSet()
			for i := range n {
				setA.Add(IntNumberTerm(i))
			}
			for i := range sizes[0] {
				setB.Add(IntNumberTerm(i))
			}
			setB.Add(IntNumberTerm(-1))
			b.ResetTimer()
			for range b.N {
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
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			setA := NewSet()
			for i := range n {
				setA.Add(IntNumberTerm(i))
			}
			key := IntNumberTerm(n - 1)
			b.ResetTimer()
			for range b.N {
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
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			s := String(strings.Repeat("a", n))
			b.ResetTimer()
			for range b.N {
				_ = s.Hash()
			}
		})
	}
}

// BenchmarkObjectString generates several objects of different sizes, and
// marshals them to JSON via two ways:
//
//	map[string]int -> ast.Value -> .String()
//
// and
//
//	map[string]int -> json.Marshal()
//
// The difference between these two is relevant for feeding input into the
// wasm vm: when calling rego.New(...) with rego.Target("wasm"), it's up to
// the caller to provide the input in parsed form (ast.Value), or
// raw (any).
func BenchmarkObjectString(b *testing.B) {
	var err error
	sizes := []int{5, 50, 500, 5000}

	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			obj := map[string]int{}
			for i := range n {
				obj[strconv.Itoa(i)] = i
			}
			val := MustInterfaceToValue(obj)

			b.Run("String()", func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
					str = val.String()
				}
			})
			b.Run("json.Marshal", func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
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
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			obj := map[string]int{}
			for i := range n {
				obj[strconv.Itoa(i)] = i
			}
			valString := MustInterfaceToValue(obj)
			valJSON := MustInterfaceToValue(obj)

			b.Run("String()", func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
					str = valString.String()
				}
			})
			b.Run("json.Marshal", func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
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
			b.Run(strconv.Itoa(n), func(b *testing.B) {
				es := []struct{ k, v int }{}
				for i := range n {
					es = append(es, struct{ k, v int }{i, i})
				}
				r := rand.New(rand.NewSource(seed)) // Seed the PRNG.
				r.Shuffle(len(es), func(i, j int) { es[i], es[j] = es[j], es[i] })
				b.ResetTimer()
				for range b.N {
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
			b.Run(strconv.Itoa(n), func(b *testing.B) {
				es := []struct{ k, v int }{}
				for v := range n {
					es = append(es, struct{ k, v int }{v, v})
				}
				b.ResetTimer()
				for range b.N {
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
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			obj := make([]string, n)
			for i := range n {
				obj[i] = strconv.Itoa(i)
			}
			val := MustInterfaceToValue(obj)

			b.Run("String()", func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
					str = val.String()
				}
			})
			b.Run("json.Marshal", func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
					bs, err = json.Marshal(obj)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// This was used primarily to test the performance of the Equal method using the
// current implementation vs that of the previous implementation, which simply called
// the Compare function to test for equality (== 0). This was about as fast as the current
// implementation when both arrays were equal, but significantly slower when they
// were not (135 nanoseconds for the old implementation vs 4 nanoseconds now).
func BenchmarkArrayEquality(b *testing.B) {
	sizes := []int{5, 50, 500, 5000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			arrA := NewArray()
			arrB := NewArray()
			for i := range n {
				arrA = arrA.Append(IntNumberTerm(i))
				arrB = arrB.Append(IntNumberTerm(i))
			}
			// make sure the arrays are not equal
			arrB = arrB.Append(IntNumberTerm(10000))
			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				if arrA.Equal(arrB) {
					b.Fatal("expected not equal")
				}
			}
		})
	}
}

func BenchmarkSetString(b *testing.B) {
	sizes := []int{5, 50, 500, 5000, 50000}

	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			val := NewSet()
			for i := range n {
				val.Add(IntNumberTerm(i))
			}

			b.Run("String()", func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
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
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			set := NewSet()
			for i := range n {
				set.Add(StringTerm(strconv.Itoa(i)))
			}

			b.Run("json.Marshal", func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
					bs, err = json.Marshal(set)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

func BenchmarkIsVarCompatibleString(b *testing.B) {
	tests := map[string]bool{
		"hello":    true,
		"5heel":    false,
		"h\nllo":   false,
		"h\tllo":   false,
		"h\x00llo": false,
		"h\"llo":   false,
		"h\\llo":   false,
		"":         false,
	}

	for _, name := range util.KeysSorted(tests) {
		exp := tests[name]
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				if IsVarCompatibleString(name) != exp {
					b.Fatalf("expected %t but got %t", exp, !exp)
				}
			}
		})
	}
}

// BenchmarkRefString benchmarks the performance of Ref.String().
func BenchmarkRefString(b *testing.B) {
	tests := map[string]struct {
		inp string
		exp string
		ref Ref
	}{
		"simple ref": {
			inp: `data.policy["main"]`,
			exp: `data.policy.main`,
		},
		"scalars ref": {
			inp: `data.policy[1234].test1[true].test2[null].test3[3.14]`,
			exp: `data.policy[1234].test1[true].test2[null].test3[3.14]`,
		},
		"with escape": {
			inp: `data.policy["ma\tin"]`,
			exp: `data.policy["ma\tin"]`,
		},
		"really long": {
			inp: `data.policy.test1.test2.test3.test4["main1"]["main2"]["main3"]["main4"]`,
			exp: `data.policy.test1.test2.test3.test4.main1.main2.main3.main4`,
		},
		"var term": {
			ref: Ref{VarTerm(`is_object`)},
			exp: `is_object`,
		},
		"dot builtin": {
			inp: `io.jwt.decode`,
			exp: `io.jwt.decode`,
		},
	}

	for _, name := range util.KeysSorted(tests) {
		tc := tests[name]
		if tc.ref == nil {
			tc.ref = MustParseRef(tc.inp)
		}
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				if tc.ref.String() != tc.exp {
					b.Fatalf("expected %s but got %s", tc.exp, tc.ref.String())
				}
			}
		})
	}
}

func reset(obj *object) {
	clear(obj.elems)
	clear(obj.keys)
	obj.keys = obj.keys[:0]
	obj.ground = 0
	obj.hash = 0
	obj.sortGuard = sync.Once{}
}

func BenchmarkInterfaceToValueInt(b *testing.B) {
	// 0 allocs
	b.Run("interned int value", func(b *testing.B) {
		exp := newIntNumberValue(123)
		var act Value

		for b.Loop() {
			act = MustInterfaceToValue(123)
		}

		if exp.Compare(act) != 0 {
			b.Fatalf("expected %v but got %v", exp, act)
		}
	})

	// 2 allocs
	b.Run("non-interned int value", func(b *testing.B) {
		exp := newIntNumberValue(12345)
		var act Value

		for b.Loop() {
			act = MustInterfaceToValue(12345)
		}

		if exp.Compare(act) != 0 {
			b.Fatalf("expected %v but got %v", exp, act)
		}
	})
}
