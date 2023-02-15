// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
)

// Tests on only single-layer composite data types.
// func BenchmarkJSONPatchAdd(b *testing.B) {
// 	ctx := context.Background()

// 	sizes := []int{10, 100, 1000, 1000}
// }

// func BenchmarkJSONPatchRemove(b *testing.B) {
// 	ctx := context.Background()

// 	sizes := []int{10, 100, 1000, 1000}
// }

// func BenchmarkJSONPatchReplace(b *testing.B) {
// 	ctx := context.Background()

// 	sizes := []int{10, 100, 1000, 1000}

// }
func BenchmarkJSONPatchAddShallowScalar(b *testing.B) {
	ctx := context.Background()

	sizes := []int{10, 100, 1000, 10000}

	// Object case
	for _, n := range sizes {
		source := genTestObject(n)
		for _, m := range sizes {
			testName := fmt.Sprintf("object-%d-%d", n, m)
			// Build dataset right before use:
			plArrayObj := make([]*ast.Term, 0, m*2)
			// add ops
			for i := 0; i < m; i++ {
				plArrayObj = append(plArrayObj, genTestJSONPatchObject("add", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, ast.IntNumberTerm(i+n)))
			}
			patchList := ast.NewArray(plArrayObj...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(ctx, b, source, patchList)
			})
		}
	}

	// Array case
	for _, n := range sizes {
		source := genTestArray(n)
		for _, m := range sizes {
			testName := fmt.Sprintf("array-%d-%d", n, m)
			// Build dataset right before use:
			plArrayObj := make([]*ast.Term, 0, m*2)
			// add ops
			for i := 0; i < m; i++ {
				plArrayObj = append(plArrayObj, genTestJSONPatchObject("add", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, ast.IntNumberTerm(i+n)))
			}
			patchList := ast.NewArray(plArrayObj...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(ctx, b, source, patchList)
			})
		}
	}
	// Set case
	for _, n := range sizes {
		source := genTestSet(n)
		for _, m := range sizes {
			testName := fmt.Sprintf("set-%d-%d", n, m)
			// Build dataset right before use:
			plSet := make([]*ast.Term, 0, m*2)
			// add ops
			for i := 0; i < m; i++ {
				plSet = append(plSet, genTestJSONPatchObject("add", ast.ArrayTerm(ast.IntNumberTerm(i+n)), nil, ast.IntNumberTerm(i+n)))
			}
			patchList := ast.NewArray(plSet...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(ctx, b, source, patchList)
			})
		}
	}
}

func BenchmarkJSONPatchAddShallowComposite(b *testing.B) {
	ctx := context.Background()

	sizes := []int{10, 100, 1000, 10000}

	// Object case
	for _, n := range sizes {
		source := genTestObject(n)
		for _, m := range sizes {
			testName := fmt.Sprintf("object-%d-%d", n, m)
			// Build dataset right before use:
			plArrayObj := make([]*ast.Term, 0, m*2)
			// add ops
			for i := 0; i < m; i++ {
				plArrayObj = append(plArrayObj, genTestJSONPatchObject("add", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, ast.ArrayTerm(ast.IntNumberTerm(i+n))))
			}
			patchList := ast.NewArray(plArrayObj...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(ctx, b, source, patchList)
			})
		}
	}

	// Array case
	for _, n := range sizes {
		source := genTestArray(n)
		for _, m := range sizes {
			testName := fmt.Sprintf("array-%d-%d", n, m)
			// Build dataset right before use:
			plArrayObj := make([]*ast.Term, 0, m*2)
			// add ops
			for i := 0; i < m; i++ {
				plArrayObj = append(plArrayObj, genTestJSONPatchObject("add", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, ast.ArrayTerm(ast.IntNumberTerm(i+n))))
			}
			patchList := ast.NewArray(plArrayObj...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(ctx, b, source, patchList)
			})
		}
	}
	// Set case
	for _, n := range sizes {
		source := genTestSet(n)
		for _, m := range sizes {
			testName := fmt.Sprintf("set-%d-%d", n, m)
			// Build dataset right before use:
			plSet := make([]*ast.Term, 0, m*2)
			// add ops
			for i := 0; i < m; i++ {
				plSet = append(plSet, genTestJSONPatchObject("add", ast.ArrayTerm(ast.ArrayTerm(ast.IntNumberTerm(i+n))), nil, ast.ArrayTerm(ast.IntNumberTerm(i+n))))
			}
			patchList := ast.NewArray(plSet...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(ctx, b, source, patchList)
			})
		}
	}
}

