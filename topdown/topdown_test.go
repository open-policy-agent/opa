// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/format"
	"sigs.k8s.io/yaml"

	iCache "github.com/open-policy-agent/opa/topdown/cache"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util"
)

func TestTopDownQueryIDsUnique(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	inputTerm := &ast.Term{}
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	compiler := compileModules([]string{
		`package x
  p { 1 }
  p { 2 }`})

	tr := []*Event{}

	query := NewQuery(ast.MustParseBody("data.x.p")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer((*BufferTracer)(&tr)).
		WithInput(inputTerm)

	_, err := query.Run(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	queryIDs := map[uint64]bool{} // set of seen queryIDs (in EnterOps)
	for _, evt := range tr {
		if evt.Op != EnterOp {
			continue
		}
		if queryIDs[evt.QueryID] {
			t.Errorf("duplicate queryID: %v", evt)
		}
		queryIDs[evt.QueryID] = true
	}
}

func TestTopDownIndexExpr(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	compiler := compileModules([]string{
		`package test

		p = true {
		     1 > 0
		     q
		}

		q = true { true }`})

	tr := []*Event{}

	query := NewQuery(ast.MustParseBody("data.test.p")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer((*BufferTracer)(&tr))

	_, err := query.Run(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	exp := []*ast.Expr{
		ast.MustParseExpr("data.test.p"),
		ast.MustParseExpr("data.test.q"),
	}

	i := 0
	for _, evt := range tr {
		if evt.Op != IndexOp {
			continue
		}

		expr, ok := evt.Node.(*ast.Expr)
		if !ok {
			t.Fatal("Expected expr node but got:", evt.Node)
		}

		exp[i].Index = i
		if ast.Compare(expr, exp[i]) != 0 {
			t.Fatalf("Expected %v but got: %v", exp[i], expr)
		}
		i++
	}
}

func TestTopDownWithKeyword(t *testing.T) {

	tests := []struct {
		note    string
		rules   []string
		modules []string
		input   string
		exp     interface{}
	}{
		{
			// NOTE(tsandall): This case assumes that partial sets are not memoized.
			// If we change that, it'll be harder to test that the comprehension
			// cache is invalidated.
			note: "invalidate comprehension cache",
			exp:  `[[{"b": ["a", "c"]}], [{"b": ["a"]}]]`,
			modules: []string{`package ex
				s[x] {
					x = {v: ks |
						v = input[i]
						ks = {k | v = input[k]}
					}
				}
			`},
			rules: []string{`p = [x, y] {
				x = data.ex.s with input as {"a": "b", "c": "b"}
				y = data.ex.s with input as {"a": "b"}
			}`},
		},
	}

	for _, tc := range tests {
		runTopDownTestCaseWithModules(t, loadSmallTestData(), tc.note, tc.rules, tc.modules, tc.input, tc.exp)
	}
}

func TestTopDownUnsupportedBuiltin(t *testing.T) {

	ast.RegisterBuiltin(&ast.Builtin{
		Name: "unsupported_builtin",
	})

	body := ast.MustParseBody(`unsupported_builtin()`)
	ctx := context.Background()
	compiler := ast.NewCompiler()
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)
	q := NewQuery(body).WithCompiler(compiler).WithStore(store).WithTransaction(txn)
	_, err := q.Run(ctx)

	expected := unsupportedBuiltinErr(body[0].Location)

	if !reflect.DeepEqual(err, expected) {
		t.Fatalf("Expected %v but got: %v", expected, err)
	}

}

func TestTopDownQueryCancellation(t *testing.T) {

	ctx := context.Background()

	compiler := compileModules([]string{
		`
		package test

		p { data.arr[_] = x; test.sleep("10ms"); x == 999 }
		`,
	})

	arr := make([]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		arr[i] = i
	}
	data := map[string]interface{}{
		"arr": arr,
	}

	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	cancel := NewCancel()
	buf := NewBufferTracer()

	query := NewQuery(ast.MustParseBody("data.test.p")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithCancel(cancel).
		WithTracer(buf)

	done := make(chan struct{})
	go func() {
		time.Sleep(time.Millisecond * 50)
		cancel.Cancel()
		close(done)
	}()

	qrs, err := query.Run(ctx)
	if err == nil || err.(*Error).Code != CancelErr {
		t.Errorf("Expected cancel error but got: %v (err: %v)", qrs, err)
		PrettyTrace(os.Stdout, []*Event(*buf))
	}

	<-done
}

func TestTopDownQueryCancellationEvery(t *testing.T) {
	ctx := context.Background()

	module := func(ev ast.Every, extra ...interface{}) *ast.Module {
		t.Helper()
		m := ast.MustParseModule(`package test
	p { true }`)
		m.Rules[0].Body = ast.NewBody(ast.NewExpr(&ev))
		return m
	}

	tests := []struct {
		note   string
		module *ast.Module
	}{
		{
			note: "large domain, simple body",
			module: module(ast.Every{ // every x in data.arr { ... }
				Value:  ast.VarTerm("x"),
				Domain: ast.RefTerm(ast.VarTerm("data"), ast.StringTerm("arr")),
				Body:   ast.MustParseBody(`print(x); test.sleep("10ms")`),
			}),
		},
		{
			note: "simple domain, long evaluation time in body",
			module: module(ast.Every{ // every x in [999] { ... }
				Value:  ast.VarTerm("x"),
				Domain: ast.MustParseTerm(`[999]`),
				Body:   ast.MustParseBody(`data.arr[_] = y; test.sleep("10ms"); print(y); x == y`),
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			compiler := ast.NewCompiler().WithEnablePrintStatements(true)
			compiler.Compile(map[string]*ast.Module{"test.rego": tc.module})
			if compiler.Failed() {
				t.Fatalf("compiler: %v", compiler.Errors)
			}

			arr := make([]interface{}, 1000)
			for i := 0; i < 1000; i++ {
				arr[i] = i
			}
			data := map[string]interface{}{
				"arr": arr,
			}

			store := inmem.NewFromObject(data)
			txn := storage.NewTransactionOrDie(ctx, store)
			cancel := NewCancel()
			buf := bytes.Buffer{}
			tr := NewBufferTracer()
			ph := NewPrintHook(&buf)
			query := NewQuery(ast.MustParseBody("data.test.p")).
				WithCompiler(compiler).
				WithStore(store).
				WithTransaction(txn).
				WithCancel(cancel).
				WithTracer(tr).
				WithPrintHook(ph)

			done := make(chan struct{})
			go func() {
				time.Sleep(time.Millisecond * 500)
				cancel.Cancel()
				close(done)
			}()

			qrs, err := query.Run(ctx)
			if err == nil || err.(*Error).Code != CancelErr {
				t.Errorf("Expected cancel error but got: %v (err: %v)", qrs, err)
			}

			notes := strings.Split(buf.String(), "\n")
			notes = notes[:len(notes)-1] // last one is empty-string because each line ends in "\n"
			if len(notes) == 0 {
				t.Errorf("expected prints, got nothing")
			}
			if len(notes) == len(arr) {
				t.Errorf("expected less than %d prints, got %d", len(arr), len(notes))
			}
			t.Logf("got %d notes", len(notes))

			if t.Failed() && testing.Verbose() {
				PrettyTrace(os.Stdout, []*Event(*tr))
			}

			<-done
		})
	}
}

func TestTopDownEarlyExit(t *testing.T) {
	// NOTE(sr): There are two ways to early-exit: don't evaluate subsequent
	// rule bodies, like
	//
	//  p {
	//    true
	//  }
	//  p {
	//    # not evaluated
	//  }
	//
	// and not evaluating subsequent "rounds" of iteration:
	//
	//  p {
	//    x[_] = "y"
	//  }

	n := func(ns ...string) []string { return ns }

	tests := []struct {
		note      string
		module    string
		notes     []string // expected note events
		extraExit int      // number of "extra" events expected, each test expects 1 note, 1 early exit
	}{
		{
			note: "complete doc",
			module: `
				package test
				p { trace("a") }
				p { trace("b") }`,
			notes: n("a"),
		},
		{
			note: "complete doc, nested, both exit early",
			module: `
				package test
				p { q; trace("a") }
				p { q; trace("b") }

				q { trace("c") }
				q { trace("d") }`,
			extraExit: 1, // p + q
			notes:     n("a", "c"),
		},
		{
			note: "complete doc, nested, both exit early (else)",
			module: `
				package test
				p { q; trace("a") }
				p { q; trace("b") }

				q { trace("c"); false }
				else = true { trace("d")}
				q { trace("e") }`,
			extraExit: 1, // p + q
			notes:     n("a", "c", "d"),
		},
		{
			note: "complete doc: other complete doc that cannot exit early",
			module: `
				package test
				p { q }

				q = x { x := true; trace("a") }
				q = x { x := true; trace("b") }`,
			notes: n("a", "b"),
		},
		{
			note: "complete doc: other complete doc that cannot exit early, one undefined",
			module: `
				package test
				p { q }

				q = x { x := true; trace("a"); false }
				q = x { x := true; trace("b") }`,
			notes: n("a", "b"),
		},
		{
			note: "complete doc: other complete doc that cannot exit early (else)",
			module: `
				package test
				p { q }

				q = x { x := true; trace("a"); false }
				else = x { x := true; trace("b") }`,
			notes: n("a", "b"),
		},
		{
			note: "complete doc: other complete doc that cannot exit early, partial doc",
			module: `
				package test
				p { q }

				q[x] { 
					x := [1, 2, 3][_]; trace("a")
				}`,
			notes: n("a", "a", "a"),
		},
		{
			note: "complete doc, iteration: other partial doc that cannot exit early",
			module: `
				package test
				p { 
					data.arr[_] = x; trace("x") 
					q 
				}

				q[x] { 
					x := [1, 2, 3][_]; trace("a")
				}`,
			notes: n("x", "a", "a", "a"),
		},
		{
			note: "complete doc, iteration: multiple other partial docs that cannot exit early",
			module: `
				package test
				p {
					data.arr[_] = x; trace("x")
					q
				}
		
				q[x] {
					x := [1, 2, 3][_]; trace("a")
				}
		
				q[x] {
					x := [4, 5, 6][_]; trace("b")
				}`,
			notes: n("x", "a", "a", "a", "b", "b", "b"),
		},
		{
			note: "complete doc: other function that cannot exit early",
			module: `
				package test
				p { q(1) }

				q(_) = x { x := true; trace("a") }
				q(_) = x { x := true; trace("b") }`,
			notes: n("a", "b"),
		},
		{
			note: "complete doc: other function that cannot exit early (else)",
			module: `
				package test
				p { q(1) }

				q(_) = x { x := true; trace("a"); false }
				else = true { trace("b") }

				q(_) = x { x := true; trace("c") }`,
			notes: n("a", "b", "c"),
		},
		{
			note: "function",
			module: `
				package test
				p = f(1)
				f(_) { trace("a") }
				f(_) { trace("b") }`,
			notes: n("a"),
		},
		{
			note: "function: other function, both exit early",
			module: `
				package test
				p = f(1)
				f(_) { g(1); trace("a") }
				f(_) { g(1); trace("b") }
				g(_) { trace("c") }
				g(_) { trace("d") }`,
			notes:     n("a", "c"),
			extraExit: 1, // f() + g()
		},
		{
			note: "function: other function, both exit early (else)",
			module: `
			package test
			p = f(1)

			f(_) { g(1); trace("a") }
			f(_) { g(1); trace("b") }

			g(_) { trace("c"); false }
			else = true { trace("d") }
			g(_) { trace("e") }`,
			notes:     n("a", "c", "d"),
			extraExit: 1, // f() + g()
		},
		{
			note: "function: other complete doc that cannot exit early",
			module: `
				package test
				p = f(1)
				f(_) { q }
				q = x { x := true; trace("a") }
				q = x { x := true; trace("b") }`,
			notes: n("a", "b"),
		},
		{
			note: "function: other complete doc that cannot exit early (else)",
			module: `
				package test
				p = f(1)
				f(_) { q }
				q = x { x := true; trace("a"); false }
				else = x { x := true; trace("b") }`,
			notes: n("a", "b"),
		},
		{
			note: "complete doc, array iteration",
			module: `
				package test
				p { data.arr[_] = _; trace("x") }
			`,
			notes: n("x"),
		},
		{
			note: "complete doc, multiple array iteration",
			module: `
				package test
				p { 
					data.arr[_] = x; trace("x") 
					data.arr[_] = y; trace("y")
					data.arr[_] = z; trace("z")
				}
			`,
			notes: n("x", "y", "z"),
		},
		{
			note: "complete doc, multiple array iteration, ref to other complete doc with iteration",
			module: `
				package test
				p {
					data.arr[_] = x; trace("x") 
					data.arr[_] = y; trace("y")
					q; trace("q")
				}

				q { 
					data.arr[_] = a; trace("a")
					data.arr[_] = b; trace("b")
				}
			`,
			notes:     n("a", "b", "x", "y", "q"),
			extraExit: 1, // p + q
		},
		{
			note: "complete doc, multiple array iteration, ref to multiple other complete docs with iteration",
			module: `
				package test
				p {
					data.arr[_] = x; trace("x") 
					data.arr[_] = y; trace("y")
					q; trace("q")
				}

				q { 
					data.arr[_] = a; trace("a")
					data.arr[_] = b; trace("b")
				}

				# Not called because of EE
				q { 
					data.arr[_] = a; trace("c")
					data.arr[_] = b; trace("d")
				}
			`,
			notes:     n("a", "b", "x", "y", "q"),
			extraExit: 1, // p + q
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, multiple array iterations, ref to other complete doc with iteration and cached result",
			module: `
				package test
				p {
					data.arr[_] = x; trace("x") 
					data.arr[_] = y; trace("y")
					q; trace("q1")
					q; trace("q2") # result of q in cache
				}

				q { 
					data.arr[_] = a; trace("a")
					data.arr[_] = b; trace("b")
				}
			`,
			notes:     n("x", "y", "q1", "q2", "a", "b"),
			extraExit: 1, // p + q
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, multiple array iterations, ref to multiple other complete docs with iteration and cached result",
			module: `
				package test
				p {
					data.arr[_] = x; trace("x") 
					data.arr[_] = y; trace("y")
					q; trace("q1")
					q; trace("q2") # result of q in cache
				}

				q { 
					data.arr[_] = a; trace("a")
					data.arr[_] = b; trace("b")
				}

				# Not called because of EE
				q { 
					data.arr[_] = a; trace("c")
					data.arr[_] = b; trace("d")
				}
			`,
			notes:     n("x", "y", "q1", "q2", "a", "b"),
			extraExit: 1, // p + q
		},
		{
			note: "complete doc, multiple array iterations, ref to other multiple complete docs with iteration",
			module: `
				package test
				p {
					data.arr[_] = x; trace("x") 
					data.arr[_] = y; trace("y")
					q; trace("q")
					r; trace("r")
				}

				q { 
					data.arr[_] = a; trace("a")
					data.arr[_] = b; trace("b")
				}

				r { 
					data.arr[_] = c; trace("c")
					data.arr[_] = d; trace("d")
				}
			`,
			notes:     n("x", "y", "a", "b", "q", "c", "d", "r"),
			extraExit: 2, // p + q + r
		},
		{
			note: "complete doc, array iteration, package-local data",
			module: `
				package test
				arr := ["a", "b", "c"]
				p { 
					arr[_] = x; trace("x")
				}
			`,
			notes:     n("x"),
			extraExit: 1, // p + arr
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, multiple array iterations, module-local data, cached result",
			module: `
				package test
				arr := ["a", "b", "c"]
				p { 
					arr[_] = x; trace("x") 
					arr[_] = y; trace("y") # arr in cache
				}
			`,
			notes:     n("x", "y"),
			extraExit: 1, // p + arr
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, array iteration, ref to other complete doc without early exit",
			module: `package test
			p {
				data.arr[_]; trace("x")
				q; trace("y")
			}

			q := x {
				x := 1
			}`,
			notes: n("x", "y"),
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, array iteration, ref to other complete doc without early exit (multiple rules)",
			module: `package test
			p {
				data.arr[_]; trace("x")
				q; trace("y")
			}

			q := x {
				x := 1; trace("a")
			}

			q := x {
				x := 1; trace("b")
			}`,
			notes: n("x", "a", "b", "y"),
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, array iteration, multiple refs to other complete docs without early exit",
			module: `package test
			p {
				data.arr[_]; trace("x")
				q; trace("y")
				r; trace("z")
			}

			q := x {
				x := 1
			}

			r := x {
				x := 2
			}`,
			notes: n("x", "y", "z"),
		},
		{
			note: "complete doc, array iteration, func call with early exit",
			module: `
				package test
				p { 
					data.arr[_] = x; trace("x")
					f(1) == 1
				}
				
				f(x) := 1 {
					trace("a")
				}
			`,
			notes:     n("x", "a"),
			extraExit: 1, // p + f()
		},
		{
			note: "complete doc, multiple array iterations, func call multiple early exit",
			module: `
				package test
				p { 
					data.arr[_] = x; trace("x")
					f(1) == 1
				}
				
				f(x) := 1 {
					trace("a")
				}
				
				f(x) := 1 {
					trace("b")
				}
			`,
			notes:     n("x", "a"),
			extraExit: 1, // p + f()
		},
		{
			note: "complete doc, multiple array iterations, func call without early exit",
			module: `
				package test
				p { 
					data.arr[_] = x; trace("x")
					f(1) == 1
				}
				
				f(x) := 1 {
					trace("a")
				}
				
				f(x) := 1 {
					trace("b")
				}

				f(x) := 2 {
					trace("c")
					false # to avoid eval_conflict_error error
				}
			`,
			notes: n("x", "a", "b", "c"),
		},
		{
			note: "complete doc, multiple array iterations, func call with early exit and iteration",
			module: `
				package test
				p { 
					data.arr[_] = x; trace("x")
					f(1) == 1
				}
				
				f(x) := 1 {
					data.arr[_] = a; trace("a")
				}
			`,
			notes:     n("x", "a"),
			extraExit: 1, // p + f()
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, array iteration, func call without early exit, static arg",
			module: `
				package test
				p { 
					data.arr[_] = x; trace("x")
					f(1) == 1
				}
				
				f(x) := x {
					trace("a")
				}
			`,
			notes: n("x", "a"),
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, array iteration -> func call without early exit, dynamic arg",
			module: `
				package test
				p { 
					data.arr[_] = x; trace("x")
					f(x) == x
				}
				
				f(x) := x {
					trace("a")
				}
			`,
			notes: n("x", "a"),
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, set iteration -> func call without early exit, dynamic arg",
			module: `
				package test
				s := { 1, 2, 3 }

				p { 
					s[_] = x; trace("x")
					f(x) == x
				}
				
				f(x) := x {
					trace("a")
				}
			`,
			notes:     n("x", "a"),
			extraExit: 1, // p + o
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, object iteration -> func call without early exit, dynamic arg",
			module: `
				package test
				o := {
					"a": 1,
					"b": 2,
					"c": 3,
				}

				p { 
					o[_] = x; trace("x")
					f(x) == x
				}
				
				f(x) := x {
					trace("a")
				}
			`,
			notes:     n("x", "a"),
			extraExit: 1, // p + o
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, array iteration -> func call without early exit, array iteration, dynamic arg",
			module: `
				package test
				p { 
					arr[_] = x; trace("x")
					f(x) == x
				}

				arr := [1, 2, 3, 4, 2]
				
				f(x) := x {
					arr[_] = y; trace("a")
					y == 2; trace("b") # y will have exactly two matches, so we expect two "b" notes, and an exhaustive number of "a" notes
				}
			`,
			notes:     n("x", "a", "a", "a", "a", "a", "b", "b"),
			extraExit: 1, // p + arr
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, set iteration -> func call without early exit, set iteration, dynamic arg",
			module: `
				package test

				s := { 1, 2, 3, 4, 5 }

				p { 
					s[_] = x; trace("x")
					f(x) == x
				}

				f(x) := x {
					s[_] = y; trace("a")
					y == 1; trace("b") # y will have exactly one match, so we expect one "b" note, and an exhaustive number of "a" notes
				}
			`,
			notes:     n("x", "a", "a", "a", "a", "a", "b"),
			extraExit: 1, // p + arr
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, object iteration -> func call without early exit, object iteration, dynamic arg",
			module: `
				package test

				o := { 
					"a": 1,
					"b": 2,
					"c": 3,
					"d": 2,
				}

				p { 
					o[_] = x; trace("x")
					f(x) == x
				}

				f(x) := x {
					o[_] = y; trace("a")
					y == 2; trace("b") # y will have exactly two matches, so we expect two "b" notes, and an exhaustive number of "a" notes
				}
			`,
			notes:     n("x", "a", "a", "a", "a", "b", "b"),
			extraExit: 1, // p + arr
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, array iteration -> func call without early exit, virtual doc array iteration, dynamic arg",
			module: `
				package test
				p { 
					data.arr[_] = x; trace("x")
					f(x) == x
				}

arr := [1, 2, 3, 4, 5]
				
				f(x) := x {
					arr[_] = y; trace("a")
					y == 3; trace("b")
				}
			`,
			notes:     n("x", "a", "a", "a", "a", "a", "b"),
			extraExit: 1, // p + arr
		},
		{ // Regression test for: https://github.com/open-policy-agent/opa/issues/6566
			note: "complete doc, array iteration -> func (multi) call without early exit, static arg",
			module: `
				package test
				p { 
					data.arr[_] = x; trace("x")
					f(1) == 1
				}
				
				f(x) := x {
					trace("a")
				}

				f(x) := x {
					trace("b")
					false
				}

				f(x) := x {
					trace("c")
				}
			`,
			notes: n("x", "a", "b", "c"),
		},
		{
			note: "complete doc, obj iteration",
			module: `
				package test
				p { data.obj[_] = _; trace("x") }
			`,
			notes: n("x"),
		},
		{
			note: "complete doc, set iteration",
			module: `
				package test
				xs := { i | data.arr[i] }
				p { xs[_] = _; trace("x") }
			`,
			notes: n("x"),
		},
		{
			note: "function doc, array iteration",
			module: `
				package test
				p = f(1)
				f(_) { data.arr[_] = _; trace("x") }
			`,
			notes: n("x"),
		},
		{
			note: "complete doc, obj iteration",
			module: `
				package test
				p = f(1)
				f(_) { data.obj[_] = _; trace("x") }
			`,
			notes: n("x"),
		},
		{
			note: "complete doc, set iteration",
			module: `
				package test
				xs := { i | data.arr[i] }
				p = f(1)
				f(_) { xs[_] = _; trace("x") }
			`,
			notes: n("x"),
		},
		{
			note: "ee -> ee -> ee",
			module: `package test
				p {
					data.arr[_] = x; trace("a")
					q; trace("b")
				}

				q {
					data.arr[_] = x; trace("c")
					r; trace("d")
				}

				r {
					data.arr[i] = x; trace("e")
				}
			`,
			notes:     n("a", "c", "e", "d", "b"),
			extraExit: 2, // p + q + r
		},
		{
			note: "ee -> no ee -> ee",
			module: `package test
				p {
					data.arr[i] = x; trace("a")
					q; trace("b")
				}

				q[x] {
					[1, 2, 3][_] = x; trace("c")
					r; trace("d")
				}

				r {
					data.arr[i] = x; trace("e")
				}
			`,
			notes:     n("a", "c", "c", "c", "e", "d", "d", "d", "b"),
			extraExit: 1, // p + r
		},
		{
			note: "ee -> no ee (multiple) -> ee",
			module: `package test
				p {
					data.arr[i] = x; trace("a")
					q; trace("b")
				}

				q[x] {
					[1, 2, 3][_] = x; trace("c")
					r; trace("d")
				}

				q[x] {
					[4, 5, 6][_] = x; trace("e")
					r; trace("f")
				}

				r {
					data.arr[i] = x; trace("g")
				}
			`,
			notes:     n("a", "c", "c", "c", "d", "d", "d", "e", "e", "e", "f", "f", "f", "g", "b"),
			extraExit: 1, // p + r
		},
		{
			note: "ee -> (no ee, ee)",
			module: `package test
				p {
					data.arr[i] = x; trace("a")
					q; trace("b")
					r; trace("c")
				}

				q[x] {
					[1, 2, 3][_] = x; trace("d")
				}

				r {
					data.arr[i] = x; trace("e")
				}
			`,
			notes:     n("a", "d", "d", "d", "b", "e", "c"),
			extraExit: 1, // p + r
		},
		// every statements
		{
			note: "complete doc with every",
			module: `package test
				import future.keywords
				p {
					data.arr[_] = x; trace("x")
					every x in [1, 2, 3] { x; trace("a") }
				}
			`,
			notes:     n("x", "a", "a", "a"),
			extraExit: 3, // p + every*3
		},
		{
			note: "complete doc -> every, array iteration",
			module: `package test
				import future.keywords
				p {
					data.arr[_] = x; trace("x")
					every x in [1, 2, 3] { 
						data.arr[_] = y; trace("a")
						x; trace("b") 
					}
				}
			`,
			notes:     n("x", "a", "a", "a", "b", "b", "b"),
			extraExit: 3, // p + every*3
		},
		{
			note: "complete doc -> every, array iteration -> complete doc with ee -> complete doc no ee",
			module: `package test
				import future.keywords
				p {
					data.arr_small[_] = x; trace("x")
					every x in [1, 2, 3] { 
						data.arr_small[_] = y; trace("e1")
						x
						q; trace("e2")
					}
				}

				q {
					data.arr_small[_] = x; trace("q1")
					r; trace("q2")
				}

				r := x {
					x := 1
					data.arr_small[_] = y; trace("r1")
				}
			`,
			notes:     n("x", "e1", "e1", "e1", "q1", "r1", "r1", "r1", "r1", "r1", "q2", "e2", "e2", "e2"),
			extraExit: 4, // p + every*3 + q
		},
		{
			note: "complete doc -> every, array iteration -> complete doc no ee -> complete doc with ee",
			module: `package test
				import future.keywords
				p {
					data.arr[_] = x; trace("x")
					every x in [1, 2, 3] { 
						data.arr_small[_] = y; trace("e1")
						x
						q; trace("e2")
					}
				}

				q := x {
					x := 1
					data.arr_small[_] = y; trace("q1")
					r; trace("q2")
				}

				r {
					data.arr[_] = y; trace("r1")
				}
			`,
			notes:     n("x", "e1", "e1", "e1", "q1", "q1", "q1", "q1", "q1", "r1", "q2", "q2", "q2", "q2", "q2", "e2", "e2", "e2"),
			extraExit: 4, // p + every*3 + r
		},
		{
			note: "complete doc -> complete doc, no ee, with every",
			module: `package test
				import future.keywords
				p {
					data.arr[_] = x; trace("x")
					q
				}

				arr := [1, 2]
		
				q := x {
					x := 1
					arr[_] = y; trace("a")
					every v in [1, 2, 3] { 
						v; trace("b") # we expect 3*len(arr)==6 "b" notes
					}
				}
			`,
			notes:     n("x", "a", "a", "b", "b", "b", "b", "b", "b"),
			extraExit: 7, // p + every*3*2 + arr
		},
		{
			note: "complete doc -> complete doc, no ee, with every -> complete doc ee",
			module: `package test
				import future.keywords
				p {
					data.arr[_] = x; trace("x")
					q
				}

				arr := [1, 2]
		
				q := x {
					x := 1
					arr[_] = y; trace("a")
					every v in [1, 2, 3] { 
						v; r; trace("b") # we expect 3*len(arr)==6 "b" notes
					}
				}
	
				r {
					data.arr[_] = x; trace("c")
				}
			`,
			notes:     n("x", "a", "a", "c", "b", "b", "b", "b", "b", "b"),
			extraExit: 8, // p + every*3*2 + arr + r
		},
		// array comprehensions
		{
			note: "complete doc, array iteration, ee -> array comprehension -> complete doc, ee",
			module: `
				package test
				p {
					data.arr[_] = x; trace("p1")
					y := [v | v = data.arr[_]; q]; trace("p2")
				}
		
				q {
					data.arr[_] = x; trace("q")
				}
			`,
			notes:     n("p1", "q", "p2"),
			extraExit: 1, // p + q
		},
		{
			note: "complete doc, array iteration, ee -> array comprehension -> complete doc, ee -> complete doc, no ee",
			module: `
				package test
				p {
					data.arr[_] = x; trace("p1")
					y := [v | v = data.arr[_]; q]; trace("p2")
				}
		
				q {
					data.arr[_] = x; trace("q")
					r
				}

				r := v {
					v := 1
					data.arr_small[_] = x; trace("r")
				}
			`,
			notes:     n("p1", "q", "r", "r", "r", "r", "r", "p2"),
			extraExit: 1, // p + q
		},
		// set comprehensions
		{
			note: "complete doc, array iteration, ee -> set comprehension -> complete doc, ee",
			module: `
				package test
				p {
					data.arr[_] = x; trace("p1")
					y := {v | v = data.arr[_]; q}; trace("p2")
				}
		
				q {
					data.arr[_] = x; trace("q")
				}
			`,
			notes:     n("p1", "q", "p2"),
			extraExit: 1, // p + q
		},
		{
			note: "complete doc, array iteration, ee -> set comprehension -> complete doc, ee -> complete doc, no ee",
			module: `
				package test
				p {
					data.arr[_] = x; trace("p1")
					y := {v | v = data.arr[_]; q}; trace("p2")
				}
		
				q {
					data.arr[_] = x; trace("q")
					r
				}

				r := v {
					v := 1
					data.arr_small[_] = x; trace("r")
				}
			`,
			notes:     n("p1", "q", "r", "r", "r", "r", "r", "p2"),
			extraExit: 1, // p + q
		},
		// object comprehensions
		{
			note: "complete doc, array iteration, ee -> object comprehension -> complete doc, ee",
			module: `
				package test
				p {
					data.arr[_] = x; trace("p1")
					y := {k: v | v = data.arr[k]; q}; trace("p2")
				}
		
				q {
					data.arr[_] = x; trace("q")
				}
			`,
			notes:     n("p1", "q", "p2"),
			extraExit: 1, // p + q
		},
		{
			note: "complete doc, array iteration, ee -> object comprehension -> complete doc, ee -> complete doc, no ee",
			module: `
				package test
				p {
					data.arr[_] = x; trace("p1")
					y := {k: v | v = data.arr[k]; q}; trace("p2")
				}
		
				q {
					data.arr[_] = x; trace("q")
					r
				}

				r := v {
					v := 1
					data.arr_small[_] = x; trace("r")
				}
			`,
			notes:     n("p1", "q", "r", "r", "r", "r", "r", "p2"),
			extraExit: 1, // p + q
		},
		// with statements
		{
			note: "complete doc, array iteration, ee -> with -> complete doc, ee",
			module: `
				package test
				p {
					data.arr[_] = x; trace("p1")
					q with input.x as data.arr; trace("p2")
				}

				q {
					input.x[_] = x; trace("q")
				}
			`,
			notes:     n("p1", "q", "p2"),
			extraExit: 1, // p + q
		},
		{
			note: "complete doc, array iteration, ee -> with -> complete doc, no ee",
			module: `
				package test
				p {
					data.arr[_] = x; trace("p1")
					q with input.x as data.arr_small; trace("p2")
				}

				q := v {
					v := 1
					input.x[_] = x; trace("q")
				}
			`,
			notes: n("p1", "q", "q", "q", "q", "q", "p2"),
		},
		{
			note: "complete doc, array iteration, ee -> with -> complete doc, ee -> complete doc, no ee",
			module: `
				package test
				p {
					data.arr[_] = x; trace("p1")
					q with input.x as data.arr_small; trace("p2")
				}

				q {
					input.x[_] = x; trace("q1")
					r; trace("q2")
				}

				r := v {
					v := 1
					input.x[_] = x; trace("r")
				}
			`,
			notes:     n("p1", "q1", "r", "r", "r", "r", "r", "q2", "p2"),
			extraExit: 1, // p + q
		},
		{
			note: "complete doc, array iteration, ee -> with -> complete doc, no ee -> complete doc, ee",
			module: `
				package test
				p {
					data.arr[_] = x; trace("p1")
					q with input.x as data.arr_small; trace("p2")
				}

				q := v {
					v := 1
					input.x[_] = x; trace("q1")
					r; trace("q2")
				}

				r {
					input.x[_] = x; trace("r")
				}
			`,
			// data.test.r is evaluated twice, as 'with' in data.test.p will pop the virtual cache before redoes.
			// Cache is however maintained through redo-sequence, so data.test.r result will be found in cache from there.
			notes:     n("p1", "q1", "q1", "q1", "q1", "q1", "q2", "q2", "q2", "q2", "q2", "r", "r", "p2"),
			extraExit: 2, // p + r + r
		},
		// negation
		{
			note: "complete doc, array iteration, ee -> negated complete doc, ee",
			module: `
				package test
				p {
					data.arr[_] = x; trace("p1")
					not q; trace("p2")
				}
		
				q := false {
					data.arr[_] = x; trace("q")
				}
			`,
			notes:     n("p1", "q", "p2"),
			extraExit: 1, // p + q
		},
		{
			note: "complete doc, array iteration, ee -> negated complete doc, no ee",
			module: `
				package test
				p {
					data.arr[_] = x; trace("p1")
					not q; trace("p2")
				}
		
				q := x {
					x := false
					data.arr_small[_] = y; trace("q")
				}
			`,
			notes: n("p1", "q", "q", "q", "q", "q", "p2"),
		},
		{
			note: "complete doc, array iteration, ee -> negated complete doc, aborted ee",
			module: `
				package test
				p {
					data.arr[_] = x; trace("p1")
					not q; trace("p2")
				}
		
				q {
					data.arr_small[_] = x; trace("q")
					false
				}
			`,
			notes: n("p1", "q", "q", "q", "q", "q", "p2"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			countExit := 1 + tc.extraExit
			ctx := context.Background()
			compiler := compileModules([]string{tc.module})
			size := 1000
			arr := make([]interface{}, size)
			obj := make(map[string]interface{}, size)
			for i := 0; i < size; i++ {
				arr[i] = i
				obj[strconv.Itoa(i)] = i
			}
			data := map[string]interface{}{
				"arr":       arr,
				"arr_small": []int{1, 2, 3, 4, 5},
				"obj":       obj,
			}

			store := inmem.NewFromObject(data)
			txn := storage.NewTransactionOrDie(ctx, store)
			buf := NewBufferTracer()

			query := NewQuery(ast.MustParseBody("data.test.p")).
				WithCompiler(compiler).
				WithStore(store).
				WithTransaction(txn).
				WithTracer(buf)

			_, err := query.Run(ctx)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			notes := []string{}
			exits := map[string]int{}

			for _, ev := range []*Event(*buf) {
				switch ev.Op {
				case NoteOp:
					notes = append(notes, ev.Message)
				case ExitOp:
					exits[ev.Message]++
				}
			}
			sort.Strings(notes)
			sort.Strings(tc.notes)
			if !reflect.DeepEqual(notes, tc.notes) {
				t.Errorf("unexpected note traces, expected %v, got %v", tc.notes, notes)
			}
			if exp, act := countExit, exits["early"]; exp != act {
				t.Errorf("expected %d early exit events, got %d", exp, act)
			}

			if t.Failed() {
				PrettyTrace(os.Stderr, *buf)
			}
		})
	}
}

func TestTopDownEvery(t *testing.T) {
	n := func(ns ...string) []string { return ns }

	tests := []struct {
		note   string
		module string
		notes  []string // expected note events, let's see if these are useful
		fail   bool
	}{
		{
			note: "domain empty",
			module: `package test
				p { every x in [] { print(x) } }
			`,
			notes: n(),
		},
		{
			note: "domain undefined",
			module: `package test
				p { every x in input { print(x) } }
			`,
			fail: true,
		},
		{
			note: "domain is call",
			module: `package test
				p {
					d := numbers.range(1, 5)
					every x in d { x >= 1; print(x) }
				}`,
			notes: n("1", "2", "3", "4", "5"),
		},
		{
			note: "simple value",
			module: `package test
				p {
					every x in [1, 2] { print(x) }
				}`,
			notes: n("1", "2"),
		},
		{
			note: "simple key+value",
			module: `package test
				p {
					every k, v in [1, 2] { k < v; print(v) }
				}`,
			notes: n("1", "2"),
		},
		{
			note: "outer bindings",
			module: `package test
				p {
					i = "outer"
					every x in [1, 2] { print(x); print(i) }
				}`,
			notes: n("1", "outer", "2", "outer"),
		},
		{
			note: "simple failure, last",
			module: `package test
				p {
					every x in [1, 2] { x < 2; print(x) }
				}`,
			notes: n("1"),
			fail:  true,
		},
		{
			note: "simple failure, first",
			module: `package test
				p {
					every x in [1, 2] { x > 1; print(x) }
				}`,
			notes: n(),
			fail:  true,
		},
		{
			note: "early exit in body eval on success",
			module: `package test
				p {
					every x in [1, 2] { y := [false, true, true][_]; print(x); y }
				}`,
			notes: n("1", "1", "2", "2"), // Would be triples if EE in the body didn't work
		},
		{
			note: "early exit suppressed in body eval",
			module: `package test
				q { print("q") }
				p {
					every x in [1, 2] { q; print(x) }
				}`,
			notes: n("q", "1", "2"), // Would be only "1" if the EE of q wasn't surppressed
		},
		{
			note: "with: domain",
			module: `package test
				p {
					every x in input { print(x) } with input as [1]
				}`,
			notes: n("1"),
		},
		{
			note: "with: body",
			module: `package test
				p {
					every x in [1, 2] { print(x); print(input) } with input as "input"
				}`,
			notes: n("1", "input", "2", "input"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := context.Background()
			c := ast.NewCompiler().WithEnablePrintStatements(true)
			mod := ast.MustParseModuleWithOpts(tc.module, ast.ParserOptions{FutureKeywords: []string{"every"}})
			if c.Compile(map[string]*ast.Module{"test": mod}); c.Failed() {
				t.Fatal(c.Errors)
			}
			if testing.Verbose() {
				t.Log(c.Modules)
			}
			buf := bytes.Buffer{}
			tr := NewBufferTracer()
			ph := NewPrintHook(&buf)
			query := NewQuery(ast.MustParseBody("data.test.p = x")).
				WithCompiler(c).
				WithPrintHook(ph).
				WithTracer(tr)

			res, err := query.Run(ctx)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if !tc.fail {
				if len(res) == 0 {
					t.Errorf("unexpected failure, empty query result set")
				}
			} else {
				if len(res) > 0 {
					t.Errorf("unexpected results: %v, expected empty query result set", res)
				}
			}

			notes := strings.Split(buf.String(), "\n")
			notes = notes[:len(notes)-1] // last one is empty-string because each line ends in "\n"
			if len(tc.notes) != 0 || len(tc.notes) == 0 && len(notes) != 0 {
				if !reflect.DeepEqual(notes, tc.notes) {
					t.Errorf("unexpected prints, expected %q, got %q", tc.notes, notes)
				}
			}

			if t.Failed() || testing.Verbose() {
				PrettyTrace(os.Stderr, *tr)
			}
		})
	}
}

type contextPropagationMock struct{}

// contextPropagationStore will accumulate values from the contexts provided to
// read calls so that the test can verify that contexts are being propagated as
// expected.
type contextPropagationStore struct {
	storage.WritesNotSupported
	storage.TriggersNotSupported
	storage.PolicyNotSupported
	calls []interface{}
}

func (*contextPropagationStore) NewTransaction(context.Context, ...storage.TransactionParams) (storage.Transaction, error) {
	return nil, nil
}

func (*contextPropagationStore) Commit(context.Context, storage.Transaction) error {
	return nil
}

func (*contextPropagationStore) Abort(context.Context, storage.Transaction) {
}

func (*contextPropagationStore) Truncate(context.Context, storage.Transaction, storage.TransactionParams, storage.Iterator) error {
	return nil
}

func (m *contextPropagationStore) Read(ctx context.Context, txn storage.Transaction, path storage.Path) (interface{}, error) {
	val := ctx.Value(contextPropagationMock{})
	m.calls = append(m.calls, val)
	return nil, nil
}

func TestTopDownContextPropagation(t *testing.T) {

	ctx := context.WithValue(context.Background(), contextPropagationMock{}, "bar")

	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{
		"mod1": ast.MustParseModule(`package ex

p[x] { data.a[i] = x }`,
		),
	})

	mockStore := &contextPropagationStore{}
	txn := storage.NewTransactionOrDie(ctx, mockStore)
	query := NewQuery(ast.MustParseBody("data.ex.p")).
		WithCompiler(compiler).
		WithStore(mockStore).
		WithTransaction(txn)

	_, err := query.Run(ctx)
	if err != nil {
		t.Fatalf("Unexpected query error: %v", err)
	}

	expectedCalls := []interface{}{"bar"}

	if !reflect.DeepEqual(expectedCalls, mockStore.calls) {
		t.Fatalf("Expected %v but got: %v", expectedCalls, mockStore.calls)
	}
}

// astStore returns a fixed ast.Value for Read.
type astStore struct {
	storage.WritesNotSupported
	storage.TriggersNotSupported
	storage.PolicyNotSupported
	path  string
	value ast.Value
}

func (*astStore) NewTransaction(context.Context, ...storage.TransactionParams) (storage.Transaction, error) {
	return nil, nil
}

func (*astStore) Commit(context.Context, storage.Transaction) error {
	return nil
}

func (*astStore) Abort(context.Context, storage.Transaction) {}

func (*astStore) Truncate(context.Context, storage.Transaction, storage.TransactionParams, storage.Iterator) error {
	return nil
}

func (a *astStore) Read(ctx context.Context, txn storage.Transaction, path storage.Path) (interface{}, error) {
	if path.String() == a.path {
		return a.value, nil
	}

	return nil, &storage.Error{
		Code:    storage.NotFoundErr,
		Message: "not found",
	}
}

func TestTopdownStoreAST(t *testing.T) {
	body := ast.MustParseBody(`data.stored = x`)
	ctx := context.Background()
	compiler := ast.NewCompiler()
	store := &astStore{path: "/stored", value: ast.String("value")}

	txn := storage.NewTransactionOrDie(ctx, store)
	q := NewQuery(body).WithCompiler(compiler).WithStore(store).WithTransaction(txn)
	qrs, err := q.Run(ctx)

	result := queryResultSetToTerm(qrs)
	exp := ast.MustParseTerm(`
                {
                        {
                                x: "value"
                        }
                }
        `)

	if err != nil || !result.Equal(exp) {
		t.Fatalf("expected %v but got %v (error: %v)", exp, result, err)
	}
}

func TestTopdownLazyObj(t *testing.T) {
	body := ast.MustParseBody(`data.stored = x`)
	ctx := context.Background()
	compiler := ast.NewCompiler()
	foo := map[string]interface{}{
		"foo": "bar",
	}
	store := inmem.NewFromObject(map[string]interface{}{
		"stored": foo,
	})
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	q := NewQuery(body).WithCompiler(compiler).WithStore(store).WithTransaction(txn)
	qrs, err := q.Run(ctx)
	if err != nil {
		t.Fatalf("expected no error got %v", err)
	}
	act, ok := qrs[0]["x"].Value.(ast.Object)
	if !ok {
		t.Errorf("expected obj, got %T: %[1]v", qrs[0]["x"].Value)
	}
	// NOTE(sr): we're using DeepEqual here because we want to assert that the structs
	// match -- as far as the interface `ast.Object` is concerned `*lazyObj` and `*object`
	// should be indistinguishable.
	if exp := ast.LazyObject(foo); !reflect.DeepEqual(act, exp) {
		t.Errorf("expected %T, got %T", exp, act)
	}
}

func TestTopdownLazyObjOptOut(t *testing.T) {
	body := ast.MustParseBody(`data.stored = x`)
	ctx := context.Background()
	compiler := ast.NewCompiler()
	foo := map[string]interface{}{
		"foo": "bar",
	}
	store := inmem.NewFromObject(map[string]interface{}{
		"stored": foo,
	})
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	q := NewQuery(body).WithCompiler(compiler).WithStore(store).WithTransaction(txn).WithStrictObjects(true)
	qrs, err := q.Run(ctx)
	if err != nil {
		t.Fatalf("expected no error got %v", err)
	}
	act, ok := qrs[0]["x"].Value.(ast.Object)
	if !ok {
		t.Errorf("expected %T, got %T: %[2]v", act, qrs[0]["x"].Value)
	}
	// NOTE(sr): We can't type-assert *ast.lazyObj because it's not exported -- but we can retry
	// the assertion that we've done in the not-opt-out case, and see that it no longer holds:
	if exp := ast.LazyObject(foo); reflect.DeepEqual(act, exp) {
		t.Errorf("expected %T, got %T", exp, act)
	}
	if exp := ast.NewObject(ast.Item(ast.StringTerm("foo"), ast.StringTerm("bar"))); exp.Compare(act) != 0 {
		t.Errorf("expected %v to be equal to %v", exp, act)
	}
}

func compileModules(input []string) *ast.Compiler {

	mods := map[string]*ast.Module{}

	for idx, i := range input {
		id := fmt.Sprintf("testMod%d", idx)
		mods[id] = ast.MustParseModule(i)
	}

	c := ast.NewCompiler()
	if c.Compile(mods); c.Failed() {
		panic(c.Errors)
	}

	return c
}

func compileRules(imports []string, input []string, modules []string) (*ast.Compiler, error) {

	is := []*ast.Import{}
	for _, i := range imports {
		is = append(is, &ast.Import{
			Path: ast.MustParseTerm(i),
		})
	}

	m := &ast.Module{
		Package: ast.MustParsePackage("package generated"),
		Imports: is,
	}

	rules := []*ast.Rule{}
	for i := range input {
		rules = append(rules, ast.MustParseRule(input[i]))
		rules[i].Module = m
	}

	m.Rules = rules

	for i := range rules {
		rules[i].Module = m
	}

	mods := map[string]*ast.Module{"testMod": m}

	for i, s := range modules {
		mods[fmt.Sprintf("testMod%d", i)] = ast.MustParseModule(s)
	}

	c := ast.NewCompiler()

	if c.Compile(mods); c.Failed() {
		return nil, c.Errors
	}

	return c, nil
}

// loadSmallTestData returns base documents that are referenced
// throughout the topdown test suite.
//
// Avoid the following top-level keys: i, j, k, p, q, r, v, x, y, z.
// These are used for rule names, local variables, etc.
func loadSmallTestData() map[string]interface{} {
	var data map[string]interface{}
	err := util.UnmarshalJSON([]byte(`{
        "a": [1,2,3,4],
        "b": {
            "v1": "hello",
            "v2": "goodbye"
        },
        "c": [{
            "x": [true, false, "foo"],
            "y": [null, 3.14159],
            "z": {"p": true, "q": false}
        }],
        "d": {
            "e": ["bar", "baz"]
        },
        "f": [
            {"xs": [1.0], "ys": [2.0]},
            {"xs": [2.0], "ys": [3.0]}
        ],
        "g": {
            "a": [1, 0, 0, 0],
            "b": [0, 2, 0, 0],
            "c": [0, 0, 0, 4]
        },
        "h": [
            [1,2,3],
            [2,3,4]
        ],
        "l": [
            {
                "a": "bob",
                "b": -1,
                "c": [1,2,3,4]
            },
            {
                "a": "alice",
                "b": 1,
                "c": [2,3,4,5],
                "d": null
            }
        ],
		"strings": {
			"foo": 1,
			"bar": 2,
			"baz": 3
		},
		"three": 3,
        "m": [],
		"numbers": [
			"1",
			"2",
			"3",
			"4"
		]
    }`), &data)
	if err != nil {
		panic(err)
	}
	return data
}

func setTime(t time.Time) func(*Query) *Query {
	return func(q *Query) *Query {
		return q.WithTime(t)
	}
}

func setInterQueryCache(c iCache.InterQueryCache) func(*Query) *Query {
	return func(q *Query) *Query {
		return q.WithInterQueryBuiltinCache(c)
	}
}

func setAllowNet(a []string) func(*Query) *Query {
	return func(q *Query) *Query {
		c := q.compiler.Capabilities()
		c.AllowNet = a
		return q.WithCompiler(q.compiler.WithCapabilities(c))
	}
}

func runTopDownTestCase(t *testing.T, data map[string]interface{}, note string, rules []string, expected interface{}, options ...func(*Query) *Query) {
	t.Helper()

	runTopDownTestCaseWithContext(context.Background(), t, data, note, rules, nil, "", expected, options...)
}

func runTopDownTestCaseWithModules(t *testing.T, data map[string]interface{}, note string, rules []string, modules []string, input string, expected interface{}) {
	t.Helper()

	runTopDownTestCaseWithContext(context.Background(), t, data, note, rules, modules, input, expected)
}

func runTopDownTestCaseWithContext(ctx context.Context, t *testing.T, data map[string]interface{}, note string, rules []string, modules []string, input string, expected interface{},
	options ...func(*Query) *Query) {
	t.Helper()

	imports := []string{}
	for k := range data {
		imports = append(imports, "data."+k)
	}

	compiler, err := compileRules(imports, rules, modules)
	if err != nil {
		if _, ok := expected.(error); ok {
			assertError(t, expected, err)
		} else {
			t.Errorf("%v: Compiler error: %v", note, err)
		}
		return
	}

	store := inmem.NewFromObject(data)

	assertTopDownWithPathAndContext(ctx, t, compiler, store, note, []string{"generated", "p"}, input, expected, options...)
}

func assertTopDownWithPathAndContext(ctx context.Context, t *testing.T, compiler *ast.Compiler, store storage.Store, note string, path []string, input string, expected interface{},
	options ...func(*Query) *Query) {
	t.Helper()

	var inputTerm *ast.Term

	if len(input) > 0 {
		inputTerm = ast.MustParseTerm(input)
	}

	txn := storage.NewTransactionOrDie(ctx, store)

	defer store.Abort(ctx, txn)

	var lhs *ast.Term
	if len(path) == 0 {
		lhs = ast.NewTerm(ast.DefaultRootRef)
	} else {
		lhs = ast.MustParseTerm("data." + strings.Join(path, "."))
	}

	rhs := ast.VarTerm(ast.WildcardPrefix + "result")
	body := ast.NewBody(ast.Equality.Expr(lhs, rhs))

	var requiresSort bool

	if rules := compiler.GetRulesExact(lhs.Value.(ast.Ref)); len(rules) > 0 && rules[0].Head.DocKind() == ast.PartialSetDoc {
		requiresSort = true
	}

	if os.Getenv("OPA_DUMP_TEST") != "" {

		data, err := store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatal(err)
		}

		dump(note, compiler.Modules, data, path, inputTerm, expected, requiresSort)
	}

	// add an inter-query cache
	config, _ := iCache.ParseCachingConfig(nil)
	interQueryCache := iCache.NewInterQueryCache(config)

	var strictBuiltinErrors bool

	switch expected.(type) {
	case *Error, error:
		strictBuiltinErrors = true
	}

	query := NewQuery(body).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(inputTerm).
		WithInterQueryBuiltinCache(interQueryCache).
		WithStrictBuiltinErrors(strictBuiltinErrors)

	var tracer BufferTracer

	if os.Getenv("OPA_TRACE_TEST") != "" {
		query = query.WithTracer(&tracer)
	}

	for _, opt := range options {
		query = opt(query)
	}

	t.Run(note, func(t *testing.T) {
		t.Helper()

		switch e := expected.(type) {
		case *Error, error:
			_, err := query.Run(ctx)
			assertError(t, expected, err)
		case string:
			qrs, err := query.Run(ctx)

			if tracer != nil {
				PrettyTrace(os.Stdout, tracer)
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(e) == 0 {
				if len(qrs) != 0 {
					t.Fatalf("Expected undefined result but got: %v", qrs)
				}
				return
			}

			if len(qrs) == 0 {
				t.Fatalf("Expected %v but got undefined", e)
			}

			result, err := ast.JSON(qrs[0][rhs.Value.(ast.Var)].Value)
			if err != nil {
				t.Fatal(err)
			}

			expected := util.MustUnmarshalJSON([]byte(e))

			if requiresSort {
				sort.Sort(resultSet(result.([]interface{})))
				if sl, ok := expected.([]interface{}); ok {
					sort.Sort(resultSet(sl))
				}
			}

			if util.Compare(expected, result) != 0 {
				t.Fatalf("Unexpected result:\nGot: %+v\nExp:\n%+v", result, expected)
			}

			// If the test case involved the input document, re-run it with partial
			// evaluation enabled and input marked as unknown. Then replay the query and
			// verify the partial evaluation result is the same. Note, we cannot evaluate
			// the result of a query against `data` because the queries need to be
			// converted into rules (which would result in recursion.)
			if len(path) > 0 {
				runTopDownPartialTestCase(ctx, t, compiler, store, txn, inputTerm, rhs, body, requiresSort, expected)
			}
		default:
			t.Fatalf("Unexpected expected value type: %+v", e)
		}
	})
}

func runTopDownPartialTestCase(ctx context.Context, t *testing.T, compiler *ast.Compiler, store storage.Store, txn storage.Transaction, input *ast.Term, output *ast.Term, body ast.Body, requiresSort bool, expected interface{}) {
	t.Helper()

	// add an inter-query cache
	config, _ := iCache.ParseCachingConfig(nil)
	interQueryCache := iCache.NewInterQueryCache(config)

	partialQuery := NewQuery(body).
		WithCompiler(compiler).
		WithStore(store).
		WithUnknowns([]*ast.Term{ast.MustParseTerm("input")}).
		WithTransaction(txn).
		WithInterQueryBuiltinCache(interQueryCache)

	partials, support, err := partialQuery.PartialRun(ctx)

	if err != nil {
		t.Fatal("Unexpected error on partial evaluation comparison:", err)
	}

	module := ast.MustParseModule("package topdown_test_partial")
	module.Rules = make([]*ast.Rule, len(partials))
	for i, body := range partials {
		module.Rules[i] = &ast.Rule{
			Head:   ast.NewHead(ast.Var("__result__"), nil, output),
			Body:   body,
			Module: module,
		}
	}

	compiler.Modules["topdown_test_partial"] = module
	for i, module := range support {
		compiler.Modules[fmt.Sprintf("topdown_test_support_%d", i)] = module
	}

	compiler.Compile(compiler.Modules)
	if compiler.Failed() {
		t.Fatal("Unexpected error on partial evaluation result compile:", compiler.Errors)
	}

	query := NewQuery(ast.MustParseBody("data.topdown_test_partial.__result__ = x")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(input).
		WithInterQueryBuiltinCache(interQueryCache)

	qrs, err := query.Run(ctx)
	if err != nil {
		t.Fatal("Unexpected error on query after partial evaluation:", err)
	}

	if len(qrs) == 0 {
		t.Fatalf("Expected %v but got undefined from query after partial evaluation", expected)
	}

	result, err := ast.JSON(qrs[0][ast.Var("x")].Value)
	if err != nil {
		t.Fatal(err)
	}

	if requiresSort {
		sort.Sort(resultSet(result.([]interface{})))
		if sl, ok := expected.([]interface{}); ok {
			sort.Sort(resultSet(sl))
		}
	}

	if util.Compare(expected, result) != 0 {
		t.Fatalf("Unexpected result after partial evaluation:\nGot:\n%v\nExp:\n%v", result, expected)
	}
}

type resultSet []interface{}

func (rs resultSet) Less(i, j int) bool {
	return util.Compare(rs[i], rs[j]) < 0
}

func (rs resultSet) Swap(i, j int) {
	tmp := rs[i]
	rs[i] = rs[j]
	rs[j] = tmp
}

func (rs resultSet) Len() int {
	return len(rs)
}

func init() {

	ast.RegisterBuiltin(&ast.Builtin{
		Name: "test.sleep",
		Decl: types.NewFunction(
			types.Args(types.S),
			types.NewNull(),
		),
	})

	RegisterBuiltinFunc("test.sleep", func(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
		d, _ := time.ParseDuration(string(operands[0].Value.(ast.String)))
		time.Sleep(d)
		return iter(ast.NullTerm())
	})

}

var testID = 0
var testIDMutex sync.Mutex

func getTestNamespace() string {
	programCounters := make([]uintptr, 20)
	n := runtime.Callers(0, programCounters)
	if n > 0 {
		frames := runtime.CallersFrames(programCounters[:n])
		for more := true; more; {
			var f runtime.Frame
			f, more = frames.Next()
			if strings.HasPrefix(f.Function, "github.com/open-policy-agent/opa/topdown.Test") {
				return strings.TrimPrefix(strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(f.Function, "github.com/open-policy-agent/opa/topdown.Test"), "TopDown")), "builtin")
			}
		}
	}
	return ""
}

func dump(note string, modules map[string]*ast.Module, data interface{}, docpath []string, input *ast.Term, exp interface{}, requiresSort bool) {

	moduleSet := []string{}
	for _, module := range modules {
		moduleSet = append(moduleSet, string(bytes.ReplaceAll(format.MustAst(module), []byte("\t"), []byte("  "))))
	}

	namespace := getTestNamespace()

	test := map[string]interface{}{
		"note":    namespace + "/" + note,
		"data":    data,
		"modules": moduleSet,
		"query":   strings.Join(append([]string{"data"}, docpath...), ".") + " = x",
	}

	if input != nil {
		test["input_term"] = input.String()
	}

	switch e := exp.(type) {
	case string:
		rs := []map[string]interface{}{}
		if len(e) > 0 {
			exp := util.MustUnmarshalJSON([]byte(e))
			if requiresSort {
				sl := exp.([]interface{})
				sort.Sort(resultSet(sl))
			}
			rs = append(rs, map[string]interface{}{"x": exp})
		}
		test["want_result"] = rs
		if requiresSort {
			test["sort_bindings"] = true
		}
	case error:
		test["want_error_code"] = e.(*Error).Code
		test["want_error"] = e.(*Error).Message
	default:
		panic("Unexpected test expectation. Cowardly refusing to generate test cases.")
	}

	bs, err := yaml.Marshal(map[string]interface{}{"cases": []interface{}{test}})
	if err != nil {
		panic(err)
	}

	dir := path.Join(os.Getenv("OPA_DUMP_TEST"), namespace)

	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(err)
	}

	testIDMutex.Lock()
	testID++
	c := testID
	testIDMutex.Unlock()

	filename := fmt.Sprintf("test-%v-%04d.yaml", namespace, c)

	if err := os.WriteFile(filepath.Join(dir, filename), bs, 0644); err != nil {
		panic(err)
	}

}

func assertError(t *testing.T, expected interface{}, actual error) {
	t.Helper()
	if actual == nil {
		t.Errorf("Expected error but got: %v", actual)
		return
	}

	errString := actual.Error()

	if reflect.TypeOf(expected) != reflect.TypeOf(actual) {
		t.Errorf("Expected error of type '%T', got '%T'", expected, actual)
	}

	switch e := expected.(type) {
	case Error:
		assertErrorContains(t, errString, e.Code)
		assertErrorContains(t, errString, e.Message)
	case *Error:
		assertErrorContains(t, errString, e.Code)
		assertErrorContains(t, errString, e.Message)
	case *ast.Error:
		assertErrorContains(t, errString, e.Code)
		assertErrorContains(t, errString, e.Message)
	case ast.Errors:
		for _, astErr := range e {
			assertErrorContains(t, errString, astErr.Code)
			assertErrorContains(t, errString, astErr.Message)
		}
	case error:
		assertErrorContains(t, errString, e.Error())
	}
}

func assertErrorContains(t *testing.T, actualErrMsg string, expected string) {
	t.Helper()
	if !strings.Contains(actualErrMsg, expected) {
		t.Errorf("Expected error '%v' but got: '%v'", expected, actualErrMsg)
	}
}
