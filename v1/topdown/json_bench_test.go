// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func BenchmarkJSONRemoveArray(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 5000} {
		b.Run(fmt.Sprintf("size=%d", n), func(b *testing.B) {
			// Create an object wrapping the array: {"a": [0, 1, ...]}
			arr := ast.NewArray(slices.Collect(ast.InternedIntRange(0, n))...)
			obj := ast.NewObject([2]*ast.Term{ast.InternedTerm("a"), ast.NewTerm(arr)})

			// Remove something inside the array to force traversal.
			paths := ast.NewSet(ast.InternedTerm("a/nonexistent"))

			operands := []*ast.Term{ast.NewTerm(obj), ast.NewTerm(paths)}
			bctx := BuiltinContext{Context: b.Context()}
			iter := func(*ast.Term) error { return nil }

			b.ResetTimer()
			for b.Loop() {
				if err := builtinJSONRemove(bctx, operands, iter); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkJSONFilterArray(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 5000} {
		b.Run(fmt.Sprintf("size=%d", n), func(b *testing.B) {
			// Create an object with n keys.
			obj := ast.NewObjectWithCapacity(n)
			pathSlice := make([]*ast.Term, n)
			for i := range n {
				k := ast.StringTerm("k" + strconv.Itoa(i))
				obj.Insert(k, ast.InternedTerm(i))
				pathSlice[i] = k
			}
			// Filter all keys: json.filter(obj, ["k0", "k1", ...]).
			// This stresses pathsToObject (creating the filter mask).
			paths := ast.NewSet(pathSlice...)

			operands := []*ast.Term{ast.NewTerm(obj), ast.NewTerm(paths)}
			bctx := BuiltinContext{Context: b.Context()}
			iter := func(*ast.Term) error { return nil }

			b.ResetTimer()
			for b.Loop() {
				if err := builtinJSONFilter(bctx, operands, iter); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkJSONFilterArrayIndices(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 5000} {
		b.Run(fmt.Sprintf("size=%d", n), func(b *testing.B) {
			// Create an object wrapping an array: {"a": [0, 1, ...]}.
			arr := ast.NewArray(slices.Collect(ast.InternedIntRange(0, n))...)
			obj := ast.NewObject([2]*ast.Term{ast.StringTerm("a"), ast.NewTerm(arr)})

			// Filter to keep the first half of the array elements:
			// json.filter(obj, ["a/0", "a/1", ... "a/n/2"]).
			filterSize := max(n/2, 1)
			pathSlice := make([]*ast.Term, filterSize)
			for i := range filterSize {
				pathSlice[i] = ast.StringTerm("a/" + strconv.Itoa(i))
			}
			paths := ast.NewSet(pathSlice...)

			operands := []*ast.Term{ast.NewTerm(obj), ast.NewTerm(paths)}
			bctx := BuiltinContext{Context: b.Context()}
			iter := func(*ast.Term) error { return nil }

			b.ResetTimer()
			for b.Loop() {
				if err := builtinJSONFilter(bctx, operands, iter); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkJSONPatchAddShallowScalar(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	maxN := slices.Max(sizes)

	// Pre-build the largest patch lists once; sub-benchmarks slice into them.
	// Cheap to keep alive between runs since patches are just term pointers.
	objArrPatches := make([]*ast.Term, maxN)
	setPatches := make([]*ast.Term, maxN)
	for i := range maxN {
		path := ast.StringTerm("/" + strconv.Itoa(i))
		value := ast.InternedTerm(i)
		objArrPatches[i] = createPatch("add", path, nil, value)
		setPatches[i] = createPatch("add", ast.ArrayTerm(value), nil, value)
	}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("object-%d", n), func(b *testing.B) {
			runJSONPatchBenchmarkTest(b, genTestObject(n), ast.NewArray(objArrPatches[:n]...))
		})
	}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("array-%d", n), func(b *testing.B) {
			source := ast.NewArray(slices.Collect(ast.InternedIntRange(0, n))...)
			runJSONPatchBenchmarkTest(b, source, ast.NewArray(objArrPatches[:n]...))
		})
	}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("set-%d", n), func(b *testing.B) {
			source := ast.NewSet(slices.Collect(ast.InternedIntRange(0, n))...)
			runJSONPatchBenchmarkTest(b, source, ast.NewArray(setPatches[:n]...))
		})
	}
}

func BenchmarkJSONPatchAddShallowComposite(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, n := range sizes {
		source := genTestObject(n)
		for _, m := range sizes {
			b.Run(fmt.Sprintf("object-%d-%d", n, m), func(b *testing.B) {
				patches := buildAddCompositePatches(n, m)
				runJSONPatchBenchmarkTest(b, source, patches)
			})
		}
	}

	for _, n := range sizes {
		source := ast.NewArray(slices.Collect(ast.InternedIntRange(0, n))...)
		for _, m := range sizes {
			b.Run(fmt.Sprintf("array-%d-%d", n, m), func(b *testing.B) {
				patches := buildAddCompositePatches(n, m)
				runJSONPatchBenchmarkTest(b, source, patches)
			})
		}
	}

	for _, n := range sizes {
		source := ast.NewSet(slices.Collect(ast.InternedIntRange(0, n))...)
		for _, m := range sizes {
			b.Run(fmt.Sprintf("set-%d-%d", n, m), func(b *testing.B) {
				patches := buildSetAddCompositePatches(n, m)
				runJSONPatchBenchmarkTest(b, source, patches)
			})
		}
	}
}

func BenchmarkJSONPatchAddRemove(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, n := range sizes {
		source := genTestObject(n)
		for _, m := range sizes {
			b.Run(fmt.Sprintf("object-%d-%d", n, m), func(b *testing.B) {
				patches := buildAddRemoveScalarPatches(n, m)
				runJSONPatchBenchmarkTest(b, source, patches)
			})
		}
	}

	for _, n := range sizes {
		source := ast.NewArray(slices.Collect(ast.InternedIntRange(0, n))...)
		for _, m := range sizes {
			b.Run(fmt.Sprintf("array-%d-%d", n, m), func(b *testing.B) {
				patches := buildAddRemoveScalarPatches(n, m)
				runJSONPatchBenchmarkTest(b, source, patches)
			})
		}
	}

	for _, n := range sizes {
		source := ast.NewSet(slices.Collect(ast.InternedIntRange(0, n))...)
		for _, m := range sizes {
			b.Run(fmt.Sprintf("set-%d-%d", n, m), func(b *testing.B) {
				patches := buildSetAddRemovePatches(n, m)
				runJSONPatchBenchmarkTest(b, source, patches)
			})
		}
	}
}

// buildAddCompositePatches builds m "add" ops appending small composite
// values at indexes/keys n..n+m-1.
func buildAddCompositePatches(n, m int) *ast.Array {
	out := make([]*ast.Term, m)
	for i := range m {
		path := ast.StringTerm("/" + strconv.Itoa(i+n))
		out[i] = createPatch("add", path, nil, ast.ArrayTerm(ast.InternedTerm(i+n)))
	}
	return ast.NewArray(out...)
}

// buildSetAddCompositePatches builds m "add" ops on a set, where each value
// is a singleton array (composite key path).
func buildSetAddCompositePatches(n, m int) *ast.Array {
	out := make([]*ast.Term, m)
	for i := range m {
		path := ast.ArrayTerm(ast.ArrayTerm(ast.InternedTerm(i + n)))
		out[i] = createPatch("add", path, nil, ast.ArrayTerm(ast.InternedTerm(i+n)))
	}
	return ast.NewArray(out...)
}

// buildAddRemoveScalarPatches builds m "add" ops followed by m matching
// "remove" ops at the same paths in reverse order.
func buildAddRemoveScalarPatches(n, m int) *ast.Array {
	out := make([]*ast.Term, 0, 2*m)
	for i := range m {
		path := ast.StringTerm("/" + strconv.Itoa(i+n))
		out = append(out, createPatch("add", path, nil, ast.InternedTerm(i+n)))
	}
	for i := m - 1; i >= 0; i-- {
		path := ast.StringTerm("/" + strconv.Itoa(i+n))
		out = append(out, createPatch("remove", path, nil, nil))
	}
	return ast.NewArray(out...)
}

// buildSetAddRemovePatches is the set-typed equivalent of
// buildAddRemoveScalarPatches.
func buildSetAddRemovePatches(n, m int) *ast.Array {
	out := make([]*ast.Term, 0, 2*m)
	for i := range m {
		v := ast.InternedTerm(i + n)
		out = append(out, createPatch("add", ast.ArrayTerm(v), nil, v))
	}
	for i := m - 1; i >= 0; i-- {
		v := ast.InternedTerm(i + n)
		out = append(out, createPatch("remove", ast.ArrayTerm(v), nil, nil))
	}
	return ast.NewArray(out...)
}

func createPatch(op string, path, from, value *ast.Term) *ast.Term {
	patchObj := ast.NewObject(
		[2]*ast.Term{ast.InternedTerm("op"), ast.InternedTerm(op)},
		[2]*ast.Term{ast.InternedTerm("path"), path},
	)
	if from != nil {
		patchObj.Insert(ast.InternedTerm("from"), from)
	}
	if value != nil {
		patchObj.Insert(ast.InternedTerm("value"), value)
	}
	return ast.NewTerm(patchObj)
}

func genTestObject(width int) ast.Value {
	out := ast.NewObjectWithCapacity(width)
	for i := range width {
		out.Insert(ast.InternedTerm(i), ast.InternedTerm(i))
	}
	return out
}

// For the purposes of addressing the original Github issue (#4409), a
// fairly shallow object with many keys ought to do the trick.
func gen3LayerObject(l1Keys, l2Keys, l3Keys int) ast.Value {
	obj := ast.NewObjectWithCapacity(l1Keys)
	for i := range l1Keys {
		l2Obj := ast.NewObjectWithCapacity(l2Keys)
		for j := range l2Keys {
			l3Obj := ast.NewObjectWithCapacity(l3Keys)
			for k := range l3Keys {
				l3Obj.Insert(ast.InternedTerm(strconv.Itoa(k)), ast.InternedTerm(true))
			}
			l2Obj.Insert(ast.InternedTerm(strconv.Itoa(j)), ast.NewTerm(l3Obj))
		}
		obj.Insert(ast.InternedTerm(strconv.Itoa(i)), ast.NewTerm(l2Obj))
	}
	return obj
}

// Generates a list of paths for JSON operations. N keys per level, M levels. P patches.
// TODO: Generate non-conflicting paths.
func genRandom3LayerObjectJSONPatchListData(rng *rand.Rand, l1Keys, l2Keys, l3Keys, p int) ast.Value {
	patchList := make([]*ast.Term, p)
	numKeys := []int{l1Keys, l2Keys, l3Keys}
	for i := range p {
		patchObj := ast.NewObject(
			[2]*ast.Term{ast.InternedTerm("op"), ast.InternedTerm("replace")},
			[2]*ast.Term{ast.InternedTerm("value"), ast.InternedTerm(2)},
		)
		depth := rng.Intn(3) + 1

		segments := make([]string, 0, 2*depth)
		for j := range depth {
			segments = append(segments, "/", strconv.Itoa(rng.Intn(numKeys[j])))
		}
		patchObj.Insert(ast.InternedTerm("path"), ast.InternedTerm(strings.Join(segments, "")))
		patchList[i] = ast.NewTerm(patchObj)
	}
	return ast.NewArray(patchList...)
}

func BenchmarkJSONPatchReplace(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, n := range sizes {
		for _, m := range sizes {
			source := gen3LayerObject(n, m, 10)
			for _, p := range sizes {
				b.Run(fmt.Sprintf("%dx%dx10-%dp", n, m, p), func(b *testing.B) {
					// Per-sub-benchmark seed keeps each (n,m,p) deterministic
					// while letting the data be GC'd between runs.
					rng := rand.New(rand.NewSource(42))
					patches := genRandom3LayerObjectJSONPatchListData(rng, n, m, 10, p)
					runJSONPatchBenchmarkTest(b, source, patches)
				})
			}
		}
	}
}

func BenchmarkJSONPatchPathologicalNestedAddChainObject(b *testing.B) {
	for _, n := range []int{10, 100, 500, 1000, 5000, 10000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			patchList := make([]*ast.Term, n)
			for i := range n {
				patchList[i] = ast.NewTerm(ast.NewObject(
					[2]*ast.Term{ast.InternedTerm("op"), ast.InternedTerm("add")},
					[2]*ast.Term{ast.InternedTerm("value"), ast.ObjectTerm()},
					[2]*ast.Term{ast.InternedTerm("path"), ast.InternedTerm(strings.Repeat("/a", i+1))},
				))
			}
			runJSONPatchBenchmarkTest(b, ast.NewObject(), ast.NewArray(patchList...))
		})
	}
}

func BenchmarkJSONPatchPathologicalNestedAddChainArray(b *testing.B) {
	for _, n := range []int{10, 100, 500, 1000, 5000, 10000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			patchList := make([]*ast.Term, n)
			for i := range n {
				patchList[i] = ast.NewTerm(ast.NewObject(
					[2]*ast.Term{ast.InternedTerm("op"), ast.InternedTerm("add")},
					[2]*ast.Term{ast.InternedTerm("value"), ast.ArrayTerm()},
					[2]*ast.Term{ast.InternedTerm("path"), ast.StringTerm(strings.Repeat("/0", i+1))},
				))
			}
			runJSONPatchBenchmarkTest(b, ast.NewArray(), ast.NewArray(patchList...))
		})
	}
}

// Sets are content-addressed, so the patch path itself has to be recursively
// constructed (each layer of nesting changes the addressing key).
func BenchmarkJSONPatchPathologicalNestedAddChainSet(b *testing.B) {
	for _, n := range []int{10, 100, 500, 1000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			patchList := make([]*ast.Term, n)
			for i := range n {
				value := ast.SetTerm(ast.InternedTerm("a"))
				constructedPath := ast.NewArray(ast.SetTerm(ast.InternedTerm("a")))
				for range i {
					constructedPath = constructedPath.Append(value)
					value = ast.SetTerm(ast.InternedTerm("a"), value)
				}

				// Reverse the path array.
				path := ast.NewArray()
				last := constructedPath.Len() - 1
				for j := range constructedPath.Len() {
					path = path.Append(constructedPath.Elem(last - j))
				}

				patchList[i] = ast.NewTerm(ast.NewObject(
					[2]*ast.Term{ast.InternedTerm("op"), ast.InternedTerm("add")},
					[2]*ast.Term{ast.InternedTerm("value"), ast.SetTerm(ast.InternedTerm("a"))},
					[2]*ast.Term{ast.InternedTerm("path"), ast.NewTerm(path)},
				))
			}
			runJSONPatchBenchmarkTest(b, ast.NewSet(ast.StringTerm("a")), ast.NewArray(patchList...))
		})
	}
}

// runJSONPatchBenchmarkTest invokes builtinJSONPatch directly. This bypasses
// the full eval pipeline (parser, compiler, store, query setup) so that the
// reported timings reflect patch application work rather than evaluation
// overhead.
//
// Patch errors are intentionally swallowed: some test data (notably the
// random replace cases) intentionally produces path-resolution errors part-
// way through a patch list, mirroring how those cases behaved in the
// previous Rego-eval form (rule failure produces no result, not a panic).
func runJSONPatchBenchmarkTest(b *testing.B, source ast.Value, patches ast.Value) {
	b.Helper()
	operands := []*ast.Term{ast.NewTerm(source), ast.NewTerm(patches)}
	bctx := BuiltinContext{Context: b.Context()}
	iter := func(*ast.Term) error { return nil }

	b.ResetTimer()
	for b.Loop() {
		_ = builtinJSONPatch(bctx, operands, iter)
	}
}
