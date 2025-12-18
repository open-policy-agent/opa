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
			for b.Loop() {
				value := obj.Get(key)
				if value == nil {
					b.Fatal("expected hit")
				}
			}
		})
	}
}

// Before NumberCompare refactor:
// // --- FAIL: BenchmarkObjectGet/existing_float_number_key_as_int
//     /Users/anderseknert/git/opa/opa/v1/ast/term_bench_test.go:111: expected hit

// BenchmarkObjectGet/lookup_in_empty_object-16         	219916140	         5.323 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/existing_interned_key-16          	149059920	         8.011 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/existing_string_key-16            	144672567	         8.314 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/existing_int_number_key-16        	 62073110	         17.62 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/existing_int_number_key_as_float-16     2310716	         519.5 ns/op	     504 B/op	      24 allocs/op
// BenchmarkObjectGet/existing_float_key-16                   1966604	         611.5 ns/op	     632 B/op	      29 allocs/op
// BenchmarkObjectGet/missing_string_key-16                 164003106	         7.293 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/missing_int_number_key-16              74759754	         15.25 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/missing_float_key-16                    4059250	         295.8 ns/op	     296 B/op	      15 allocs/op

// After NumberCompare refactor:
// BenchmarkObjectGet/lookup_in_empty_object-16         	680466268	         1.767 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/existing_interned_key-16          	263787909	         4.498 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/existing_string_key-16            	156048259	         7.646 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/existing_int_number_key-16        	 99076318	         12.40 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/existing_float_number_key_as_int-16    98104674	         12.47 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/existing_int_number_key_as_float-16    53441701	         22.83 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/existing_float_key-16                  53429703	         20.98 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/missing_string_key-16                	193661902	         6.084 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/missing_int_number_key-16             156364982	         7.695 ns/op	       0 B/op	       0 allocs/op
// BenchmarkObjectGet/missing_float_key-16                   67888981	         16.26 ns/op	       0 B/op	       0 allocs/op

// But do note that this was not done to improve performance, but to improve our code. The faster float comparisons
// are a nice side effect.