func BenchmarkJSONPatchAddRemove(b *testing.B) {
	ctx := context.Background()

	sizes := []int{10, 100, 1000, 10000}

	// Object case
	for _, n := range sizes {
		source := genTestObject(n)
		for _, m := range sizes {
			testName := fmt.Sprintf("object-%d-%d", n, m)
			// Build dataset right before use:
			plArrayObj := make([]*ast.Term, 0, m*2)
			// add ops
			for i := 0; i < m; i++ {
				plArrayObj = append(plArrayObj, genTestJSONPatchObject("add", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, ast.IntNumberTerm(i+n)))
			}
			// remove ops
			for i := m - 1; i >= 0; i-- {
				plArrayObj = append(plArrayObj, genTestJSONPatchObject("remove", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, nil))
			}
			patchList := ast.NewArray(plArrayObj...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(ctx, b, source, patchList)
			})
		}
	}

	// Array case
	for _, n := range sizes {
		source := genTestArray(n)
		for _, m := range sizes {
			testName := fmt.Sprintf("array-%d-%d", n, m)
			// Build dataset right before use:
			plArrayObj := make([]*ast.Term, 0, m*2)
			// add ops
			for i := 0; i < m; i++ {
				plArrayObj = append(plArrayObj, genTestJSONPatchObject("add", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, ast.IntNumberTerm(i+n)))
			}
			// remove ops
			for i := m - 1; i >= 0; i-- {
				plArrayObj = append(plArrayObj, genTestJSONPatchObject("remove", ast.StringTerm("/"+strconv.FormatInt(int64(i+n), 10)), nil, nil))
			}
			patchList := ast.NewArray(plArrayObj...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(ctx, b, source, patchList)
			})
		}
	}

	// Set case
	for _, n := range sizes {
		source := genTestSet(n)
		for _, m := range sizes {
			testName := fmt.Sprintf("set-%d-%d", n, m)
			// Build dataset right before use:
			plSet := make([]*ast.Term, 0, m*2)
			// add ops
			for i := 0; i < m; i++ {
				plSet = append(plSet, genTestJSONPatchObject("add", ast.ArrayTerm(ast.IntNumberTerm(i+n)), nil, ast.IntNumberTerm(i+n)))
			}
			// remove ops
			for i := m - 1; i >= 0; i-- {
				plSet = append(plSet, genTestJSONPatchObject("remove", ast.ArrayTerm(ast.IntNumberTerm(i+n)), nil, nil))
			}
			patchList := ast.NewArray(plSet...)

			b.ResetTimer()
			b.Run(testName, func(b *testing.B) {
				runJSONPatchBenchmarkTest(ctx, b, source, patchList)
			})
		}
	}
}

func genTestJSONPatchObject(op string, path, from, value *ast.Term) *ast.Term {
	patchObj := ast.NewObject(
		[2]*ast.Term{ast.StringTerm("op"), ast.StringTerm(op)},
		[2]*ast.Term{ast.StringTerm("path"), path},
	)
	if from != nil {
		patchObj.Insert(ast.StringTerm("from"), from)
	}
	if value != nil {
		patchObj.Insert(ast.StringTerm("value"), value)
	}
	return ast.NewTerm(patchObj)
}

func genTestObject(width int) ast.Value {
	out := ast.NewObject()
	for i := 0; i < width; i++ {
		out.Insert(ast.IntNumberTerm(i), ast.IntNumberTerm(i))
	}
	return out
}

func genTestArray(width int) ast.Value {
	out := ast.NewArray()
	for i := 0; i < width; i++ {
		out = out.Append(ast.IntNumberTerm(i))
	}
	return out
}

func genTestSet(width int) ast.Value {
	out := ast.NewSet()
	for i := 0; i < width; i++ {
		out.Add(ast.IntNumberTerm(i))
	}
	return out
}

// For the purposes of addressing the original Github issue (#4409), a
// fairly shallow object with many keys ought to do the trick.
func gen3LayerObject(l1Keys, l2Keys, l3Keys int) ast.Value {
	obj := ast.NewObject()
	for i := 0; i < l1Keys; i++ {
		l2Obj := ast.NewObject()
		for j := 0; j < l2Keys; j++ {
			l3Obj := ast.NewObject()
			for k := 0; k < l3Keys; k++ {
				l3Obj.Insert(ast.StringTerm(fmt.Sprintf("%d", k)), ast.BooleanTerm(true))
			}
			l2Obj.Insert(ast.StringTerm(fmt.Sprintf("%d", j)), ast.NewTerm(l3Obj))
		}
		obj.Insert(ast.StringTerm(fmt.Sprintf("%d", i)), ast.NewTerm(l2Obj))
	}
	return obj
}

// Generates a list of paths for JSON operations. N keys per level, M levels. P patches.
// TODO: Generate non-conflicting paths.
func genRandom3LayerObjectJSONPatchListData(l1Keys, l2Keys, l3Keys, p int) ast.Value {
	patchList := make([]*ast.Term, p)
	numKeys := []int{l1Keys, l2Keys, l3Keys}
	for i := 0; i < p; i++ {
		patchObj := ast.NewObject(
			[2]*ast.Term{ast.StringTerm("op"), ast.StringTerm("replace")},
			[2]*ast.Term{ast.StringTerm("value"), ast.IntNumberTerm(2)},
		)
		// Random path depth.
		depth := rand.Intn(3) + 1 // (max - min) + min method of getting a random range.

		// Random values for each path segment.
		segments := []string{}
		for j := 0; j < depth; j++ {
			pathSegment := strconv.FormatInt(int64(rand.Intn(numKeys[j])), 10)
			segments = append(segments, "/", pathSegment)
		}
		path := strings.Join(segments, "")
		patchObj.Insert(ast.StringTerm("path"), ast.StringTerm(path))
		patchList[i] = ast.NewTerm(patchObj)
	}
	return ast.NewArray(patchList...)
}

