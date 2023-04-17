// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package lineage

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown"
)

func TestFilter(t *testing.T) {

	tests := []struct {
		note   string
		module string
		exp    string
	}{
		{
			note: "lineage",
			module: `package test

			p { q }
			q { r }
			r { trace("R") }`,
			exp: `
Enter data.test.p = x
| Enter data.test.p
| | Enter data.test.q
| | | Enter data.test.r
| | | | Note "R"`,
		},
		{
			note: "conjunction",
			module: `package test

			p { trace("P1"); trace("P2") }`,
			exp: `
Enter data.test.p = x
| Enter data.test.p
| | Note "P1"
| | Note "P2"`,
		},
		{
			note: "conjunction (multiple enters)",
			module: `package test

			p { q; r }
			q { trace("Q") }
			r { trace("Q") }
			`,
			exp: `
Enter data.test.p = x
| Enter data.test.p
| | Enter data.test.q
| | | Note "Q"
| | Enter data.test.r
| | | Note "Q"`,
		},
		{
			note: "disjunction",
			module: `package test

			p = x { x := true; trace("P1") }
			p = x { x := true; false }
			p = x { x := true; trace("P2") }
			`,
			exp: `
Enter data.test.p = x
| Enter data.test.p
| | Note "P1"
Redo data.test.p = x
| Enter data.test.p
| | Note "P2"`,
		},
		{
			note: "disjunction (failure)",
			module: `package test

			p = x { x := true; trace("P1") }
			p = x { x := true; trace("P2"); false }
			p = x { x := true; trace("P3") }
			`,
			exp: `
Enter data.test.p = x
| Enter data.test.p
| | Note "P1"
Redo data.test.p = x
| Enter data.test.p
| | Note "P2"
| Enter data.test.p
| | Note "P3"`,
		},
		{
			note: "disjunction (iteration)",
			module: `package test
			q[1]
			q[2]
			p { q[x]; trace(sprintf("x=%d", [x])) }`,
			exp: `
Enter data.test.p = x
| Enter data.test.p
| | Note "x=1"
`,
		},
		{
			note: "parent child",
			module: `package test

			p { trace("P"); q }
			q { trace("Q") }`,
			exp: `
Enter data.test.p = x
| Enter data.test.p
| | Note "P"
| | Enter data.test.q
| | | Note "Q"`,
		},
		{
			note: "negation",
			module: `package test

			p { not q }
			q = false { trace("Q") }`,
			exp: `
Enter data.test.p = x
| Enter data.test.p
| | Enter data.test.q
| | | Enter data.test.q
| | | | Note "Q"`,
		},
		{
			note: "fail",
			module: `package test

			p { not q }
			q { trace("P"); 1 = 2 }`,
			exp: `
Enter data.test.p = x
| Enter data.test.p
| | Enter data.test.q
| | | Enter data.test.q
| | | | Note "P"`,
		},
		{
			note: "comprehensions",
			module: `package test

			p { [true | true; trace("X")] }`,
			exp: `
Enter data.test.p = x
| Enter data.test.p
| | Enter true; trace("X")
| | | Note "X"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			buf := topdown.NewBufferTracer()
			compiler := ast.MustCompileModules(map[string]string{
				"test.rego": tc.module,
			})
			query := topdown.NewQuery(ast.MustParseBody("data.test.p = x")).WithCompiler(compiler).WithTracer(buf)
			rs, err := query.Run(context.TODO())
			if err != nil {
				t.Fatal(err)
			} else if len(rs) != 1 || !rs[0][ast.Var("x")].Equal(ast.BooleanTerm(true)) {
				t.Fatalf("Unexpected result: %v", rs)
			}

			filtered := Notes(*buf)

			buffer := bytes.NewBuffer(nil)
			topdown.PrettyTrace(buffer, filtered)

			if strings.TrimSpace(buffer.String()) != strings.TrimSpace(tc.exp) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", tc.exp, buffer.String())
			}
		})
	}

}
