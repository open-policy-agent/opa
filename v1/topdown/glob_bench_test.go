// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gobwas/glob"
	"github.com/open-policy-agent/opa/v1/ast"
)

func BenchmarkBuiltinGlobMatch(b *testing.B) {
	iter := func(*ast.Term) error { return nil }
	ctx := BuiltinContext{}

	for _, reusePattern := range []bool{true, false} {
		for _, patternCount := range []int{10, 100, 1000} {
			b.Run(fmt.Sprintf("reuse-pattern=%v, pattern-count=%d", reusePattern, patternCount), func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
					// Clearing the cache
					globCache = make(map[string]glob.Glob)

					for i := range patternCount {
						var operands []*ast.Term
						if reusePattern {
							operands = []*ast.Term{
								ast.NewTerm(ast.String("foo/*")),
								ast.NullTerm(),
								ast.NewTerm(ast.String("foo/bar")),
							}
						} else {
							operands = []*ast.Term{
								ast.NewTerm(ast.String(fmt.Sprintf("foo/*/%d", i))),
								ast.NullTerm(),
								ast.NewTerm(ast.String(fmt.Sprintf("foo/bar/%d", i))),
							}
						}
						if err := builtinGlobMatch(ctx, operands, iter); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
		}
	}
}

func BenchmarkBuiltinGlobMatchAsync(b *testing.B) {
	iter := func(*ast.Term) error { return nil }
	ctx := BuiltinContext{}

	for _, reusePattern := range []bool{true, false} {
		for _, clientCount := range []int{100, 200} {
			for _, patternCount := range []int{10, 100, 1000} {
				b.Run(fmt.Sprintf("reuse-pattern=%v, clients=%d, pattern-count=%d", reusePattern, clientCount, patternCount), func(b *testing.B) {
					b.ResetTimer()
					for range b.N {
						// Clearing the cache
						globCache = make(map[string]glob.Glob)

						wg := sync.WaitGroup{}
						for i := range clientCount {
							clientID := i
							wg.Add(1)
							go func() {
								for j := range patternCount {
									var operands []*ast.Term
									if reusePattern {
										operands = []*ast.Term{
											ast.NewTerm(ast.String("foo/*")),
											ast.NullTerm(),
											ast.NewTerm(ast.String("foo/bar")),
										}
									} else {
										operands = []*ast.Term{
											ast.NewTerm(ast.String(fmt.Sprintf("foo/*/%d/%d", clientID, j))),
											ast.NullTerm(),
											ast.NewTerm(ast.String(fmt.Sprintf("foo/bar/%d/%d", clientID, j))),
										}
									}
									if err := builtinGlobMatch(ctx, operands, iter); err != nil {
										b.Error(err)
										return
									}
								}
								wg.Done()
							}()
						}
						wg.Wait()
					}
				})
			}
		}
	}
}
