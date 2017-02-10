// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package repl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
)

func TestComplete(t *testing.T) {
	store := newTestStore()
	ctx := context.Background()

	mod1 := ast.MustParseModule(`package a.b.c
	p = 1
	q = 2`)

	mod2 := ast.MustParseModule(`package a.b.d
	r = 3`)

	if err := storage.InsertPolicy(ctx, store, "mod1", mod1, nil, false); err != nil {
		panic(err)
	}

	if err := storage.InsertPolicy(ctx, store, "mod2", mod2, nil, false); err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	repl := newRepl(store, &buf)
	repl.OneShot(ctx, "s = 4")
	buf.Reset()

	result := repl.complete("")
	expected := []string{
		"data.a.b.c.p",
		"data.a.b.c.q",
		"data.a.b.d.r",
		"data.repl.s",
		"data.repl.version",
	}

	sort.Strings(result)
	sort.Strings(expected)

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}

	result = repl.complete("data.a.b")
	expected = []string{
		"data.a.b.c.p",
		"data.a.b.c.q",
		"data.a.b.d.r",
	}

	sort.Strings(result)
	sort.Strings(expected)

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}

	result = repl.complete("data.a.b.c.p[x]")
	expected = nil

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}

	repl.OneShot(ctx, "import data.a.b.c.p as xyz")
	repl.OneShot(ctx, "import data.a.b.d")

	result = repl.complete("x")
	expected = []string{
		"xyz",
	}

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}
}

func TestDump(t *testing.T) {
	ctx := context.Background()
	input := `{"a": [1,2,3,4]}`
	var data map[string]interface{}
	err := util.UnmarshalJSON([]byte(input), &data)
	if err != nil {
		panic(err)
	}
	store := storage.New(storage.InMemoryWithJSONConfig(data))
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "dump")
	expectOutput(t, buffer.String(), "{\"a\":[1,2,3,4]}\n")
}

func TestDumpPath(t *testing.T) {
	ctx := context.Background()
	input := `{"a": [1,2,3,4]}`
	var data map[string]interface{}
	err := util.UnmarshalJSON([]byte(input), &data)
	if err != nil {
		panic(err)
	}
	store := storage.New(storage.InMemoryWithJSONConfig(data))
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	dir, err := ioutil.TempDir("", "dump-path-test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "tmpfile")
	repl.OneShot(ctx, fmt.Sprintf("dump %s", file))

	if buffer.String() != "" {
		t.Errorf("Expected no output but got: %v", buffer.String())
	}

	bs, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("Expected file read to succeed but got: %v", err)
	}

	var result map[string]interface{}
	if err := util.UnmarshalJSON(bs, &result); err != nil {
		t.Fatalf("Expected json unmarhsal to suceed but got: %v", err)
	}

	if !reflect.DeepEqual(data, result) {
		t.Fatalf("Expected dumped json to equal %v but got: %v", data, result)
	}
}

func TestHelp(t *testing.T) {
	topics["deadbeef"] = topicDesc{
		fn: func(w io.Writer) error {
			fmt.Fprintln(w, "blah blah blah")
			return nil
		},
	}

	ctx := context.Background()
	store := storage.New(storage.InMemoryConfig())
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "help deadbeef")

	expected := "blah blah blah\n"

	if buffer.String() != expected {
		t.Fatalf("Unexpected output from help topic: %v", buffer.String())
	}
}

func TestShow(t *testing.T) {
	ctx := context.Background()
	store := storage.New(storage.InMemoryConfig())
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	repl.OneShot(ctx, "package repl_test")
	repl.OneShot(ctx, "show")
	assertREPLText(t, buffer, "package repl_test\n")
	buffer.Reset()

	repl.OneShot(ctx, "import input.xyz")
	repl.OneShot(ctx, "show")

	expected := `package repl_test

import input.xyz` + "\n"
	assertREPLText(t, buffer, expected)
	buffer.Reset()

	repl.OneShot(ctx, "import data.foo as bar")
	repl.OneShot(ctx, "show")

	expected = `package repl_test

import input.xyz
import data.foo as bar` + "\n"
	assertREPLText(t, buffer, expected)
	buffer.Reset()

	repl.OneShot(ctx, "p[1] :- true")
	repl.OneShot(ctx, "p[2] :- true")
	repl.OneShot(ctx, "show")

	expected = `package repl_test

import input.xyz
import data.foo as bar

p[1] :- true
p[2] :- true` + "\n"
	assertREPLText(t, buffer, expected)
	buffer.Reset()

	repl.OneShot(ctx, "package abc")
	repl.OneShot(ctx, "show")

	assertREPLText(t, buffer, "package abc\n")
	buffer.Reset()

	repl.OneShot(ctx, "package repl_test")
	repl.OneShot(ctx, "show")

	assertREPLText(t, buffer, expected)
	buffer.Reset()
}

