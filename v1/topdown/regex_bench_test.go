// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"regexp"
	"sync"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func BenchmarkBuiltinRegexMatch(b *testing.B) {
	iter := func(*ast.Term) error { return nil }
	ctx := BuiltinContext{}

	for _, reusePattern := range []bool{true, false} {
		for _, patternCount := range []int{10, 100, 1000} {
			b.Run(fmt.Sprintf("reuse-pattern=%v, pattern-count=%d", reusePattern, patternCount), func(b *testing.B) {
				b.ResetTimer()
				for range b.N {
					// Clearing the cache
					regexpCache = make(map[string]*regexp.Regexp)

					for i := range patternCount {
						var operands []*ast.Term
						if reusePattern {
							operands = []*ast.Term{
								ast.NewTerm(ast.String("foo.*")),
								ast.NewTerm(ast.String("foobar")),
							}
						} else {
							operands = []*ast.Term{
								ast.NewTerm(ast.String(fmt.Sprintf("foo%d.*", i))),
								ast.NewTerm(ast.String(fmt.Sprintf("foo%dbar", i))),
							}
						}
						if err := builtinRegexMatch(ctx, operands, iter); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
		}
	}
}

func BenchmarkBuiltinRegexMatchAsync(b *testing.B) {
	iter := func(*ast.Term) error { return nil }
	ctx := BuiltinContext{}

	for _, reusePattern := range []bool{true, false} {
		for _, clientCount := range []int{100, 200} {
			for _, patternCount := range []int{10, 100, 1000} {
				b.Run(fmt.Sprintf("reuse-pattern=%v, clients=%d, pattern-count=%d", reusePattern, clientCount, patternCount), func(b *testing.B) {
					b.ResetTimer()
					for range b.N {
						// Clearing the cache
						regexpCache = make(map[string]*regexp.Regexp)

						wg := sync.WaitGroup{}
						for i := range clientCount {
							clientID := i
							wg.Add(1)
							go func() {
								for j := range patternCount {
									var operands []*ast.Term
									if reusePattern {
										operands = []*ast.Term{
											ast.NewTerm(ast.String("foo.*")),
											ast.NewTerm(ast.String("foobar")),
										}
									} else {
										operands = []*ast.Term{
											ast.NewTerm(ast.String(fmt.Sprintf("foo%d_%d.*", clientID, j))),
											ast.NewTerm(ast.String(fmt.Sprintf("foo%d_%dbar", clientID, j))),
										}
									}
									if err := builtinRegexMatch(ctx, operands, iter); err != nil {
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
