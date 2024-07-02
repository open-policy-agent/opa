// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"regexp"
	"sync"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func BenchmarkBuiltinRegexMatchDifferent(b *testing.B) {
	iter := func(*ast.Term) error { return nil }
	for _, useCache := range []bool{true, false} {
		for _, regexCount := range []int{10, 100, 1000} {
			ctx := BuiltinContext{}
			if useCache {
				ctx.Capabilities = &ast.Capabilities{
					RegexCache: true,
				}
			}

			b.Run(fmt.Sprintf("cache=%v, regex-count=%d", useCache, regexCount), func(b *testing.B) {
				b.ResetTimer()
				for n := 0; n < b.N; n++ {
					// Clearing the cache
					regexpCache = make(map[string]*regexp.Regexp)

					for i := 0; i < regexCount; i++ {
						operands := []*ast.Term{
							ast.NewTerm(ast.String(fmt.Sprintf("foo%d.*", i))),
							ast.NewTerm(ast.String(fmt.Sprintf("foo%dbar", i))),
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

func BenchmarkBuiltinRegexMatchSame(b *testing.B) {
	iter := func(*ast.Term) error { return nil }
	for _, useCache := range []bool{true, false} {
		for _, regexCount := range []int{10, 100, 1000} {
			ctx := BuiltinContext{}
			if useCache {
				ctx.Capabilities = &ast.Capabilities{
					RegexCache: true,
				}
			}

			b.Run(fmt.Sprintf("cache=%v, regex-count=%d", useCache, regexCount), func(b *testing.B) {
				b.ResetTimer()
				for n := 0; n < b.N; n++ {
					// Clearing the cache
					regexpCache = make(map[string]*regexp.Regexp)

					for i := 0; i < regexCount; i++ {
						operands := []*ast.Term{
							ast.NewTerm(ast.String("foo.*")),
							ast.NewTerm(ast.String("foobar")),
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

func BenchmarkBuiltinRegexMatchDifferentAsync(b *testing.B) {
	iter := func(*ast.Term) error { return nil }
	for _, useCache := range []bool{true, false} {
		for _, clientCount := range []int{100, 200} {
			for _, regexCount := range []int{10, 100, 1000} {
				ctx := BuiltinContext{}
				if useCache {
					ctx.Capabilities = &ast.Capabilities{
						RegexCache: true,
					}
				}

				b.Run(fmt.Sprintf("cache=%v, clients=%d, regex-count=%d", useCache, clientCount, regexCount), func(b *testing.B) {
					b.ResetTimer()
					for n := 0; n < b.N; n++ {
						// Clearing the cache
						regexpCache = make(map[string]*regexp.Regexp)

						wg := sync.WaitGroup{}
						for i := 0; i < clientCount; i++ {
							clientId := i
							wg.Add(1)
							go func() {
								for j := 0; j < regexCount; j++ {
									operands := []*ast.Term{
										ast.NewTerm(ast.String(fmt.Sprintf("foo%d_%d.*", clientId, j))),
										ast.NewTerm(ast.String(fmt.Sprintf("foo%d_%dbar", clientId, j))),
									}
									if err := builtinRegexMatch(ctx, operands, iter); err != nil {
										b.Fatal(err)
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

func BenchmarkBuiltinRegexMatchSameAsync(b *testing.B) {
	iter := func(*ast.Term) error { return nil }
	for _, useCache := range []bool{true, false} {
		for _, clientCount := range []int{100, 200} {
			for _, regexCount := range []int{10, 100, 1000} {
				ctx := BuiltinContext{}
				if useCache {
					ctx.Capabilities = &ast.Capabilities{
						RegexCache: true,
					}
				}

				b.Run(fmt.Sprintf("cache=%v, clients=%d, regex-count=%d", useCache, clientCount, regexCount), func(b *testing.B) {
					b.ResetTimer()
					for n := 0; n < b.N; n++ {
						// Clearing the cache
						regexpCache = make(map[string]*regexp.Regexp)

						wg := sync.WaitGroup{}
						for i := 0; i < clientCount; i++ {
							wg.Add(1)
							go func() {
								for i := 0; i < regexCount; i++ {
									operands := []*ast.Term{
										ast.NewTerm(ast.String("foo.*")),
										ast.NewTerm(ast.String("foobar")),
									}
									if err := builtinRegexMatch(ctx, operands, iter); err != nil {
										b.Fatal(err)
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