func TestUnset(t *testing.T) {
	ctx := context.Background()
	store := storage.New(storage.InMemoryConfig())
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	repl.OneShot(ctx, "magic = 23")
	repl.OneShot(ctx, "p = 3.14")
	repl.OneShot(ctx, "unset p")

	err := repl.OneShot(ctx, "p")
	if _, ok := err.(ast.Errors); !ok {
		t.Fatalf("Expected AST error but got: %v", err)
	}

	buffer.Reset()
	repl.OneShot(ctx, "p = 3.14")
	repl.OneShot(ctx, "p = 3 :- false")
	repl.OneShot(ctx, "unset p")

	err = repl.OneShot(ctx, "p")
	if _, ok := err.(ast.Errors); !ok {
		t.Fatalf("Expected AST error but got err: %v, output: %v", err, buffer.String())
	}

	if err := repl.OneShot(ctx, "unset "); err == nil {
		t.Fatalf("Expected unset error for bad syntax but got: %v", buffer.String())
	}

	if err := repl.OneShot(ctx, "unset 1=1"); err == nil {
		t.Fatalf("Expected unset error for bad syntax but got: %v", buffer.String())
	}

	if err := repl.OneShot(ctx, `unset "p"`); err == nil {
		t.Fatalf("Expected unset error for bad syntax but got: %v", buffer.String())
	}

	buffer.Reset()
	repl.OneShot(ctx, `unset q`)
	if buffer.String() != "warning: no matching rules in current module\n" {
		t.Fatalf("Expected unset error for missing rule but got: %v", buffer.String())
	}

	buffer.Reset()
	repl.OneShot(ctx, `magic`)
	if buffer.String() != "23\n" {
		t.Fatalf("Expected magic to be defined but got: %v", buffer.String())
	}

	buffer.Reset()
	repl.OneShot(ctx, `package data.other`)
	repl.OneShot(ctx, `unset magic`)
	if buffer.String() != "warning: no matching rules in current module\n" {
		t.Fatalf("Expected unset error for bad syntax but got: %v", buffer.String())
	}
}

func TestOneShotEmptyBufferOneExpr(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "data.a[i].b.c[j] = 2")
	expectOutput(t, buffer.String(), "+---+---+\n| i | j |\n+---+---+\n| 0 | 1 |\n+---+---+\n")
	buffer.Reset()
	repl.OneShot(ctx, "data.a[i].b.c[j] = \"deadbeef\"")
	expectOutput(t, buffer.String(), "false\n")
}

func TestOneShotEmptyBufferOneRule(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "p[x] :- data.a[i] = x")
	expectOutput(t, buffer.String(), "")
}

func TestOneShotBufferedExpr(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "data.a[i].b.c[j] = ")
	expectOutput(t, buffer.String(), "")
	repl.OneShot(ctx, "2")
	expectOutput(t, buffer.String(), "")
	repl.OneShot(ctx, "")
	expectOutput(t, buffer.String(), "+---+---+\n| i | j |\n+---+---+\n| 0 | 1 |\n+---+---+\n")
}

func TestOneShotBufferedRule(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "p[x] :- ")
	expectOutput(t, buffer.String(), "")
	repl.OneShot(ctx, "data.a[i]")
	expectOutput(t, buffer.String(), "")
	repl.OneShot(ctx, " = ")
	expectOutput(t, buffer.String(), "")
	repl.OneShot(ctx, "x")
	expectOutput(t, buffer.String(), "")
	repl.OneShot(ctx, "")
	expectOutput(t, buffer.String(), "")
}

func TestOneShotJSON(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.outputFormat = "json"
	repl.OneShot(ctx, "data.a[i] = x")
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
	if err := util.UnmarshalJSON([]byte(input), &expected); err != nil {
		panic(err)
	}

	var result interface{}

	if err := util.UnmarshalJSON(buffer.Bytes(), &result); err != nil {
		t.Errorf("Unexpected output format: %v", err)
		return
	}
	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Expected %v but got: %v", expected, result)
	}
}

