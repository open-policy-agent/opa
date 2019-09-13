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
	"github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

func TestFunction(t *testing.T) {
	store := newTestStore()
	ctx := context.Background()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	mod1 := []byte(`package a.b.c

foo(x) = y {
	split(x, ".", y)
}

bar([x, y]) = z {
	trim(x, y, z)
}
`)

	mod2 := []byte(`package a.b.d

baz(_) = y {
	data.a.b.c.foo("barfoobar.bar", x)
	data.a.b.c.bar(x, y)
}`)

	if err := store.UpsertPolicy(ctx, txn, "mod1", mod1); err != nil {
		panic(err)
	}

	if err := store.UpsertPolicy(ctx, txn, "mod2", mod2); err != nil {
		panic(err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	repl := newRepl(store, &buf)
	repl.OneShot(ctx, "json")
	repl.OneShot(ctx, "data.a.b.d.baz(null, x)")
	exp := util.MustUnmarshalJSON([]byte(`{"result": [{"expressions": [{"text":"data.a.b.d.baz(null, x)", "value": true, "location": {"row": 1, "col": 1}}], "bindings": {"x": "foo"}}]}`))
	result := util.MustUnmarshalJSON(buf.Bytes())
	if !reflect.DeepEqual(exp, result) {
		t.Fatalf("expected data.a.b.d.baz(x) to be %v, got %v", exp, result)
	}

	err := repl.OneShot(ctx, "p(x) = y { y = x+4 }")
	if err != nil {
		t.Fatalf("failed to compile repl function: %v", err)
	}

	buf.Reset()
	repl.OneShot(ctx, "data.repl.p(5, y)")
	exp = util.MustUnmarshalJSON([]byte(`{
		"result": [
			{
				"expressions": [
					{
						"text": "data.repl.p(5, y)",
						"value": true,
						"location": {
							"col": 1,
							"row": 1
						}
					}
				],
				"bindings": {
					"y": 9
				}
			}
		]
	}`))
	result = util.MustUnmarshalJSON(buf.Bytes())
	if !reflect.DeepEqual(exp, result) {
		t.Fatalf("expected datrepl.p(x) to be %v, got %v", exp, result)
	}

	repl.OneShot(ctx, "f(1, x) = y { y = x }")
	repl.OneShot(ctx, "f(2, x) = y { y = x*2 }")

	buf.Reset()
	repl.OneShot(ctx, "data.repl.f(1, 2, y)")
	exp = util.MustUnmarshalJSON([]byte(`{
		"result": [
			{
				"expressions": [
					{
						"text": "data.repl.f(1, 2, y)",
						"location": {
							"col": 1,
							"row": 1
						},
						"value": true
					}
				],
				"bindings": {
					"y": 2
				}
			}
		]
	}`))
	result = util.MustUnmarshalJSON(buf.Bytes())
	if !reflect.DeepEqual(exp, result) {
		t.Fatalf("expected data.repl.f(1, 2, y) to be %v, got %v", exp, result)
	}
	buf.Reset()
	repl.OneShot(ctx, "data.repl.f(2, 2, y)")
	exp = util.MustUnmarshalJSON([]byte(`{
		"result": [
			{
				"expressions": [
					{
						"text": "data.repl.f(2, 2, y)",
						"location": {
							"col": 1,
							"row": 1
						},
						"value": true
					}
				],
				"bindings": {
					"y": 4
				}
			}
		]
	}`))
	result = util.MustUnmarshalJSON(buf.Bytes())
	if !reflect.DeepEqual(exp, result) {
		t.Fatalf("expected data.repl.f(2, 2, y) to be %v, got %v", exp, result)
	}
}

func TestComplete(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	mod1 := []byte(`package a.b.c

p = 1 { true }
q = 2 { true }
q = 3 { false }`)

	mod2 := []byte(`package a.b.d

r = 3 { true }`)

	if err := store.UpsertPolicy(ctx, txn, "mod1", mod1); err != nil {
		panic(err)
	}

	if err := store.UpsertPolicy(ctx, txn, "mod2", mod2); err != nil {
		panic(err)
	}

	if err := store.Commit(ctx, txn); err != nil {
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
	expected = []string{}

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
	store := inmem.NewFromObject(data)
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
	store := inmem.NewFromObject(data)
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
		t.Fatalf("Expected json unmarshal to succeed but got: %v", err)
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
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "help deadbeef")

	expected := "blah blah blah\n"

	if buffer.String() != expected {
		t.Fatalf("Unexpected output from help topic: %v", buffer.String())
	}
}

func TestShowDebug(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "show debug")

	var result replDebugState

	if err := util.Unmarshal(buffer.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	var exp replDebugState
	exp.Explain = explainOff

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected %+v but got %+v", exp, result)
	}

	buffer.Reset()

	repl.OneShot(ctx, "trace")
	repl.OneShot(ctx, "metrics")
	repl.OneShot(ctx, "instrument")
	repl.OneShot(ctx, "profile")
	repl.OneShot(ctx, "show debug")

	exp.Explain = explainFull
	exp.Metrics = true
	exp.Instrument = true
	exp.Profile = true

	if err := util.Unmarshal(buffer.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(result, exp) {
		t.Fatalf("Expected %+v but got %+v", exp, result)
	}
}

func TestShow(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	repl.OneShot(ctx, `package repl_test`)
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

import data.foo as bar
import input.xyz` + "\n"
	assertREPLText(t, buffer, expected)
	buffer.Reset()

	repl.OneShot(ctx, `p[1] { true }`)
	buffer.Reset()
	repl.OneShot(ctx, `p[2] { true }`)
	buffer.Reset()
	repl.OneShot(ctx, "show")

	expected = `package repl_test

import data.foo as bar
import input.xyz

p[1]

p[2]` + "\n"
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

func TestTypes(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	repl.OneShot(ctx, "types")
	repl.OneShot(ctx, `p[x] = y { x := "a"; y := 1 }`)
	repl.OneShot(ctx, `p[x]`)

	output := strings.TrimSpace(buffer.String())

	exp := []string{
		"# data.repl.p[x]: number",
		"# x: string",
	}

	for i := range exp {
		if !strings.Contains(output, exp[i]) {
			t.Fatalf("Expected output to contain %q but got: %v", exp[i], output)
		}
	}

}

func TestUnknown(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	repl.OneShot(ctx, "xs = [1,2,3]")
	buffer.Reset()

	err := repl.OneShot(ctx, "unknown input")
	if err != nil {
		t.Fatal("Unexpected command error:", err)
	}

	repl.OneShot(ctx, "data.repl.xs[i] = x; input.x = x")

	output := strings.TrimSpace(buffer.String())
	expected := strings.TrimSpace(`
+---------+-------------+
| Query 1 | input.x = 1 |
|         | i = 0       |
|         | x = 1       |
+---------+-------------+
| Query 2 | input.x = 2 |
|         | i = 1       |
|         | x = 2       |
+---------+-------------+
| Query 3 | input.x = 3 |
|         | i = 2       |
|         | x = 3       |
+---------+-------------+
`)

	if output != expected {
		t.Fatalf("Unexpected output. Expected:\n\n%v\n\nGot:\n\n%v", expected, output)
	}
}
func TestUnknownMetrics(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	repl.OneShot(ctx, "xs = [1,2,3]")
	buffer.Reset()

	err := repl.OneShot(ctx, "unknown input")
	if err != nil {
		t.Fatal("Unexpected command error:", err)
	}

	repl.OneShot(ctx, "metrics")

	repl.OneShot(ctx, "data.repl.xs[i] = x; input.x = x")

	output := strings.TrimSpace(buffer.String())
	expected := strings.TrimSpace(`
+---------+-------------+
| Query 1 | input.x = 1 |
|         | i = 0       |
|         | x = 1       |
+---------+-------------+
| Query 2 | input.x = 2 |
|         | i = 1       |
|         | x = 2       |
+---------+-------------+
| Query 3 | input.x = 3 |
|         | i = 2       |
|         | x = 3       |
+---------+-------------+
`)

	if !strings.HasPrefix(output, expected) {
		t.Fatalf("Unexpected partial eval results. Expected:\n\n%v\n\nGot:\n\n%v", expected, output)
	}

	if !strings.Contains(output, "timer_rego_partial_eval_ns") {
		t.Fatal("Expected timer_rego_partial_eval_ns but got:\n\n", output)
	}
}

func TestUnknownJSON(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	repl.OneShot(ctx, "xs = [1,2,3]")
	buffer.Reset()

	err := repl.OneShot(ctx, "unknown input")
	if err != nil {
		t.Fatal("Unexpected command error:", err)
	}

	repl.OneShot(ctx, "json")
	repl.OneShot(ctx, "data.repl.xs[i] = x; input.x = x")

	var result presentation.Output

	if err := json.NewDecoder(&buffer).Decode(&result); err != nil {
		t.Fatal(err)
	}

	if len(result.Partial.Queries) != 3 {
		t.Fatalf("Expected exactly 3 queries in partial evaluation output but got: %v", result)
	}
}

func TestUnknownInvalid(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	err := repl.OneShot(ctx, "unknown x-1")
	if err == nil || !strings.Contains(err.Error(), "usage: unknown <input/data reference>") {
		t.Fatal("expected error from setting bad unknown but got:", err)
	}

	// Ensure that partial evaluation has not been enabled.
	buffer.Reset()
	repl.OneShot(ctx, "1+2")
	result := strings.TrimSpace(buffer.String())
	if result != "3" {
		t.Fatal("want true but got:", result)
	}
}

func TestUnset(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	var err error

	repl.OneShot(ctx, "magic = 23")
	repl.OneShot(ctx, "p = 3.14")
	repl.OneShot(ctx, "unset p")

	err = repl.OneShot(ctx, "p")

	if _, ok := err.(ast.Errors); !ok {
		t.Fatalf("Expected AST error but got: %v", err)
	}

	buffer.Reset()
	repl.OneShot(ctx, "p = 3.14")
	repl.OneShot(ctx, `p = 3 { false }`)
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
	repl.OneShot(ctx, "p(x) = y { y = x }")
	repl.OneShot(ctx, "unset p")

	err = repl.OneShot(ctx, "data.repl.p(1, 2)")
	if err == nil || err.Error() != `1 error occurred: 1:1: rego_type_error: undefined function data.repl.p` {
		t.Fatalf("Expected eval error (undefined built-in) but got err: '%v'", err)
	}

	buffer.Reset()
	repl.OneShot(ctx, "p(1, x) = y { y = x }")
	repl.OneShot(ctx, "p(2, x) = y { y = x+1 }")
	repl.OneShot(ctx, "unset p")

	err = repl.OneShot(ctx, "data.repl.p(1, 2, 3)")
	if err == nil || err.Error() != `1 error occurred: 1:1: rego_type_error: undefined function data.repl.p` {
		t.Fatalf("Expected eval error (undefined built-in) but got err: '%v'", err)
	}

	buffer.Reset()
	repl.OneShot(ctx, `unset q`)
	if buffer.String() != "warning: no matching rules in current module\n" {
		t.Fatalf("Expected unset error for missing rule but got: %v", buffer.String())
	}

	buffer.Reset()
	repl.OneShot(ctx, `unset q`)
	if buffer.String() != "warning: no matching rules in current module\n" {
		t.Fatalf("Expected unset error for missing function but got: %v", buffer.String())
	}

	buffer.Reset()
	repl.OneShot(ctx, `magic`)
	if buffer.String() != "23\n" {
		t.Fatalf("Expected magic to be defined but got: %v", buffer.String())
	}

	buffer.Reset()
	repl.OneShot(ctx, `package data.other`)
	err = repl.OneShot(ctx, `unset magic`)
	if buffer.String() != "warning: no matching rules in current module\n" {
		t.Fatalf("Expected unset error for bad syntax but got: %v", buffer.String())
	}

	repl.OneShot(ctx, `input = {}`)

	if err := repl.OneShot(ctx, `unset input`); err != nil {
		t.Fatalf("Expected unset to succeed for input: %v", err)
	}

	buffer.Reset()
	repl.OneShot(ctx, `not input`)

	if buffer.String() != "true\n" {
		t.Fatalf("Expected unset input to remove input document: %v", buffer.String())
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
	expectOutput(t, buffer.String(), "undefined\n")
}

func TestOneShotEmptyBufferOneRule(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, `p[x] { data.a[i] = x }`)
	expectOutput(t, buffer.String(), "Rule 'p' defined in package repl. Type 'show' to see rules.\n")
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
	repl.OneShot(ctx, "p[x] { ")
	expectOutput(t, buffer.String(), "")
	repl.OneShot(ctx, "data.a[i].b.c[1]")
	expectOutput(t, buffer.String(), "")
	repl.OneShot(ctx, " = ")
	expectOutput(t, buffer.String(), "")
	repl.OneShot(ctx, "x")
	expectOutput(t, buffer.String(), "")
	repl.OneShot(ctx, "}")
	expectOutput(t, buffer.String(), "")
	repl.OneShot(ctx, "")
	expectOutput(t, buffer.String(), "Rule 'p' defined in package repl. Type 'show' to see rules.\n")
	buffer.Reset()
	repl.OneShot(ctx, "p[2]")
	expectOutput(t, buffer.String(), "2\n")
}

func TestOneShotJSON(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.outputFormat = "json"
	repl.OneShot(ctx, "data.a[i] = x")
	var expected interface{}
	if err := util.UnmarshalJSON([]byte(`{
		"result": [
		  {
			"expressions": [
			  {
				"value": true,
				"text": "data.a[i] = x",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
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
			}
		  },
		  {
			"expressions": [
			  {
				"value": true,
				"text": "data.a[i] = x",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
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
		  }
		]
	  }`), &expected); err != nil {
		panic(err)
	}

	var result interface{}

	if err := util.UnmarshalJSON(buffer.Bytes(), &result); err != nil {
		t.Errorf("Unexpected output format: %v", err)
		return
	}

	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Expected %v but got: %v", expected, buffer.String())
	}
}

func TestEvalData(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	testMod := []byte(`package ex

p = [1, 2, 3] { true }`)

	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	if err := store.UpsertPolicy(ctx, txn, "test", testMod); err != nil {
		panic(err)
	}

	if err := store.Commit(ctx, txn); err != nil {
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
	if result != "Rule 'pi' defined in package repl. Type 'show' to see rules.\n" {
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
	err := repl.OneShot(ctx, "pi.deadbeef")
	result = buffer.String()
	if result != "" || !strings.Contains(err.Error(), "undefined ref: data.repl.pi.deadbeef") {
		t.Fatalf("Expected pi.deadbeef to fail/error but got:\nresult: %q\nerr: %v", result, err)
	}
	buffer.Reset()
	repl.OneShot(ctx, "pi > 3")
	result = buffer.String()
	if result != "true\n" {
		t.Errorf("Expected pi > 3 to be true but got: %v", result)
		return
	}
}

func TestEvalConstantRuleDefaultRootDoc(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "input = 1")
	buffer.Reset()
	repl.OneShot(ctx, "input = 2")
	assertREPLText(t, buffer, "undefined\n")
	buffer.Reset()
	repl.OneShot(ctx, "input = 1")
	assertREPLText(t, buffer, "true\n")
}

func TestEvalConstantRuleAssignment(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer

	defined := "Rule 'x' defined in package repl. Type 'show' to see rules.\n"
	redefined := "Rule 'x' re-defined in package repl. Type 'show' to see rules.\n"
	definedInput := "Rule 'input' defined in package repl. Type 'show' to see rules.\n"
	redefinedInput := "Rule 'input' re-defined in package repl. Type 'show' to see rules.\n"

	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "x = 1")
	assertREPLText(t, buffer, defined)
	buffer.Reset()
	repl.OneShot(ctx, "x := 2")
	assertREPLText(t, buffer, redefined)
	buffer.Reset()

	repl.OneShot(ctx, "show")
	assertREPLText(t, buffer, `package repl

x := 2
`)
	buffer.Reset()

	repl.OneShot(ctx, "x := 3")
	assertREPLText(t, buffer, redefined)
	buffer.Reset()
	repl.OneShot(ctx, "x")
	result := buffer.String()
	if result != "3\n" {
		t.Fatalf("Expected 3 but got: %v", result)
	}

	buffer.Reset()
	repl.OneShot(ctx, "x = 3")
	result = buffer.String()
	if result != "true\n" {
		t.Fatalf("Expected true but got: %v", result)
	}

	buffer.Reset()
	repl.OneShot(ctx, "input = 0")
	assertREPLText(t, buffer, definedInput)
	buffer.Reset()
	repl.OneShot(ctx, "input := 1")
	assertREPLText(t, buffer, redefinedInput)
	buffer.Reset()
	repl.OneShot(ctx, "input")
	result = buffer.String()
	if result != "1\n" {
		t.Fatalf("Expected 1 but got: %v", result)
	}

	buffer.Reset()
	err := repl.OneShot(ctx, "assign()")
	if err == nil || !strings.Contains(err.Error(), "too few arguments") {
		t.Fatal("Expected type check error but got:", err)
	}
}

func TestEvalSingleTermMultiValue(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.outputFormat = "json"

	input := `{
		"result": [
		  {
			"expressions": [
			  {
				"value": true,
				"text": "data.a[i].b.c[_]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "i": 0
			}
		  },
		  {
			"expressions": [
			  {
				"value": 2,
				"text": "data.a[i].b.c[_]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "i": 0
			}
		  },
		  {
			"expressions": [
			  {
				"value": true,
				"text": "data.a[i].b.c[_]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "i": 1
			}
		  },
		  {
			"expressions": [
			  {
				"value": 1,
				"text": "data.a[i].b.c[_]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "i": 1
			}
		  }
		]
	  }`

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
		t.Errorf("Expected %v but got: %v", expected, buffer.String())
		return
	}

	buffer.Reset()

	repl.OneShot(ctx, "data.deadbeef[x]")
	s := buffer.String()
	if s != "{}\n" {
		t.Errorf("Expected undefined from reference but got: %v", s)
		return
	}

	buffer.Reset()

	repl.OneShot(ctx, `p[x] { a = [1, 2, 3, 4]; a[_] = x }`)
	buffer.Reset()
	repl.OneShot(ctx, "p[x]")

	input = `
	{
		"result": [
		  {
			"expressions": [
			  {
				"value": 1,
				"text": "p[x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 1
			}
		  },
		  {
			"expressions": [
			  {
				"value": 2,
				"text": "p[x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 2
			}
		  },
		  {
			"expressions": [
			  {
				"value": 3,
				"text": "p[x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 3
			}
		  },
		  {
			"expressions": [
			  {
				"value": 4,
				"text": "p[x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 4
			}
		  }
		]
	}
	`

	if err := util.UnmarshalJSON([]byte(input), &expected); err != nil {
		panic(err)
	}

	if err := util.UnmarshalJSON(buffer.Bytes(), &result); err != nil {
		t.Errorf("Expected valid JSON document: %v: %v", err, buffer.String())
		return
	}

	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Exepcted %v but got: %v", expected, buffer.String())
	}
}

func TestEvalSingleTermMultiValueSetRef(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.outputFormat = "json"
	repl.OneShot(ctx, `p[1] { true }`)
	repl.OneShot(ctx, `p[2] { true }`)
	repl.OneShot(ctx, `q = {3, 4} { true }`)
	repl.OneShot(ctx, `r = [x, y] { x = {5, 6}; y = [7, 8] }`)

	repl.OneShot(ctx, "p[x]")
	expected := parseJSON(`{
		"result": [
		  {
			"expressions": [
			  {
				"value": 1,
				"text": "p[x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 1
			}
		  },
		  {
			"expressions": [
			  {
				"value": 2,
				"text": "p[x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 2
			}
		  }
		]
	  }`)
	result := parseJSON(buffer.String())
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, buffer.String())
	}

	buffer.Reset()
	repl.OneShot(ctx, "q[x]")
	expected = parseJSON(`{
		"result": [
		  {
			"expressions": [
			  {
				"value": 3,
				"text": "q[x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 3
			}
		  },
		  {
			"expressions": [
			  {
				"value": 4,
				"text": "q[x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 4
			}
		  }
		]
	  }`)
	result = parseJSON(buffer.String())
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, buffer.String())
	}

	// Example below shows behavior for ref that iterates an embedded set. The
	// tricky part here is that r[_] may refer to multiple collection types. If
	// we eventually have a way of distinguishing between the bindings added for
	// refs to sets, then those bindings could be filtered out. For now this is
	// acceptable, as it should be an edge case.
	buffer.Reset()
	repl.OneShot(ctx, "r[_][x]")
	expected = parseJSON(`{
		"result": [
		  {
			"expressions": [
			  {
				"value": 5,
				"text": "r[_][x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 5
			}
		  },
		  {
			"expressions": [
			  {
				"value": 6,
				"text": "r[_][x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 6
			}
		  },
		  {
			"expressions": [
			  {
				"value": 7,
				"text": "r[_][x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 0
			}
		  },
		  {
			"expressions": [
			  {
				"value": 8,
				"text": "r[_][x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 1
			}
		  }
		]
	  }`)
	result = parseJSON(buffer.String())
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, buffer.String())
	}
}

func TestEvalRuleCompileError(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	err := repl.OneShot(ctx, `p[x] { true }`)
	expected := "x is unsafe"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("Expected error to contain %v but got: %v (err: %v)", expected, buffer.String(), err)
		return
	}
	buffer.Reset()
	err = repl.OneShot(ctx, `p = true { true }`)
	result := buffer.String()
	if err != nil || result != "Rule 'p' defined in package repl. Type 'show' to see rules.\n" {
		t.Errorf("Expected valid rule to compile (because state should be unaffected) but got: %v (err: %v)", result, err)
	}
}

func TestEvalBodyCompileError(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.outputFormat = "json"
	err := repl.OneShot(ctx, `x = 1; y > x`)
	if _, ok := err.(ast.Errors); !ok {
		t.Fatalf("Expected error message in output but got`: %v", buffer.String())
	}
	buffer.Reset()
	repl.OneShot(ctx, `x = 1; y = 2; y > x`)
	result := util.MustUnmarshalJSON(buffer.Bytes())
	exp := util.MustUnmarshalJSON([]byte(`{
		"result": [
		  {
			"expressions": [
			  {
				"value": true,
				"text": "x = 1",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  },
			  {
				"value": true,
				"text": "y = 2",
				"location": {
				  "row": 1,
				  "col": 8
				}
			  },
			  {
				"value": true,
				"text": "y \u003e x",
				"location": {
				  "row": 1,
				  "col": 15
				}
			  }
			],
			"bindings": {
			  "x": 1,
			  "y": 2
			}
		  }
		]
	  }`))
	if !reflect.DeepEqual(exp, result) {
		t.Errorf(`Expected %v but got: %v"`, exp, buffer.String())
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

	repl.OneShot(ctx, `package repl`)
	repl.OneShot(ctx, `input["foo.bar"] = "hello" { true }`)
	repl.OneShot(ctx, `input["baz"] = data.a[0].b.c[2] { true }`)
	repl.OneShot(ctx, `package test`)
	repl.OneShot(ctx, "import input.baz")
	repl.OneShot(ctx, `p = true { input["foo.bar"] = "hello"; baz = false }`)
	buffer.Reset()
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
	buffer.Reset()
	repl.OneShot(ctx, `input`)

	result := buffer.String()
	if result != "1\n" {
		t.Fatalf("Expected 1 but got: %v", result)
	}

	buffer.Reset()

	// Test that input is as expected
	repl.OneShot(ctx, `package ex1`)
	repl.OneShot(ctx, `x = input`)
	buffer.Reset()
	repl.OneShot(ctx, `x`)

	result = buffer.String()
	if result != "1\n" {
		t.Fatalf("Expected 1 but got: %v", result)
	}

	// Test that local input replaces other inputs
	repl.OneShot(ctx, `package ex2`)
	repl.OneShot(ctx, `input = 2`)
	buffer.Reset()
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

	// Test that deferencing undefined input results in undefined
	buffer.Reset()

	repl = newRepl(store, &buffer)
	repl.OneShot(ctx, `input.p`)
	result = buffer.String()
	if result != "undefined\n" {
		t.Fatalf("Expected undefined but got: %v", result)
	}

	buffer.Reset()
	repl.OneShot(ctx, `input.p = false`)
	result = buffer.String()
	if result != "undefined\n" {
		t.Fatalf("Expected undefined but got: %v", result)
	}

}

func TestEvalBodyWith(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	repl.OneShot(ctx, `p = true { input.foo = "bar" }`)
	buffer.Reset()
	repl.OneShot(ctx, "p")

	if buffer.String() != "undefined\n" {
		t.Fatalf("Expected undefined but got: %v", buffer.String())
	}

	buffer.Reset()

	repl.OneShot(ctx, `p with input.foo as "bar"`)

	result := buffer.String()
	expected := "true\n"

	if result != expected {
		t.Fatalf("Expected true but got: %v", result)
	}
}

func TestEvalBodyRewrittenBuiltin(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "json")
	repl.OneShot(ctx, `p[x] { a[x]; a = [1,2,3,4] }`)
	repl.OneShot(ctx, "p[x] > 1")
	result := util.MustUnmarshalJSON(buffer.Bytes())
	expected := util.MustUnmarshalJSON([]byte(`{
		"result": [
		  {
			"expressions": [
			  {
				"value": true,
				"text": "p[x] \u003e 1",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 2
			}
		  },
		  {
			"expressions": [
			  {
				"value": true,
				"text": "p[x] \u003e 1",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 3
			}
		  }
		]
	  }`))
	if util.Compare(result, expected) != 0 {
		t.Fatalf("Expected %v but got: %v", expected, buffer.String())
	}
}

func TestEvalBodyRewrittenRef(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "json")
	repl.OneShot(ctx, `i = 1`)
	repl.OneShot(ctx, `data.a[0].b.c[i]`)
	result := util.MustUnmarshalJSON(buffer.Bytes())
	expected := util.MustUnmarshalJSON([]byte(`{
		"result": [
		  {
			"expressions": [
			  {
				"value": 2,
				"text": "data.a[0].b.c[i]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			]
		  }
		]
	  }`))
	if util.Compare(result, expected) != 0 {
		t.Fatalf("Expected %v but got: %v", expected, buffer.String())
	}
	buffer.Reset()
	repl.OneShot(ctx, "p = {1,2,3}")
	repl.OneShot(ctx, "p")
	result = util.MustUnmarshalJSON(buffer.Bytes())
	expected = util.MustUnmarshalJSON([]byte(`{
		"result": [
		  {
			"expressions": [
			  {
				"value": [
				  1,
				  2,
				  3
				],
				"text": "p",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			]
		  }
		]
	  }`))
	if util.Compare(result, expected) != 0 {
		t.Fatalf("Expected %v but got: %v", expected, buffer.String())
	}
	buffer.Reset()
	repl.OneShot(ctx, "p[x]")
	result = util.MustUnmarshalJSON(buffer.Bytes())
	expected = util.MustUnmarshalJSON([]byte(`{
		"result": [
		  {
			"expressions": [
			  {
				"value": 1,
				"text": "p[x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 1
			}
		  },
		  {
			"expressions": [
			  {
				"value": 2,
				"text": "p[x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 2
			}
		  },
		  {
			"expressions": [
			  {
				"value": 3,
				"text": "p[x]",
				"location": {
				  "row": 1,
				  "col": 1
				}
			  }
			],
			"bindings": {
			  "x": 3
			}
		  }
		]
	  }`))
	if util.Compare(result, expected) != 0 {
		t.Fatalf("Expected %v but got: %v", expected, buffer.String())
	}
}

func TestEvalBodySomeDecl(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "json")
	repl.OneShot(ctx, "some x; x = 1")
	exp := util.MustUnmarshalJSON([]byte(`{
		"result": [
			{
				"expressions": [
					{
						"value": true,
						"text": "x = 1",
						"location": {
							"row": 1,
							"col": 9
						}
					}
				],
				"bindings": {
					"x": 1
				}
			}
		]
	}`))
	result := util.MustUnmarshalJSON(buffer.Bytes())
	if util.Compare(result, exp) != 0 {
		t.Fatalf("Expected %v but got: %v", exp, result)
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
	repl.OneShot(ctx, `package foo.bar`)
	repl.OneShot(ctx, `p = true { true }`)
	repl.OneShot(ctx, `package baz.qux`)
	buffer.Reset()
	err := repl.OneShot(ctx, "p")
	if !strings.Contains(err.Error(), "p is unsafe") {
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

func TestMetrics(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer

	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "a = {[1,2], [3,4]}")
	repl.OneShot(ctx, "metrics")
	repl.OneShot(ctx, `[x | a[x]]`)
	if !strings.Contains(buffer.String(), "timer_rego_query_compile_ns") {
		t.Fatal("Expected output to contain well known metric key but got:", buffer.String())
	}

	buffer.Reset()
	repl.OneShot(ctx, `[x | a[x]]`)
	if !strings.Contains(buffer.String(), "timer_rego_query_compile_ns") {
		t.Fatal("Expected output to contain well known metric key but got:", buffer.String())
	}

	buffer.Reset()
	repl.OneShot(ctx, "metrics")
	repl.OneShot(ctx, `[x | a[x]]`)

	expected := `[
  [
    1,
    2
  ],
  [
    3,
    4
  ]
]
`

	if expected != buffer.String() {
		t.Fatalf("Expected output to be exactly:\n%v\n\nGot:\n\n%v\n", expected, buffer.String())
	}
}

func TestProfile(t *testing.T) {
	store := newTestStore()
	ctx := context.Background()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	const numLines = 21

	mod2 := []byte(`package rbac

		input = {
		"subject": "bob",
			"resource": "foo123",
			"action": "write",
	}
		bindings = [
	{
		"user": "alice",
		"roles": ["dev", "test"],
	},
	{
		"user": "bob",
		"roles": ["test"],
	},
]

	roles = [
	{
		"name": "dev",
		"permissions": [
		{"resource": "foo123", "action": "write"},
		{"resource": "foo123", "action": "read"},
	],
	},
	{
		"name": "test",
		"permissions": [{"resource": "foo123", "action": "read"}],
	},
]

default allow = false

	allow {
	user_has_role[role_name]
	role_has_permission[role_name]
	}

	user_has_role[role_name] {
	binding := bindings[_]
	binding.user = input.subject
	role_name := binding.roles[_]
	}

	role_has_permission[role_name] {
	role := roles[_]
	role_name := role.name
	perm := role.permissions[_]
	perm.resource = input.resource
	perm.action = input.action
	}`)

	if err := store.UpsertPolicy(ctx, txn, "mod2", mod2); err != nil {
		panic(err)
	}
	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "profile")
	repl.OneShot(ctx, "data.rbac.allow")
	result := buffer.String()
	lines := strings.Split(result, "\n")
	if len(lines) != numLines {
		t.Fatal("Expected 21 lines, got :", len(lines))
	}
	buffer.Reset()
}

func TestInstrument(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer

	repl := newRepl(store, &buffer)

	// Turn on instrumentation w/o turning on metrics.
	repl.OneShot(ctx, "instrument")
	repl.OneShot(ctx, "true")

	result := buffer.String()

	if !strings.Contains(result, "histogram_eval_op_plug") {
		t.Fatal("Expected plug histogram in output but got:", result)
	}

	buffer.Reset()

	// Turn off instrumentation.
	repl.OneShot(ctx, "instrument")
	repl.OneShot(ctx, "true")

	result = buffer.String()

	if strings.Contains(result, "histogram_eval_op_plug") {
		t.Fatal("Expected instrumentation to be turned off but got:", result)
	}

	buffer.Reset()

	// Turn on metrics and then turn on instrumentation.
	repl.OneShot(ctx, "metrics")
	repl.OneShot(ctx, "true")

	result = buffer.String()

	if strings.Contains(result, "histogram_eval_op_plug") {
		t.Fatal("Expected instrumentation to be turned off but got:", result)
	}

	if !strings.Contains(result, "timer_rego_query_eval_ns") {
		t.Fatal("Expected metrics to be turned on but got:", result)
	}

	buffer.Reset()

	repl.OneShot(ctx, "instrument")
	repl.OneShot(ctx, "true")

	result = buffer.String()

	if !strings.Contains(result, "histogram_eval_op_plug") {
		t.Fatal("Expected instrumentation to be turned on but got:", result)
	}

	if !strings.Contains(result, "timer_rego_query_eval_ns") {
		t.Fatal("Expected metrics to be turned on but got:", result)
	}

}

func TestEvalTrace(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, "trace")
	repl.OneShot(ctx, `data.a[i].b.c[j] = x; data.a[k].b.c[x] = 1`)
	expected := strings.TrimSpace(`
Enter data.a[i].b.c[j] = x; data.a[k].b.c[x] = 1
| Eval data.a[i].b.c[j] = x
| Eval data.a[k].b.c[x] = 1
| Fail data.a[k].b.c[x] = 1
| Redo data.a[i].b.c[j] = x
| Eval data.a[k].b.c[x] = 1
| Exit data.a[i].b.c[j] = x; data.a[k].b.c[x] = 1
Redo data.a[i].b.c[j] = x; data.a[k].b.c[x] = 1
| Redo data.a[k].b.c[x] = 1
| Redo data.a[i].b.c[j] = x
| Eval data.a[k].b.c[x] = 1
| Fail data.a[k].b.c[x] = 1
| Redo data.a[i].b.c[j] = x
| Eval data.a[k].b.c[x] = 1
| Fail data.a[k].b.c[x] = 1
| Redo data.a[i].b.c[j] = x
| Eval data.a[k].b.c[x] = 1
| Fail data.a[k].b.c[x] = 1
| Redo data.a[i].b.c[j] = x
| Eval data.a[k].b.c[x] = 1
| Fail data.a[k].b.c[x] = 1
| Redo data.a[i].b.c[j] = x
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

func TestEvalNotes(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot(ctx, `p { a = [1,2,3]; a[i] = x; x > 1; trace(sprintf("x = %d", [x])) }`)
	repl.OneShot(ctx, "notes")
	buffer.Reset()
	repl.OneShot(ctx, "p")
	expected := strings.TrimSpace(`Enter data.repl.p = _
| Enter data.repl.p
| | Note "x = 2"
Redo data.repl.p = _
| Redo data.repl.p
| | Note "x = 3"
true`)
	expected += "\n"
	if expected != buffer.String() {
		t.Fatalf("Expected output to be exactly:\n%v\n\nGot:\n\n%v\n", expected, buffer.String())
	}
}

func TestTruncatePrettyOutput(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.prettyLimit = 1000 // crank up limit to test repl command
	repl.OneShot(ctx, "pretty-limit 80")
	repl.OneShot(ctx, "data[x]")
	for _, line := range strings.Split(buffer.String(), "\n") {
		// | "repl" | {"version": <elided>... |
		if len(line) > 96 {
			t.Fatalf("Expected len(line) to be < 96 but got:\n\n%v", buffer)
		}
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "pretty-limit"); err == nil || !strings.Contains(err.Error(), "usage: pretty-limit <n>") {
		t.Fatalf("Expected usage error but got: %v", err)
	}
}

func assertREPLText(t *testing.T, buf bytes.Buffer, expected string) {
	t.Helper()
	result := buf.String()
	if result != expected {
		t.Fatalf("Expected:\n%v\n\nString:\n\n%v\nGot:\n%v\n\nString:\n\n%v", []byte(expected), expected, []byte(result), result)
	}
}

func expectOutput(t *testing.T, output string, expected string) {
	t.Helper()
	if output != expected {
		t.Errorf("Repl output: expected %#v but got %#v", expected, output)
	}
}

func newRepl(store storage.Store, buffer *bytes.Buffer) *REPL {
	repl := New(store, "", buffer, "", 0, "")
	return repl
}

func newTestStore() storage.Store {
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
	return inmem.NewFromObject(data)
}

func parseJSON(s string) interface{} {
	var v interface{}
	if err := util.UnmarshalJSON([]byte(s), &v); err != nil {
		panic(err)
	}
	return v
}