func BenchmarkObjectGet(b *testing.B) {
	obj := NewObject(
		Item(InternedTerm("env"), InternedTerm("production")), // known interned string key
		Item(StringTerm("a"), InternedTerm(1)),
		Item(IntNumberTerm(222), InternedTerm("b")),
		Item(NumberTerm("3.14"), InternedTerm("c")),
		Item(NumberTerm("2.0"), InternedTerm("d")),
	)

	empty := NewObject()
	b.Run("lookup in empty object", func(b *testing.B) {
		key := StringTerm("a")
		for b.Loop() {
			if empty.Get(key) != nil {
				b.Fatal("expected miss")
			}
		}
	})

	b.Run("existing interned key", func(b *testing.B) {
		key := InternedTerm("env")
		for b.Loop() {
			if obj.Get(key) == nil {
				b.Fatal("expected hit")
			}
		}
	})

	b.Run("existing string key", func(b *testing.B) {
		key := StringTerm("a")
		for b.Loop() {
			if obj.Get(key) == nil {
				b.Fatal("expected hit")
			}
		}
	})

	b.Run("existing int number key", func(b *testing.B) {
		key := IntNumberTerm(222)
		for b.Loop() {
			if obj.Get(key) == nil {
				b.Fatal("expected hit")
			}
		}
	})

	b.Run("existing float number key as int", func(b *testing.B) {
		key := IntNumberTerm(2)
		for b.Loop() {
			if obj.Get(key) == nil {
				b.Fatal("expected hit")
			}
		}
	})

	b.Run("existing int number key as float", func(b *testing.B) {
		key := NumberTerm("222.0")
		for b.Loop() {
			if obj.Get(key) == nil {
				b.Fatal("expected hit")
			}
		}
	})

	b.Run("existing float key", func(b *testing.B) {
		key := NumberTerm("3.14")
		for b.Loop() {
			if obj.Get(key) == nil {
				b.Fatal("expected hit")
			}
		}
	})

	b.Run("missing string key", func(b *testing.B) {
		key := StringTerm("missing")
		for b.Loop() {
			if obj.Get(key) != nil {
				b.Fatal("expected miss")
			}
		}
	})

	b.Run("missing int number key", func(b *testing.B) {
		key := IntNumberTerm(999)
		for b.Loop() {
			if obj.Get(key) != nil {
				b.Fatal("expected miss")
			}
		}
	})

	b.Run("missing float key", func(b *testing.B) {
		key := NumberTerm("9.99")
		for b.Loop() {
			if obj.Get(key) != nil {
				b.Fatal("expected miss")
			}
		}
	})
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
				for b.Loop() {
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

		for b.Loop() {
			for range nums {
				obj.Insert(nums[0], nums[0])
			}
		}
	})

	b.Run("existing key, new value", func(b *testing.B) {
		obj := newobject(0)
		obj.Insert(nums[0], StringTerm("foo"))

		for b.Loop() {
			for i := range nums {
				obj.Insert(nums[0], nums[i])
			}
		}
	})

	b.Run("new key", func(b *testing.B) {
		obj := newobject(0)

		for b.Loop() {
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

		for b.Loop() {
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
			for b.Loop() {
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
			for b.Loop() {
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
				for b.Loop() {
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
			for b.Loop() {
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
			for b.Loop() {
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
			for b.Loop() {
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
			for b.Loop() {
				if !setA.Contains(key) {
					b.Fatal("expected hit")
				}
			}
		})
	}
}

// 241.9 ns/op	     472 B/op	      10 allocs/op
// 207.7 ns/op	     424 B/op	       9 allocs/op
func BenchmarkSetCopy(b *testing.B) {
	s := NewSet(InternedTerm(1), InternedTerm(2), InternedTerm(3), InternedTerm(4), InternedTerm(5))
	for b.Loop() {
		_ = s.Copy()
	}
}

// 396.9 ns/op	     664 B/op	      19 allocs/op
func BenchmarkObjectCopy(b *testing.B) {
	o := NewObject(
		Item(InternedTerm("a"), InternedTerm(1)),
		Item(InternedTerm("b"), InternedTerm(2)),
		Item(InternedTerm("c"), InternedTerm(3)),
		Item(InternedTerm("d"), InternedTerm(4)),
		Item(InternedTerm("e"), InternedTerm(5)),
	)
	for b.Loop() {
		_ = o.Copy()
	}
}

func BenchmarkTermHashing(b *testing.B) {
	sizes := []int{10, 100, 1000}
	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			s := String(strings.Repeat("a", n))
			b.ResetTimer()
			for b.Loop() {
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
				for b.Loop() {
					str = val.String()
				}
			})
			b.Run("json.Marshal", func(b *testing.B) {
				b.ResetTimer()
				for b.Loop() {
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
				for b.Loop() {
					str = valString.String()
				}
			})
			b.Run("json.Marshal", func(b *testing.B) {
				b.ResetTimer()
				for b.Loop() {
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
				for b.Loop() {
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
				for b.Loop() {
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
				for b.Loop() {
					str = val.String()
				}
			})
			b.Run("json.Marshal", func(b *testing.B) {
				b.ResetTimer()
				for b.Loop() {
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
			for b.Loop() {
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
				for b.Loop() {
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
				for b.Loop() {
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

// without_conflict   258.8 ns/op     440 B/op      11 allocs/op // use NewObject
// with_conflict      290.6 ns/op     440 B/op      11 allocs/op //
// without_conflict   220.5 ns/op     408 B/op       9 allocs/op // use newobject with size
// with_conflict      231.5 ns/op     424 B/op       9 allocs/op //
func BenchmarkObjectMergeWith(b *testing.B) {
	o1 := NewObject(InternedItem("a", 1), InternedItem("b", 2))
	overwrite := func(_, v2 *Term) (*Term, bool) {
		return v2, false
	}

	b.Run("without conflict", func(b *testing.B) {
		o2 := NewObject(InternedItem("c", 2), InternedItem("d", 3))

		for b.Loop() {
			_, _ = o1.MergeWith(o2, overwrite)
		}
	})

	b.Run("with conflict", func(b *testing.B) {
		o2 := NewObject(InternedItem("a", 2), InternedItem("c", 3), InternedItem("d", 4))

		for b.Loop() {
			_, _ = o1.MergeWith(o2, overwrite)
		}
	})
}

// 17.88 ns/op       0 B/op       0 allocs/op
func BenchmarkConstantPrefix(b *testing.B) {
	ref := MustParseRef("data.example.internal[foo].bar")
	exp := MustParseRef("data.example.internal")

	for b.Loop() {
		prefix := ref.ConstantPrefix()
		if !prefix.Equal(exp) {
			b.Fatalf("expected %v but got %v", exp, prefix)
		}
	}
}

// 14.30 ns/op       0 B/op       0 allocs/op
func BenchmarkStringPrefix(b *testing.B) {
	ref := MustParseRef("data.example.internal[foo].bar")
	exp := MustParseRef("data.example.internal")

	for b.Loop() {
		prefix := ref.StringPrefix()
		if !prefix.Equal(exp) {
			b.Fatalf("expected %v but got %v", exp, prefix)
		}
	}
}

// 17.07 ns/op       0 B/op       0 allocs/op
func BenchmarkGroundPrefix(b *testing.B) {
	ref := MustParseRef("data.example.internal[foo].bar")
	exp := MustParseRef("data.example.internal")

	for b.Loop() {
		prefix := ref.GroundPrefix()
		if !prefix.Equal(exp) {
			b.Fatalf("expected %v but got %v", exp, prefix)
		}
	}
}

// BenchmarkPtr/with_escape-16         	11040984       109.5 ns/op     96 B/op       3 allocs/op // strings.Join
// BenchmarkPtr/without_escape-16      	13879591        91.02 ns/op    88 B/op       2 allocs/op
// BenchmarkPtr/with_escape-16         	12612588        88.95 ns/op    32 B/op       2 allocs/op // strings.Builder
// BenchmarkPtr/without_escape-16      	19047051        64.84 ns/op    24 B/op       1 allocs/op
func BenchmarkPtr(b *testing.B) {
	b.Run("with escape", func(b *testing.B) {
		ref := MustParseRef("data.example[\"e~s/c\"].foo.bar")
		exp := "example/e~s%2Fc/foo/bar"

		for b.Loop() {
			ptr, err := ref.Ptr()
			if err != nil {
				b.Fatal(err)
			}
			if ptr != exp {
				b.Fatalf("expected %v but got %v", exp, ptr)
			}
		}
	})

	b.Run("without escape", func(b *testing.B) {
		ref := MustParseRef("data.example.internal.foo.bar")
		exp := "example/internal/foo/bar"

		for b.Loop() {
			ptr, err := ref.Ptr()
			if err != nil {
				b.Fatal(err)
			}
			if ptr != exp {
				b.Fatalf("expected %v but got %v", exp, ptr)
			}
		}
	})
}