func TestEvalData(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	testmod := ast.MustParseModule(`package ex
	p = [1,2,3]`)
	if err := storage.InsertPolicy(ctx, store, "test", testmod, nil, false); err != nil {
		panic(err)
	}
	repl.OneShot(ctx, "data")
	expected := parseJSON(`
	{
		"a": [
			{
			"b": {
				"c": [
				true,
				2,
				false
				]
			}
			},
			{
			"b": {
				"c": [
				false,
				true,
				1
				]
			}
			}
		],
		"ex": {
			"p": [
			1,
			2,
			3
			]
		}
	}`)
	result := parseJSON(buffer.String())

	// Strip REPL documents out as these change depending on build settings.
	data := result.(map[string]interface{})
	delete(data, "repl")

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected:\n%v\n\nGot:\n%v", expected, result)
	}
}

func TestEvalFalse(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "false")
	result := buffer.String()
	if result != "false\n" {
		t.Errorf("Expected result to be false but got: %v", result)
	}
}

func TestEvalConstantRule(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "pi = 3.14")
	result := buffer.String()
	if result != "" {
		t.Errorf("Expected rule to be defined but got: %v", result)
		return
	}
	buffer.Reset()
	repl.OneShot(ctx, "pi")
	result = buffer.String()
	expected := "3.14\n"
	if result != expected {
		t.Errorf("Expected pi to evaluate to 3.14 but got: %v", result)
		return
	}
	buffer.Reset()
	repl.OneShot(ctx, "pi.deadbeef")
	result = buffer.String()
	if result != "undefined\n" {
		t.Errorf("Expected pi.deadbeef to be undefined but got: %v", result)
		return
	}
	buffer.Reset()
	repl.OneShot(ctx, "pi > 3")
	result = buffer.String()
	if result != "true\n" {
		t.Errorf("Expected pi > 3 to be true but got: %v", result)
		return
	}
}

func TestEvalSingleTermMultiValue(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
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
	if err := util.UnmarshalJSON([]byte(input), &expected); err != nil {
		panic(err)
	}

	repl.OneShot(ctx, "data.a[i].b.c[_]")
	var result interface{}
	if err := util.UnmarshalJSON(buffer.Bytes(), &result); err != nil {
		t.Errorf("Expected valid JSON document: %v: %v", err, buffer.String())
		return
	}

	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Expected %v but got: %v", expected, result)
		return
	}

	buffer.Reset()

	repl.OneShot(ctx, "data.deadbeef[x]")
	s := buffer.String()
	if s != "undefined\n" {
		t.Errorf("Expected undefined from reference but got: %v", s)
		return
	}

	buffer.Reset()

	repl.OneShot(ctx, "p[x] :- a = [1,2,3,4], a[_] = x")
	buffer.Reset()
	repl.OneShot(ctx, "p[x]")

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

	if err := util.UnmarshalJSON([]byte(input), &expected); err != nil {
		panic(err)
	}

	if err := util.UnmarshalJSON(buffer.Bytes(), &result); err != nil {
		t.Errorf("Expected valid JSON document: %v: %v", err, buffer.String())
		return
	}

	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Exepcted %v but got: %v", expected, result)
	}
}

func TestEvalSingleTermMultiValueSetRef(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.outputFormat = "json"
	repl.OneShot(ctx, "p[1] :- true")
	repl.OneShot(ctx, "p[2] :- true")
	repl.OneShot(ctx, "q = {3,4} :- true")
	repl.OneShot(ctx, "r = [x, y] :- x = {5,6}, y = [7,8]")

	repl.OneShot(ctx, "p[x]")
	expected := parseJSON(`[{"x": 1}, {"x": 2}]`)
	result := parseJSON(buffer.String())
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}

	buffer.Reset()
	repl.OneShot(ctx, "q[x]")
	expected = parseJSON(`[{"x": 3}, {"x": 4}]`)
	result = parseJSON(buffer.String())
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}

	// Example below shows behavior for ref that iterates an embedded set. The
	// tricky part here is that r[_] may refer to multiple collection types. If
	// we eventually have a way of distinguishing between the bindings added for
	// refs to sets, then those bindings could be filtered out. For now this is
	// acceptable, as it should be an edge case.
	buffer.Reset()
	repl.OneShot(ctx, "r[_][x]")
	expected = parseJSON(`[{"x": 5, "r[_][x]": 5}, {"x": 6, "r[_][x]": 6}, {"x": 0, "r[_][x]": 7}, {"x": 1, "r[_][x]": 8}]`)
	result = parseJSON(buffer.String())
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}
}

