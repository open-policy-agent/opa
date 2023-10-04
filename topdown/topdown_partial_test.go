// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
	"github.com/open-policy-agent/opa/util"
)

func TestTopDownPartialEval(t *testing.T) {
	tests := []struct {
		note                 string
		unknowns             []string
		disableInlining      []string
		shallow              bool
		skipPartialNamespace bool
		query                string
		modules              []string
		moduleASTs           []*ast.Module
		data                 string
		input                string
		wantQueries          []string
		wantQueryASTs        []ast.Body
		wantSupport          []string
		wantSupportASTs      []*ast.Module
		ignoreOrder          bool
	}{
		{
			note:        "empty",
			query:       "true = true",
			wantQueries: []string{``},
		},
		{
			note:        "query vars",
			query:       "x = 1",
			wantQueries: []string{`x = 1`},
		},
		{
			note:        "trivial",
			query:       "input.x = 1",
			wantQueries: []string{`input.x = 1`},
		},
		{
			note:        "trivial reverse",
			query:       "1 = input.x",
			wantQueries: []string{`1 = input.x`},
		},
		{
			note:  "trivial both",
			query: "input.x = input.y",
			wantQueries: []string{
				`input.x = input.y`,
			},
		},
		{
			note:        "transitive",
			query:       "input.x = y; y[0] = z; z = 1; plus(z, 2, 3)",
			wantQueries: []string{`input.x = y; y[0] = 1; plus(1, 2, 3); z = 1`},
		},
		{
			note:  "vars",
			query: "x = 1; y = 2; input.x = x; y = input.y",
			wantQueries: []string{
				`input.x = 1; 2 = input.y; x = 1; y = 2`,
			},
		},
		{
			note:  "complete: substitute",
			query: "input.x = data.test.p; data.test.q = input.y",
			modules: []string{
				`package test
				p = x { x = "foo" }
				q = x { x = "bar" }`,
			},
			wantQueries: []string{
				`"foo" = input.x; "bar" = input.y`,
			},
		},
		{
			note:  "iterate vars",
			query: "a = [1,2]; a[i] = x",
			wantQueries: []string{
				`a = [1, 2]; i = 0; x = 1`,
				`a = [1, 2]; i = 1; x = 2`,
			},
		},
		{
			note:  "iterate data",
			query: "data.x[i] = input.x",
			data:  `{"x": [1,2,3]}`,
			wantQueries: []string{
				`input.x = 1; i = 0`,
				`input.x = 2; i = 1`,
				`input.x = 3; i = 2`,
			},
		},
		{
			note:  "iterate data - unknown key",
			query: `data.test.p = true`,
			data:  `{"x": {"foo": 7, "bar": 7}}`,
			modules: []string{
				`
					package test

					p {
						input.x = k
						data.x[k] = 7
					}
				`,
			},
			wantQueries: []string{`input.x = "foo"`, `input.x = "bar"`},
		},
		{
			note:  "iterate data - unknown key undefined",
			query: `data.test.p = true`,
			data:  `{"x": {"foo": 8, "bar": 8}}`,
			modules: []string{
				`
					package test

					p {
						input.x = k
						data.x[k] = 7
					}
				`,
			},
			wantQueries: []string{},
		},
		{ // TODO: duplicate for general refs?
			note:  "iterate rules: partial object",
			query: `data.test.p[x] = input.x`,
			modules: []string{
				`package test
				p["a"] = "b"
				p["b"] = "c"
				p["c"] = "d"`,
			},
			wantQueries: []string{
				`"b" = input.x; x = "a"`,
				`"c" = input.x; x = "b"`,
				`"d" = input.x; x = "c"`,
			},
		},
		{ // TODO: duplicate for general refs?
			note:  "iterate rules: partial set",
			query: `input.x = x; data.test.p[x]`,
			modules: []string{
				`package test
				p[1]
				p[2]
				p[3]`,
			},
			wantQueries: []string{
				`input.x = 1; x = 1`,
				`input.x = 2; x = 2`,
				`input.x = 3; x = 3`,
			},
		},
		{
			note:  "iterate keys: sets",
			query: `input = x; s = {1,2}; s[x] = y`,
			wantQueries: []string{
				`input = 1; s = {1, 2}; x = 1; y = 1`,
				`input = 2; s = {1, 2}; x = 2; y = 2`,
			},
		},
		{
			note:  "iterate keys: objects",
			query: `input = x; o = {"a": 1, "b": 2}; o[x] = y`,
			wantQueries: []string{
				`input = "a"; o = {"a": 1, "b": 2}; x = "a"; y = 1`,
				`input = "b"; o = {"a": 1, "b": 2}; x = "b"; y = 2`,
			},
		},
		{
			note:  "iterate keys: saved",
			query: `x = input; y = [x]; z = y[i][j] `,
			wantQueries: []string{
				`x = input; x[j] = z; y = [x]; i = 0`,
			},
		},
		{ // TODO: duplicate for general refs?
			note:  "single term: save",
			query: `input.x = x; data.test.p[x]`,
			modules: []string{
				`package test
				p[y] { y = "foo" }
				p[z] { z = "bar" }`,
			},
			wantQueries: []string{
				`input.x = "foo"; x = "foo"`,
				`input.x = "bar"; x = "bar"`,
			},
		},
		{
			note:  "single term: false save",
			query: `input = x; x = false; x`, // last expression must be preserved
			wantQueries: []string{
				`input = false; false; x = false`,
			},
		},
		{
			note:  "reference: partial object",
			query: "data.test.p[x].foo = 1",
			modules: []string{
				`package test
				p[x] = {y: z} { x = input.x; y = "foo"; z = 1 }
				p[x] = {y: z} { x = input.y; y = "bar"; z = 2 }`,
			},
			wantQueries: []string{
				`x = input.x`,
			},
		},
		{
			note:  "reference: partial object, general ref",
			query: "data.test.p[x].q.foo = 1",
			modules: []string{
				`package test
				p[a].q = {b: c} { a = input.a; b = "foo"; c = 1 }
				p[a].q = {b: c} { a = input.b; b = "bar"; c = 2 }`,
			},
			wantQueries: []string{
				`x = input.a`,
			},
		},
		{
			note:  "reference: partial object, general ref (2)",
			query: "data.test.p[x].q.foo",
			modules: []string{
				`package test
				p[a].q = {b: c} { a = input.a; b = "foo"; c = 1 }
				p[a].q = {b: c} { a = input.b; b = "bar"; c = 2 }`,
			},
			wantQueries: []string{
				`x = input.a`,
			},
		},
		{
			note:  "reference: partial object, general ref (3)",
			query: "data.test.p[x].q",
			modules: []string{
				`package test
				p[a].q = {b: c} { a = input.a; b = "foo"; c = 1 }
				p[a].q = {b: c} { a = input.b; b = "bar"; c = 2 }`,
			},
			wantQueries: []string{
				`x = input.a`,
				`x = input.b`,
			},
		},
		{
			note:  "reference: partial object, general ref (4)",
			query: "data.test.p[x].q[y]",
			modules: []string{
				`package test
				p[a].q = {b: c} { a = input.a; b = "foo"; c = 1 }
				p[a].q = {b: c} { a = input.b; b = "bar"; c = 2 }`,
			},
			wantQueries: []string{
				`x = input.a; y = "foo"`,
				`x = input.b; y = "bar"`,
			},
		},
		{
			note:  "reference: partial object, general ref (5)",
			query: "data.test.p[x].q[y] = z",
			modules: []string{
				`package test
				p[a].q = {b: c} { a = input.a; b = "foo"; c = 1 }
				p[a].q = {b: c} { a = input.b; b = "bar"; c = 2 }`,
			},
			wantQueries: []string{
				`x = input.a; y = "foo"; z = 1`,
				`x = input.b; y = "bar"; z = 2`,
			},
		},
		{
			note:  "reference: partial object, general ref (6)",
			query: "data.test.p = z",
			modules: []string{
				`package test
				p[a].q = {b: c} { a = input.a; b = "foo"; c = 1 }
				p[a].q = {b: c} { a = input.b; b = "bar"; c = 2 }`,
			},
			wantQueries: []string{
				`data.partial.test.p = z`,
			},
			wantSupport: []string{
				`package partial.test
				p[a2].q = {"foo": 1} { a2 = input.a }
				p[a1].q = {"bar": 2} { a1 = input.b }`,
			},
		},
		{
			note:  "reference: partial object, general ref (7)",
			query: "data.test.p[x] = z",
			modules: []string{
				`package test
				p[a].q = {b: c} { a = input.a; b = "foo"; c = 1 }
				p[a].q = {b: c} { a = input.b; b = "bar"; c = 2 }`,
			},
			wantQueries: []string{
				`x = input.a; z = {"q": {"foo": 1}}`,
				`x = input.b; z = {"q": {"bar": 2}}`,
			},
		},
		{
			note:  "reference: partial object, general ref (8)",
			query: "data.test.p = z",
			modules: []string{
				`package test
				p[a].q = {b: c} { a = input.a; b = "foo"; c = 1 }
				p[a].q = {b: c} { a = input.b; b = "bar"; c = 2 }
				p.foo.r = a { a = "baz" }
				p.foo.s = a { a = input.c }`,
			},
			wantQueries: []string{
				`data.partial.test.p = z`,
			},
			wantSupport: []string{
				`package partial.test
				p[a4].q = {"foo": 1} { a4 = input.a }
				p[a3].q = {"bar": 2} { a3 = input.b }
				p.foo.r = "baz" { true }
				p.foo.s = a2 { a2 = input.c }`,
			},
		},
		{
			note:  "reference: partial object, general ref (9)",
			query: "data.test.p[x] = z",
			modules: []string{
				`package test
				p[a].q = {b: c} { a = input.a; b = "foo"; c = 1 }
				p[a].q = {b: c} { a = input.b; b = "bar"; c = 2 }
				p.foo.r = a { a = "baz" }
				p.foo.s = a { a = input.c }`,
			},
			wantQueries: []string{
				`x = input.a; z = {"q": {"foo": 1}}`,
				`x = input.b; z = {"q": {"bar": 2}}`,
				`x = "foo"; z = {"r": "baz"}`,
				`z = {"s": input.c}; x = "foo"`,
			},
		},
		{
			note:  "reference: partial object, general ref, multiple vars",
			query: `data.test.p = x`,
			modules: []string{
				`package test
				p[q].r[s] := v { v := "foo"; q := 42; s := "bar" }
				p[q].r[s].t := v { v := input.x; q := input.y; s := "baz" }`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				p[42].r.bar = "foo" { true }
				p[__local4__2].r.baz.t = __local3__2 { __local3__2 = input.x; __local4__2 = input.y }`,
			},
		},
		{
			note:  "reference: partial object, general ref, multiple vars (2)",
			query: `data.test.p[42] = x`,
			modules: []string{
				`package test
				p[q].r[s] := v { v := "foo"; q := 42; s := "bar" }
				p[q].r[s].t := v { v := input.x; q := input.y; s := "baz" }`,
			},
			wantQueries: []string{
				`x = {"r": {"bar": "foo"}}`,
				`42 = input.y; x = {"r": {"baz": {"t": input.x}}}`,
			},
		},
		{
			note:    "reference: partial object, general ref, multiple vars (2) (shallow)",
			query:   `data.test.p[42] = x`,
			shallow: true,
			modules: []string{
				`package test
				#p[q].r[s] := v { v := "foo"; q := 42; s := "bar" }
				#p[q].r[s].t := v { v := input.x; q := input.y; s := "baz" }
				p[q][r][s].t := v { v := input.x; q := input.y; s := input.z; r := "known" }`,
			},
			wantQueries: []string{
				`data.partial.test.p[42] = x`,
			},
			wantSupport: []string{
				`package partial.test
				p[__local1__1].known[__local2__1].t = __local0__1 { __local0__1 = input.x; __local1__1 = input.y; __local2__1 = input.z }`,
			},
		},
		{
			note:  "reference: partial set",
			query: "data.test.p[x].foo = 1",
			modules: []string{
				`package test
				p[x] { x = {y: z}; y = "foo"; z = input.x }
				p[x] { x = {y: z}; y = "bar"; z = input.x }`,
			},
			wantQueries: []string{
				`1 = input.x; x = {"foo": 1}`,
			},
		},
		{
			note:  "reference: partial set, general ref",
			query: "data.test.p[x][y].foo = 1",
			modules: []string{
				`package test
				import future.keywords.contains
				p[x] contains y { y = {a: b}; a = "foo"; b = input.x; x := 42 }
				p[x] contains y { y = {a: b}; a = "bar"; b = input.x; x := input.y }`,
			},
			wantQueries: []string{
				`1 = input.x; y = {"foo": 1}; x = 42`,
			},
		},
		{
			note:  "reference: partial set, general ref (2)",
			query: "data.test.p[x][y].bar = 1",
			modules: []string{
				`package test
				import future.keywords.contains
				p[x] contains y { y = {a: b}; a = "foo"; b = input.x; x = 42 }
				p[x] contains y { y = {a: b}; a = "bar"; b = input.x; x = input.y }`,
			},
			wantQueries: []string{
				`1 = input.x; x = input.y; y = {"bar": 1}`,
			},
		},
		{
			note:  "reference: partial set, general ref (3)",
			query: "data.test.p[42][y].foo = 1",
			modules: []string{
				`package test
				import future.keywords.contains
				p[x] contains y { y = {a: b}; a = "foo"; b = input.x; x := 42 }
				p[x] contains y { y = {a: b}; a = "bar"; b = input.x; x := input.y }`,
			},
			wantQueries: []string{
				`1 = input.x; y = {"foo": 1}`,
			},
		},
		{
			note:  "reference: partial set, general ref (4)",
			query: `data.test.p[x][y] = {"foo": 1}`,
			modules: []string{
				`package test
				import future.keywords.contains
				p[x] contains y { y = {a: b}; a = "foo"; b = input.x; x := 42 }
				p[x] contains y { y = {a: b}; a = "bar"; b = input.x; x := input.y }`,
			},
			wantQueries: []string{
				`1 = input.x; y = {"foo": 1}; x = 42`,
			},
		},
		{
			note:  "reference: partial set, general ref (5)",
			query: `data.test.p[x] = {{"foo": 1}}`,
			modules: []string{
				`package test
				import future.keywords.contains
				p[x] contains y { y = {a: b}; a = "foo"; b = input.x; x := 42 }
				p[x] contains y { y = {a: b}; a = "bar"; b = input.x; x := input.y }`,
			},
			wantQueries: []string{
				`{{"foo": input.x}} = {{"foo": 1}}; x = 42`, // `1 = input.x; x = 42` would be a more precise optimization (?)
			},
		},
		{
			note:  "reference: partial set, general ref (6)",
			query: `data.test.p`,
			modules: []string{
				`package test
				import future.keywords.contains
				p[x] contains y { y = {a: b}; a = "foo"; b = input.x; x := 42 }
				p[x] contains y { y = {a: b}; a = "bar"; b = input.x; x := input.y }`,
			},
			wantQueries: []string{
				`data.partial.test.p`,
			},
			wantSupport: []string{
				`package partial.test
				import future.keywords.contains
				p[42] contains {"foo": b1} { b1 = input.x }
				p[__local1__2] contains {"bar": b2} { b2 = input.x; __local1__2 = input.y }`,
			},
		},
		{
			note:  "reference: partial set, general ref (7)",
			query: `data.test.p = x`,
			modules: []string{
				`package test
				import future.keywords.contains
				p[x] contains y { y = {a: b}; a = "foo"; b = input.x; x := 42 }
				p[x] contains y { y = {a: b}; a = "bar"; b = input.x; x := input.y }`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				import future.keywords.contains
				p[42] contains {"foo": b1} { b1 = input.x }
				p[__local1__2] contains {"bar": b2} { b2 = input.x; __local1__2 = input.y }`,
			},
		},
		{
			note:  "reference: partial set, general ref (8)",
			query: `data.test.p = x`,
			modules: []string{
				`package test
				import future.keywords.contains
				p[x].r contains y { y = {a: b}; a = "foo"; b = input.x; x := 42 }
				p[x].r contains y { y = {a: b}; a = "bar"; b = input.x; x := input.y }`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				import future.keywords.contains
				p[42].r contains {"foo": b1} { b1 = input.x }
				p[__local1__2].r contains {"bar": b2} { b2 = input.x; __local1__2 = input.y }`,
			},
		},
		{
			note:  "reference: partial set, general ref, multiple vars",
			query: `data.test.p = x`,
			modules: []string{
				`package test
				import future.keywords.contains
				p[q].r[s] contains x { x = "foo"; q := 42; s = "bar" }
				p[q].r[s].t contains x { x = input.x; q := input.y; s = "baz" }`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				import future.keywords.contains
				p[42].r.bar contains "foo" { true }
				p[__local1__2].r.baz.t contains x2 { x2 = input.x; __local1__2 = input.y }`,
			},
		},
		{
			note:  "reference: partial set, general ref, multiple vars (2)",
			query: `data.test.p[42] = x`,
			modules: []string{
				`package test
				import future.keywords.contains
				p[q].r[s] contains v { v := "foo"; q := 42; s := "bar" }
				p[q].r[s].t contains v { v := input.x; q := input.y; s := "baz" }`,
			},
			wantQueries: []string{
				`x = {"r": {"bar": {"foo"}}}`,
				`42 = input.y; x = {"r": {"baz": {"t": {input.x}}}}`,
			},
		},
		{
			note:  "reference: partial set, general ref, multiple vars (3)",
			query: `data.test.p.foo = x`,
			modules: []string{
				`package test
				import future.keywords.contains
				p[q].r[s] contains x { x = "foo"; q := 42; s = "bar" }
				p[q].r[s].t contains x { x = input.x; q := input.y; s = "baz" }`,
			},
			wantQueries: []string{
				`"foo" = input.y; x = {"r": {"baz": {"t": {input.x}}}}`,
			},
		},
		{
			note:  "reference: partial object, unknown in query ref",
			query: "data.test.p[input.x]",
			modules: []string{
				`package test
				p[q].r[s] = v { q = {"foo", "bar"}[s]; v = "baz" }
				p.q.r.s := 1`,
			},
			wantQueries: []string{
				`"foo" = input.x`,
				`"bar" = input.x`,
				`"q" = input.x`,
			},
		},
		{
			note:  "reference: partial object, unknown in query ref (2)",
			query: "data.test.p.foo.r[input.x]",
			modules: []string{
				`package test
				p[q].r[s] = v { q = {"foo", "bar"}[s]; v = "baz" }
				p.q.r.s := 1`,
			},
			wantQueries: []string{
				`"foo" = input.x`,
			},
		},
		{
			note:  "reference: partial object, unknown in query ref (3)",
			query: "data.test.p[input.x].r[input.y]",
			modules: []string{
				`package test
				p[q].r[s] = v { q = {"foo", "bar"}[s]; v = "baz" }
				p.q.r.s := 1`,
			},
			wantQueries: []string{
				`"foo" = input.x; "foo" = input.y`,
				`"bar" = input.x; "bar" = input.y`,
				`"q" = input.x; "s" = input.y`,
			},
		},
		{
			note:  "reference: partial object, unknown in query ref (4)",
			query: "data.test.p[x].r[y][input.x]",
			modules: []string{
				`package test
				p[q].r[s] = {v: w} { q = {"foo", "bar"}[s]; v = "baz"; w = "bax" }
				p.q.r.s := {1: 2}`,
			},
			wantQueries: []string{
				`"baz" = input.x; x = "foo"; y = "foo"`,
				`"baz" = input.x; x = "bar"; y = "bar"`,
				`1 = input.x; x = "q"; y = "s"`,
			},
		},
		{
			note:  "reference: partial object, unknown in query ref (5)",
			query: "data.test.p[x].r[y][input.x] = input.y",
			modules: []string{
				`package test
				p[q].r[s] = {v: w} { q = {"foo", "bar"}[s]; v = "baz"; w = "bax" }
				p.q.r.s := {1: 2}`,
			},
			wantQueries: []string{
				`"baz" = input.x; "bax" = input.y; x = "foo"; y = "foo"`,
				`"baz" = input.x; "bax" = input.y; x = "bar"; y = "bar"`,
				`1 = input.x; 2 = input.y; x = "q"; y = "s"`,
			},
		},
		{
			note:  "reference: partial object, unknown in query ref (6)",
			query: `data.test.p[x].r[y][input.x] = "bax"`,
			modules: []string{
				`package test
				p[q].r[s] = {v: w} { q = {"foo", "bar"}[s]; v = "baz"; w = "bax" }
				p.q.r.s := {1: 2}`,
			},
			wantQueries: []string{
				`"baz" = input.x; x = "foo"; y = "foo"`,
				`"baz" = input.x; x = "bar"; y = "bar"`,
			},
		},
		{
			note:  "reference: partial object, unknown in query ref (7)",
			query: `data.test.p[x].r[y][input.x] = 2`,
			modules: []string{
				`package test
				p[q].r[s] = {v: w} { q = {"foo", "bar"}[s]; v = "baz"; w = "bax" }
				p.q.r.s := {1: 2}`,
			},
			wantQueries: []string{
				`1 = input.x; x = "q"; y = "s"`,
			},
		},
		{
			note:  "reference: complete",
			query: "data.test.p = 1",
			modules: []string{
				`package test

				p = x { input.x = x }`,
			},
			wantQueries: []string{
				`input.x = 1`,
			},
		},
		{
			note:  "reference: complete, ref head",
			query: "data.test.p.q = 1",
			modules: []string{
				`package test

				p.q = x { input.x = x }`,
			},
			wantQueries: []string{
				`input.x = 1`,
			},
		},
		{
			note:  "reference: complete: suffix",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p {
					a = 1
					q[a]
				}

				q = a {
					a = input
				}`,
			},
			wantQueries: []string{`input[1]`},
		},
		{
			note:  "reference: complete: suffix: ensure unique var",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p {
					a = 1
					b = 2
					q[a] = r[b]
				}

				q = a {
					a = input.a
				}

				r = b {
					b = input.b
				}`,
			},
			wantQueries: []string{`input.b[2] = input.a[1]`},
		},
		{
			note:  "reference: head: from query",
			query: "data.test.p[y] = 1",
			modules: []string{
				`package test

				p[x] = 1 {
					input.foo[x] = z
					x.bar = 1
				}
				`,
			},
			wantQueries: []string{
				`y.bar = 1; z1 = input.foo[y]`,
			},
		},
		{
			note:  "reference: ref head: from query",
			query: "data.test.p.q[y] = 1",
			modules: []string{
				`package test

				p.q[x] = 1 {
					input.foo[x] = z
					x.bar = 1
				}
				`,
			},
			wantQueries: []string{
				`y.bar = 1; z1 = input.foo[y]`,
			},
		},
		{
			note:  "reference: general ref head: from query",
			query: "data.test.p.q[y].s = 1",
			modules: []string{
				`package test

				p.q[x].s = 1 {
					input.foo[x] = z
					x.bar = 1
				}
				`,
			},
			wantQueries: []string{
				`y.bar = 1; z1 = input.foo[y]`,
			},
		},
		{
			note:  "reference: head: applied",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p {
					q[x]
					x.a = 1
				}

				q[x] {
					input[x]
					x.b = 2
				}`,
			},
			// FIXME: is this a problem?
			wantQueries: []string{`
				input[x_ref_01]
				x_ref_01.b = 2
				x_ref_01
				x_ref_01.a = 1
			`},
		},
		{
			note:  "reference: default not required",
			query: "data.test.p = true",
			modules: []string{
				`package test

				default p = false
				p {
					input.x = 1
				}`,
			},
			wantQueries: []string{`input.x = 1`},
		},
		{
			note:  "namespace: complete",
			query: "data.test.p = x",
			modules: []string{
				`package test
				 p = 1 { input.y = x; x = 2 }`,
			},
			wantQueries: []string{
				`input.y = 2; x = 1`,
			},
		},
		{
			note:  "namespace: complete head",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p = x { input.x = x }`,
			},
			wantQueries: []string{
				`input.x = x`,
			},
		},
		{
			note:  "namespace: partial set",
			query: "data.test.p[[x, y]]",
			modules: []string{
				`package test
				p[[y, x]] { input.z = z; z = y; a = input.a; a = x }`,
			},
			wantQueries: []string{
				`input.z = x; y = input.a; x_term_0_0 = [x, y]`,
			},
		},
		{
			note:  "namespace: partial object",
			query: "input.x = x; data.test.p[x] = y; y = 2",
			modules: []string{
				`package test
				p[y] = x { y = "foo"; x = 2 }`,
			},
			wantQueries: []string{
				`input.x = "foo"; x = "foo"; y = 2`,
			},
		},
		{
			note:  "namespace: partial object, ref head",
			query: "input.x = x; data.test.p.q[x] = y; y = 2",
			modules: []string{
				`package test
				p.q[y] = x { y = "foo"; x = 2 }`,
			},
			wantQueries: []string{
				`input.x = "foo"; x = "foo"; y = 2`,
			},
		},
		{
			note:  "namespace: partial object, general ref head",
			query: "input.x = x; input.y = y; data.test.p.q[x][y] = z; z = 2",
			modules: []string{
				`package test
				p.q[x][y] = z { x = "foo"; y = "bar"; z = 2 }`,
			},
			wantQueries: []string{
				`input.x = "foo"; input.y = "bar"; x = "foo"; y = "bar"; z = 2`,
			},
		},
		{
			note:  "namespace: embedding",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p = x { input.x = [y]; y = x }`,
			},
			wantQueries: []string{
				`input.x = [x]`,
			},
		},
		{
			note:  "namespace: multiple",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p = [x, z] { input.x = y; y = x; q = z }
				q = x { input.y = y; x =  y }`,
			},
			wantQueries: []string{
				`x = [input.x, input.y]`,
			},
		},
		{
			note:  "namespace: calls",
			query: "data.test.p = x",
			modules: []string{
				`package test

				p {
					a = "a"
					b = input.b
					a != b
				}
				`,
			},
			wantQueries: []string{
				`"a" != input.b; x = true`,
			},
		},
		{
			note:  "namespace: reference head",
			query: "data.test.p = x",
			modules: []string{
				`package test

				p {
					input = x
					x.foo = true
				}`,
			},
			wantQueries: []string{
				`input.foo = true; x = true`,
			},
		},
		{
			note:  "namespace: reference head: from caller",
			query: "data.test.p[x] = 1",
			modules: []string{
				`package test

				p[x] = 1 {
					x = input
					x[0] = 1
				}
				`,
			},
			wantQueries: []string{
				`x = input; x[0] = 1`,
			},
		},
		{
			note:  "namespace: function with call composite result (array, nested)",
			query: `data.test.foo(input, [[x, _]]); startswith(x, "foo")`,
			modules: []string{
				`package test
				foo(x) = o {
				  o := [[x.x, x.y]]
				}
				`},
			wantQueries: []string{
				`x = input.x; _ = input.y; startswith(x, "foo")`,
			},
		},
		{
			note:  "namespace: function with call composite result (object)",
			query: `data.test.foo(input, {"x": x}); startswith(x, "foo")`,
			modules: []string{
				`package test
				foo(x) = o {
				  o := { "x": x.y }
				}
				`},
			wantQueries: []string{
				`x = input.y; startswith(x, "foo")`,
			},
		},
		{
			note:  "namespace: function with call composite result (object, nested)",
			query: `data.test.foo(input, {"x": [y, z]}); startswith(y, "foo")`,
			modules: []string{
				`package test
				foo(y) = z {
				  z := { "x": [y.y, y.z] }
				}
				`},
			wantQueries: []string{
				`y = input.y; z = input.z; startswith(y, "foo")`,
			},
		},
		{
			note:  "namespace: function with call composite result (array/object, mixed)",
			query: `data.test.foo(input, {"x": [ { "a": y }, _]}); startswith(y, "foo")`,
			modules: []string{
				`package test
				foo(y) = o {
				  o := { "x": [ {"a": y.y }, y.z] }
				}
				`},
			wantQueries: []string{
				`y = input.y; _ = input.z; startswith(y, "foo")`,
			},
		},
		{
			note:  "ignore conflicts: complete",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p = true { input.x = 1 }
				p = false { input.x = 2 }`,
			},
			wantQueries: []string{
				`input.x = 1; x = true`,
				`input.x = 2; x = false`,
			},
		},
		{
			note:  "ignore conflicts: functions",
			query: "data.test.f(1, x)",
			modules: []string{
				`package test
				f(x) = true { input.x = x }
				f(x) = false { input.y = x }`,
			},
			wantQueries: []string{
				`input.x = 1; x = true`,
				`input.y = 1; x = false`,
			},
		},
		{
			note:  "ignore conflicts: functions: unknowns",
			query: "data.test.f(input) = x",
			modules: []string{
				`package test
				f(x) = true { x = 1 }
				f(x) = false { x = 2 }
				`,
			},
			wantQueries: []string{
				`1 = input; x = true`,
				`2 = input; x = false`,
			},
		},
		{
			note:        "comprehensions: evaluated",
			query:       `x = [true | true]; y = {true | true}; z = {a: true | a = "foo"}`,
			wantQueries: []string{`x = [true]; y = {true}; z = {"foo": true}`},
		},
		{
			note:        "comprehensions: saved",
			query:       `x = [true | input.x = 1]`,
			wantQueries: []string{`x = [true | input.x = 1]`},
		},
		{
			note:  "comprehensions: saved (with namespacing)",
			query: "data.test.p = x; data.test.p = y",
			modules: []string{
				`package test

				p = c {
					a = input
					c = [1 | b = a[0]]
				}
			`},
			wantQueries: []string{`x = [1 | b1 = input[0]]; y = [1 | b2 = input[0]]`},
		},
		{
			note:        "comprehensions: closure",
			query:       `i = 1; xs = [x | x = data.foo[i]]`,
			wantQueries: []string{`i = 1; xs = ["b"]`},
			data:        `{"foo": ["a", "b", "c"]}`,
		},
		{
			note:        "comprehensions: closure saved",
			query:       `i = 1; xs = [x | x = input.foo[i]]`,
			wantQueries: []string{`xs = [x | x = input.foo[1]; 1 = 1]; i = 1`},
		},
		{
			note:  "tree: no unknown dependencies",
			query: "data.test = x",
			modules: []string{
				`package test.a
				p = 1`,
				`package test
				q["a"] = 2`,
				`package test.b
				r[1]`,
			},
			wantQueries: []string{`x = {"a": {"p": 1}, "q": {"a": 2}, "b": {"r": {1,}}}`},
		},
		{
			note:  "with: disabled inlining",
			query: "data.test.p = true",
			modules: []string{
				`package test
				p { input.x = 1; q with input as {"y": 2} }
				q { input.y = r }
				r = 2`,
			},
			wantQueries: []string{
				`input.x = 1; data.partial.test.q = x_term_1_11 with input as {"y": 2}; x_term_1_11 with input as {"y": 2}`,
			},
			wantSupport: []string{
				`package partial.test
				q = true { 2 = input.y }`,
			},
		},
		{
			note:  "with: no unknowns",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { q[x] = y with input as 1 }
				q[y] { x = 1; y = x }
				q[2]`,
			},
			wantQueries: []string{
				`data.partial.test.q[x1] = y1 with input as 1`,
			},
			wantSupport: []string{
				`package partial.test

				q[1]
				q[2]`,
			},
		},
		{
			note:  "with: iteration",
			query: `data.test.p = true`,
			modules: []string{
				`package test

				p { q = true with input as 1 }
				q { r[x] = input }
				r[1]
				r[2]`,
			},
			wantQueries: []string{
				`data.partial.test.q = true with input as 1`,
			},
			wantSupport: []string{
				`package partial.test

				q { 1 = input }
				q { 2 = input }`,
			},
		},
		{
			note:  "with: unknown value",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { input.x = z; [z] = x; q with data.foo as x }
				q { data.foo = [1] }`,
			},
			wantQueries: []string{"data.test.q with data.foo as [input.x]"},
		},
		{
			note:  "with: unknown value propagates to outputs (eq)",
			query: "data.test.p = z",
			modules: []string{
				`package test

				q = 1 { input.foo = 1 }
				p = y { x = q with data.bar as input.bar; plus(x, 1, y) }`,
			},
			wantQueries: []string{"x1 = data.test.q with data.bar as input.bar; plus(x1, 1, z)"},
		},
		{
			note:  "with: unknown value propagates to outputs (ref)",
			query: "data.test.p = z",
			modules: []string{
				`package test

				q[1] { input.foo = 1 }
				p = y { q[x] with data.bar as input.bar; plus(x, 1, y) }`,
			},
			wantQueries: []string{"data.test.q[x1] with data.bar as input.bar; plus(x1, 1, z)"},
		},
		{
			note:  "with: unknown value propagates to outputs (call)",
			query: "data.test.p = z",
			modules: []string{
				`package test

				f(t) = 1 { input.foo = t }
				p = y { f(1, x) with data.bar as input.bar; plus(x, 1, y) }`,
			},
			wantQueries: []string{"data.test.f(1, x1) with data.bar as input.bar; plus(x1, 1, z)"},
		},
		{
			note:  "with: unknown value propagates to outputs (built-in)",
			query: "data.test.p = z",
			modules: []string{
				`package test

				p = y { time.now_ns(x) with data.bar as input.bar; plus(x, 1, y) }`,
			},
			wantQueries: []string{"time.now_ns(x1) with data.bar as input.bar; plus(x1, 1, z)"},
		},
		{
			note:  "with: ground prefix disabled",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { q[1] = 1 with input as 1 }
				q[x] { x = 1 }`,
			},
			wantQueries: []string{`data.partial.test.q[1] = 1 with input as 1`},
			wantSupport: []string{
				`package partial.test

				q[1]`,
			},
		},
		{
			note:  "with: ground prefix disabled with var",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { q[x] = 1 with input as 1 }
				q[x] { x = 1 }`,
			},
			wantQueries: []string{`data.partial.test.q[x1] = 1 with input as 1`},
			wantSupport: []string{
				`package partial.test

				q[1]`,
			},
		},
		{
			note:    "with+shallow: partial set elem",
			shallow: true,
			query:   "data.test.p = a",
			modules: []string{
				`package test

				p[x] { q[x] }
				q[7] { false with input as 1 }`,
			},
			wantQueries: []string{`set() = a`},
		},
		{
			note:    "with+shallow: partial obj key",
			shallow: true,
			query:   "data.test.p = a",
			modules: []string{
				`package test

				p[x] = y { q[x] = y }
				q[7] = 8 { false with input as 1 }`,
			},
			wantQueries: []string{`{} = a`},
		},
		{
			note:  "with+builtin: no unknowns",
			query: "data.test.p = a",
			modules: []string{
				`package test

				mock_concat(_, _) = "foo/bar"
				p { q with concat as mock_concat }
				q { concat("/", ["a", "b"], "foo/bar") }`,
			},
			wantQueries: []string{`a = true`},
		},
		{
			note:  "with+builtin: value replacement",
			query: "data.test.p = a",
			modules: []string{
				`package test

				p { q with concat as "foo/bar" }
				q { concat("/", ["a", "b"], "foo/bar") }`,
			},
			wantQueries: []string{`a = true`},
		},
		{
			note:  "with+function: no unknowns",
			query: "data.test.p = a",
			modules: []string{
				`package test
				f(_, _) = "x"
				mock_f(_, _) = "foo/bar"
				p { q with f as mock_f }
				q { f("/", ["a", "b"], "foo/bar") }`,
			},
			wantQueries: []string{`a = true`},
		},
		{
			note:  "with+function: value replacement",
			query: "data.test.p = a",
			modules: []string{
				`package test
				f(_, _) = "x"
				p { q with f as "foo/bar" }
				q { f("/", ["a", "b"], "foo/bar") }`,
			},
			wantQueries: []string{`a = true`},
		},
		{
			note:  "with+builtin: unknowns in replacement function",
			query: "data.test.p = a",
			modules: []string{
				`package test

				mock_concat(x, _) = concat(x, input)
				p { q with concat as mock_concat}
				q { concat("/", ["a", "b"], "foo/bar") }`,
			},
			wantQueries: []string{`data.partial.test.mock_concat("/", ["a", "b"], "foo/bar"); a = true`},
			wantSupport: []string{
				`package partial.test

				mock_concat(__local0__3, __local1__3) = __local2__3 {
					__local3__3 = input
					concat(__local0__3, __local3__3, __local2__3)
				}`,
			},
		},
		{
			note:  "with+function: unknowns in replacement function",
			query: "data.test.p = a",
			modules: []string{
				`package test
				f(_) = "x/y"
				mock_f(_) = "foo/bar" { input.y }
				p { q with f as mock_f}
				q { f("/", "foo/bar") }`,
			},
			wantQueries: []string{`data.partial.test.mock_f("/", "foo/bar"); a = true`},
			wantSupport: []string{
				`package partial.test

				mock_f(__local1__3) = "foo/bar" {
					input.y = x_term_3_03
					x_term_3_03
				}`,
			},
		},
		{
			note:  "with+builtin: unknowns in replaced function's args",
			query: "data.test.p = a",
			modules: []string{
				`package test

				mock_concat(_, _) = ["foo", "bar"]
				p {
					q with array.concat as mock_concat
				}
				q {
					array.concat(["foo"], input, ["foo", "bar"])
				}`,
			},
			wantQueries: []string{`
				data.partial.test.q
				a = true
			`},
			wantSupport: []string{`package partial.test

				q {
					data.partial.test.mock_concat(["foo"], input, ["foo", "bar"])
				}
				mock_concat(__local0__3, __local1__3) = ["foo", "bar"]
			`},
		},
		{
			note:  "with+function: unknowns in replaced function's args",
			query: "data.test.p = a",
			modules: []string{
				`package test
				my_concat(x, y) = concat(x, y)
				mock_concat(_, _) = "foo,bar"
				p {
					q with my_concat as mock_concat
				}
				q {
					my_concat("/", input, "foo,bar")
				}`,
			},
			wantQueries: []string{`
				data.partial.test.q
				a = true
			`},
			wantSupport: []string{`package partial.test

				q {
					data.partial.test.mock_concat("/", input, "foo,bar")
				}
				mock_concat(__local2__3, __local3__3) = "foo,bar"
			`},
		},
		{
			note:  "with+builtin: unknowns in replacement function's bodies",
			query: "data.test.p = a",
			modules: []string{
				`package test

				mock_concat(_, _) = ["foo", "bar"] { input.foo }
				mock_concat(_, _) = ["bar", "baz"] { input.bar }

				p { q with array.concat as mock_concat }
				q { x := array.concat(["foo"], input) }`,
			},
			wantQueries: []string{`
				data.partial.test.q
				a = true
			`},
			wantSupport: []string{`package partial.test

			q {
				__local6__2 = input
				data.partial.test.mock_concat(["foo"], __local6__2, __local5__2)
				__local4__2 = __local5__2
			}
			mock_concat(__local0__3, __local1__3) = ["foo", "bar"] {
				input.foo = x_term_3_03
				x_term_3_03
			}
			mock_concat(__local2__4, __local3__4) = ["bar", "baz"] {
				input.bar = x_term_4_04
				x_term_4_04
			}`},
		},
		{
			note:  "with+function: unknowns in replacement function's bodies",
			query: "data.test.p = a",
			modules: []string{
				`package test
				my_concat(x, y) = concat(x, y)
				mock_concat(_, _) = "foo,bar" { input.foo }
				mock_concat(_, _) = "bar,baz" { input.bar }

				p { q with my_concat as mock_concat }
				q { x := my_concat(",", input) }`,
			},
			wantQueries: []string{`
				data.partial.test.q
				a = true
			`},
			wantSupport: []string{`package partial.test

			q {
				__local9__2 = input
				data.partial.test.mock_concat(",", __local9__2, __local8__2)
				__local6__2 = __local8__2
			}
			mock_concat(__local2__3, __local3__3) = "foo,bar" {
				input.foo = x_term_3_03
				x_term_3_03
			}
			mock_concat(__local4__4, __local5__4) = "bar,baz" {
				input.bar = x_term_4_04
				x_term_4_04
			}`},
		},
		{
			note:  "with+builtin+negation: when replacement has no unknowns (args, defs), save negated expr without replacement",
			query: "data.test.p = true",
			modules: []string{`
				package test

				mock_count(_) = 100
				p {
					not q with input.x as 1 with count as mock_count
				}

				q {
					count([1,2,3]) = input.x
				}
			`},
			wantQueries: []string{"not data.partial.test.q with input.x as 1"},
			wantSupport: []string{`
				package partial.test

				q { 100 = input.x }
			`},
		},
		{
			note:  "with+function+negation: when replacement has no unknowns (args, defs), save negated expr without replacement",
			query: "data.test.p = true",
			modules: []string{`
				package test
				my_count(x) = count(x)
				mock_count(_) = 100
				p {
					not q with input.x as 1 with my_count as mock_count
				}

				q {
					my_count([1,2,3]) = input.x
				}
			`},
			wantQueries: []string{"not data.partial.test.q with input.x as 1"},
			wantSupport: []string{`
				package partial.test

				q { 100 = input.x }
			`},
		},
		{
			note:  "with+builtin+negation: when replacement args have unknowns, save negated expr with replacement",
			query: "data.test.p = true",
			modules: []string{`
				package test

				mock_count(_) = 100
				p {
					not q with input.x as 1 with count as mock_count
				}

				q {
					count(input.y) = input.x # unknown arg for mocked func
				}
			`},
			wantQueries: []string{"not data.partial.test.q with input.x as 1"},
			wantSupport: []string{`
				package partial.test

				q { data.partial.test.mock_count(input.y, __local1__3); __local1__3 = input.x }
				mock_count(__local0__4) = 100 
			`},
		},
		{
			note:  "with+function+negation: when replacement args have unknowns, save negated expr with replacement",
			query: "data.test.p = true",
			modules: []string{`
				package test
				my_count(x) = count(x)
				mock_count(_) = 100
				p {
					not q with input.x as 1 with my_count as mock_count
				}

				q {
					my_count(input.y) = input.x # unknown arg for mocked func
				}
			`},
			wantQueries: []string{`not data.partial.test.q with input.x as 1`},
			wantSupport: []string{`
				package partial.test

				q { data.partial.test.mock_count(input.y, __local3__3); __local3__3 = input.x }
				mock_count(__local1__4) = 100
			`},
		},
		{
			note:  "with+builtin+negation: when replacement defs have unknowns, save negated expr with replacement",
			query: "data.test.p = true",
			modules: []string{`
				package test

				mock_count(_) = 100 { input.y }
				mock_count(_) = 101 { input.z }
				p {
					not q with input.x as 1 with count as mock_count
				}

				q {
					count([1]) = input.x # unknown arg for mocked func
				}
			`},
			wantQueries: []string{"not data.partial.test.q with input.x as 1"},
			wantSupport: []string{`
				package partial.test

				q { data.partial.test.mock_count([1], __local2__3); __local2__3 = input.x }
				mock_count(__local0__4) = 100 { input.y = x_term_4_04; x_term_4_04 }
				mock_count(__local1__5) = 101 { input.z = x_term_5_05; x_term_5_05 }
			`},
		},
		{
			note:  "with+function+negation: when replacement defs have unknowns, save negated expr with replacement",
			query: "data.test.p = true",
			modules: []string{`
				package test
				my_count(x) = count(x)
				mock_count(_) = 100 { input.y }
				mock_count(_) = 101 { input.z }
				p {
					not q with input.x as 1 with my_count as mock_count
				}

				q {
					my_count([1]) = input.x # unknown arg for mocked func
				}
			`},
			wantQueries: []string{"not data.partial.test.q with input.x as 1"},
			wantSupport: []string{`
				package partial.test

				q { data.partial.test.mock_count([1], __local4__3); __local4__3 = input.x }
				mock_count(__local1__4) = 100 { input.y = x_term_4_04; x_term_4_04 }
				mock_count(__local2__5) = 101 { input.z = x_term_5_05; x_term_5_05 }
			`},
		},
		{
			note:  "save: sub path",
			query: "input.x = 1; input.y = 2; input.z.a = 3; input.z.b = x",
			input: `{"x": 1, "z": {"b": 4}}`,
			wantQueries: []string{
				`input.y = 2; input.z.a = 3; x = 4`,
			},
			unknowns: []string{
				"input.y",
				"input.z.a",
			},
		},
		{
			note:  "save: virtual doc",
			query: "data.test.p = 1; data.test.q = 2",
			modules: []string{
				`package test
				p = x { x = 1 }
				q = y { y = input.y }`,
			},
			wantQueries: []string{
				`data.test.p = 1; 2 = input.y`,
			},
			unknowns: []string{
				"input",
				"data.test.p",
			},
		},
		{
			note:  "automatic shallow inlining: full extent: partial set",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p[x] { input.x = x }
				p[x] { input.y = x }`,
			},
			wantQueries: []string{`data.partial.test.p = x`},
			wantSupport: []string{`
				package partial.test
				p[x1] { input.y = x1 }
				p[x2] { input.x = x2 }
			`},
		},
		{
			note:  "automatic shallow inlining: full extent: partial set, general ref head",
			query: "data.test.p.q = x",
			modules: []string{
				`package test
				import future.keywords.contains
				p.q contains x { input.x = x }
				p.q[r].s contains t { input.r = r; input.t = t }`,
			},
			wantQueries: []string{`data.partial.test.p.q = x`},
			wantSupport: []string{`
				package partial.test.p
				import future.keywords.contains
				q[x2] { input.x = x2 }
				q[r1].s contains t1 { input.r = r1; input.t = t1 }
			`},
		},
		{
			note:  "automatic shallow inlining: full extent: partial object",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p[x] = y { x = input.x; y = input.y }
				p[x] = y { x = input.z; y = input.a }`,
			},
			wantQueries: []string{`data.partial.test.p = x`},
			wantSupport: []string{`
				package partial.test
				p[x1] = y1 { x1 = input.z; y1 = input.a }
				p[x2] = y2 { x2 = input.x; y2 = input.y }
			`},
		},
		{
			note:  "automatic shallow inlining: full extent: partial object, general ref head",
			query: "data.test.p.q = x",
			modules: []string{
				`package test
				p.q[x] = y { x = input.x; y = input.y }
				p.q[r].s[t] = y { r = input.r; t = input.t; y = input.y }`,
			},
			wantQueries: []string{`data.partial.test.p.q = x`},
			wantSupport: []string{`
				package partial.test.p
				q[x2] = y2 { x2 = input.x; y2 = input.y }
				q[r1].s[t1] = y1 { r1 = input.r; t1 = input.t; y1 = input.y }
			`},
		},
		{
			note:  "automatic shallow inlining: full extent: no solutions",
			query: "data.test.p = x",
			modules: []string{
				`package test

				p[1] { input = 1; false }`,
			},
			wantQueries: []string{`x = set()`},
		},
		{
			note:  "automatic shallow inlining: full extent: iteration",
			query: "data.test[x] = y",
			modules: []string{
				`package test
				import future.keywords.contains
				s[x] { x = input.x }
				s2[x].u contains y { x = input.x; y = input.y }
				p[x] = y { x = input.x; y = input.y }
				p2[x].r[y] = z { x = input.x; y = input.y; z = input.z }
				r = x { x = input.x }`,
			},
			wantQueries: []string{
				`data.partial.test.s = y; x = "s"`,
				`data.partial.test.s2 = y; x = "s2"`,
				`data.partial.test.p = y; x = "p"`,
				`data.partial.test.p2 = y; x = "p2"`,
				`y = input.x; x = "r"`,
			},
			wantSupport: []string{`
				package partial.test
				import future.keywords.contains
				p[x1] = y1 { x1 = input.x; y1 = input.y }
				p2[x2].r[y2] = z2 { x2 = input.x; y2 = input.y; z2 = input.z }
				s[x4] { x4 = input.x }
				s2[x5].u contains y5 { x5 = input.x; y5 = input.y }
			`},
		},
		{
			note:  "save: set embedded",
			query: `data.test.p = true`,
			modules: []string{`
				package test
				p { x = input; {x} = {1} }`},
			wantQueries: []string{`{input} = {1}`},
		},
		{
			note:  "save: call embedded",
			query: "x = input; a = [x]; count([a], n)",
			wantQueries: []string{
				`x = input; count([[x]], n); a = [x]`,
			},
		},
		{
			note:  "save: function with call composite result (array)",
			query: `split(input, "@", [x]); startswith(x, "foo")`,
			wantQueries: []string{
				`split(input, "@", [x]); startswith(x, "foo")`,
			},
		},
		{
			note:  "save: function: ordered",
			query: `input = x; data.test.f(x)`,
			modules: []string{`
				package test
				f(x) = true { x = 1 }
				else = false { x = 2 }`},
			wantQueries: []string{
				`input = x; data.test.f(x)`,
			},
		},
		{
			note:  "save: with but no unknowns",
			query: "data.test.p = {1,2}",
			modules: []string{
				`package test
				p[1]
				p[2] { 1 with data.foo as 1 }`,
			},
			wantQueries: []string{`data.partial.test.p = {1,2}`}, // can't evaluate full extent of `p` because it depends on with statements that will be saved.
			wantSupport: []string{`
				package partial.test

				p[1] { true }
				p[2] { true }   # note: the expression containing 'with' gets partially evaluated because it does not depend on unknowns
			`},
		},
		{
			note:  "else: no unknown dependencies",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p = x { q = x }
				q = 100 { false } else = 200 { true }`,
			},
			wantQueries: []string{
				`x = 200`,
			},
		},
		{
			note:  "else: saved",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p = x { q = x }
				q = 100 { input.x = 1 } else = 200 { true }`,
			},
			wantQueries: []string{
				`data.test.q = x`,
			},
		},
		{
			note:  "else: func args unknown transitive",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { input = z; [z] = x; f(x, true) }
				f(x) { x > 1 } else = false { x < 0 }`,
			},
			wantQueries: []string{
				`data.test.f([input], true)`,
			},
		},
		{
			note:  "save: ignore ast",
			query: "time.now_ns(x)",
			wantQueries: []string{
				`time.now_ns(x)`,
			},
		},
		{
			note:  "save: ignore ast transitive",
			query: "data.test.p = true",
			modules: []string{
				`package test
				p { q = x }
				q[1] { time.now_ns() == 1579276766010057000 }`, // full extent, must save caller because time.now_ns() should not be partially evaluated
			},
			wantQueries: []string{"x1 = data.partial.test.q"},
			wantSupport: []string{`
				package partial.test
				q[1] { time.now_ns(1579276766010057000) }
			`},
		},
		{
			note:  "support: default trivial",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = false
				p { q }
				q { input.x = 1 }
				`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				p = true { input.x = 1 }
				default p = false`,
			},
		},
		{
			note:  "support: default with iteration (disjunction)",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = false
				p { q }
				q { input.x = 1 }
				q { input.x = 2 }
				`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				p = true { input.x = 1 }
				p = true { input.x = 2 }
				default p = false
				`,
			},
		},
		{
			note:  "support: default with iteration (data)",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = false
				p { q }
				q { input.x = a[i] }
				a = [1, 2]
				`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				p = true { 1 = input.x }
				p = true { 2 = input.x }
				default p = false`,
			},
		},
		{
			note:  "support: default with disjunction",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = 0
				p = 1 { q }
				p = 2 { r }
				q { input.x = 1 }
				r { input.x = 2 }
				`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				p = 1 { input.x = 1 }
				p = 2 { input.x = 2 }
				default p = 0
				`,
			},
		},
		{
			note:  "support: default head vars",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = 0
				p = x { x = 1; input.x = 1 }
				p = x { input.x = x }
				`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				p = 1 { input.x = 1 }
				p = x1 { input.x = x1 }
				default p = 0
				`,
			},
		},
		{
			note:  "support: default multiple",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = false
				p { q = true; s } # using q = true syntax to avoid dealing with implicit != false expr
				default q = true  # same value as expr above so default must be kept
				q { r }
				r { input.x = 1 }
				r { input.y = 2 }
				s { input.z = 3 }`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				q = true { input.x = 1 }
				q = true { input.y = 2 }
				default q = true
				p = true { data.partial.test.q = true; input.z = 3 }
				default p = false
				`,
			},
		},
		{
			note:  "support: default bindings",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = false
				p { q[x]; not r[x] }
				q[1] { input.x = 1 }
				q[2] { input.y = 2 }
				r[1] { input.z = 3 }`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test

				p = true { input.x = 1; not input.z = 3 }
				p = true { input.y = 2 }
				default p = false
				`,
			},
		},
		{
			note:  "support: iterate default",
			query: "data.test[x] = y",
			modules: []string{
				`package test
				default p = 0
				p = 1 { q }
				q { input.x = 1 }
				`,
			},
			wantQueries: []string{
				`data.partial.test.p = y; x = "p"`,
				`input.x = 1; x = "q"; y = true`,
			},
			wantSupport: []string{
				`package partial.test
				p = 1 { input.x = 1 }
				default p = 0`,
			},
		},
		{
			note:  "support: default memoized",
			query: "data.test.q[x] = y; data.test.p = z",
			modules: []string{
				`package test

				q = [1,2]

				default p = false
				p { input.x = 1 }`,
			},
			wantQueries: []string{
				`data.partial.test.p = z; x = 0; y = 1`,
				`data.partial.test.p = z; x = 1; y = 2`,
			},
			wantSupport: []string{
				`package partial.test

				p = true { input.x = 1 }
				default p = false`,
			},
		},
		{
			note:  "copy propagation: basic",
			query: "input.x > 1",
			wantQueries: []string{
				"input.x > 1",
			},
		},
		{
			note:  "copy propagation: call terms",
			query: "input.x+1 > 1",
			wantQueries: []string{
				"input.x+1 > 1",
			},
		},
		{
			note:  "copy propagation: virtual",
			query: "data.test.p > 1",
			modules: []string{
				`package test

				p = x { input.x = y; y = z; z = x }`,
			},
			wantQueries: []string{
				`input.x > 1`,
			},
		},
		{
			note:  "copy propagation: virtual: call",
			query: "data.test.p > 1",
			modules: []string{
				`package test

				p = y { input.x = x; plus(x, 1, y) }`,
			},
			wantQueries: []string{
				`input.x+1 > 1`,
			},
		},
		{
			note:  "copy propagation: composite",
			query: "data.test.p[0][0] = 1",
			modules: []string{
				`package test

				p = x { x = [input.x] }
				`,
			},
			wantQueries: []string{
				`input.x[0] = 1`,
			},
		},
		{
			note:  "copy propagation: reference head",
			query: "data.test.p[0] > 1",
			modules: []string{
				`package test

				p = x { input.x = x }`,
			},
			wantQueries: []string{
				`input.x[0] > 1`,
			},
		},
		{
			note:  "copy propagation: reference head: call",
			query: "data.test.p[0] > 1",
			modules: []string{
				`package test

				p = x { sort(input.x, y); y = x }`,
			},
			wantQueries: []string{
				// copy propagation cannot remove the intermediate variable currently because
				// sort(input.x, y) is not killed (since y is ultimately used as a ref head.)
				`sort(input.x, x_ref_0); x_ref_0[0] > 1`,
			},
		},
		{
			note:  "copy propagation: var vs dot vs set",
			query: `data.test.p = true`,
			modules: []string{
				`package test

				p {
					input.x[i] = a; a.foo = 1	# same semantics as next line
					input.y[j].bar = 2;
					input.z[k]; k.baz = 3		# different semantics from previous two lines
				}`,
			},
			wantQueries: []string{`
				input.x[i1].foo = 1;
				input.y[j1].bar = 2;
				input.z[k1]; k1.baz = 3`,
			},
		},
		{
			note:  "copy propagation: reference head: call transitive with union-find",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p {
					split(input, ":", x)
					y = x
					y[0] = "a"
				}`,
			},
			wantQueries: []string{
				`split(input, ":", x1); x1[0] = "a"`,
			},
		},
		{
			note:  "copy propagation: live built-in output",
			query: "plus(input, 1, x); x = y",
			wantQueries: []string{
				`plus(input, 1, x); y = x`,
			},
		},
		{
			note:  "copy propagation: declared var built-in output",
			query: "some x; plus(input, 1, x); x = y",
			wantQueries: []string{
				`plus(input, 1, y)`,
			},
		},
		{
			note:  "copy propagation: no dependencies",
			query: "data.test.p",
			modules: []string{
				`package test

				p {
					input.x = ["foo", a]
					input.y = a
				}`,
			},
			wantQueries: []string{
				`input.x = ["foo", a1]; a1 = input.y`,
			},
		},
		{
			note:  "copy propagation: union-find replace head",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p {
					input = y
					x = y
					x.foo = 1
				}`,
			},
			wantQueries: []string{`input.foo = 1`},
		},
		{
			note:  "copy propagation: union-find skip ref head",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p {
					input = y
					x = y
					x.foo = 1
					x = {"foo": 1}
				}`,
			},
			wantQueries: []string{`input.foo = 1; input = {"foo": 1}`},
		},
		{
			note:  "copy propagation: remove equal(A,A) nop",
			query: "data.test.p == 100",
			modules: []string{
				`package test

				p = x {
					input = x
					x = 100
				}`,
			},
			wantQueries: []string{
				"input = 100",
			},
		},
		{
			note:  "copy propagation: apply to support rules",
			query: `data.test.p = true`,
			modules: []string{`
				package test

				p {
					not q
				}

				q {
					input.x = x
					x = y
					y = 1
				}
			`},
			wantQueries: []string{`not input.x = 1`},
		},
		{
			note:  "copy propagation: apply to support rules: head vars are live",
			query: `data.test.p = true`,
			modules: []string{`
				package test

				p {
					input.x = z; not q[z]
				}

				q[y] {
					x = 1
					x = a
					a = y
				}
			`},
			wantQueries: []string{`not input.x = 1`},
		},
		{
			note:  "copy propagation: negation safety",
			query: `data.test.p = true`,
			modules: []string{
				`package test

				p {
					input.x[i] = x
					not f(x)
				}

				f(x) {
					input.y = x
				}`,
			},
			wantQueries: []string{
				"not input.y = x1; x1 = input.x[i1]",
			},
		},
		{
			note:  "copy propagation: negation safety needs extra expr",
			query: `data.test.p = true`,
			modules: []string{
				`package test

				p {
				  x = data.y[c]
				  x.z = 1
				  not x.z = 2
				}
				`,
			},
			unknowns: []string{`data.y`},
			wantQueries: []string{
				`data.y[c1].z = 1; not x1.z = 2; x1 = data.y[c1]`,
			},
		},
		{
			note:  "copy propagation: negation safety needs extra expr - no live var overlap",
			query: `data.test.p = true`,
			modules: []string{
				`package test

				p {
				  x = input.y[c]
				  x.z = 1
				  not x.z = 2
				}
				`,
			},
			unknowns: []string{`input.y`},
			wantQueries: []string{
				`input.y[c1].z = 1; not x1.z = 2; x1 = input.y[c1]`,
			},
		},
		{
			note:  "copy propagation: negation safety no extra expr",
			query: `data.test.p = true`,
			modules: []string{
				`package test

				p {
				  x = data.y[c]
				  not x.z = 2
				}
				`,
			},
			unknowns: []string{`data.y`},
			wantQueries: []string{
				`not x1.z = 2; x1 = data.y[c1]`,
			},
		},
		{
			note:  "copy propagation: rewrite object key (bug 1177)",
			query: `data.test.p = true`,
			modules: []string{
				`
					package test

					p {
						x = input.x
						y = input.y
						x = {y: 1}
					}
				`,
			},
			wantQueries: []string{`input.x = {input.y: 1}`},
		},
		{
			note:  "copy propagation: single term test intact",
			query: "data.test.p = true",
			modules: []string{`
				package test

				p {
					input = x
					y = x == 1
					y
				}

			`},
			wantQueryASTs: []ast.Body{
				ast.NewBody(
					ast.NewExpr(
						ast.CallTerm(
							ast.NewTerm(ast.Equal.Ref()),
							ast.NewTerm(ast.InputRootRef),
							ast.IntNumberTerm(1),
						),
					),
				),
			},
		},
		{
			note:  "copy propagation: circular reference (bug 3559)",
			query: "data.test.p",
			modules: []string{`package test
				p {
					q[_]
				}
				q[x] {
					x = input[x]
				}`,
			},
			wantQueries: []string{`x_term_1_01; x_term_1_01 = input[x_term_1_01]`},
		},
		{
			note:  "copy propagation: circular reference (bug 3071)",
			query: "data.test.p",
			modules: []string{`package test
				p[y] {
					s := { i | input[i] }
					s & set() != s
					y := sprintf("%v", [s])
				}`,
			},
			wantQueries: []string{`data.partial.test.p`},
			wantSupport: []string{`package partial.test
				p[__local1__1] { __local0__1 = {i1 | input[i1]}; neq(and(__local0__1, set()), __local0__1); sprintf("%v", [__local0__1], __local1__1) }
			`},
		},
		{
			note:        "copy propagation: tautology in query, input ref",
			query:       "input.a == input.a",
			wantQueries: []string{`__localq1__ = input.a`},
		},
		{
			note:        "copy propagation: tautology in query, var ref, var is input",
			query:       "x := input; x.a == x.a",
			wantQueries: []string{`__localq2__ = input.a`},
		},
		{
			note:  "copy propagation: tautology, input ref",
			query: "data.test.p",
			modules: []string{`package test
				p {
					input.a == input.a
				}`,
			},
			wantQueries: []string{`__localcp0__ = input.a`},
		},
		{
			note:  "copy propagation: tautology, var ref, ref is input",
			query: "data.test.p",
			modules: []string{`package test
				p {
					x := input
					x.a == x.a
				}`,
			},
			wantQueries: []string{`__localcp0__ = input.a`},
		},
		{
			note:     "copy propagation: tautology, var ref, ref is unknown data",
			query:    "data.test.p",
			unknowns: []string{"data.bar.foo"},
			modules: []string{`package test
				p {
					data.bar.foo.a == data.bar.foo.a
				}`,
			},
			wantQueries: []string{`__localcp0__ = data.bar.foo.a`},
		},
		{
			note: "copy propagation: tautology, var ref, ref is input, via unknown",
			// NOTE(sr): If we were having unkowns: [input.foo] and the rule body was
			// input.a == input.a, we'd never reach copy-propagation -- partial eval would
			// have failed before.
			query:    "data.test.p",
			unknowns: []string{"input"},
			modules: []string{`package test
				p {
					input.foo.a == input.foo.a
				}`,
			},
			wantQueries: []string{`__localcp0__ = input.foo.a`},
		},
		{
			note:  "copy propagation: tautology, var ref, ref is head var",
			query: "data.test.p(input)",
			modules: []string{`package test
				p(x) {
					x.a == x.a
				}`,
			},
			wantQueries: []string{`__localcp1__ = input.a`},
		},
		{
			note:  "save set vars are namespaced",
			query: "input = x; data.test.f(1)",
			modules: []string{
				`package test

				f(x) { x >= x }`,
			},
			wantQueries: []string{
				`input = x`,
			},
		},
		{
			note:  "negation: inline compound",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { not q }
				q { ((input.x + 7) / input.y) > 100 }`,
			},
			wantQueries: []string{
				`not data.partial.__not1_0_2__`,
			},
			wantSupport: []string{
				`package partial

				__not1_0_2__ {
					((input.x + 7) / input.y) > 100
				}`,
			},
		},
		{
			note:  "negation: inline conjunction",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { not q }
				q { a = input.x + 7; b = a / input.y; b > 100 }`,
			},
			wantQueries: []string{
				`not data.partial.__not1_0_2__`,
			},
			wantSupport: []string{
				`package partial

				__not1_0_2__ {
					((input.x + 7) / input.y) > 100
				}`,
			},
		},
		{
			note:  "negation: inline safety",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p {
					input.x = 1;					# no op
					not q;							# support
					not r;							# fail
					not s;							# inline (simple)
					input.z = [z]; z1 = z; t(z1)	# inline transitive
				}

				q {
					input.a[i] = 1
				}

				r { false }

				s { input.y = 2 }

				t(z2) {
					z2 = z3
					z3[0] = 1
				}
				`,
			},
			wantQueries: []string{
				`input.x = 1; not data.partial.__not1_1_2__; not input.y = 2; input.z = [z38]; z38[0] = 1`,
			},
			wantSupport: []string{
				`package partial

				__not1_1_2__ {
					input.a[i3] = 1
				}`,
			},
		},
		{
			note:  "negation: support safety without args",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p {
					q
					not r
				}

				q {
					input.x[i] = a
					startswith(a, "foo")
				}

				r {
					input.y[i] = 1
				}`,
			},
			wantQueries: []string{`startswith(input.x[i2], "foo"); not data.partial.__not1_1_3__`},
			wantSupport: []string{
				`package partial

				__not1_1_3__ { input.y[i4] = 1 }`,
			},
		},
		{
			note:  "negation: support safety with args",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p {
					input.x = x; not f(x)
				}

				f(x) {
					input.y[i] = a
					sort(x, z)
					z[a] = 1
				}`,
			},
			wantQueries: []string{`not data.partial.__not1_1_2__(input.x)`},
			wantSupport: []string{`
				package partial

				__not1_1_2__(x1) {
					sort(x1, z3)
					z3[input.y[i3]] = 1
				}
			`},
		},
		{
			note:  "negation: inline safety with live var",
			query: "input = x; not data.test.f(x)",
			modules: []string{
				`package test

				f(x) {
					count(x) != 3
				}`,
			},
			wantQueries: []string{
				`input = x; not data.partial.__not0_1_1__(x)`,
			},
			wantSupport: []string{
				`package partial

				__not0_1_1__(x) {
					count(x) != 3
				}`,
			},
		},
		{
			note:  "negation: inline namespacing",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p {
					input = x; not f(x)
				}

				f(x) {
					count(x) > 3
				}`,
			},
			wantQueries: []string{
				`not data.partial.__not1_1_2__(input)`,
			},
			wantSupport: []string{
				`package partial

				__not1_1_2__(x1) { count(x1) > 3 }`,
			},
		},
		{
			note:  "negation: inline namespacing embedded",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p {
					y = input.y
					z = y
					x = [z, 1]
					not f(x)
				}

				f(x) {
					sum(x) > 3
				}`,
			},
			wantQueries: []string{
				`not data.partial.__not1_3_2__(input.y)`,
			},
			wantSupport: []string{
				`package partial

				__not1_3_2__(z1) {
					sum([z1, 1]) > 3
				}`,
			},
		},
		{
			note:  "negation: inline disjunction",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { not q }
				q { input.x = 1 }
				q { input.x = 2 }
				`,
			},
			wantQueries: []string{
				`not input.x = 1; not input.x = 2`,
			},
			ignoreOrder: true,
		},
		{
			note:  "negation: inline disjunction with args",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { input.x = x; not q(x) }
				q(x) { x = 1 }
				q(x) { x = 2 }`,
			},
			wantQueries: []string{
				`not input.x = 1; not input.x = 2`,
			},
			ignoreOrder: true,
		},
		{
			note:  "negation: inline double negation (for all or universal quantifier pattern)",
			query: `data.test.p = true`,
			modules: []string{`
				package test

				p {
					x = input[i]
					not f(x)
				}

				f(x) {
					q[y]
					not g(y, x)
				}

				g(1, x) {
					x.a = "foo"
				}

				g(2, x) {
					x.b < 7
				}

				q = {
					1, 2
				}
			`},
			wantQueries: []string{
				`input[i1].a = "foo"; data.partial.__not3_1_8__(input[i1])`,
			},
			wantSupport: []string{
				`package partial

				__not3_1_8__(__local0__3) { __local0__3.b < 7 }`,
			},
		},
		{
			note:  "negation: inline cross product",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p {
					not q
				}

				q {
					x = r[_]
					not f(x)
				}

				f({"key": "a", "values": values}) {
					input.x = values[_]
				}

				f({"key": "b", "values": values}) {
					input.y = values[_]
				}

				f({"key": "c", "values": values}) {
					input.z = values[_]
				}

				r = [
					{"key": "a", "values": [1,2]},
					{"key": "b", "values": [3,4,5]},
					{"key": "c", "values": [6]},
				]`,
			},
			wantQueries: []string{
				`1 = input.x; 3 = input.y; 6 = input.z`,
				`1 = input.x; 4 = input.y; 6 = input.z`,
				`1 = input.x; 5 = input.y; 6 = input.z`,
				`2 = input.x; 3 = input.y; 6 = input.z`,
				`2 = input.x; 4 = input.y; 6 = input.z`,
				`2 = input.x; 5 = input.y; 6 = input.z`,
			},
		},
		{
			note:  "negation: inline cross product with live vars",
			query: "input.x = x; input.y = y; not data.test.p[[x,y]]",
			modules: []string{
				`package test

					p[[0, 1]]
					p[[2, 3]]`,
			},
			wantQueries: []string{
				`input.x = x; input.y = y; not x = 0; not x = 2`,
				`input.x = x; input.y = y; not x = 0; not y = 3`,
				`input.x = x; input.y = y; not y = 1; not x = 2`,
				`input.x = x; input.y = y; not y = 1; not y = 3`,
			},
		},
		{
			note:  "negation: cross product limit",
			query: "data.test.p = true",
			modules: []string{
				`package test
				p {
					not q
				}
				q {
					# size of cross product is 27 which exceeds default limit
					a = {1,2,3}
					a[x]
					input.x = x
					input.y = x
					input.z = 0
				}
				`,
			},
			wantQueries: []string{`not data.partial.__not1_0_2__`},
			wantSupport: []string{
				`package partial

				__not1_0_2__ { input.x = 1; input.y = 1; input.z = 0 }
				__not1_0_2__ { input.x = 2; input.y = 2; input.z = 0 }
				__not1_0_2__ { input.x = 3; input.y = 3; input.z = 0 }
				`,
			},
		},
		{
			note:  "negation: inlining namespaced variables",
			query: "data.test.p[x]",
			modules: []string{
				`package test

				p[y] {
					y = input
					not y = 1
				}
				`,
			},
			wantQueries: []string{
				`x = input; not x = 1; x`,
			},
		},
		{
			note:  "negation: inlining transitive unknown",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { input = z; [z] = x; not q[x] }

				q[[1]]
				q[[2]]`,
			},
			wantQueries: []string{
				`not input = 1; not input = 2`,
			},
		},
		{
			note:  "function inlining: output checked",
			query: "data.test.p = true",
			modules: []string{`
					package test
					f(x) = y {
						y = x == 1
					}
					p {
						f(input)
					}
				`},
			wantQueryASTs: []ast.Body{
				ast.NewBody(
					ast.NewExpr(
						ast.CallTerm(
							ast.NewTerm(ast.Equal.Ref()),
							ast.NewTerm(ast.InputRootRef),
							ast.IntNumberTerm(1),
						),
					),
				),
			},
		},
		{
			note:  "disable inlining: complete doc",
			query: "data.test.p = true",
			modules: []string{`
				package test
				p { q; r }
				q { s[input] }
				q { t[input] }
				r { s[input] }
				s[1]
				s[2]
				t[3]
			`},
			wantQueries: []string{
				"data.partial.test.q; 1 = input",
				"data.partial.test.q; 2 = input",
			},
			wantSupport: []string{
				`package partial.test

				q { 1 = input }
				q { 2 = input }
				q { 3 = input } `,
			},
			disableInlining: []string{`data.test.q`},
		},
		{
			note:  "disable inlining: complete doc with suffix",
			query: "data.test.p = true",
			modules: []string{`
				package test
				p { s; q[x] }
				q = ["a", "b"] { r[_] = input }
				r = [1, 2]
				s { r[_] = input }
			`},
			wantQueries: []string{
				"1 = input; data.partial.test.q[x1]",
				"2 = input; data.partial.test.q[x1]",
			},
			wantSupport: []string{
				`package partial.test

				q = ["a", "b"] { 1 = input }
				q = ["a", "b"] { 2 = input }`,
			},
			disableInlining: []string{`data.test.q`},
		},
		{
			note:  "disable inlining: function",
			query: "data.test.p = true",
			modules: []string{`
				package test

				p { q[x]; f(x) }
				q = {"a", "b"}
				f(x) { input = x }
			`},
			wantQueries: []string{
				`data.partial.test.f("a")`,
				`data.partial.test.f("b")`,
			},
			wantSupport: []string{
				`package partial.test

				f(__local0__3) { input = __local0__3 }`,
			},
			disableInlining: []string{"data.test.f"},
		},
		{
			note:  "disable inlining: partial doc",
			query: "data.test.p = true",
			modules: []string{`
				package test
				p { q[x]; r[x] }
				q[x] { s[x] = input }
				r[x] { s[x] = input }
				s[1]
				s[2]
			`},
			wantQueries: []string{
				"data.partial.test.q[1]; 1 = input",
				"data.partial.test.q[2]; 2 = input",
			},
			wantSupport: []string{
				`package partial.test

				q[1] { 1 = input }
				q[2] { 2 = input }`,
			},
			disableInlining: []string{`data.test.q`},
		},
		{
			note:  "disable inlining: partial doc with suffix",
			query: "data.test.p = true",
			modules: []string{`
				package test
				p { y = 0; q[x][y]; r }
				q[x] = [1, 2] { s[x] = input }
				r { input = 1 }
				r { input = 2 }
				s["a"] = 3
				s["b"] = 4
			`},
			wantQueries: []string{
				"data.partial.test.q[x1][0]; input = 1",
				"data.partial.test.q[x1][0]; input = 2",
			},
			wantSupport: []string{
				`package partial.test

				q["a"] = [1, 2] { 3 = input }
				q["b"] = [1, 2] { 4 = input }`,
			},
			disableInlining: []string{`data.test.q`},
		},
		{
			note:            "disable inlining: partial rule namespaced variables (negation)",
			query:           "data.test.p[x]",
			disableInlining: []string{"data.test.p"},
			modules: []string{
				`package test

				p[y] {
					y = input
					not y = 1
				}
				`,
			},
			wantQueries: []string{
				`data.partial.test.p[x]`,
			},
			wantSupport: []string{
				`package partial.test

				p[y1] { y1 = input; not y1 = 1 }`,
			},
		},
		{
			note:            "disable inlining: complete rule namespaced variables (negation)",
			query:           "data.test.p = x",
			disableInlining: []string{"data.test.p"},
			modules: []string{
				`package test

				p = y {
					y = input
					not y = 1
				}
				`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test

				p = y1 { y1 = input; not y1 = 1 }`,
			},
		},
		{
			note:  "disable inlining: disable on prefix",
			query: "data.test.foo.p = true",
			modules: []string{
				`package test.foo

				p {
					data.test.bar.q[input.x]
				}`,

				`package test.bar

				q[x] { data.test.baz.r[x] }`,

				`package test.baz

				r[1]
				r[2]`,
			},
			disableInlining: []string{"data.test.bar"},
			wantQueries:     []string{`data.partial.test.bar.q[input.x]`},
			wantSupport: []string{
				`package partial.test.bar

				q[1]
				q[2]`,
			},
		},
		{
			note:  "disable inlining: base document enumeration",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { k = "foo"; m = "bar"; data.base[k][x][m] = 1 }`,
			},
			disableInlining: []string{"data.base"},
			wantQueries:     []string{"data.base.foo[x1].bar = 1"},
		},
		{
			note:  "disable inlining: base document extent",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { k = "bar"; data.base.foo[k].baz = 1 }`,
			},
			disableInlining: []string{"data.base"},
			wantQueries:     []string{"data.base.foo.bar.baz = 1"},
		},
		{
			note:  "disable inlining: negation treats as unknown",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { not q }

				q { r }

				r = false`,
			},
			disableInlining: []string{"data.test.r"},
			wantQueries:     []string{"not data.partial.test.r"},
			wantSupport: []string{
				`package partial.test

				r = false`,
			},
		},
		{
			note:  "disable inlining: comprehension treats as unknown",
			query: "data.test.p = [1]",
			modules: []string{
				`package test

				p = x { x = [1 | q] }

				q { r }

				r = true`,
			},
			disableInlining: []string{"data.test.r"},
			wantQueries:     []string{"[1] = [1 | data.test.q]"},
		},
		{
			note:  "disable inlining: partial rule full extent treats as unknown",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p { q = {1,2,3} }

				q[1]
				q[2]
				q[3] { r }

				r = true`,
			},
			disableInlining: []string{"data.test.r"},
			wantQueries:     []string{"data.partial.test.q = {1, 2, 3}"},
			wantSupport: []string{`
				package partial.test

				q[1]
				q[2]
				q[3] { data.partial.test.r }
				r = true
			`},
		},
		{
			note:  "disable inlining: ref prefix",
			query: "data.test.p = true",
			modules: []string{
				`package test

				p {
					q[input.x]
				}

				q = {a | data.base[a]}`,
			},
			disableInlining: []string{"data.base.foo.bar"},
			wantQueries:     []string{`x_ref_01 = {a2 | data.base[a2]}; x_ref_01[input.x]`},
		},
		{
			note:  "shallow inlining: complete rules",
			query: "data.test.p = true",
			modules: []string{
				`
					package test

					p {
						q = 1
					}

					q = x {
						r  # 'r' should be inlined completely
						y = input.x
						x = y
					}

					r { s }

					s = true
				`,
			},
			shallow:     true,
			wantQueries: []string{"data.partial.test.p = true"},
			wantSupport: []string{
				`package partial.test

				q = x2 { y2 = input.x; x2 = y2 }
				p { data.partial.test.q = 1 }
				`,
			},
		},
		{
			note:  "shallow inlining: iteration and negation",
			query: "data.test.p = true",
			modules: []string{
				`
					package test

					p {
						r[x]
						not input[x]
					}

					r[1]
					r[2]
				`,
			},
			shallow:     true,
			wantQueries: []string{"data.partial.test.p = true"},
			wantSupport: []string{
				`
					package partial.test

					p { not data.partial.__not1_1_4__ }
					p { not data.partial.__not1_1_5__ }
				`,
				`
					package partial

					__not1_1_4__ { input[1] = x_term_4_01; x_term_4_01 }
					__not1_1_5__ { input[2] = x_term_5_01; x_term_5_01 }
				`,
			},
		},
		{
			note:  "shallow inlining: function not inlined if no unknowns in rule bodies, but in args",
			query: "data.test.p = true",
			modules: []string{`
					package test

					f(x) = y {
						y = x == 1
					}
					f(x) = y {
						y = x == 2
					}
					p {
						f(input)
					}
				`},
			shallow:     true,
			wantQueries: []string{"data.partial.test.p = true"},
			wantSupport: []string{
				`package partial.test

				p = true { __local4__1 = input; data.partial.test.f(__local4__1) }
				f(__local0__2) = y2 { equal(__local0__2, 1, __local2__2); y2 = __local2__2 }
				f(__local1__3) = y3 { equal(__local1__3, 2, __local3__3); y3 = __local3__3 }`,
			},
		},
		{
			note:    "shallow inlining: function with unknowns in rule body",
			query:   "data.test.f(1, x)",
			shallow: true,
			modules: []string{
				`package test
				f(x) = true { input.x = x }
				f(x) = false { input.y = x }`,
			},
			wantQueries: []string{`data.partial.test.f(1, x)`},
			wantSupport: []string{
				`package partial.test
				f(__local0__2) = true { input.x = __local0__2 }
				f(__local1__1) = false { input.y = __local1__1 }`,
			},
		},
		{
			note:    "shallow inlining: functions with no unknowns in rule body or output, always true",
			query:   "data.test.f(1, y)",
			shallow: true,
			modules: []string{
				`package test
				f(x) = true { x >= 1 }
				f(x) = false { x < 0 }
				f(x) = "meow" { false }`,
			},
			wantQueries: []string{`y = true`},
		},
		{
			note:    "shallow inlining: functions with multiple args, no unknowns",
			query:   "data.test.f(1, [1,2,3], y)",
			shallow: true,
			modules: []string{
				`package test
				f(x, y) = true { x > 1 }
				f(x, y) = false {
					x <= 0
					count(y) == 3
				}`,
			},
			wantQueries: []string{},
		},
		{
			note:    "shallow inlining: functions that are always undefined",
			query:   "data.test.f(1, y)",
			shallow: true,
			modules: []string{
				`package test
				f(x) = "uhm" { input.x = "x"; false }
				f(x) = "like" { input.y = "y"; false }
				f(x) = "whatever" { false }`,
			},
			wantQueries: []string{},
		},
		{
			note:    "shallow inlining: functions with non-var arguments",
			query:   "data.test.f(1, y)",
			shallow: true,
			modules: []string{
				`package test
				f(true) = true
				f(x) = false { x != true }`,
			},
			wantQueries: []string{`y = false`},
		},
		{
			note:    "shallow inlining: functions with unknown call-site arguments",
			query:   "input = x; data.test.f([1, x])",
			shallow: true,
			modules: []string{
				`package test
				f([x, y]) {
				  z = 7
				  x > (y+z)
				}`,
			},
			wantQueries: []string{`input = x; data.partial.test.f([1, x])`},
			wantSupport: []string{
				`package partial.test
				f([__local0__1, __local1__1]) = true {
					plus(__local1__1, 7, __local2__1)
					gt(__local0__1, __local2__1)
				}`,
			},
		},
		{
			note:    "shallow inlining: function unknowns transitive",
			query:   "data.test.p = true",
			shallow: true,
			modules: []string{
				`
					package test

					p {
						f(1)
					}

					f(x) {
						g(x)
					}

					g(x) {
						x = input
					}
				`,
			},
			wantQueries: []string{`data.partial.test.p = true`},
			wantSupport: []string{
				`
					package partial.test

					p { data.partial.test.f(1) }
					f(__local0__2) { data.partial.test.g(__local0__2) }
					g(__local1__3) { __local1__3 = input }
				`,
			},
		},
		{
			note:    "shallow inlining: function unknowns transitive - mixed",
			query:   "data.test.p = true",
			shallow: true,
			modules: []string{
				`
					package test

					p {
						f(1) # unknown dependency so must be saved
						h(8) # known so can be evaluated
					}

					f(x) {
						g(x)
					}

					g(x) {
						x = input
					}

					h(x) {
						x > 7
					}
				`,
			},
			wantQueries: []string{`data.partial.test.p = true`},
			wantSupport: []string{
				`
					package partial.test

					p { data.partial.test.f(1) }
					f(__local0__2) { data.partial.test.g(__local0__2) }
					g(__local1__3) { __local1__3 = input }
				`,
			},
		},
		{
			note:    "shallow inlining: functions with unknowns in body, result passed to builtin",
			query:   "data.test.p",
			shallow: true,
			modules: []string{
				`package test
				p {
				  y = f(1)
				  count(y)
				}

				f(x) = [] {
					# NOTE(sr): if we use '_' here, we cannot ever have a match
					# when comparing the actual and expected support modules.
					_x = input  # anything dependent on an unknown will do
				}`,
			},
			wantQueries: []string{`data.partial.test.p = x_term_0_0; x_term_0_0`},
			wantSupport: []string{
				`package partial.test
				p {
					data.partial.test.f(1, __local1__1)
					y1 = __local1__1
					count(y1)
				}
				f(__local0__2) = [] { _x2 = input }
				`,
			},
		},
		{
			note:  "comprehensions: ref heads (with namespacing)",
			query: "data.test.p = true; input.x = x",
			modules: []string{ // include an unknown in the comprehension to force saving
				`package test

				p {
					x = [0]; y = {true | x[0]; input.y = 1}
				}
			`},
			wantQueries: []string{`y1 = {true | x1[0]; input.y = 1; x1 = [0]}; input.x = x`},
		},
		{
			note:  "comprehensions: vars in scope, unused in comprehension",
			query: `data.test.p`,
			modules: []string{
				`package test

				p[x] { q[x] }
				q[x] {
					y = { 1 | input }
					x = true
				}
			`},
			wantQueries: []string{`data.partial.test.p`},
			wantSupport: []string{`package partial.test
				p[true] { y2 = {1 | input} }
			`},
		},
		{
			note:  "comprehensions: vars in scope, used in lhs body (set)",
			query: `data.test.p`,
			modules: []string{
				`package test

				p[x] { q[x] }
				q[x] {
					{ 1 | input; x } = y
					x = true
				}
			`},
			wantQueries: []string{`data.partial.test.p`},
			wantSupport: []string{`package partial.test
				p[true] { {1 | input; x2; x2 = true} = y2 }
			`},
		},
		{
			note:  "comprehensions: vars in scope, used in lhs term (set)",
			query: `data.test.p`,
			modules: []string{
				`package test

				p[x] { q[x] }
				q[x] {
					{ x | input } = y
					x = true
				}
			`},
			wantQueries: []string{`data.partial.test.p`},
			wantSupport: []string{`package partial.test
				p[true] { {x2 | input; x2 = true} = y2 }
			`},
		},
		{
			// NOTE(sr): To actually have the vars in the rhs, we'll need to provide two
			// comprehensions -- otherwise, the arguments would be flipped and we'd have
			// the vars in lhs again.
			note:  "comprehensions: vars in scope, used in rhs body (set)",
			query: `data.test.p`,
			modules: []string{
				`package test

				p[x] { q[x] }
				q[x] {
					{ false | input }  = { true | input; x }
					x = true
				}
			`},
			wantQueries: []string{`data.partial.test.p`},
			wantSupport: []string{`package partial.test
				p[true] { {false | input} = {true | input; x2; x2 = true} }
			`},
		},
		{
			note:  "comprehensions: vars in scope, used in rhs term (set)",
			query: `data.test.p`,
			modules: []string{
				`package test

				p[x] { q[x] }
				q[x] {
					{ false | input } = { x | input }
					x = true
				}
			`},
			wantQueries: []string{`data.partial.test.p`},
			wantSupport: []string{`package partial.test
				p[true] { {false | input} = {x2 | input; x2 = true} }
			`},
		},
		{
			note:  "comprehensions: vars in scope, used in rhs value (object)",
			query: `data.test.p`,
			modules: []string{
				`package test

				p[x] { q[x] }
				q[x] {
					{ "foo": false | input } = { "foo": x | input }
					x = true
				}
			`},
			wantQueries: []string{`data.partial.test.p`},
			wantSupport: []string{`package partial.test
				p[true] { {"foo": false | input} = {"foo": x2 | input; x2 = true} }
			`},
		},
		{
			note:        "comprehensions: ref heads (with live vars)",
			query:       "x = [0]; y = {true | x[0]; input.y = 1}", // include an unknown in the comprehension to force saving
			wantQueries: []string{`y = {true | x[0]; input.y = 1; x = [0]}; x = [0]`},
		},
		{
			note:        "negation: save inline negated with",
			query:       `not input with data.x as 2; data.x = 1`,
			data:        `{"x": 1}`,
			wantQueries: []string{"not input with data.x as 2"},
		},
		{
			note:  "negation: save negated expr using plugged with value",
			query: "data.test.p = true",
			modules: []string{`
				package test

				p {
					x = 1
					not q with input.x as x
				}

				q {
					r[input.x]
				}

				r[1]
				r[2]
			`},
			disableInlining: []string{"data.test.q"},
			wantQueries:     []string{"not data.partial.test.q with input.x as 1"},
			wantSupport: []string{`
				package partial.test

				q { 1 = input.x }
				q { 2 = input.x }
			`},
		},
		{
			note:        "negation: save inline negated with (undefined)",
			query:       `not input with data.x as 1; data.x = 1`,
			wantQueries: []string{},
		},
		{
			note:  "multiple removed eqs",
			query: "data.test.p",
			modules: []string{`
				package test

				p = x {
					a = input.foo1
					b = input.foo2
					c = input.foo3
					d = input.foo4
					e = input.foo5
					x = true
				}`,
			},
			wantQueries: []string{`
				e1 = input.foo5
				d1 = input.foo4
				c1 = input.foo3
				b1 = input.foo2
				a1 = input.foo1`},
		},
		{
			note:  "partial object rules not memoized",
			query: "data.test.p",
			modules: []string{`
				package test

				p { q.foo }
				p { q.foo }

				q[x] = 1 { input[x] }`,
			},
			wantQueries: []string{`input.foo`, `input.foo`},
		},
		{
			note:  "partial set rules not memoized",
			query: "data.test.p",
			modules: []string{`
				package test

				p { q.foo }
				p { q.foo }

				q[x] { input[x] }`,
			},
			wantQueries: []string{`input.foo`, `input.foo`},
		},
		{
			note:  "package path copied when skip partial namespace enabled (bug 3302)",
			query: "data.test.p = x",
			modules: []string{`
				package test
				pkg = "foo" { input.x = "foo" }
				pkg = "bar" { input.x = "bar" }
				p = x { k = pkg; x = data.other[k].p }
			`, `
				package other.foo
				p = 1 { input = a }
			`, `
				package other.bar
				p = 2 { input = a }
			`},
			wantQueries: []string{"data.test.p = x"},
			wantSupport: []string{
				`
					package other.foo

					p = 1 { input = a5 }
				`,
				`
					package other.bar

					p = 2 { input = a4 }
				`,
				`
					package test

					pkg = "foo" { input.x = "foo" }

					pkg = "bar" { input.x = "bar" }

					p = x1 { data.test.pkg = k1; "bar" = k1; data.other[k1].p = x1 }
					p = x1 { data.test.pkg = k1; "foo" = k1; data.other[k1].p = x1 }
				`,
			},
			shallow:              true,
			skipPartialNamespace: true,
		},
		{
			note:  "every: empty domain, no unknowns",
			query: "data.test.p",
			modules: []string{`package test
				p {
					every x in [] { x }
				}`},
			wantQueries: []string{``},
		},
		{
			note:  "every: no unknowns",
			query: "data.test.p",
			modules: []string{`package test
				p {
					every x in [1, 2, 3] { x != 4 }
				}`},
			wantQueries: []string{``},
		},
		{
			note:  "every: empty domain, unknowns in body",
			query: "data.test.p",
			modules: []string{`package test
				p {
					every x in [] { x > input }
				}`},
			wantQueries: []string{`every __local0__1, __local1__1 in [] {
				__local3__1 = input
				__local1__1 > __local3__1
			}`},
		},
		{
			note:  "every: known domain, unknowns in body",
			query: "data.test.p",
			modules: []string{`package test
				p {
					every x in [1, 2, 3] { x > input }
				}`},
			wantQueries: []string{`every __local0__1, __local1__1 in [1, 2, 3] {
				__local3__1 = input
				__local1__1 > __local3__1
			}`},
		},
		{
			note:  "every: known domain, unknowns in body (with call+assignment)",
			query: "data.test.p",
			modules: []string{`package test
				p {
					every x in [1, 2, 3] { y := x+10; y > input }
				}`},
			wantQueries: []string{`every __local0__1, __local1__1 in [1, 2, 3] {
				plus(__local1__1, 10, __local4__1)
				__local2__1 = __local4__1
				__local5__1 = input
				__local2__1 > __local5__1
			}`},
		},
		{
			note:  "every: known domain, unknowns in body, body impossible",
			query: "data.test.p",
			modules: []string{`package test
				p {
					every x in [1, 2, 3] { false; x > input }
				}`},
			wantQueries: []string{`every __local0__1, __local1__1 in [1, 2, 3] {
				false
				__local3__1 = input
				__local1__1 > __local3__1
			}`},
		},
		{
			note:  "every: unknown domain",
			query: "data.test.p",
			modules: []string{`package test
				p {
					every x in input { x > 1 }
				}`},
			wantQueries: []string{`every __local0__1, __local1__1 in input { __local1__1 > 1 }`},
		},
		{
			note:  "every: in-scope var in body",
			query: "data.test.p",
			modules: []string{`package test
				p {
					y := 3
					every x in [1, 2] { x != 0; input > y }
				}`},
			wantQueries: []string{`every __local1__1, __local2__1 in [1, 2] { __local2__1 != 0; __local4__1 = input; __local4__1 > 3 }`},
		},
		{
			note:  "every: unknown domain, call in body",
			query: "data.test.p",
			modules: []string{`package test
				p {
					every x in input {
						y = concat(",", [x])
					}
				}`},
			wantQueries: []string{`every __local0__1, __local1__1 in input { concat(",", [__local1__1], __local3__1); y1 = __local3__1 }`},
		},
		{
			note:  "every: closing over function args",
			query: "data.test.p",
			modules: []string{`package test
				p {
					f(input)
				}
				f(x) {
					every y in [1] {
						a = x
						1 == y
					}
				}`},
			wantQueries: []string{`every __local1__2, __local2__2 in [1] { a2 = input; 1 = __local2__2 }`},
		},
		{
			note:  "every: nested and closing over function args",
			query: "data.test.p",
			modules: []string{`package test
				p {
					f(input)
				}
				f(x) {
					every y in [1] {
						every z in [2] {
							a = x
							z > y
						}
					}
				}`},
			wantQueries: []string{`every __local1__2, __local2__2 in [1] {
				__local6__2 = [2]
				every __local3__2, __local4__2 in __local6__2 {
					a2 = input; __local4__2 > __local2__2 }
				}`},
		},
		{ // https://github.com/open-policy-agent/opa/issues/5367
			note:  "copypropagation: keep equations that are only found in comprehensions, inlined function call",
			query: "data.test.p",
			modules: []string{`package test
			key_exists(obj, k) { x = obj[k] }
			
			p {
				key_exists(input, "foo")
				{ true | input.foo }
			}`},
			wantQueries: []string{`{true | input.foo} = x_term_1_21; x_term_1_21; x2 = input.foo`},
		},
		{ // condensed form of the test above
			note:  "copypropagation: keep equations that are only found in comprehensions",
			query: "data.test.p",
			modules: []string{`package test
			p {
				x = input.foo
				{ true | input.foo }
			}`},
			wantQueries: []string{`{true | input.foo} = x_term_1_11; x_term_1_11; x1 = input.foo`},
		},
		{
			note:  "copypropagation: keep equations that are only found in 'every' body",
			query: "data.test.p",
			modules: []string{`package test
			p {
				x = input.foo
				every y in input.ys { y = input.foo }
			}`},
			wantQueries: []string{`every __local0__1, __local1__1 in input.ys { __local1__1 = input.foo }; x1 = input.foo`},
		},
		{ // https://github.com/open-policy-agent/opa/issues/6027
			note:  "ref heads: \"double\" unification, single-value rule",
			query: "data.test.foo[input.a][input.b]",
			modules: []string{`package test
			foo.bar[baz] {
				baz := "baz"
			}`},
			wantQueries: []string{`"bar" = input.a; "baz" = input.b`},
		},
		{
			note:  "general ref heads: \"triple\" unification, single-value rule",
			query: "data.test.foo[input.a][input.b][input.c]",
			modules: []string{`package test
			foo.bar[baz][bax] {
				baz := "baz"
				bax := "bax"
			}`},
			wantQueries: []string{`"bar" = input.a; "baz" = input.b; "bax" = input.c`},
		},
		{ // https://github.com/open-policy-agent/opa/issues/6027
			note:  "ref heads: \"double\" unification, multi-value rule",
			query: "data.test.foo[input.a][input.b]",
			modules: []string{`package test
			import future.keywords.contains
			foo.bar contains baz {
				baz := "baz"
			}`},
			wantQueries: []string{`"bar" = input.a; "baz" = input.b`},
		},
		{
			note:  "general ref heads: \"triple\" unification, multi-value rule",
			query: "data.test.foo[input.a][input.b][input.c]",
			modules: []string{`package test
			import future.keywords.contains
			foo.bar[baz] contains bax {
				baz := "baz"
				bax := "bax"
			}`},
			wantQueries: []string{`"bar" = input.a; "baz" = input.b; "bax" = input.c`},
		},
		{
			note:    "ref heads: unknown rule value",
			query:   "data.test.p.q[x]",
			shallow: false,
			modules: []string{`package test
			p.q[x] := y {
				x := "foo"
				y := input.y
			}`},
			wantQueries: []string{`input.y; x = "foo"`},
		},
		{
			note:  "ref heads: unknown ref var, unknown rule value",
			query: "data.test.p.q[x]",
			modules: []string{`package test
			p.q[x] := y {
				x := input.x
				y := input.y
			}`},
			wantQueries: []string{`x = input.x; input.y`},
		},
		{
			note:    "ref heads: unknown rule value, shallow inlining",
			query:   "data.test.p.q.r[x]",
			shallow: true,
			modules: []string{`package test
			p.q.r.s := y {
				y := input.y
			}`},
			wantQueries: []string{`data.partial.test.p.q.r.s = x_term_0_0; x_term_0_0; x = "s"`},
			wantSupport: []string{`package partial.test.p.q.r
			s = __local0__1 { 
				__local0__1 = input.y 
			}`},
		},
		{
			note:    "ref heads: unknown rule value, part-way query, shallow inlining",
			query:   "y = data.test.p.q[x]",
			shallow: true,
			modules: []string{`package test
			p.q.r.s := y {
				y := input.y
			}`},
			wantQueries: []string{`data.test.p.q.r = y; x = "r"`},
		},
		{ // https://github.com/open-policy-agent/opa/issues/6094
			note:    "ref heads: ref var, unknown rule value, shallow inlining",
			query:   "data.test.p.q[x]",
			shallow: true,
			modules: []string{`package test
			p.q[x] := y {
				x := "foo"
				y := input.y
			}`},
			wantQueries: []string{`data.partial.test.p.q[x] = x_term_0_0; x_term_0_0`},
			wantSupport: []string{`package partial.test.p
			q.foo = __local1__1 { 
				__local1__1 = input.y 
			}`},
		},
		{ // https://github.com/open-policy-agent/opa/issues/6094
			note:    "ref heads: unknown ref var, unknown rule value, shallow inlining",
			query:   "data.test.p.q[x]",
			shallow: true,
			modules: []string{`package test
			p.q[x] := y {
				x := input.x
				y := input.y
			}`},
			wantQueries: []string{`data.partial.test.p.q[x] = x_term_0_0; x_term_0_0`},
			wantSupport: []string{`package partial.test.p
			q[__local0__1] = __local1__1 { 
				__local0__1 = input.x
				__local1__1 = input.y
			}`},
		},
		{ // https://github.com/open-policy-agent/opa/issues/6094
			note:    "ref heads: unknown ref var, unknown rule value, shallow inlining",
			query:   "data.test.p.q.r.s[x]",
			shallow: true,
			modules: []string{`package test
			p.q.r.s[x] := y {
				x := input.x
				y := input.y
			}`},
			wantQueries: []string{`data.partial.test.p.q.r.s[x] = x_term_0_0; x_term_0_0`},
			wantSupport: []string{`package partial.test.p.q.r
			s[__local0__1] = __local1__1 { 
				__local0__1 = input.x
				__local1__1 = input.y
			}`},
		},
		{ // https://github.com/open-policy-agent/opa/issues/6094
			note:    "ref heads, partial set: unknown key, shallow inlining",
			query:   "data.test.p.q[x]",
			shallow: true,
			modules: []string{`package test
			import future.keywords.contains
			p.q contains y {
				y := input.y
			}`},
			wantQueries: []string{`data.partial.test.p.q[x] = x_term_0_0; x_term_0_0`},
			wantSupport: []string{`package partial.test.p
			q[__local0__1] { 
				__local0__1 = input.y
			}`},
		},
	}

	ctx := context.Background()

	for _, tc := range tests {
		params := fixtureParams{
			note:       tc.note,
			query:      tc.query,
			modules:    tc.modules,
			moduleASTs: tc.moduleASTs,
			data:       tc.data,
			input:      tc.input,
		}
		prepareTest(ctx, t, params, func(ctx context.Context, t *testing.T, f fixture) {

			var save []string

			if tc.unknowns == nil {
				save = []string{"input"}
			} else {
				save = tc.unknowns
			}

			unknowns := make([]*ast.Term, len(save))
			for i, s := range save {
				unknowns[i] = ast.MustParseTerm(s)
			}

			disableInlining := make([]ast.Ref, len(tc.disableInlining))
			for i, s := range tc.disableInlining {
				disableInlining[i] = ast.MustParseRef(s)
			}

			var buf BufferTracer

			query := NewQuery(f.query).
				WithCompiler(f.compiler).
				WithStore(f.store).
				WithTransaction(f.txn).
				WithInput(f.input).
				WithTracer(&buf).
				WithUnknowns(unknowns).
				WithDisableInlining(disableInlining).
				WithSkipPartialNamespace(tc.skipPartialNamespace).
				WithShallowInlining(tc.shallow)

			// Set genvarprefix so that tests can refer to vars in generated
			// expressions.
			query.genvarprefix = "x"

			partials, support, err := query.PartialRun(ctx)

			if err != nil {
				if buf != nil {
					PrettyTrace(os.Stdout, buf)
				}
				t.Fatal(err)
			}

			var expectedQueries []ast.Body

			opts := ast.ParserOptions{AllFutureKeywords: true}
			if len(tc.wantQueryASTs) > 0 {
				expectedQueries = tc.wantQueryASTs
			} else {
				expectedQueries = make([]ast.Body, len(tc.wantQueries))
				for i := range tc.wantQueries {
					expectedQueries[i] = ast.MustParseBodyWithOpts(tc.wantQueries[i], opts)
				}
			}

			queriesA, queriesB := bodySet(partials), bodySet(expectedQueries)
			if !queriesB.Equal(queriesA, tc.ignoreOrder) {
				missing := queriesB.Diff(queriesA, tc.ignoreOrder)
				extra := queriesA.Diff(queriesB, tc.ignoreOrder)
				t.Errorf("Partial evaluation results differ. Expected %d queries but got %d queries:\nMissing:\n%v\nExtra:\n%v", len(queriesB), len(queriesA), missing, extra)
			}

			var expectedSupport []*ast.Module
			if len(tc.wantSupportASTs) > 0 {
				expectedSupport = tc.wantSupportASTs
			} else {
				for i := range tc.wantSupport {
					expectedSupport = append(expectedSupport, ast.MustParseModule(tc.wantSupport[i]))
				}
			}
			supportA, supportB := moduleSet(support), moduleSet(expectedSupport)
			if !supportA.Equal(supportB) {
				missing := supportB.Diff(supportA)
				extra := supportA.Diff(supportB)
				t.Errorf("Partial evaluation results differ. Expected %d modules but got %d:\nMissing:\n%v\nExtra:\n%v", len(supportB), len(supportA), missing, extra)
			}
		})
	}
}

type fixtureParams struct {
	note       string
	data       string
	modules    []string
	moduleASTs []*ast.Module
	query      string
	input      string
}

type fixture struct {
	query    ast.Body
	compiler *ast.Compiler
	store    storage.Store
	txn      storage.Transaction
	input    *ast.Term
}

func prepareTest(ctx context.Context, t *testing.T, params fixtureParams, f func(context.Context, *testing.T, fixture)) {

	t.Run(params.note, func(t *testing.T) {

		var store storage.Store

		if len(params.data) > 0 {
			j := util.MustUnmarshalJSON([]byte(params.data))
			store = inmem.NewFromObject(j.(map[string]interface{}))
		} else {
			store = inmem.New()
		}

		err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

			compiler := ast.NewCompiler()
			modules := map[string]*ast.Module{}
			opts := ast.ParserOptions{AllFutureKeywords: true}

			if len(params.moduleASTs) > 0 {
				for i, module := range params.moduleASTs {
					modules[fmt.Sprint(i)] = module
				}
			}
			for i, module := range params.modules {
				j := len(params.moduleASTs) + i
				modules[fmt.Sprint(j)] = ast.MustParseModuleWithOpts(module, opts)
			}

			if compiler.Compile(modules); compiler.Failed() {
				t.Fatal(compiler.Errors)
			}

			var input *ast.Term
			if len(params.input) > 0 {
				input = ast.MustParseTerm(params.input)
			}

			queryContext := ast.NewQueryContext()

			queryCompiler := compiler.QueryCompiler().WithContext(queryContext)

			compiledQuery, err := queryCompiler.Compile(ast.MustParseBody(params.query))
			if err != nil {
				t.Fatal(err)
			}

			f(ctx, t, fixture{
				query:    compiledQuery,
				compiler: compiler,
				store:    store,
				txn:      txn,
				input:    input,
			})

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

type bodySet []ast.Body

func (s bodySet) String() string {
	buf := make([]string, len(s))
	for i := range s {
		buf[i] = fmt.Sprintf("body %d: %v", i+1, s[i].String())
	}
	return strings.Join(buf, "\n")
}

func (s bodySet) Contains(b ast.Body, ignoreOrder bool) bool {
	for i := range s {
		if ignoreOrder {
			if bodyEqualUnordered(b, s[i]) {
				return true
			}
		} else {
			if s[i].Equal(b) {
				return true
			}
		}
	}
	return false
}

func (s bodySet) Diff(other bodySet, ignoreOrder bool) (r bodySet) {
	for i := range s {
		if !other.Contains(s[i], ignoreOrder) {
			r = append(r, s[i])
		}
	}
	return r
}

func (s bodySet) Equal(other bodySet, ignoreOrder bool) bool {
	return len(s.Diff(other, ignoreOrder)) == 0 && len(other.Diff(s, ignoreOrder)) == 0
}

func bodyEqualUnordered(a, b ast.Body) bool {
	for i := range a {
		found := false
		for j := range b {
			cpy := b[j].Copy()
			cpy.Index = a[i].Index // overwrite index to ensure comparison is unordered.
			if a[i].Compare(cpy) == 0 {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

type moduleSet []*ast.Module

func (s moduleSet) String() string {
	buf := make([]string, len(s))
	for i := range s {
		buf[i] = fmt.Sprintf("module %d: %v", i+1, s[i].String())
	}
	return strings.Join(buf, "\n")
}

func (s moduleSet) Contains(b *ast.Module) bool {
	for i := range s {
		if s[i].Package.Equal(b.Package) {
			rs1 := ast.NewRuleSet(s[i].Rules...)
			rs2 := ast.NewRuleSet(b.Rules...)
			if rs1.Equal(rs2) {
				return true
			}
		}
	}
	return false
}

func (s moduleSet) Diff(other moduleSet) (r moduleSet) {
	for i := range s {
		if !other.Contains(s[i]) {
			r = append(r, s[i])
		}
	}
	return r
}

func (s moduleSet) Equal(other moduleSet) bool {
	return len(s.Diff(other)) == 0 && len(other.Diff(s)) == 0
}
