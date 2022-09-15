// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package planner

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/ir"
)

func TestPlannerHelloWorld(t *testing.T) {

	// NOTE(tsandall): These tests are not meant to give comprehensive coverage
	// of the planner. Currently we have a suite of end-to-end tests in the
	// test/wasm/ directory that are specified in YAML, compiled into Wasm, and
	// executed inside of a node program. For the time being, the planner is
	// simple enough that exhaustive unit testing is not as valuable as
	// end-to-end testing. These tests provide a quick consistency check that the
	// planner is not failing on simple inputs.
	tests := []struct {
		note    string
		queries []string
		modules []string
	}{
		{
			note:    "empty",
			queries: []string{},
		},
		{
			note:    "hello world",
			queries: []string{"input.a = 1"},
		},
		{
			note:    "conjunction",
			queries: []string{"1 = 1; 2 = 2"},
		},
		{
			note:    "disjunction",
			queries: []string{"input.a = 1", "input.b = 2"},
		},
		{
			note:    "iteration",
			queries: []string{"input.a[i] = 1; input.b = 2"},
		},
		{
			note:    "iteration: compare key",
			queries: []string{"input.a[i] = 1; input.b = i"},
		},
		{
			note:    "iteration: nested",
			queries: []string{"input.a[i] = 1; input.b[j] = 2"},
		},
		{
			note:    "iteration: chained",
			queries: []string{"input.a[i][j] = 1"},
		},
		{
			note:    "negation",
			queries: []string{"not input.x.y = 1"},
		},
		{
			note:    "not and known vars", // https://github.com/open-policy-agent/opa/issues/3279
			queries: []string{`x = "foo"; not data.tenants[x]`},
		},
		{
			note:    "array ref pattern match",
			queries: []string{"input.x = [1, [y]]"},
		},
		{
			note:    "arrays pattern match",
			queries: []string{"[x, 3, [2]] = [1, 3, [y]]"},
		},
		{
			note:    "sets",
			queries: []string{"x = {1,2,3}; x[y]"},
		},
		{
			note:    "vars",
			queries: []string{"x = 1"},
		},
		{
			note:    "complete rules",
			queries: []string{"true"},
			modules: []string{`
				package test
				p = x { x = 1 }
				p = y { y = 2 }
			`},
		},
		{
			note:    "complete rule reference",
			queries: []string{"data.test.p = 10"},
			modules: []string{`
				package test
				p = x { x = 10 }
			`},
		},
		{
			note:    "functions",
			queries: []string{"data.test.f([1,x])"},
			modules: []string{`
				package test
				f([a, b]) {
					a = b
				}
			`},
		},
		{
			note:    "else",
			queries: []string{"data.test.p = 1"},
			modules: []string{`
				package test
				p = 0 {
					false
				} else = 1 {
					true
				}
			`},
		},
		{
			note:    "partial set",
			queries: []string{"data.test.p = {1,2}"},
			modules: []string{`
				package test
				p[1]
				p[2]
			`},
		},
		{
			note:    "partial object",
			queries: []string{`data.test.p = {"a": 1, "b": 2}`},
			modules: []string{`
				package test
				p["a"] = 1
				p["b"] = 2
			`},
		},
		{
			note:    "every",
			queries: []string{`data.test.p`},
			modules: []string{`
				package test
				p { xs = [1]; every k, v in xs { k < v } }
			`},
		},
		{
			note:    "virtual extent",
			queries: []string{`data`},
			modules: []string{`
				package test

				p = 1
				q = 2 { false }
			`},
		},
		{
			note:    "comprehension",
			queries: []string{`{x | input[_] = x}`},
		},
		{
			note:    "object comprehension in policy",
			queries: []string{`data.test.a = x`},
			modules: []string{`
				package test

				a = { "a": "b" |  1 > 0 }
			`},
		},
		{
			note:    "closure",
			queries: []string{`a = [1]; {x | a[_] = x}`},
		},
		{
			note:    "iteration: packages and rules",
			queries: []string{"data.test[x][y] = 3"},
			modules: []string{
				`
					package test.a

					p = 1
					q = 2 { false }
					r = 3
				`,
				`
					package test.z

					s = 3
					t = 4
				`,
			},
		},
		{
			note:    "variables in query",
			queries: []string{"x = 1", "y = 2", "x = 1; y = 2"},
		},
		{
			note: "with keyword",
			queries: []string{
				`input[i] = 1 with input as [1]; i > 1`,
			},
		},
		{
			note:    "with keyword data",
			queries: []string{`data = x with data.foo as 1 with data.bar.r as 3`},
			modules: []string{
				`package foo

				p = 1`,
				`package bar

				q = 2`,
			},
		},
		{
			note:    "with keyword - virtual doc iteration",
			queries: []string{`x = data[i][j] with data.bar as 1; y = "a"`},
			modules: []string{
				`package foo

				p = 0
				q = 1
				r = 2`,
			},
		},
		{
			note:    "relation non-empty",
			queries: []string{`walk(input)`},
		},
		{
			note:    "relation unify",
			queries: []string{`walk(input, [["foo", y], x])`},
		},
		{
			note:    "else conflict-1",
			queries: []string{`data.p.q`},
			modules: []string{
				`package p

				q {
					false
				}
				else = true {
					true
				}
				q = false
				`,
			},
		},
		{
			note:    "else conflict-2",
			queries: []string{`data.p.q`},
			modules: []string{
				`package p

				q {
					false
				}
				else = false {
					true
				}
				q {
					false
				}
				else = true {
					true
				}`,
			},
		},
		{
			note:    "multiple function outputs (single)",
			queries: []string{`data.p.r`},
			modules: []string{
				`package p

				p(a) = y {
				  y = a[_]
				}

				r = y {
				  data.p.p([1, 2, 3], y)
				}
				`,
			},
		},
		{
			note:    "multiple function outputs (multiple)",
			queries: []string{`data.p.r`},
			modules: []string{
				`package p

				p(1, a) = y {
					y = a
				}
				p(x, y) = z {
					z = x
				}

				r = y {
					data.p.p(1, 0, y)
				}
				`,
			},
		},
		{
			note:    "cross product with non-ground refs to packages (simplified)",
			queries: []string{`x := "aaa"; y := "bbb"; z := "q"; w := data.foo[x].bar[y].baz[z]`},
			modules: []string{`package foo.aaa.bar.bbb.baz
p = 1
q = 2`,
				`package foo.ccc.bar.bbb.baz
p = 10
q = 20`,
			},
		},
		{
			note: `non-ground refs to packages (including "with")`,
			// NOTE(sr): data.foo.bbb does not change the outcomes, but triggers
			// a gen++ in p.funcs, leading to g1, g2 function being planned and
			// referenced
			queries: []string{`x := "aaa"; y := "bbb"; z := "q"; w := data.foo[x].bar[y].baz[z] with data.foo.bbb as true`},
			modules: []string{`package foo.aaa.bar.bbb.baz
p = 1
q = 2`,
			},
		},
		{
			note:    "cross product with non-ground refs to packages, 'no rules leaves' case",
			queries: []string{`x := "aaa"; y := data.foo[x].bar.baz`},
			modules: []string{`package foo.aaa.bar`, `package foo.bbb.bar`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			queries := make([]ast.Body, len(tc.queries))
			for i := range queries {
				queries[i] = ast.MustParseBody(tc.queries[i])
			}
			modules := make([]*ast.Module, len(tc.modules))
			for i := range modules {
				file := fmt.Sprintf("module-%d.rego", i)
				opts := ast.ParserOptions{AllFutureKeywords: true}
				m, err := ast.ParseModuleWithOpts(file, tc.modules[i], opts)
				if err != nil {
					t.Fatal(err)
				}
				modules[i] = m
			}
			planner := New().WithQueries([]QuerySet{
				{
					Name:    "test",
					Queries: queries,
				},
			}).WithModules(modules).WithBuiltinDecls(ast.BuiltinMap)

			if testing.Verbose() {
				planner = planner.WithDebug(os.Stderr)
			}
			policy, err := planner.Plan()
			if err != nil {
				t.Fatal(err)
			}
			if testing.Verbose() {
				err = ir.Pretty(os.Stderr, policy)
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

type cmpWalker struct {
	needle interface{}
	loc    string
	found  bool // stop comparing after first found needle
}

func (*cmpWalker) Before(interface{}) {}
func (*cmpWalker) After(interface{})  {}

// Visit takes, for example,
//
//	*ir.MakeNullStmt{Location: ir.Location{Index:0, Col:1, Row:1}},
//
// and for the first MakeNullStmt it finds, extracts its location,
// and compares it to the one passed was needle. Other fields of the
// struct, such as Target for ir.MakeNullStmt, are ignored.
//
// Caveat: If NO value of the desired type is found, there's no error
// returned. This trap can be avoided by starting with a failing test,
// and proceeding with caution. ;)
func (f *cmpWalker) Visit(x interface{}) (ir.Visitor, error) {
	if !f.found && reflect.TypeOf(f.needle) == reflect.TypeOf(x) {
		f.found = true
		expLoc := f.loc
		actLoc := getLocation(x)
		if expLoc != actLoc {
			return f, fmt.Errorf("unexpected location for %T:\nwant: %s\ngot:  %s", x, expLoc, actLoc)
		}
	}
	return f, nil
}

func getLocation(x interface{}) string {
	v := reflect.ValueOf(x).Elem().FieldByName("Location")
	li := v.Interface()
	file := v.FieldByName("file").String()
	text := v.FieldByName("text").String()
	if loc, ok := li.(ir.Location); ok {
		return fmt.Sprintf("%s:%d:%d: %s", file, loc.Row, loc.Col, text)
	}
	return "unknown"
}

func findInPolicy(needle interface{}, loc string, p interface{}) error {
	return ir.Walk(&cmpWalker{needle: needle, loc: loc}, p)
}

// Assert some selected statements' location mappings. Note that for debugging,
// it's worthwhile to no use tabs in the multi-line strings, as they may be
// counted differently in the editor vs. in code.
func TestPlannerLocations(t *testing.T) {

	funcs := func(p *ir.Policy) interface{} {
		return p.Funcs
	}

	tests := []struct {
		note    string
		queries []string
		modules []string
		exps    map[ir.Stmt]string           // stmt -> expected location "file:row:col: text"
		where   func(*ir.Policy) interface{} // where to start walking search for `exps`
	}{
		{
			note:    "complete rule reference",
			queries: []string{"data.test.p = 10"},
			modules: []string{`
package test
p = x {
  1 > 0
  x = 10
  true
}
`},
			exps: map[ir.Stmt]string{
				&ir.CallStmt{}:          "<query>:1:1: data.test.p = 10",
				&ir.AssignVarStmt{}:     "module-0.rego:5:3: x = 10",
				&ir.AssignVarOnceStmt{}: "module-0.rego:3:1: p = x",
				&ir.ReturnLocalStmt{}:   "module-0.rego:3:1: p = x",
			},
		},
		{
			note:    "partial set",
			queries: []string{"data.test.p = {1,2}"},
			modules: []string{`
package test
p[1]
p[2]
			`},
			exps: map[ir.Stmt]string{
				&ir.MakeSetStmt{}:     "module-0.rego:3:1: p[1]",
				&ir.ReturnLocalStmt{}: "module-0.rego:3:1: p[1]",
			},
			where: funcs,
		},
		{
			note:    "partial set with rule body",
			queries: []string{"data.test.p = {1,2}"},
			modules: []string{`
package test
p[1] {
  1 > 2
}
			`},
			exps: map[ir.Stmt]string{
				&ir.CallStmt{}:   "module-0.rego:4:3: 1 > 2",
				&ir.SetAddStmt{}: "module-0.rego:3:1: p[1]",
			},
			where: funcs,
		},
		{
			note:    "partial object",
			queries: []string{`data.test.p = {"a": 1, "b": 2}`},
			modules: []string{`
package test
p["a"] = 1 {
  false
}
			`},
			exps: map[ir.Stmt]string{
				&ir.MakeObjectStmt{}:       `module-0.rego:3:1: p["a"] = 1`,
				&ir.ObjectInsertOnceStmt{}: `module-0.rego:3:1: p["a"] = 1`,
			},
			where: funcs,
		},
		{
			note:    "default rule",
			queries: []string{`data.test.p = x`},
			modules: []string{`
package test
default p = {"foo": "bar"}
p = x {
  x := {"baz": "quz"}
}
			`},
			exps: map[ir.Stmt]string{
				&ir.IsUndefinedStmt{}:   `module-0.rego:3:9: p = {"foo": "bar"}`,
				&ir.MakeObjectStmt{}:    `module-0.rego:3:9: p = {"foo": "bar"}`,
				&ir.AssignVarOnceStmt{}: `module-0.rego:3:9: p = {"foo": "bar"}`,
			},
			where: func(p *ir.Policy) interface{} {
				return p.Funcs.Funcs[0].Blocks[2] // default rule block
			},
		},
		{
			note:    "object comprehension in policy",
			queries: []string{`data.test.a = x`},
			modules: []string{
				`package test
a = { "a": "b" |
  1 > 0
}`},
			exps: map[ir.Stmt]string{
				&ir.CallStmt{}:             "module-0.rego:3:3: 1 > 0",
				&ir.ObjectInsertOnceStmt{}: "module-0.rego:2:5: { \"a\": \"b\" |\n  1 > 0\n}",
			},
			where: funcs,
		},
		{
			note:    "array comprehension in policy",
			queries: []string{`data.test.a = x`},
			modules: []string{
				`package test
a = [ "a" |
  1 > 0
]`},
			exps: map[ir.Stmt]string{
				&ir.CallStmt{}:        "module-0.rego:3:3: 1 > 0",
				&ir.ArrayAppendStmt{}: "module-0.rego:2:5: [ \"a\" |\n  1 > 0\n]",
			},
			where: funcs,
		},
		{
			note:    "set comprehension in policy",
			queries: []string{`data.test.a = x`},
			modules: []string{
				`package test
a = { "a" |
  1 > 0
}`},
			exps: map[ir.Stmt]string{
				&ir.CallStmt{}:   "module-0.rego:3:3: 1 > 0",
				&ir.SetAddStmt{}: "module-0.rego:2:5: { \"a\" |\n  1 > 0\n}",
			},
			where: funcs,
		},
		{
			note:    "set in policy",
			queries: []string{`data.test.a = x`},
			modules: []string{`package test
a = { "a", 10 }`},
			exps: map[ir.Stmt]string{
				&ir.SetAddStmt{}: "module-0.rego:2:1: a = { \"a\", 10 }",
			},
		},
		{
			note:    "virtual extent",
			queries: []string{`data`},
			modules: []string{`package test
p = 1
q = 2 {
  false
}`},
			exps: map[ir.Stmt]string{
				&ir.CallStmt{}:         "<query>:1:1: data",
				&ir.ObjectInsertStmt{}: "<query>:1:1: data",
			},
			where: func(p *ir.Policy) interface{} {
				return p.Plans.Plans[0].Blocks[0].Stmts[4]
			},
		},
		{
			note:    "non-ground ref in query",
			queries: []string{`data[y].a = x`},
			modules: []string{`package test
a = true`},
			exps: map[ir.Stmt]string{
				&ir.CallStmt{}:         "<query>:1:1: data[y].a = x",
				&ir.MakeObjectStmt{}:   "<query>:1:1: data[y].a = x",
				&ir.ObjectInsertStmt{}: "<query>:1:1: data[y].a = x",
				&ir.ResultSetAddStmt{}: "<query>:1:1: data[y].a = x",
				&ir.DotStmt{}:          "<query>:1:1: data[y].a = x",
			},
			where: func(p *ir.Policy) interface{} {
				return p.Plans.Plans[0]
			},
		},
		{
			note:    "non-ground ref in policy",
			queries: []string{`data.test.a = x`},
			modules: []string{`package test
a {
  data.test1[_].y = "z"
}`},
			exps: map[ir.Stmt]string{
				&ir.CallStmt{}:         "<query>:1:1: data.test.a = x",
				&ir.MakeObjectStmt{}:   "<query>:1:1: data.test.a = x",
				&ir.ObjectInsertStmt{}: "<query>:1:1: data.test.a = x",
				&ir.ResultSetAddStmt{}: "<query>:1:1: data.test.a = x",
				&ir.ScanStmt{}:         `module-0.rego:3:3: data.test1[_].y = "z"`,
			},
		},
		{
			note:    "CallDynamicStmt optimization",
			queries: []string{`x := "a"; data.test[x] = y`},
			modules: []string{`package test
a {
  true
}`},
			exps: map[ir.Stmt]string{
				&ir.CallDynamicStmt{}: "<query>:1:11: data.test[x] = y",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			queries := make([]ast.Body, len(tc.queries))
			for i := range queries {
				queries[i] = ast.MustParseBody(tc.queries[i])
			}
			modules := make([]*ast.Module, len(tc.modules))
			for i := range modules {
				file := fmt.Sprintf("module-%d.rego", i)
				m, err := ast.ParseModule(file, tc.modules[i])
				if err != nil {
					t.Fatal(err)
				}
				modules[i] = m
			}
			planner := New().WithQueries([]QuerySet{
				{
					Name:    "test",
					Queries: queries,
				},
			}).WithModules(modules).WithBuiltinDecls(ast.BuiltinMap)
			policy, err := planner.Plan()
			if err != nil {
				t.Fatal(err)
			}
			if testing.Verbose() {
				err = ir.Pretty(os.Stderr, policy)
				if err != nil {
					t.Fatal(err)
				}
			}
			start := interface{}(policy)
			if tc.where != nil {
				start = tc.where(policy)
			}
			for exp, loc := range tc.exps {
				if err := findInPolicy(exp, loc, start); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

func TestMultipleNamedQueries(t *testing.T) {

	q1 := []ast.Body{
		ast.MustParseBody(`a=1`),
	}

	q2 := []ast.Body{
		ast.MustParseBody(`a=2`),
	}

	planner := New().WithQueries([]QuerySet{
		{
			Name:    "q1",
			Queries: q1,
		},
		{
			Name:    "q2",
			Queries: q2,
		},
	})

	policy, err := planner.Plan()
	if err != nil {
		t.Fatal(err)
	}

	if testing.Verbose() {
		err = ir.Pretty(os.Stderr, policy)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Consistency check to make sure two expected plans are emitted.
	if len(policy.Plans.Plans) != 2 {
		t.Fatal("expected two plans")
	} else if policy.Plans.Plans[0].Name != "q1" || policy.Plans.Plans[1].Name != "q2" {
		t.Fatal("expected to find plans for 'q1' and 'q2'")
	}
}

func ref(r string) ast.Ref {
	return ast.MustParseRef("data." + r)[1:]
}

func TestOptimizeLookup(t *testing.T) {
	r0, r1, r2 := ast.Rule{}, ast.Rule{}, ast.Rule{}

	t.Run("seen variable (last), one ruleset", func(t *testing.T) {
		r := newRuletrie()
		val := r.LookupOrInsert(ref("foo.bar"))
		val.rules = append(val.rules, &r0, &r1, &r2)

		p := New()
		l := p.newLocal()
		p.vars.Put(ast.Var("x"), l)

		rulesets, path, index, opt := p.optimizeLookup(r, ast.MustParseRef("data.foo[x]"))
		if exp, act := true, opt; exp != act {
			t.Errorf("expected 'optimize' %v, got %v\n", exp, act)
		}
		if exp, act := 2, index; exp != act {
			t.Errorf("expected 'index' %d, got %d\n", exp, act)
		}
		if exp, act := 1, len(rulesets); exp != act {
			t.Fatalf("expected %d rulesets, got %d\n", exp, act)
		}
		if exp, act := 3, len(rulesets[0]); exp != act {
			t.Errorf("expected %d rules, got %d\n", exp, act)
		}
		// 3 = g0 + foo + x
		if exp, act := 3, len(path); exp != act {
			t.Fatalf("expected path len %d, got %d\n", exp, act)
		}
		last, ok := path[len(path)-1].Value.(ir.Local)
		if exp, act := true, ok; exp != act {
			t.Fatalf("expected last path pieces to be local, got %T\n", last)
		}
		if exp, act := l, last; exp != act {
			t.Errorf("expected last local to be %v, got %v\n", exp, act)
		}
	})

	t.Run("ref shorter than ruletrie depth", func(t *testing.T) {
		r := newRuletrie()
		val := r.LookupOrInsert(ref("foo.bar.baz"))
		val.rules = append(val.rules, &r0, &r1, &r2)

		p := New()
		l := p.newLocal()
		p.vars.Put(ast.Var("x"), l)

		_, _, _, opt := p.optimizeLookup(r, ast.MustParseRef("data.foo[x]"))
		if exp, act := false, opt; exp != act {
			t.Errorf("expected 'optimize' %v, got %v\n", exp, act)
		}
	})

	t.Run("seen variable (last), multiple rulesets", func(t *testing.T) {
		r := newRuletrie()
		val := r.LookupOrInsert(ref("foo.bar"))
		val.rules = append(val.rules, &r0, &r1)
		val = r.LookupOrInsert(ref("foo.baz"))
		val.rules = append(val.rules, &r2)

		p := New()
		p.vars.Put(ast.Var("x"), p.newLocal())

		rulesets, _, _, opt := p.optimizeLookup(r, ast.MustParseRef("data.foo[x]"))
		if exp, act := true, opt; exp != act {
			t.Errorf("expected 'optimize' %v, got %v\n", exp, act)
		}
		if exp, act := 2, len(rulesets); exp != act {
			t.Fatalf("expected %d rulesets, got %d\n", exp, act)
		}
		if exp, act := 2, len(rulesets[0]); exp != act {
			t.Errorf("expected %d rules in ruleset 0, got %d\n", exp, act)
		}
		if exp, act := 1, len(rulesets[1]); exp != act {
			t.Errorf("expected %d rules in ruleset 1, got %d\n", exp, act)
		}
	})

	t.Run("unseen variable (last)", func(t *testing.T) {
		r := newRuletrie()
		val := r.LookupOrInsert(ref("foo.bar"))
		val.rules = append(val.rules, &r0, &r1, &r2)

		p := New()

		_, _, _, opt := p.optimizeLookup(r, ast.MustParseRef("data.foo[x]"))
		if exp, act := false, opt; exp != act {
			t.Errorf("expected 'optimize' %v, got %v\n", exp, act)
		}
	})

	t.Run("all ground refs", func(t *testing.T) {
		r := newRuletrie()
		val := r.LookupOrInsert(ref("foo.bar.baz"))
		val.rules = append(val.rules, &r0, &r1, &r2)

		p := New()

		_, _, _, opt := p.optimizeLookup(r, ast.MustParseRef("data.foo.bar.baz"))
		if exp, act := false, opt; exp != act {
			t.Errorf("expected 'optimize' %v, got %v\n", exp, act)
		}
	})

	t.Run("multiple seen vars, one rule set", func(t *testing.T) {
		r := newRuletrie()
		val := r.LookupOrInsert(ref("foo.aaa.bar.bbb.q"))
		val.rules = append(val.rules, &r0, &r1, &r2)

		p := New()
		lx, ly := p.newLocal(), p.newLocal()
		p.vars.Put(ast.Var("x"), lx)
		p.vars.Put(ast.Var("y"), ly)

		rulesets, path, index, opt := p.optimizeLookup(r, ast.MustParseRef("data.foo[x].bar[y].q"))
		if exp, act := true, opt; exp != act {
			t.Errorf("expected 'optimize' %v, got %v\n", exp, act)
		}
		if exp, act := 5, index; exp != act {
			t.Errorf("expected 'index' %d, got %d\n", exp, act)
		}
		if exp, act := 1, len(rulesets); exp != act {
			t.Fatalf("expected %d rulesets, got %d\n", exp, act)
		}
		if exp, act := 3, len(rulesets[0]); exp != act {
			t.Errorf("expected %d rules in ruleset 0, got %d\n", exp, act)
		}
		// 6 = g0 + foo + x + bar + y + q
		if exp, act := 6, len(path); exp != act {
			t.Fatalf("expected path len %d, got %d\n", exp, act)
		}
	})

	t.Run("one seen var, one unseen, one rule set", func(t *testing.T) {
		r := newRuletrie()
		val := r.LookupOrInsert(ref("foo.aaa.bar.bbb.q"))
		val.rules = append(val.rules, &r0, &r1, &r2)

		p := New()
		p.vars.Put(ast.Var("x"), p.newLocal())

		_, _, _, opt := p.optimizeLookup(r, ast.MustParseRef("data.foo[x].bar[y].q"))
		if exp, act := false, opt; exp != act {
			t.Errorf("expected 'optimize' %v, got %v\n", exp, act)
		}
	})

	t.Run("one seen var, one rule set and children left", func(t *testing.T) {
		r := newRuletrie()
		val := r.LookupOrInsert(ref("foo.aaa.bar.bbb.q"))
		val.rules = append(val.rules, &r0)
		val = r.LookupOrInsert(ref("foo.ccc.bar"))
		val.rules = append(val.rules, &r1, &r2)

		p := New()
		p.vars.Put(ast.Var("x"), p.newLocal())

		_, _, _, opt := p.optimizeLookup(r, ast.MustParseRef("data.foo[x].bar"))
		if exp, act := false, opt; exp != act {
			t.Errorf("expected 'optimize' %v, got %v\n", exp, act)
		}
	})

	t.Run("ref goes into the rules' result", func(t *testing.T) {
		r := newRuletrie()
		val := r.LookupOrInsert(ref("foo.aaa.bar.q"))
		val.rules = append(val.rules, &r0, &r1, &r2)

		p := New()
		p.vars.Put(ast.Var("x"), p.newLocal())

		rulesets, path, index, opt := p.optimizeLookup(r, ast.MustParseRef("data.foo[x].bar.q.p.r"))
		if exp, act := true, opt; exp != act {
			t.Errorf("expected 'optimize' %v, got %v\n", exp, act)
		}
		if exp, act := 4, index; exp != act {
			t.Errorf("expected 'index' %d, got %d\n", exp, act)
		}
		if exp, act := 1, len(rulesets); exp != act {
			t.Fatalf("expected %d rulesets, got %d\n", exp, act)
		}
		if exp, act := 3, len(rulesets[0]); exp != act {
			t.Errorf("expected %d rules in ruleset 0, got %d\n", exp, act)
		}
		// 5 = g0 + foo + x + bar + q
		if exp, act := 5, len(path); exp != act {
			t.Fatalf("expected path len %d, got %d\n", exp, act)
		}
	})

	t.Run("one leaf without rules", func(t *testing.T) {
		r := newRuletrie()
		val := r.LookupOrInsert(ref("foo.aaa.bar.q"))
		val.rules = append(val.rules, &r0, &r1)
		r.LookupOrInsert(ref("foo.bbb.bar.q"))

		p := New()
		p.vars.Put(ast.Var("x"), p.newLocal())

		rulesets, _, index, opt := p.optimizeLookup(r, ast.MustParseRef("data.foo[x].bar.q"))
		if exp, act := true, opt; exp != act {
			t.Errorf("expected 'optimize' %v, got %v\n", exp, act)
		}
		if exp, act := 4, index; exp != act {
			t.Errorf("expected 'index' %d, got %d\n", exp, act)
		}
		if exp, act := 1, len(rulesets); exp != act {
			t.Fatalf("expected %d rulesets, got %d\n", exp, act)
		}
		if exp, act := 2, len(rulesets[0]); exp != act {
			t.Errorf("expected %d rules in ruleset 0, got %d\n", exp, act)
		}
	})

	t.Run("all leaves without rules", func(t *testing.T) {
		r := newRuletrie()
		r.LookupOrInsert(ref("foo.aaa.bar.q"))
		r.LookupOrInsert(ref("foo.bbb.bar.q"))

		p := New()
		p.vars.Put(ast.Var("x"), p.newLocal())

		rulesets, _, _, opt := p.optimizeLookup(r, ast.MustParseRef("data.foo[x].bar.q"))
		if exp, act := true, opt; exp != act {
			t.Errorf("expected 'optimize' %v, got %v\n", exp, act)
		}
		if exp, act := 0, len(rulesets); exp != act {
			t.Fatalf("expected %d rulesets, got %d\n", exp, act)
		}
	})
}