func TestEvalRuleCompileError(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "p[x] :- true")
	result := buffer.String()
	expected := "error: 1 error occurred: 1:1: p: x is unsafe (variable x must appear in at least one expression within the body of p)\n"
	if result != expected {
		t.Errorf("Expected error message in output but got: %v", result)
		return
	}
	buffer.Reset()
	repl.OneShot(ctx, "p = true :- true")
	result = buffer.String()
	if result != "" {
		t.Errorf("Expected valid rule to compile (because state should be unaffected) but got: %v", result)
	}
}

func TestEvalBodyCompileError(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.outputFormat = "json"
	err := repl.OneShot(ctx, "x = 1, y > x")
	if _, ok := err.(ast.Errors); !ok {
		t.Fatalf("Expected error message in output but got`: %v", buffer.String())
	}
	buffer.Reset()
	repl.OneShot(ctx, "x = 1, y = 2, y > x")
	var result2 []interface{}
	err = util.UnmarshalJSON(buffer.Bytes(), &result2)
	if err != nil {
		t.Errorf("Expected valid JSON output but got: %v", buffer.String())
		return
	}
	expected2 := []interface{}{
		map[string]interface{}{
			"x": json.Number("1"),
			"y": json.Number("2"),
		},
	}
	if !reflect.DeepEqual(expected2, result2) {
		t.Errorf(`Expected [{"x": 1, "y": 2}] but got: %v"`, result2)
		return
	}
}

func TestEvalBodyContainingWildCards(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "data.a[_].b.c[_] = x")
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

func TestEvalBodyInput(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	repl.OneShot(ctx, "package repl")
	repl.OneShot(ctx, `input["foo.bar"] = "hello" :- true`)
	repl.OneShot(ctx, `input["baz"] = data.a[0].b.c[2] :- true`)
	repl.OneShot(ctx, "package test")
	repl.OneShot(ctx, "import input.baz")
	repl.OneShot(ctx, `p :- input["foo.bar"] = "hello", baz = false`)
	repl.OneShot(ctx, "p")

	result := buffer.String()
	if result != "true\n" {
		t.Fatalf("expected true but got: %v", result)
	}
}

func TestEvalBodyInputComplete(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	// Test that input can be defined completely:
	// https://github.com/open-policy-agent/opa/issues/231
	repl.OneShot(ctx, `package repl`)
	repl.OneShot(ctx, `input = 1`)
	repl.OneShot(ctx, `input`)

	result := buffer.String()
	if result != "1\n" {
		t.Fatalf("Expected 1 but got: %v", result)
	}

	buffer.Reset()

	// Test that input is as expected
	repl.OneShot(ctx, `package ex1`)
	repl.OneShot(ctx, `x = input`)
	repl.OneShot(ctx, `x`)

	result = buffer.String()
	if result != "1\n" {
		t.Fatalf("Expected 1 but got: %v", result)
	}

	buffer.Reset()

	// Test that local input replaces other inputs
	repl.OneShot(ctx, `package ex2`)
	repl.OneShot(ctx, `input = 2`)
	repl.OneShot(ctx, `input`)

	result = buffer.String()

	if result != "2\n" {
		t.Fatalf("Expected 2 but got: %v", result)
	}

	buffer.Reset()

	// Test that original input is intact
	repl.OneShot(ctx, `package ex3`)
	repl.OneShot(ctx, `input`)

	result = buffer.String()

	if result != "1\n" {
		t.Fatalf("Expected 1 but got: %v", result)
	}

}

func TestEvalBodyWith(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	repl.OneShot(ctx, `p :- input.foo = "bar"`)
	err := repl.OneShot(ctx, "p")

	if err == nil || !strings.Contains(err.Error(), "input document undefined") {
		t.Fatalf("Expected input document undefined error")
	}

	repl.OneShot(ctx, `p with input.foo as "bar"`)

	result := buffer.String()
	expected := "true\n"

	if result != expected {
		t.Fatalf("Expected true but got: %v", result)
	}
}

func TestEvalImport(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "import data.a")
	if len(buffer.Bytes()) != 0 {
		t.Errorf("Expected no output but got: %v", buffer.String())
		return
	}
	buffer.Reset()
	repl.OneShot(ctx, "a[0].b.c[0] = true")
	result := buffer.String()
	expected := "true\n"
	if result != expected {
		t.Errorf("Expected expression to evaluate successfully but got: %v", result)
		return
	}

	// https://github.com/open-policy-agent/opa/issues/158 - re-run query to
	// make sure import is not lost
	buffer.Reset()
	repl.OneShot(ctx, "a[0].b.c[0] = true")
	result = buffer.String()
	expected = "true\n"
	if result != expected {
		t.Fatalf("Expected expression to evaluate successfully but got: %v", result)
	}
}

