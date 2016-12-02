// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package explain

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
)

func TestTruth(t *testing.T) {

	module := `
    package test
	p :- q[x], r[x]
	q[x] :- a = [1,2,3,4], x = a[_]
	r[z] :- z = 3
	r[a] :- a = 4
    `

	q := ast.MustParseRule(`q[x] :- a = [1,2,3,4], x = a[_]`)
	ra := ast.MustParseRule(`r[a] :- a = 4`)

	runTruthTestCase(t, "", module, 14, map[int]*topdown.Event{
		6: &topdown.Event{
			Op:       topdown.RedoOp,
			Node:     parseExpr("x = a[_]", 1),
			QueryID:  3,
			ParentID: 2,
		},
		7: &topdown.Event{
			Op:       topdown.ExitOp,
			Node:     q,
			QueryID:  3,
			ParentID: 2,
			Locals:   parseBindings(`{x: 4}`),
		},
		9: &topdown.Event{
			Op:       topdown.RedoOp,
			Node:     ra,
			QueryID:  11,
			ParentID: 2,
			Locals:   parseBindings(`{a: 4}`),
		},
	})
}

func TestTruthAllPaths(t *testing.T) {

	module := `
    package test
	p :- q = {"a": 1, "d": 1}
	q[k] = 1 :- a = ["a", "b", "c", "d"], a[_] = k, r[k]
	r[x] :- x = "d"
	r[y] :- y = "a"
    `

	runTruthTestCaseIdentity(t, module)
}

func TestTruthAllPathsComprehension(t *testing.T) {
	module := `
    package test
	p :- x = [y | a=[1,2,3,4], a[_] = y, y != 2], count(x, 3)
    `

	runTruthTestCaseIdentity(t, module)
}

func TestTruthAllPathsNegation(t *testing.T) {

	module := `
    package test
    p :- not q
	q :- a = [1,2,3], a[_] = 100
    `

	runTruthTestCaseIdentity(t, module)
}

func TestExample(t *testing.T) {

	data := `
        {
            "servers": [
                {"id": "s1", "name": "app", "protocols": ["https", "ssh"], "ports": ["p1", "p2", "p3"]},
                {"id": "s2", "name": "db", "protocols": ["mysql"], "ports": ["p3"]},
                {"id": "s3", "name": "cache", "protocols": ["memcache", "http"], "ports": ["p3"]},
                {"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p1", "p2"]}
            ],
            "networks": [
                {"id": "n1", "public": false},
                {"id": "n2", "public": false},
                {"id": "n3", "public": true}
            ],
            "ports": [
                {"id": "p1", "networks": ["n1"]},
                {"id": "p2", "networks": ["n3"]},
                {"id": "p3", "networks": ["n2"]}
            ]
        }
    `

	module := `
	package test

	import data.servers
	import data.networks
	import data.ports

	p :- public_servers[x]

	public_servers[server] :-
		server = servers[server_index],
		server.ports[port_index] = ports[i].id,
		ports[i].networks[network_index] = networks[j].id,
		networks[j].public = true
    `

	runTruthTestCase(t, data, module, 12, map[int]*topdown.Event{
		6: &topdown.Event{
			Op:       topdown.RedoOp,
			Node:     parseExpr(`server.ports[port_index] = data.ports[i].id`, 1),
			QueryID:  3,
			ParentID: 2,
			Locals:   parseBindings(`{server: data.servers[3]}`),
		},
	})
}

func runTruthTestCaseIdentity(t *testing.T, module string) {
	answer, raw, err := explainQuery("", module)
	if err != nil {
		t.Fatalf("Unexpected explanation error: %v", err)
	}

	if len(answer) != len(raw) {
		t.Errorf("Expected %d events but got: %v", len(raw), len(answer))
	}
	for i := range raw {
		if i >= len(answer) {
			t.Errorf("Expected %d events, cannot check event #%d", len(raw), i)
			continue
		}
		if !raw[i].Equal(answer[i]) {
			t.Errorf("Expected event #%d to be %v but got: %v", i, raw[i], answer[i])
		}
	}
}

func runTruthTestCase(t *testing.T, data string, module string, n int, events map[int]*topdown.Event) {

	answer, _, err := explainQuery(data, module)
	if err != nil {
		t.Fatalf("Unexpected explanation error: %v", err)
	}

	if len(answer) != n {
		t.Errorf("Expected %d events but got: %v", n, len(answer))
	}

	for i, event := range events {
		if i >= len(answer) {
			t.Errorf("Got %d events, cannot check event #%d", len(answer), i)
			continue
		}
		result := answer[i]
		bindings := ast.NewValueMap()
		event.Locals.Iter(func(k, v ast.Value) bool {
			if b := result.Locals.Get(k); b != nil {
				bindings.Put(k, b)
			}
			return false
		})
		result.Locals = bindings
		if !result.Equal(event) {
			t.Errorf("Expected event #%d to be %v but got: %v", i, event, result)
		}
	}
}

func executeQuery(data string, compiler *ast.Compiler, tracer topdown.Tracer) {
	topdown.ResetQueryIDs()

	d := map[string]interface{}{}

	if len(data) > 0 {
		if err := util.UnmarshalJSON([]byte(data), &d); err != nil {
			panic(err)
		}
	}

	store := storage.New(storage.InMemoryWithJSONConfig(d))

	txn := storage.NewTransactionOrDie(store)
	defer store.Close(txn)

	params := topdown.NewQueryParams(compiler, store, txn, nil, ast.MustParseRef("data.test.p"))
	params.Tracer = tracer

	_, err := topdown.Query(params)
	if err != nil {
		panic(err)
	}
}

func explainQuery(data string, module string) ([]*topdown.Event, []*topdown.Event, error) {

	compiler := ast.NewCompiler()
	mods := map[string]*ast.Module{"": ast.MustParseModule(module)}

	if compiler.Compile(mods); compiler.Failed() {
		panic(compiler.Errors)
	}

	buf := topdown.NewBufferTracer()
	executeQuery(data, compiler, buf)

	answer, err := Truth(compiler, *buf)
	return answer, *buf, err
}

func parseBindings(s string) *ast.ValueMap {
	t := ast.MustParseTerm(s)
	obj, ok := t.Value.(ast.Object)
	if !ok {
		return nil
	}
	r := ast.NewValueMap()
	for _, pair := range obj {
		k, v := pair[0], pair[1]
		r.Put(k.Value, v.Value)
	}
	return r
}

func parseExpr(input string, index int) *ast.Expr {
	expr := ast.MustParseExpr(input)
	expr.Index = index
	return expr
}
