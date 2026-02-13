// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"fmt"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

func BenchmarkJSONRemoveArray(b *testing.B) {
	sizes := []int{10, 100, 1000, 5000}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("size=%d", n), func(b *testing.B) {
			// Create an object wrapping the array: {"a": [0, 1, ...]}
			terms := slices.Collect(ast.InternedIntRange(0, n))
			arr := ast.NewArray(terms...)
			obj := ast.NewObject([2]*ast.Term{ast.InternedTerm("a"), ast.NewTerm(arr)})

			// Remove something inside the array to force traversal
			paths := ast.NewSet(ast.InternedTerm("a/nonexistent"))

			operands := []*ast.Term{
				ast.NewTerm(obj),
				ast.NewTerm(paths),
			}

			for b.Loop() {
				if err := builtinJSONRemove(
					BuiltinContext{
						Context: context.Background(),
					},
					operands,
					func(*ast.Term) error {
						return nil
					},
				); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkJSONFilterArray(b *testing.B) {
	sizes := []int{10, 100, 1000, 5000}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("size=%d", n), func(b *testing.B) {
			// Create an object with n keys
			obj := ast.NewObjectWithCapacity(n)
			pathSlice := make([]*ast.Term, n)
			for i := range n {
				k := ast.StringTerm(fmt.Sprintf("k%d", i))
				obj.Insert(k, ast.InternedTerm(i))
				pathSlice[i] = k
			}
			// Filter all keys: json.filter(obj, ["k0", "k1", ...])
			// This stresses pathsToObject (creating the filter mask)
			paths := ast.NewSet(pathSlice...)

			operands := []*ast.Term{
				ast.NewTerm(obj),
				ast.NewTerm(paths),
			}

			for b.Loop() {
				if err := builtinJSONFilter(
					BuiltinContext{
						Context: context.Background(),
					},
					operands,
					func(*ast.Term) error {
						return nil
					},
				); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkJSONFilterArrayIndices(b *testing.B) {
	sizes := []int{10, 100, 1000, 5000}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("size=%d", n), func(b *testing.B) {
			// Create an object wrapping an array: {"a": [0, 1, ...]}
			terms := slices.Collect(ast.InternedIntRange(0, n))
			arr := ast.NewArray(terms...)
			obj := ast.NewObject([2]*ast.Term{ast.StringTerm("a"), ast.NewTerm(arr)})

			// Filter to keep the first half of the array elements
			// json.filter(obj, ["a/0", "a/1", ... "a/n/2"])
			filterSize := n / 2
			if filterSize == 0 {
				filterSize = 1
			}
			pathSlice := make([]*ast.Term, filterSize)
			for i := range filterSize {
				pathSlice[i] = ast.StringTerm(fmt.Sprintf("a/%d", i))
			}
			paths := ast.NewSet(pathSlice...)

			operands := []*ast.Term{
				ast.NewTerm(obj),
				ast.NewTerm(paths),
			}

			for b.Loop() {
				if err := builtinJSONFilter(
					BuiltinContext{
						Context: context.Background(),
					},
					operands,
					func(*ast.Term) error {
						return nil
					},
				); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkJSONPatchAddShallowScalar(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	m := slices.Max(sizes)
	objArrPatches, setPatches := make([]*ast.Term, 0, m), make([]*ast.Term, 0, m)

	for i := range m {
		path := ast.StringTerm(fmt.Sprintf("/%d", i))
		value := ast.InternedTerm(i)

		objArrPatches = append(objArrPatches, createPatch("add", path, nil, value))
		setPatches = append(setPatches, createPatch("add", ast.ArrayTerm(value), nil, value))
	}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("object-%d", n), func(b *testing.B) {
			runJSONPatchBenchmarkTest(b, genTestObject(n), ast.NewArray(objArrPatches[:n]...))
		})
	}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("array-%d", n), func(b *testing.B) {
			runJSONPatchBenchmarkTest(b,
				ast.NewArray(slices.Collect(ast.InternedIntRange(0, n))...),
				ast.NewArray(objArrPatches[:n]...))
		})
	}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("set-%d", n), func(b *testing.B) {
			runJSONPatchBenchmarkTest(b,
				ast.NewSet(slices.Collect(ast.InternedIntRange(0, n))...),
				ast.NewArray(setPatches[:n]...))
		})
	}
}

func BenchmarkJSONPatchAddShallowComposite(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	// Object case
	for _, n := range sizes {
		source := genTestObject(n)
		for _, m := range sizes {
			testName := fmt.Sprintf("object-%d-%d", n, m)
			// Build dataset right before use:
			plArrayObj := make([]*ast.Term, 0, m*2)
			// add ops
			for i := range m {
				plArrayObj = append(plArrayObj, createPatch("add", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, ast.ArrayTerm(ast.IntNumberTerm(i+n))))
			}
			patchList := ast.NewArray(plArrayObj...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(b, source, patchList)
			})
		}
	}

	// Array case
	for _, n := range sizes {
		source := ast.NewArray(slices.Collect(ast.InternedIntRange(0, n))...)
		for _, m := range sizes {
			testName := fmt.Sprintf("array-%d-%d", n, m)
			// Build dataset right before use:
			plArrayObj := make([]*ast.Term, 0, m*2)
			// add ops
			for i := range m {
				plArrayObj = append(plArrayObj, createPatch("add", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, ast.ArrayTerm(ast.IntNumberTerm(i+n))))
			}
			patchList := ast.NewArray(plArrayObj...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(b, source, patchList)
			})
		}
	}
	// Set case
	for _, n := range sizes {
		source := ast.NewSet(slices.Collect(ast.InternedIntRange(0, n))...)
		for _, m := range sizes {
			testName := fmt.Sprintf("set-%d-%d", n, m)
			// Build dataset right before use:
			plSet := make([]*ast.Term, 0, m*2)
			// add ops
			for i := range m {
				plSet = append(plSet, createPatch("add", ast.ArrayTerm(ast.ArrayTerm(ast.IntNumberTerm(i+n))), nil, ast.ArrayTerm(ast.IntNumberTerm(i+n))))
			}
			patchList := ast.NewArray(plSet...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(b, source, patchList)
			})
		}
	}
}