func TestEvalPackage(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "package foo.bar")
	repl.OneShot(ctx, "p = true :- true")
	repl.OneShot(ctx, "package baz.qux")
	buffer.Reset()
	err := repl.OneShot(ctx, "p")
	if err.Error() != "1 error occurred: 1:1: p is unsafe (variable p must appear in the output position of at least one non-negated expression)" {
		t.Fatalf("Expected unsafe variable error but got: %v", err)
	}
	repl.OneShot(ctx, "import data.foo.bar.p")
	buffer.Reset()
	repl.OneShot(ctx, "p")
	if buffer.String() != "true\n" {
		t.Errorf("Expected expression to eval successfully but got: %v", buffer.String())
		return
	}
}

func TestEvalTrace(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "trace")
	repl.OneShot(ctx, "data.a[i].b.c[j] = x, data.a[k].b.c[x] = 1")
	expected := strings.TrimSpace(`
Enter data.a[i].b.c[j] = x, data.a[k].b.c[x] = 1
| Eval data.a[i].b.c[j] = x
| Eval data.a[k].b.c[true] = 1
| Fail data.a[k].b.c[true] = 1
| Redo data.a[0].b.c[0] = x
| Eval data.a[k].b.c[2] = 1
| Fail data.a[0].b.c[2] = 1
| Redo data.a[0].b.c[2] = 1
| Exit data.a[i].b.c[j] = x, data.a[k].b.c[x] = 1
Redo data.a[i].b.c[j] = x, data.a[k].b.c[x] = 1
| Redo data.a[0].b.c[1] = x
| Eval data.a[k].b.c[false] = 1
| Fail data.a[k].b.c[false] = 1
| Redo data.a[0].b.c[2] = x
| Eval data.a[k].b.c[false] = 1
| Fail data.a[k].b.c[false] = 1
| Redo data.a[1].b.c[0] = x
| Eval data.a[k].b.c[true] = 1
| Fail data.a[k].b.c[true] = 1
| Redo data.a[1].b.c[1] = x
| Eval data.a[k].b.c[1] = 1
| Fail data.a[0].b.c[1] = 1
| Redo data.a[0].b.c[1] = 1
| Fail data.a[1].b.c[1] = 1
+---+---+---+---+
| i | j | k | x |
+---+---+---+---+
| 0 | 1 | 1 | 2 |
+---+---+---+---+`)
	expected += "\n"

	if expected != buffer.String() {
		t.Fatalf("Expected output to be exactly:\n%v\n\nGot:\n\n%v\n", expected, buffer.String())
	}
}

func TestEvalTruth(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "truth")
	repl.OneShot(ctx, "data.a[i].b.c[j] = x, data.a[k].b.c[x] = 1")
	expected := strings.TrimSpace(`
Enter data.a[i].b.c[j] = x, data.a[k].b.c[x] = 1
| Redo data.a[0].b.c[0] = x
| Redo data.a[0].b.c[2] = 1
| Exit data.a[i].b.c[j] = x, data.a[k].b.c[x] = 1
+---+---+---+---+
| i | j | k | x |
+---+---+---+---+
| 0 | 1 | 1 | 2 |
+---+---+---+---+`)
	expected += "\n"

	if expected != buffer.String() {
		t.Fatalf("Expected output to be exactly:\n%v\n\nGot:\n\n%v\n", expected, buffer.String())
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

func assertREPLText(t *testing.T, buf bytes.Buffer, expected string) {
	result := buf.String()
	if result != expected {
		t.Fatalf("Expected:\n%v\n\nGot:\n%v", expected, result)
	}
}

func expectOutput(t *testing.T, output string, expected string) {
	if output != expected {
		t.Errorf("Repl output: expected %#v but got %#v", expected, output)
	}
}

func newRepl(store *storage.Storage, buffer *bytes.Buffer) *REPL {
	repl := New(store, "", buffer, "", "")
	return repl
}

func newTestStore() *storage.Storage {
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
	err := util.UnmarshalJSON([]byte(input), &data)
	if err != nil {
		panic(err)
	}
	return storage.New(storage.InMemoryWithJSONConfig(data))
}

func parseJSON(s string) interface{} {
	var v interface{}
	if err := util.UnmarshalJSON([]byte(s), &v); err != nil {
		panic(err)
	}
	return v
}
