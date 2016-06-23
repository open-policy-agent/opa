// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package repl

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
)

func TestDump(t *testing.T) {
	input := `{"a": [1,2,3,4]}`
	var data map[string]interface{}
	err := json.Unmarshal([]byte(input), &data)
	if err != nil {
		panic(err)
	}
	store := storage.NewDataStoreFromJSONObject(data)
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("dump")
	expectOutput(t, buffer.String(), "map[a:[1 2 3 4]]\n")
}

func TestUnset(t *testing.T) {
	store := storage.NewDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	repl.OneShot("magic = 23")
	repl.OneShot("p = 3.14")
	repl.OneShot("unset p")
	repl.OneShot("p")
	result := buffer.String()
	if result != "error: 1 error occurred: unsafe variables in repl2: [p]\n" {
		t.Errorf("Expected p to be unsafe but got: %v", result)
		return
	}

	buffer.Reset()
	repl.OneShot("p = 3.14")
	repl.OneShot("p = 3 :- false")
	repl.OneShot("unset p")
	repl.OneShot("p")
	result = buffer.String()
	if result != "error: 1 error occurred: unsafe variables in repl4: [p]\n" {
		t.Errorf("Expected p to be unsafe but got: %v", result)
		return
	}

	buffer.Reset()
	repl.OneShot("unset ")
	result = buffer.String()
	if result != "error: unset <var>: expects exactly one argument\n" {
		t.Errorf("Expected unset error for bad syntax but got: %v", result)
	}

	buffer.Reset()
	repl.OneShot("unset 1=1")
	result = buffer.String()
	if result != "error: argument must identify a rule\n" {
		t.Errorf("Expected unset error for bad syntax but got: %v", result)
	}

	buffer.Reset()
	repl.OneShot(`unset "p"`)
	result = buffer.String()
	if result != "error: argument must identify a rule\n" {
		t.Errorf("Expected unset error for bad syntax but got: %v", result)
	}

	buffer.Reset()
	repl.OneShot(`unset q`)
	result = buffer.String()
	if result != "warning: no matching rules in current module\n" {
		t.Errorf("Expected unset error for missing rule but got: %v", result)
	}

	buffer.Reset()
	repl.OneShot(`magic`)
	result = buffer.String()
	if result != "23\n" {
		t.Errorf("Expected magic to be defined but got: %v", result)
	}

	buffer.Reset()
	repl.OneShot(`package data.other`)
	repl.OneShot(`unset magic`)
	result = buffer.String()
	if result != "warning: no matching rules in current module\n" {
		t.Errorf("Expected unset error for bad syntax but got: %v", result)
	}
}

func TestOneShotEmptyBufferOneExpr(t *testing.T) {
	store := newTestDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("data.a[i].b.c[j] = 2")
	expectOutput(t, buffer.String(), "+---+---+\n| i | j |\n+---+---+\n| 0 | 1 |\n+---+---+\n")
	buffer.Reset()
	repl.OneShot("data.a[i].b.c[j] = \"deadbeef\"")
	expectOutput(t, buffer.String(), "false\n")
}

func TestOneShotEmptyBufferOneRule(t *testing.T) {
	store := newTestDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("p[x] :- data.a[i] = x")
	expectOutput(t, buffer.String(), "")
}

func TestOneShotBufferedExpr(t *testing.T) {
	store := newTestDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("data.a[i].b.c[j] = ")
	expectOutput(t, buffer.String(), "")
	repl.OneShot("2")
	expectOutput(t, buffer.String(), "")
	repl.OneShot("")
	expectOutput(t, buffer.String(), "+---+---+\n| i | j |\n+---+---+\n| 0 | 1 |\n+---+---+\n")
}

func TestOneShotBufferedRule(t *testing.T) {
	store := newTestDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("p[x] :- ")
	expectOutput(t, buffer.String(), "")
	repl.OneShot("data.a[i]")
	expectOutput(t, buffer.String(), "")
	repl.OneShot(" = ")
	expectOutput(t, buffer.String(), "")
	repl.OneShot("x")
	expectOutput(t, buffer.String(), "")
	repl.OneShot("")
	expectOutput(t, buffer.String(), "")
}

