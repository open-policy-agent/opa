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
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

func TestTopDownPartialEval(t *testing.T) {

	tests := []struct {
		note            string
		unknowns        []string
		disableInlining []string
		query           string
		modules         []string
		data            string
		input           string
		wantQueries     []string
		wantSupport     []string
		ignoreOrder     bool
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
		{
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
		{
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
			wantQueries: []string{`
				input[x_term_1_01]
				x_term_1_01.b = 2
				x_term_1_01
				x_term_1_01.a = 1
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
			note:        "comprehensions: save",
			query:       `x = [true | true]; y = {true | true}; z = {a: true | a = "foo"}`,
			wantQueries: []string{`x = [true | true]; y = {true | true}; z = {a: true | a = "foo"}`},
		},
		{
			note:        "comprehensions: closure",
			query:       `i = 1; xs = [x | x = data.foo[i]]`,
			wantQueries: []string{`xs = [x | x = data.foo[1]; 1 = 1]; i = 1`},
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
			note:  "save: full extent: partial set",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p[x] { input.x = x }
				p[x] { input.y = x }`,
			},
			wantQueries: []string{`data.test.p = x`},
		},
		{
			note:  "save: full extent: partial object",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p[x] = y { x = input.x; y = input.y }
				p[x] = y { x = input.z; y = input.a }`,
			},
			wantQueries: []string{`data.test.p = x`},
		},
		{
			note:  "save: full extent",
			query: "data.test = x",
			modules: []string{
				`package test.a
				p = 1`,
				`package test
				q = 2`,
			},
			wantQueries: []string{`data.test = x`},
		},
		{
			note:  "save: full extent: iteration",
			query: "data.test[x] = y",
			modules: []string{
				`package test
				s[x] { x = input.x }
				p[x] = y { x = input.x; y = input.y }
				r = x { x = input.x }`,
			},
			wantQueries: []string{
				`data.test.s = y; x = "s"`,
				`data.test.p = y; x = "p"`,
				`y = input.x; x = "r"`,
			},
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
			note:  "save: with",
			query: "data.test.p = true",
			modules: []string{
				`package test
				p { input.x = 1; q with input as {"y": 2} }
				q { input.y = 2 }`,
			},
			wantQueries: []string{
				`input.x = 1; data.test.q with input as {"y": 2}`,
			},
		},
		{
			note:  "save: else",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p = x { q = x }
				q = 100 { false } else = 200 { true }`,
			},
			wantQueries: []string{
				`data.test.q = x`,
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
				`sort(input.x, x1); x1[0] > 1`,
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
				`not ((input.x + 7) / input.y) > 100`,
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
				`not ((input.x + 7) / input.y) > 100`,
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
				`input.x = 1; not data.partial.__not1_1__; not input.y = 2; input.z = [z38]; z38[0] = 1`,
			},
			wantSupport: []string{
				`package partial

				__not1_1__ {
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
			wantQueries: []string{`startswith(input.x[i2], "foo"); not data.partial.__not1_1__`},
			wantSupport: []string{
				`package partial

				__not1_1__ { input.y[i4] = 1 }`,
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
			wantQueries: []string{`not data.partial.__not1_1__(input.x)`},
			wantSupport: []string{`
				package partial

				__not1_1__(x1) {
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
				`input = x; not count(x) != 3`,
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
				`not count(input) > 3`,
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
				`not sum([input.y, 1]) > 3`,
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
				`input[i1].a = "foo"; input[i1].b < 7`,
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
			wantQueries: []string{`not data.partial.__not1_0__`},
			wantSupport: []string{
				`package partial

				__not1_0__ { input.x = 1; input.y = 1; input.z = 0 }
				__not1_0__ { input.x = 2; input.y = 2; input.z = 0 }
				__not1_0__ { input.x = 3; input.y = 3; input.z = 0 }
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
			note:  "comprehensions: ref heads (with namespacing)",
			query: "data.test.p = true; input.x = x",
			modules: []string{
				`package test

				p {
					x = [0]; y = {true | x[0]}
				}
			`},
			wantQueries: []string{`y1 = {true | x1[0]; x1 = [0]}; input.x = x`},
		},
		{
			note:        "comprehensions: ref heads (with live vars)",
			query:       "x = [0]; y = {true | x[0]}",
			wantQueries: []string{`y = {true | x[0]; x = [0]}; x = [0]`},
		},
	}

	ctx := context.Background()

	for _, tc := range tests {
		params := fixtureParams{
			note:    tc.note,
			query:   tc.query,
			modules: tc.modules,
			data:    tc.data,
			input:   tc.input,
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
				WithDisableInlining(disableInlining)

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

			expectedQueries := make([]ast.Body, len(tc.wantQueries))
			for i := range tc.wantQueries {
				expectedQueries[i] = ast.MustParseBody(tc.wantQueries[i])
			}

			queriesA, queriesB := bodySet(partials), bodySet(expectedQueries)
			if !queriesB.Equal(queriesA, tc.ignoreOrder) {
				missing := queriesB.Diff(queriesA, tc.ignoreOrder)
				extra := queriesA.Diff(queriesB, tc.ignoreOrder)
				t.Errorf("Partial evaluation results differ. Expected %d queries but got %d queries:\nMissing:\n%v\nExtra:\n%v", len(queriesB), len(queriesA), missing, extra)
			}

			expectedSupport := make([]*ast.Module, len(tc.wantSupport))
			for i := range tc.wantSupport {
				expectedSupport[i] = ast.MustParseModule(tc.wantSupport[i])
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
	note    string
	data    string
	modules []string
	query   string
	input   string
}

type fixture struct {
	query    ast.Body
	compiler *ast.Compiler
	store    storage.Store
	txn      storage.Transaction
	input    *ast.Term
}

func prepareTest(ctx context.Context, t *testing.T, params fixtureParams, f func(context.Context, *testing.T, fixture)) {

	test.Subtest(t, params.note, func(t *testing.T) {

		var store storage.Store

		if len(params.data) > 0 {
			j := util.MustUnmarshalJSON([]byte(params.data))
			store = inmem.NewFromObject(j.(map[string]interface{}))
		} else {
			store = inmem.New()
		}

		storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

			compiler := ast.NewCompiler()
			modules := map[string]*ast.Module{}

			for i, module := range params.modules {
				modules[fmt.Sprint(i)] = ast.MustParseModule(module)
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
