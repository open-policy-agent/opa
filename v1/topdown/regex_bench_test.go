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

var reuseOperands = []*ast.Term{
	ast.NewTerm(ast.String("foo.*")),
	ast.NewTerm(ast.String("foobar")),
}

func BenchmarkBuiltinRegexMatch(b *testing.B) {
	iter := func(*ast.Term) error { return nil }
	ctx := BuiltinContext{}

	for _, reusePattern := range []bool{true, false} {
		for _, patternCount := range []int{10, 100, 1000} {
			b.Run(fmt.Sprintf("reuse-pattern=%v, pattern-count=%d", reusePattern, patternCount), func(b *testing.B) {
				b.ResetTimer()
				for b.Loop() {
					// Clearing the cache
					regexpCache = make(map[string]*regexp.Regexp)

					for i := range patternCount {
						var operands []*ast.Term
						if reusePattern {
							operands = reuseOperands
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
					for b.Loop() {
						// Clearing the cache
						regexpCache = make(map[string]*regexp.Regexp)

						wg := sync.WaitGroup{}
						for i := range clientCount {
							clientID := i
							wg.Go(func() {
								for j := range patternCount {
									var operands []*ast.Term
									if reusePattern {
										operands = reuseOperands
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
							})
						}
						wg.Wait()
					}
				})
			}
		}
	}
}

// Matching a template whose regex is already cached.
//
//	                    before                             after
//	vars=1     3959 ns     2505 B/op    43 allocs     1255 ns    840 B/op   15 allocs
//	vars=3    16229 ns    13895 B/op   187 allocs     3383 ns   2640 B/op   43 allocs
//
// The before column is what this recorded while generating the pattern also compiled
// every variable's regex into a slice that was never read.
func BenchmarkBuiltinRegexMatchTemplate(b *testing.B) {
	iter := func(*ast.Term) error { return nil }
	ctx := BuiltinContext{}

	tests := []struct {
		name     string
		template string
		match    string
	}{
		{"vars=1", "urn:foo:{.*}", "urn:foo:bar:baz"},
		{"vars=3", "urn:{[a-z]+}:{[0-9]{2,10}}:{.*}", "urn:foo:12345:bar"},
	}

	for _, tc := range tests {
		b.Run(tc.name, func(b *testing.B) {
			operands := []*ast.Term{
				ast.StringTerm(tc.template),
				ast.StringTerm(tc.match),
				ast.StringTerm("{"),
				ast.StringTerm("}"),
			}

			b.ReportAllocs()
			for b.Loop() {
				if err := builtinRegexMatchTemplate(ctx, operands, iter); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