func TestOneShotJSON(t *testing.T) {
	store := newTestDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.outputFormat = "json"
	repl.OneShot("data.a[i] = x")
	var expected interface{}
	input := `
	[
		{
			"i": 0,
			"x": {
			"b": {
				"c": [
				true,
				2,
				false
				]
			}
			}
		},
		{
			"i": 1,
			"x": {
			"b": {
				"c": [
				false,
				true,
				1
				]
			}
			}
		}
	]
	`
	if err := json.Unmarshal([]byte(input), &expected); err != nil {
		panic(err)
	}

	var result interface{}

	if err := json.Unmarshal(buffer.Bytes(), &result); err != nil {
		t.Errorf("Unexpected output format: %v", err)
		return
	}
	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Expected %v but got: %v", expected, result)
	}
}

func TestEvalFalse(t *testing.T) {
	store := newTestDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("false")
	result := buffer.String()
	if result != "false\n" {
		t.Errorf("Expected result to be false but got: %v", result)
	}
}

func TestEvalConstantRule(t *testing.T) {
	store := newTestDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("pi = 3.14")
	result := buffer.String()
	if result != "" {
		t.Errorf("Expected rule to be defined but got: %v", result)
		return
	}
	buffer.Reset()
	repl.OneShot("pi")
	result = buffer.String()
	expected := "3.14\n"
	if result != expected {
		t.Errorf("Expected pi to evaluate to 3.14 but got: %v", result)
		return
	}
	buffer.Reset()
	repl.OneShot("pi.deadbeef")
	result = buffer.String()
	if result != "undefined\n" {
		t.Errorf("Expected pi.deadbeef to be undefined but got: %v", result)
		return
	}
	buffer.Reset()
	repl.OneShot("pi > 3")
	result = buffer.String()
	if result != "true\n" {
		t.Errorf("Expected pi > 3 to be true but got: %v", result)
		return
	}
}

func TestEvalSingleTermMultiValue(t *testing.T) {
	store := newTestDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.outputFormat = "json"

	input := `
	[
		{
			"data.a[i].b.c[_]": true,
			"i": 0
		},
		{
			"data.a[i].b.c[_]": 2,
			"i": 0
		},
		{
			"data.a[i].b.c[_]": false,
			"i": 0
		},
		{
			"data.a[i].b.c[_]": false,
			"i": 1
		},
		{
			"data.a[i].b.c[_]": true,
			"i": 1
		},
		{
			"data.a[i].b.c[_]": 1,
			"i": 1
		}
	]`

	var expected interface{}
	if err := json.Unmarshal([]byte(input), &expected); err != nil {
		panic(err)
	}

	repl.OneShot("data.a[i].b.c[_]")
	var result interface{}
	if err := json.Unmarshal(buffer.Bytes(), &result); err != nil {
		t.Errorf("Expected valid JSON document: %v: %v", err, buffer.String())
		return
	}

	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Expected %v but got: %v", expected, result)
		return
	}

	buffer.Reset()

	repl.OneShot("data.deadbeef[x]")
	s := buffer.String()
	if s != "undefined\n" {
		t.Errorf("Expected undefined from reference but got: %v", s)
		return
	}

	buffer.Reset()

	repl.OneShot("p[x] :- a = [1,2,3,4], a[_] = x")
	buffer.Reset()
	repl.OneShot("p[x]")

	input = `
	[
		{
			"x": 1
		},
		{
			"x": 2
		},
		{
			"x": 3
		},
		{
			"x": 4
		}
	]
	`

	if err := json.Unmarshal([]byte(input), &expected); err != nil {
		panic(err)
	}

	if err := json.Unmarshal(buffer.Bytes(), &result); err != nil {
		t.Errorf("Expected valid JSON document: %v: %v", err, buffer.String())
		return
	}

	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Exepcted %v but got: %v", expected, result)
	}
}

func TestEvalRuleCompileError(t *testing.T) {
	store := newTestDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("p[x] :- true")
	result := buffer.String()
	expected := "error: 1 error occurred: unsafe variable from head of p: x\n"
	if result != expected {
		t.Errorf("Expected error message in output but got: %v", result)
		return
	}
	buffer.Reset()
	repl.OneShot("p = true :- true")
	result = buffer.String()
	if result != "" {
		t.Errorf("Expected valid rule to compile (because state should have been rolled back) but got: %v", result)
		return
	}
}