func BenchmarkJSONPatchAddRemove(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	// Object case
	for _, n := range sizes {
		source := genTestObject(n)
		for _, m := range sizes {
			testName := fmt.Sprintf("object-%d-%d", n, m)
			// Build dataset right before use:
			plArrayObj := make([]*ast.Term, 0, m*2)
			// add ops
			for i := range m {
				plArrayObj = append(plArrayObj, createPatch("add", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, ast.IntNumberTerm(i+n)))
			}
			// remove ops
			for i := m - 1; i >= 0; i-- {
				plArrayObj = append(plArrayObj, createPatch("remove", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, nil))
			}
			patchList := ast.NewArray(plArrayObj...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(b, source, patchList)
			})
		}
	}

	// Array case
	for _, n := range sizes {
		source := ast.NewArray(slices.Collect(ast.InternedIntRange(0, n))...)
		for _, m := range sizes {
			testName := fmt.Sprintf("array-%d-%d", n, m)
			// Build dataset right before use:
			plArrayObj := make([]*ast.Term, 0, m*2)
			// add ops
			for i := range m {
				plArrayObj = append(plArrayObj, createPatch("add", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, ast.IntNumberTerm(i+n)))
			}
			// remove ops
			for i := m - 1; i >= 0; i-- {
				plArrayObj = append(plArrayObj, createPatch("remove", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, nil))
			}
			patchList := ast.NewArray(plArrayObj...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(b, source, patchList)
			})
		}
	}

	// Set case
	for _, n := range sizes {
		source := ast.NewSet(slices.Collect(ast.InternedIntRange(0, n))...)
		for _, m := range sizes {
			testName := fmt.Sprintf("set-%d-%d", n, m)
			// Build dataset right before use:
			plSet := make([]*ast.Term, 0, m*2)
			// add ops
			for i := range m {
				plSet = append(plSet, createPatch("add", ast.ArrayTerm(ast.IntNumberTerm(i+n)), nil, ast.IntNumberTerm(i+n)))
			}
			// remove ops
			for i := m - 1; i >= 0; i-- {
				plSet = append(plSet, createPatch("remove", ast.ArrayTerm(ast.IntNumberTerm(i+n)), nil, nil))
			}
			patchList := ast.NewArray(plSet...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(b, source, patchList)
			})
		}
	}
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
	obj := ast.NewObject()
	for i := range l1Keys {
		l2Obj := ast.NewObject()
		for j := range l2Keys {
			l3Obj := ast.NewObject()
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
func genRandom3LayerObjectJSONPatchListData(l1Keys, l2Keys, l3Keys, p int) ast.Value {
	patchList := make([]*ast.Term, p)
	numKeys := []int{l1Keys, l2Keys, l3Keys}
	for i := range p {
		patchObj := ast.NewObject(
			[2]*ast.Term{ast.InternedTerm("op"), ast.InternedTerm("replace")},
			[2]*ast.Term{ast.InternedTerm("value"), ast.InternedTerm(2)},
		)
		// Random path depth.
		depth := rand.Intn(3) + 1 // (max - min) + min method of getting a random range.

		// Random values for each path segment.
		segments := make([]string, 0, 2*depth)
		for j := range depth {
			pathSegment := strconv.FormatInt(int64(rand.Intn(numKeys[j])), 10)
			segments = append(segments, "/", pathSegment)
		}
		path := strings.Join(segments, "")
		patchObj.Insert(ast.InternedTerm("path"), ast.InternedTerm(path))
		patchList[i] = ast.NewTerm(patchObj)
	}
	return ast.NewArray(patchList...)
}

func BenchmarkJSONPatchReplace(b *testing.B) {
	ctx := b.Context()

	sizes := []int{10, 100, 1000}

	// Pre-generate the test datasets/patches.
	testdata := map[string][2]ast.Value{}
	for _, n := range sizes {
		for _, m := range sizes {
			testObj := gen3LayerObject(n, m, 10)
			for _, p := range sizes {
				testdata[fmt.Sprintf("%dx%dx10-%dp", n, m, p)] = [2]ast.Value{testObj, genRandom3LayerObjectJSONPatchListData(n, m, 10, p)}
			}
		}
	}

	for _, n := range sizes {
		for _, m := range sizes {
			for _, p := range sizes {
				testName := fmt.Sprintf("%dx%dx10-%dp", n, m, p)
				b.Run(testName, func(b *testing.B) {
					store := inmem.NewFromObject(map[string]any{
						"obj":     testdata[testName][0],
						"patches": testdata[testName][1],
					})

					module := `package test

					result := json.patch(data.obj, data.patches)`

					query := ast.MustParseBody("data.test.result")
					compiler := ast.MustCompileModules(map[string]string{
						"test.rego": module,
					})

					b.ResetTimer()

					for b.Loop() {

						err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

							q := NewQuery(query).
								WithCompiler(compiler).
								WithStore(store).
								WithTransaction(txn)

							_, err := q.Run(ctx)
							if err != nil {
								return err
							}

							return nil
						})

						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}

func BenchmarkJSONPatchPathologicalNestedAddChainObject(b *testing.B) {
	sizes := []int{10, 100, 500, 1000, 5000, 10000}
	// Pre-generate the test datasets/patches.
	testdata := map[string]ast.Value{}
	for _, n := range sizes {
		patchList := make([]*ast.Term, n)
		path := ""
		for i := range n {
			patchObj := ast.NewObject(
				[2]*ast.Term{ast.InternedTerm("op"), ast.InternedTerm("add")},
				[2]*ast.Term{ast.InternedTerm("value"), ast.ObjectTerm()},
			)

			path += "/a"

			patchObj.Insert(ast.InternedTerm("path"), ast.InternedTerm(path))
			patchList[i] = ast.NewTerm(patchObj)
		}
		testdata[strconv.Itoa(n)] = ast.NewArray(patchList...)
	}

	for _, n := range sizes {
		testName := strconv.Itoa(n)
		b.Run(testName, func(b *testing.B) {
			runJSONPatchBenchmarkTest(b, ast.NewObject(), testdata[testName])
		})
	}
}

func BenchmarkJSONPatchPathologicalNestedAddChainArray(b *testing.B) {
	sizes := []int{10, 100, 500, 1000, 5000, 10000}
	// Pre-generate the test datasets/patches.
	testdata := map[string]ast.Value{}
	for _, n := range sizes {
		patchList := make([]*ast.Term, n)
		path := ""
		for i := range n {
			patchObj := ast.NewObject(
				[2]*ast.Term{ast.InternedTerm("op"), ast.InternedTerm("add")},
				[2]*ast.Term{ast.InternedTerm("value"), ast.ArrayTerm()},
			)

			path += "/0"

			patchObj.Insert(ast.InternedTerm("path"), ast.StringTerm(path))
			patchList[i] = ast.NewTerm(patchObj)
		}
		testdata[strconv.Itoa(n)] = ast.NewArray(patchList...)
	}

	for _, n := range sizes {
		testName := strconv.Itoa(n)
		b.Run(testName, func(b *testing.B) {
			runJSONPatchBenchmarkTest(b, ast.NewArray(), testdata[testName])
		})
	}
}

// This one is tricky, because sets used content-based addressing.
// That means our sets for the path have to be recursively constructed!
func BenchmarkJSONPatchPathologicalNestedAddChainSet(b *testing.B) {
	sizes := []int{10, 100, 500, 1000}

	// Pre-generate the test datasets/patches.
	testdata := map[string]ast.Value{}
	for _, n := range sizes {
		patchList := make([]*ast.Term, n)
		for i := range n {
			patchObj := ast.NewObject(
				[2]*ast.Term{ast.InternedTerm("op"), ast.InternedTerm("add")},
			)
			value := ast.SetTerm(ast.InternedTerm("a"))
			constructedPath := ast.NewArray(ast.SetTerm(ast.InternedTerm("a")))
			for range i {
				constructedPath = constructedPath.Append(value)
				value = ast.SetTerm(ast.InternedTerm("a"), value)
			}

			// Reverse the ast.Array slice.
			path := ast.NewArray()
			pathLength := constructedPath.Len() - 1
			for j := range constructedPath.Len() {
				path = path.Append(constructedPath.Elem(pathLength - j))
			}

			patchObj.Insert(ast.InternedTerm("value"), ast.SetTerm(ast.InternedTerm("a")))
			patchObj.Insert(ast.InternedTerm("path"), ast.NewTerm(path))
			patchList[i] = ast.NewTerm(patchObj)
		}
		testdata[strconv.Itoa(n)] = ast.NewArray(patchList...)
	}

	for _, n := range sizes {
		testName := strconv.Itoa(n)
		b.Run(testName, func(b *testing.B) {
			runJSONPatchBenchmarkTest(b, ast.NewSet(ast.StringTerm("a")), testdata[testName])
		})
	}
}

func runJSONPatchBenchmarkTest(b *testing.B, source ast.Value, patches ast.Value) {
	store := inmem.NewFromObject(map[string]any{"source": source, "patches": patches})

	module := "package test\n\nresult := json.patch(data.source, data.patches)"
	query := ast.MustParseBody("data.test.result")
	compiler := ast.MustCompileModules(map[string]string{"test.rego": module})

	err := storage.Txn(b.Context(), store, storage.TransactionParams{}, func(txn storage.Transaction) error {
		q := NewQuery(query).WithCompiler(compiler).WithStore(store).WithTransaction(txn)

		for b.Loop() {
			if _, err := q.Run(b.Context()); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		b.Fatal(err)
	}
}