func BenchmarkJSONPatchReplace(b *testing.B) {
	ctx := context.Background()

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
					store := inmem.NewFromObject(map[string]interface{}{
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

					for i := 0; i < b.N; i++ {

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
	ctx := context.Background()

	sizes := []int{10, 100, 500, 1000, 5000, 10000}
	// Pre-generate the test datasets/patches.
	testdata := map[string]ast.Value{}
	for _, n := range sizes {
		patchList := make([]*ast.Term, n)
		path := ""
		for i := 0; i < n; i++ {
			patchObj := ast.NewObject(
				[2]*ast.Term{ast.StringTerm("op"), ast.StringTerm("add")},
				[2]*ast.Term{ast.StringTerm("value"), ast.ObjectTerm()},
			)

			path = path + "/a"

			patchObj.Insert(ast.StringTerm("path"), ast.StringTerm(path))
			patchList[i] = ast.NewTerm(patchObj)
		}
		testdata[fmt.Sprintf("%d", n)] = ast.NewArray(patchList...)
	}

	for _, n := range sizes {
		testName := fmt.Sprintf("%d", n)
		b.Run(testName, func(b *testing.B) {
			runJSONPatchBenchmarkTest(ctx, b, ast.NewObject(), testdata[testName])
		})
	}
}

func BenchmarkJSONPatchPathologicalNestedAddChainArray(b *testing.B) {
	ctx := context.Background()

	sizes := []int{10, 100, 500, 1000, 5000, 10000}
	// Pre-generate the test datasets/patches.
	testdata := map[string]ast.Value{}
	for _, n := range sizes {
		patchList := make([]*ast.Term, n)
		path := ""
		for i := 0; i < n; i++ {
			patchObj := ast.NewObject(
				[2]*ast.Term{ast.StringTerm("op"), ast.StringTerm("add")},
				[2]*ast.Term{ast.StringTerm("value"), ast.ArrayTerm()},
			)

			path = path + "/0"

			patchObj.Insert(ast.StringTerm("path"), ast.StringTerm(path))
			patchList[i] = ast.NewTerm(patchObj)
		}
		testdata[fmt.Sprintf("%d", n)] = ast.NewArray(patchList...)
	}

	for _, n := range sizes {
		testName := fmt.Sprintf("%d", n)
		b.Run(testName, func(b *testing.B) {
			runJSONPatchBenchmarkTest(ctx, b, ast.NewArray(), testdata[testName])
		})
	}
}

// This one is tricky, because sets used content-based addressing.
// That means our sets for the path have to be recursively constructed!
func BenchmarkJSONPatchPathologicalNestedAddChainSet(b *testing.B) {
	ctx := context.Background()
	sizes := []int{10, 100, 500, 1000}

	// Pre-generate the test datasets/patches.
	testdata := map[string]ast.Value{}
	for _, n := range sizes {
		patchList := make([]*ast.Term, n)
		for i := 0; i < n; i++ {
			patchObj := ast.NewObject(
				[2]*ast.Term{ast.StringTerm("op"), ast.StringTerm("add")},
			)
			value := ast.SetTerm(ast.StringTerm("a"))
			constructedPath := ast.NewArray(ast.SetTerm(ast.StringTerm("a")))
			for j := 0; j < i; j++ {
				constructedPath = constructedPath.Append(value)
				value = ast.SetTerm(ast.StringTerm("a"), value)
			}

			// Reverse the ast.Array slice.
			path := ast.NewArray()
			pathLength := constructedPath.Len() - 1
			for j := 0; j < constructedPath.Len(); j++ {
				path = path.Append(constructedPath.Elem(pathLength - j))
			}

			patchObj.Insert(ast.StringTerm("value"), ast.SetTerm(ast.StringTerm("a")))
			patchObj.Insert(ast.StringTerm("path"), ast.NewTerm(path))
			patchList[i] = ast.NewTerm(patchObj)
		}
		testdata[fmt.Sprintf("%d", n)] = ast.NewArray(patchList...)
	}

	for _, n := range sizes {
		testName := fmt.Sprintf("%d", n)
		b.Run(testName, func(b *testing.B) {
			runJSONPatchBenchmarkTest(ctx, b, ast.NewSet(ast.StringTerm("a")), testdata[testName])
		})
	}
}

func runJSONPatchBenchmarkTest(ctx context.Context, b *testing.B, source ast.Value, patches ast.Value) {
	store := inmem.NewFromObject(map[string]interface{}{
		"source":  source,
		"patches": patches,
	})

	module := `package test

			result := json.patch(data.source, data.patches)`

	query := ast.MustParseBody("data.test.result")
	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": module,
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {

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

}