func TestEvalBodyCompileError(t *testing.T) {
	store := newTestDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.outputFormat = "json"
	repl.OneShot("x = 1, y > x")
	result1 := buffer.String()
	expected1 := "error: 1 error occurred: unsafe variables in repl0: [y]\n"
	if result1 != expected1 {
		t.Errorf("Expected error message in output but got : %v", result1)
		return
	}
	buffer.Reset()
	repl.OneShot("x = 1, y = 2, y > x")
	var result2 []interface{}
	err := json.Unmarshal(buffer.Bytes(), &result2)
	if err != nil {
		t.Errorf("Expected valid JSON output but got: %v", buffer.String())
		return
	}
	expected2 := []interface{}{
		map[string]interface{}{
			"x": float64(1),
			"y": float64(2),
		},
	}
	if !reflect.DeepEqual(expected2, result2) {
		t.Errorf(`Expected [{"x": 1, "y": 2}] but got: %v"`, result2)
		return
	}
}

func TestEvalBodyContainingWildCards(t *testing.T) {
	store := newTestDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("data.a[_].b.c[_] = x")
	expected := strings.TrimSpace(`
+-------+
|   x   |
+-------+
| true  |
| 2     |
| false |
| false |
| true  |
| 1     |
+-------+`)
	result := strings.TrimSpace(buffer.String())
	if result != expected {
		t.Errorf("Expected only a single column of output but got:\n%v", result)
	}

}

func TestEvalImport(t *testing.T) {
	store := newTestDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("import data.a")
	if len(buffer.Bytes()) != 0 {
		t.Errorf("Expected no output but got: %v", buffer.String())
		return
	}
	buffer.Reset()
	repl.OneShot("a[0].b.c[0] = true")
	result := buffer.String()
	expected := "true\n"
	if result != expected {
		t.Errorf("Expected expression to evaluate successfully but got: %v", result)
		return
	}
}

func TestEvalPackage(t *testing.T) {
	store := newTestDataStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("package foo.bar")
	repl.OneShot("p = true :- true")
	repl.OneShot("package baz.qux")
	buffer.Reset()
	repl.OneShot("p")
	if buffer.String() != "error: 1 error occurred: unsafe variables in repl0: [p]\n" {
		t.Errorf("Expected unsafe variable error but got: %v", buffer.String())
		return
	}
	repl.OneShot("import data.foo.bar.p")
	buffer.Reset()
	repl.OneShot("p")
	if buffer.String() != "true\n" {
		t.Errorf("Expected expression to eval successfully but got: %v", buffer.String())
		return
	}
}

func TestBuildHeader(t *testing.T) {
	expr := ast.MustParseStatement(`[{"a": x, "b": data.a.b[y]}] = [{"a": 1, "b": 2}]`).(ast.Body)[0]
	terms := expr.Terms.([]*ast.Term)
	result := map[string]struct{}{}
	buildHeader(result, terms[1])
	expected := map[string]struct{}{
		"x": struct{}{}, "y": struct{}{},
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Build header expected %v but got %v", expected, result)
	}
}

func expectOutput(t *testing.T, output string, expected string) {
	if output != expected {
		t.Errorf("Repl output: expected %#v but got %#v", expected, output)
	}
}

func newRepl(dataStore *storage.DataStore, buffer *bytes.Buffer) *REPL {
	policyStore := storage.NewPolicyStore(dataStore, "")
	repl := New(dataStore, policyStore, "", buffer, "")
	return repl
}

func newTestDataStore() *storage.DataStore {
	input := `
    {
        "a": [
            {
                "b": {
                    "c": [true,2,false]
                }
            },
            {
                "b": {
                    "c": [false,true,1]
                }
            }
        ]
    }
    `
	var data map[string]interface{}
	err := json.Unmarshal([]byte(input), &data)
	if err != nil {
		panic(err)
	}
	return storage.NewDataStoreFromJSONObject(data)
}
