// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/eval"
)

func TestDump(t *testing.T) {
	input := `{"a": [1,2,3,4]}`
	var data map[string]interface{}
	err := json.Unmarshal([]byte(input), &data)
	if err != nil {
		panic(err)
	}
	store := eval.NewStorageFromJSONObject(data)
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.cmdDump()
	expectOutput(t, buffer.String(), "map[a:[1 2 3 4]]\n")
}

func TestOneShotEmptyBufferOneExpr(t *testing.T) {
	store := newTestStorage()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("a[i].b.c[j] = 2")
	expectOutput(t, buffer.String(), "+---+---+\n| i | j |\n+---+---+\n| 0 | 1 |\n+---+---+\n")
	buffer.Reset()
	repl.OneShot("a[i].b.c[j] = \"deadbeef\"")
	expectOutput(t, buffer.String(), "false\n")
}

func TestOneShotEmptyBufferOneRule(t *testing.T) {
	store := newTestStorage()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("p[x] :- a[i] = x")
	expectOutput(t, buffer.String(), "defined\n")
}

func TestOneShotBufferedExpr(t *testing.T) {
	store := newTestStorage()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("a[i].b.c[j] = ")
	expectOutput(t, buffer.String(), "")
	repl.OneShot("2")
	expectOutput(t, buffer.String(), "")
	repl.OneShot("")
	expectOutput(t, buffer.String(), "+---+---+\n| i | j |\n+---+---+\n| 0 | 1 |\n+---+---+\n")
}

func TestOneShotBufferedRule(t *testing.T) {
	store := newTestStorage()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.OneShot("p[x] :- ")
	expectOutput(t, buffer.String(), "")
	repl.OneShot("a[i]")
	expectOutput(t, buffer.String(), "")
	repl.OneShot(" = ")
	expectOutput(t, buffer.String(), "")
	repl.OneShot("x")
	expectOutput(t, buffer.String(), "")
	repl.OneShot("")
	expectOutput(t, buffer.String(), "defined\n")
}

func TestBuildHeader(t *testing.T) {
	expr := ast.MustParseStatement(`[{"a": x, "b": a.b[y]}] = [{"a": 1, "b": 2}]`).(ast.Body)[0]
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

func newRepl(store *eval.Storage, buffer *bytes.Buffer) *Repl {
	runtime := &Runtime{Store: store}
	repl := NewRepl(runtime, "", buffer)
	return repl
}

func newTestStorage() *eval.Storage {
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
	return eval.NewStorageFromJSONObject(data)
}
