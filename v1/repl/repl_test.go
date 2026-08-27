// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package repl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/reeflective/readline"

	"github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
	"github.com/open-policy-agent/opa/v1/util"
)

func TestFunction(t *testing.T) {
	store := newTestStore()
	ctx := t.Context()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	mod1 := []byte(`package a.b.c
import rego.v1

foo(x) = y if {
	split(x, ".", y)
}

bar([x, y]) = z if {
	trim(x, y, z)
}
`)

	mod2 := []byte(`package a.b.d
import rego.v1

baz(_) = y if {
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

	// We import rego.v1 to ensure we're compatible with both v0 and v1 as default rego-version.
	if err := repl.OneShot(ctx, "import rego.v1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, "json"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "data.a.b.d.baz(null, x)"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	exp := util.MustUnmarshalJSON([]byte(`{"result": [{"expressions": [{"text":"data.a.b.d.baz(null, x)", "value": true, "location": {"row": 1, "col": 1}}], "bindings": {"x": "foo"}}]}`))
	result := util.MustUnmarshalJSON(buf.Bytes())
	if !reflect.DeepEqual(exp, result) {
		t.Fatalf("expected data.a.b.d.baz(x) to be %v, got %v", exp, result)
	}

	if err := repl.OneShot(ctx, "p(x) = y if { y = x+4 }"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	buf.Reset()
	if err := repl.OneShot(ctx, "data.repl.p(5, y)"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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

	if err := repl.OneShot(ctx, "f(1, x) = y if { y = x }"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "f(2, x) = y if { y = x*2 }"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	buf.Reset()
	if err := repl.OneShot(ctx, "data.repl.f(1, 2, y)"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	if err := repl.OneShot(ctx, "data.repl.f(2, 2, y)"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	ctx := t.Context()
	store := newTestStore()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	mod1 := []byte(`package a.b.c
import rego.v1

p = 1 if { true }
q = 2 if { true }
q = 3 if { false }`)

	mod2 := []byte(`package a.b.d
import rego.v1

r = 3 if { true }`)

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
	if err := repl.OneShot(ctx, "s = 4"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buf.Reset()

	result := repl.completeCandidates("")
	expected := []string{
		"data.a.b.c.p",
		"data.a.b.c.q",
		"data.a.b.d.r",
		"data.repl.s",
	}

	slices.Sort(result)
	slices.Sort(expected)

	if !slices.Equal(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}

	result = repl.completeCandidates("data.a.b")
	expected = []string{
		"data.a.b.c.p",
		"data.a.b.c.q",
		"data.a.b.d.r",
	}

	slices.Sort(result)
	slices.Sort(expected)

	if !slices.Equal(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}

	result = repl.completeCandidates("data.a.b.c.p[x]")
	expected = []string{}

	if !slices.Equal(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}

	if err := repl.OneShot(ctx, "import data.a.b.c.p as xyz"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "import data.a.b.d"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result = repl.completeCandidates("x")
	expected = []string{"xyz"}

	if !slices.Equal(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}
}

func TestREPLBracketedPasteTabNotCompleted(t *testing.T) {
	feed := func(seq string) (line string, completerCalls int) {
		store := newTestStore()
		var buf bytes.Buffer
		repl := newRepl(store, &buf)
		// Seed a completion candidate so a tab has something to complete to.
		if err := repl.OneShot(t.Context(), "import data.foo.bar as barbaz"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		shell := repl.newShell()
		shell.Completer = func(l []rune, c int) readline.Completions {
			completerCalls++
			return repl.complete(l, c)
		}

		// The line-reader renders terminal escapes to os.Stdout; redirect it to
		// keep test output clean. Safe because these tests do not run in parallel.
		devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			t.Fatalf("open devnull: %v", err)
		}
		defer devnull.Close()
		origStdout := os.Stdout
		os.Stdout = devnull
		defer func() { os.Stdout = origStdout }()

		shell.Keys.Feed(false, []rune(seq)...)
		got, err := shell.Readline()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return got, completerCalls
	}

	const (
		bpStart = "\x1b[200~"
		bpEnd   = "\x1b[201~"
		payload = "x = [\"a\",\tbarbaz]"
	)

	// Bracketed paste: the tab must survive and completion must not fire.
	if got, calls := feed(bpStart + payload + bpEnd + "\r"); got != payload || calls != 0 {
		t.Fatalf("bracketed paste corrupted (regression of #962): got %q (want %q), completer called %d time(s) (want 0)", got, payload, calls)
	}

	// Sanity check: a raw tab (no paste markers) does trigger completion, which
	// is the behaviour bracketed paste protects against. This guards the test
	// itself from silently passing due to a mis-wired completer.
	if _, calls := feed(payload + "\r"); calls == 0 {
		t.Fatal("expected a raw tab to invoke the completer; test setup is not exercising completion")
	}
}

func TestREPLLoopNonInteractiveInput(t *testing.T) {
	// runLoop feeds input as a non-terminal reader; it fails if Loop doesn't
	// return before the deadline (a regression would hang or spin here).
	runLoop := func(t *testing.T, input string) (string, error) {
		t.Helper()

		var buf bytes.Buffer
		repl := New(newTestStore(), "", &buf, "", 0, "").
			WithStderrWriter(&buf).
			WithConsoleInput(strings.NewReader(input))

		done := make(chan error, 1)
		go func() { done <- repl.Loop(t.Context()) }()

		select {
		case err := <-done:
			return buf.String(), err
		case <-time.After(10 * time.Second):
			t.Fatal("Loop did not return for non-interactive input; it should stop at EOF, not block or spin")
			return "", nil
		}
	}

	t.Run("evaluates piped lines and stops at EOF", func(t *testing.T) {
		out, err := runLoop(t, "1 + 1\n2 + 3\n")
		if err != nil {
			t.Fatalf("Loop returned error: %v", err)
		}
		if !strings.Contains(out, "2") || !strings.Contains(out, "5") {
			t.Fatalf("expected both piped queries to be evaluated, got: %q", out)
		}
	})

	t.Run("exit command stops the loop", func(t *testing.T) {
		// Anything after "exit" must not be evaluated.
		out, err := runLoop(t, "exit\n1 + 1\n")
		if err != nil {
			t.Fatalf("Loop returned error: %v", err)
		}
		if strings.Contains(out, "2") {
			t.Fatalf("expected input after exit to be ignored, got: %q", out)
		}
	})

	t.Run("empty input returns immediately at EOF", func(t *testing.T) {
		if _, err := runLoop(t, ""); err != nil {
			t.Fatalf("Loop returned error for empty input: %v", err)
		}
	})
}

func TestREPLHistoryMigratesLegacyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")

	// A legacy peterh/liner history file: one command per line, plain text.
	legacy := "a := 1\ndata.a.b.c\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	h, err := newREPLHistory(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Prior history must be preserved, in order, rather than silently dropped.
	if got := []string{lineAt(t, h, 0), lineAt(t, h, 1)}; !reflect.DeepEqual(got, []string{"a := 1", "data.a.b.c"}) {
		t.Fatalf("legacy history not preserved: got %v", got)
	}

	// The file must have been rewritten in the JSON-lines format so a later
	// launch reads it natively.
	bs, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimSpace(string(bs)), "\n") {
		var item replHistoryItem
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("line %d not migrated to JSON: %q (%v)", i, line, err)
		}
	}

	// Reloading the migrated file yields the same entries (no double-counting).
	h2, err := newREPLHistory(path)
	if err != nil {
		t.Fatalf("unexpected error reloading: %v", err)
	}
	if h2.Len() != 2 {
		t.Fatalf("expected 2 entries after reload, got %d", h2.Len())
	}
}

func TestREPLHistoryDoesNotPersistControlInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	h, err := newREPLHistory(path)
	if err != nil {
		t.Fatal(err)
	}

	// "exit" is control input, never a query, so it must not be recorded.
	if _, err := h.Write("exit"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Write("  EXIT  "); err != nil { // parsed case-insensitively
		t.Fatal(err)
	}
	if h.Len() != 0 {
		t.Fatalf("exit should not be persisted, got %d entries", h.Len())
	}

	// While paused (exit prompt on screen), confirmation answers are dropped.
	h.pause()
	if _, err := h.Write("y"); err != nil {
		t.Fatal(err)
	}
	if h.Len() != 0 {
		t.Fatalf("paused history should not persist, got %d entries", h.Len())
	}

	// Resuming restores normal persistence, and "y" is a legitimate query.
	h.resume()
	if _, err := h.Write("y"); err != nil {
		t.Fatal(err)
	}
	// Consecutive duplicates collapse, matching readline's file source.
	if _, err := h.Write("y"); err != nil {
		t.Fatal(err)
	}
	if h.Len() != 1 || lineAt(t, h, 0) != "y" {
		t.Fatalf("expected single \"y\" entry, got %d entries", h.Len())
	}
}

func lineAt(t *testing.T, h *replHistory, pos int) string {
	t.Helper()
	line, err := h.GetLine(pos)
	if err != nil {
		t.Fatalf("GetLine(%d): %v", pos, err)
	}
	return line
}

func TestDump(t *testing.T) {
	ctx := t.Context()
	input := `{"a": [1,2,3,4]}`
	var data map[string]any
	err := util.UnmarshalJSON([]byte(input), &data)
	if err != nil {
		panic(err)
	}
	store := inmem.NewFromObject(data)
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "dump"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "{\"a\":[1,2,3,4]}\n")
}

func TestDumpPath(t *testing.T) {
	ctx := t.Context()
	input := `{"a": [1,2,3,4]}`
	var data map[string]any
	err := util.UnmarshalJSON([]byte(input), &data)
	if err != nil {
		panic(err)
	}
	store := inmem.NewFromObject(data)
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	file := filepath.Join(t.TempDir(), "tmpfile")
	if err := repl.OneShot(ctx, "dump "+file); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if buffer.String() != "" {
		t.Errorf("Expected no output but got: %v", buffer.String())
	}

	bs, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("Expected file read to succeed but got: %v", err)
	}

	var result map[string]any
	if err := util.UnmarshalJSON(bs, &result); err != nil {
		t.Fatalf("Expected json unmarshal to succeed but got: %v", err)
	}

	if !reflect.DeepEqual(data, result) {
		t.Fatalf("Expected dumped json to equal %v but got: %v", data, result)
	}
}

func TestDumpPathCaseSensitive(t *testing.T) {
	ctx := t.Context()
	input := `{"a": [1,2,3,4]}`
	var data map[string]any
	err := util.UnmarshalJSON([]byte(input), &data)
	if err != nil {
		panic(err)
	}
	store := inmem.NewFromObject(data)
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	file := filepath.Join(t.TempDir(), "tmpfile")
	if err := repl.OneShot(ctx, "dump "+file); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if buffer.String() != "" {
		t.Errorf("Expected no output but got: %v", buffer.String())
	}

	bs, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("Expected file read to succeed but got: %v", err)
	}

	var result map[string]any
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

	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "help deadbeef"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "blah blah blah\n"

	if buffer.String() != expected {
		t.Fatalf("Unexpected output from help topic: %v", buffer.String())
	}
}

func TestHelpWithOPAVersionReport(t *testing.T) {
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	// empty report
	repl.SetOPAVersionReport(nil)
	if err := repl.OneShot(ctx, "help"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if strings.Contains(buffer.String(), "Version Info") {
		t.Fatalf("Unexpected output from help: \"%v\"", buffer.String())
	}

	buffer.Reset()

	repl.SetOPAVersionReport([][2]string{
		{"Latest Upstream Version", "0.19.2"},
		{"Download", "https://openpolicyagent.org/downloads/v0.19.2/opa_darwin_amd64"},
		{"Release Notes", "https://github.com/open-policy-agent/opa/releases/tag/v0.19.2"},
	})
	if err := repl.OneShot(ctx, "help"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	exp := `Latest Upstream Version : 0.19.2
Download                : https://openpolicyagent.org/downloads/v0.19.2/opa_darwin_amd64
Release Notes           : https://github.com/open-policy-agent/opa/releases/tag/v0.19.2`

	if !strings.Contains(buffer.String(), exp) {
		t.Fatalf("Expected output from help to contain: \"%v\" but got \"%v\"", exp, buffer.String())
	}
}

func TestShowDebug(t *testing.T) {
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "show debug"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

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

	if err := repl.OneShot(ctx, "trace"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "metrics"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "instrument"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "profile"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "show debug"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

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

// The rego.v1 import will be stripped from the output if the default rego-version is v1,
// so we need two flavours of this test: v0, and v1.
func TestShowV0(t *testing.T) {
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer).WithRegoVersion(ast.RegoV0)

	if err := repl.OneShot(ctx, `package repl_test`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	assertREPLText(t, buffer, "package repl_test\n")
	buffer.Reset()

	if err := repl.OneShot(ctx, "import input.xyz"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := `package repl_test

import input.xyz` + "\n"
	assertREPLText(t, buffer, expected)
	buffer.Reset()

	if err := repl.OneShot(ctx, "import data.foo as bar"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected = `package repl_test

import data.foo as bar
import input.xyz` + "\n"
	assertREPLText(t, buffer, expected)
	buffer.Reset()

	if err := repl.OneShot(ctx, `p[1] { true }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, `p[2] { true }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected = `package repl_test

import data.foo as bar
import input.xyz

p[1]

p[2]` + "\n"
	assertREPLText(t, buffer, expected)
	buffer.Reset()

	if err := repl.OneShot(ctx, "package abc"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	assertREPLText(t, buffer, "package abc\n")
	buffer.Reset()

	if err := repl.OneShot(ctx, "package repl_test"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	assertREPLText(t, buffer, expected)
	buffer.Reset()
}

func TestShowV1(t *testing.T) {
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer).WithRegoVersion(ast.RegoV1)

	if err := repl.OneShot(ctx, `package repl_test`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	assertREPLText(t, buffer, "package repl_test\n")
	buffer.Reset()

	if err := repl.OneShot(ctx, "import input.xyz"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := `package repl_test

import input.xyz` + "\n"
	assertREPLText(t, buffer, expected)
	buffer.Reset()

	if err := repl.OneShot(ctx, "import data.foo as bar"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected = `package repl_test

import data.foo as bar
import input.xyz` + "\n"
	assertREPLText(t, buffer, expected)
	buffer.Reset()

	if err := repl.OneShot(ctx, `p contains 1 if { true }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, `p contains 2 if { true }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected = `package repl_test

import data.foo as bar
import input.xyz

p contains 1

p contains 2` + "\n"
	assertREPLText(t, buffer, expected)
	buffer.Reset()

	if err := repl.OneShot(ctx, "package abc"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	assertREPLText(t, buffer, "package abc\n")
	buffer.Reset()

	if err := repl.OneShot(ctx, "package repl_test"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	assertREPLText(t, buffer, expected)
	buffer.Reset()
}

func TestTypes(t *testing.T) {
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	if err := repl.OneShot(ctx, "types"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// We import rego.v1 to ensure we're compatible with both v0 and v1 as default rego-version.
	if err := repl.OneShot(ctx, "import rego.v1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, `p[x] = y if { x := "a"; y := 1 }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `p[x]`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

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
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	if err := repl.OneShot(ctx, "xs = [1,2,3]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()

	err := repl.OneShot(ctx, "unknown input")
	if err != nil {
		t.Fatal("Unexpected command error:", err)
	}

	if err := repl.OneShot(ctx, "data.repl.xs[i] = x; input.x = x"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	output := strings.TrimSpace(buffer.String())
	expected := strings.TrimSpace(`
┌─────────┬─────────────┐
│ Query 1 │ input.x = 1 │
│         │ i = 0       │
│         │ x = 1       │
├─────────┼─────────────┤
│ Query 2 │ input.x = 2 │
│         │ i = 1       │
│         │ x = 2       │
├─────────┼─────────────┤
│ Query 3 │ input.x = 3 │
│         │ i = 2       │
│         │ x = 3       │
└─────────┴─────────────┘
`)

	if output != expected {
		t.Fatalf("Unexpected output. Expected:\n\n%v\n\nGot:\n\n%v", expected, output)
	}
}
func TestUnknownMetrics(t *testing.T) {
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	if err := repl.OneShot(ctx, "xs = [1,2,3]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()

	err := repl.OneShot(ctx, "unknown input")
	if err != nil {
		t.Fatal("Unexpected command error:", err)
	}

	if err := repl.OneShot(ctx, "metrics"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, "data.repl.xs[i] = x; input.x = x"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	output := strings.TrimSpace(buffer.String())
	expected := strings.TrimSpace(`
┌─────────┬─────────────┐
│ Query 1 │ input.x = 1 │
│         │ i = 0       │
│         │ x = 1       │
├─────────┼─────────────┤
│ Query 2 │ input.x = 2 │
│         │ i = 1       │
│         │ x = 2       │
├─────────┼─────────────┤
│ Query 3 │ input.x = 3 │
│         │ i = 2       │
│         │ x = 3       │
└─────────┴─────────────┘
`)

	if !strings.HasPrefix(output, expected) {
		t.Fatalf("Unexpected partial eval results. Expected:\n\n%v\n\nGot:\n\n%v", expected, output)
	}

	if !strings.Contains(output, "timer_rego_partial_eval_ns") {
		t.Fatal("Expected timer_rego_partial_eval_ns but got:\n\n", output)
	}
}

func TestUnknownJSON(t *testing.T) {
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	if err := repl.OneShot(ctx, "xs = [1,2,3]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()

	err := repl.OneShot(ctx, "unknown input")
	if err != nil {
		t.Fatal("Unexpected command error:", err)
	}

	if err := repl.OneShot(ctx, "json"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "data.repl.xs[i] = x; input.x = x"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result presentation.Output

	if err := json.NewDecoder(&buffer).Decode(&result); err != nil {
		t.Fatal(err)
	}

	if len(result.Partial.Queries) != 3 {
		t.Fatalf("Expected exactly 3 queries in partial evaluation output but got: %v", result)
	}
}

func TestUnknownInvalid(t *testing.T) {
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	err := repl.OneShot(ctx, "unknown x-1")
	if err == nil || !strings.Contains(err.Error(), "usage: unknown <input/data reference>") {
		t.Fatal("expected error from setting bad unknown but got:", err)
	}

	// Ensure that partial evaluation has not been enabled.
	buffer.Reset()
	if err := repl.OneShot(ctx, "1+2"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result := strings.TrimSpace(buffer.String())
	if result != "3" {
		t.Fatal("want true but got:", result)
	}
}

func TestUnset(t *testing.T) {
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	var err error

	// We import rego.v1 to ensure we're compatible with both v0 and v1 as default rego-version.
	if err := repl.OneShot(ctx, "import rego.v1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, "magic = 23"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "p = 3.14"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "unset p"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	err = repl.OneShot(ctx, "p")

	if _, ok := err.(ast.Errors); !ok {
		t.Fatalf("Expected AST error but got: %v", err)
	}

	buffer.Reset()
	if err := repl.OneShot(ctx, "p = 3.14"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `p = 3 if { false }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "unset p"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

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
	if err := repl.OneShot(ctx, "p(x) = y if { y = x }"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "unset p"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	err = repl.OneShot(ctx, "data.repl.p(1, 2)")
	if err == nil || err.Error() != `1 error occurred: 1:1: rego_type_error: undefined function data.repl.p` {
		t.Fatalf("Expected eval error (undefined built-in) but got err: '%v'", err)
	}

	buffer.Reset()
	if err := repl.OneShot(ctx, "p(1, x) = y if { y = x }"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "p(2, x) = y if { y = x+1 }"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "unset p"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	err = repl.OneShot(ctx, "data.repl.p(1, 2, 3)")
	if err == nil || err.Error() != `1 error occurred: 1:1: rego_type_error: undefined function data.repl.p` {
		t.Fatalf("Expected eval error (undefined built-in) but got err: '%v'", err)
	}

	buffer.Reset()
	if err := repl.OneShot(ctx, `unset q`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if buffer.String() != "warning: no matching rules in current module\n" {
		t.Fatalf("Expected unset error for missing rule but got: %v", buffer.String())
	}

	buffer.Reset()
	if err := repl.OneShot(ctx, `unset q`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if buffer.String() != "warning: no matching rules in current module\n" {
		t.Fatalf("Expected unset error for missing function but got: %v", buffer.String())
	}

	buffer.Reset()
	if err := repl.OneShot(ctx, `magic`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if buffer.String() != "23\n" {
		t.Fatalf("Expected magic to be defined but got: %v", buffer.String())
	}

	buffer.Reset()
	if err := repl.OneShot(ctx, `package data.other`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `unset magic`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if buffer.String() != "warning: no matching rules in current module\n" {
		t.Fatalf("Expected unset error for bad syntax but got: %v", buffer.String())
	}
}

func TestUnsetInputDocument(t *testing.T) {
	// input is only allowed to be overridden in rego v0, so we only assert the following when that's the active version.

	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer).WithRegoVersion(ast.RegoV0)

	if err := repl.OneShot(ctx, `input = {}`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, `unset input`); err != nil {
		t.Fatalf("Expected unset to succeed for input: %v", err)
	}

	buffer.Reset()
	if err := repl.OneShot(ctx, `not input`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if buffer.String() != "true\n" {
		t.Fatalf("Expected unset input to remove input document: %v", buffer.String())
	}
}

func TestOneShotEmptyBufferOneExpr(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "data.a[i].b.c[j] = 2"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), `┌───┬───┐
│ i │ j │
├───┼───┤
│ 0 │ 1 │
└───┴───┘
`)
	buffer.Reset()
	if err := repl.OneShot(ctx, "data.a[i].b.c[j] = \"deadbeef\""); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "undefined\n")
}

func TestOneShotEmptyBufferOneRule(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	// We import rego.v1 to ensure we're compatible with both v0 and v1 as default rego-version.
	if err := repl.OneShot(ctx, "import rego.v1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, `p contains x if { data.a[i] = x }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "Rule 'p' defined in package repl. Type 'show' to see rules.\n")
}

func TestOneShotRefHeadRulePrinted(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.regoVersion = ast.RegoV1

	if err := repl.OneShot(ctx, `foo.bar.baz if { true }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "Rule 'foo.bar.baz' defined in package repl. Type 'show' to see rules.\n")
}

// Ref head rules can be defined interactively, i.e. without the `if` keyword
// and a rule body, see https://github.com/open-policy-agent/opa/issues/5498
func TestOneShotRefHeadRuleDefinition(t *testing.T) {
	tests := []struct {
		note  string
		stmts []string
		exp   []string
	}{
		{
			note:  "number key",
			stmts: []string{`a[0] := 1`, `a`},
			exp: []string{
				"Rule 'a[0]' defined in package repl. Type 'show' to see rules.\n",
				"{\n  \"0\": 1\n}\n",
			},
		},
		{
			note:  "string key",
			stmts: []string{`a["foo"] := "bar"`, `a`},
			exp: []string{
				"Rule 'a.foo' defined in package repl. Type 'show' to see rules.\n",
				"{\n  \"foo\": \"bar\"\n}\n",
			},
		},
		{
			note:  "distinct keys are kept",
			stmts: []string{`a[0] := 1`, `a[1] := 2`, `a`},
			exp: []string{
				"Rule 'a[0]' defined in package repl. Type 'show' to see rules.\n",
				"Rule 'a[1]' defined in package repl. Type 'show' to see rules.\n",
				"{\n  \"0\": 1,\n  \"1\": 2\n}\n",
			},
		},
		{
			note:  "same key is re-defined",
			stmts: []string{`a[0] := 1`, `a[1] := 2`, `a[0] := 3`, `a`},
			exp: []string{
				"Rule 'a[0]' defined in package repl. Type 'show' to see rules.\n",
				"Rule 'a[1]' defined in package repl. Type 'show' to see rules.\n",
				"Rule 'a[0]' re-defined in package repl. Type 'show' to see rules.\n",
				"{\n  \"0\": 3,\n  \"1\": 2\n}\n",
			},
		},
		{
			note:  "complete rule replaces keys",
			stmts: []string{`a[0] := 1`, `a := 2`, `a`},
			exp: []string{
				"Rule 'a[0]' defined in package repl. Type 'show' to see rules.\n",
				"Rule 'a' re-defined in package repl. Type 'show' to see rules.\n",
				"2\n",
			},
		},
		{
			note:  "dotted ref",
			stmts: []string{`p.q.r := 1`, `p.q.s := 2`, `p`},
			exp: []string{
				"Rule 'p.q.r' defined in package repl. Type 'show' to see rules.\n",
				"Rule 'p.q.s' defined in package repl. Type 'show' to see rules.\n",
				"{\n  \"q\": {\n    \"r\": 1,\n    \"s\": 2\n  }\n}\n",
			},
		},
		{
			note:  "assignment to var key is unsafe",
			stmts: []string{`a[i] := 1`},
			exp:   []string{""},
		},
		{
			note:  "eq statement defines rule",
			stmts: []string{`p.q.r = 1`, `p.q.r`},
			exp: []string{
				"Rule 'p.q.r' defined in package repl. Type 'show' to see rules.\n",
				"1\n",
			},
		},
		{
			// The rule isn't re-defined, the statement is a query about the
			// existing document.
			note:  "eq statement about defined rule is a query",
			stmts: []string{`p.q.r := 1`, `p.q.r = 1`, `p.q.r = 2`, `p.q[i] = 1`},
			exp: []string{
				"Rule 'p.q.r' defined in package repl. Type 'show' to see rules.\n",
				"true\n",
				"undefined\n",
				"┌─────┐\n│  i  │\n├─────┤\n│ \"r\" │\n└─────┘\n",
			},
		},
		{
			note:  "data ref is a query",
			stmts: []string{`data.foo.bar = 1`, `show`},
			exp: []string{
				"undefined\n",
				"no rules defined\n",
			},
		},
		{
			note:  "input ref is a query",
			stmts: []string{`input.foo.bar = 1`, `show`},
			exp: []string{
				"undefined\n",
				"no rules defined\n",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := t.Context()
			var buffer bytes.Buffer
			repl := newRepl(inmem.New(), &buffer)

			for i, stmt := range tc.stmts {
				buffer.Reset()
				err := repl.OneShot(ctx, stmt)
				if tc.exp[i] == "" {
					if err == nil {
						t.Fatalf("%q: expected error but got output: %q", stmt, buffer.String())
					}
					continue
				}
				if err != nil {
					t.Fatalf("%q: unexpected error: %v", stmt, err)
				}
				if act := buffer.String(); act != tc.exp[i] {
					t.Fatalf("%q: expected output %q but got %q", stmt, tc.exp[i], act)
				}
			}
		})
	}
}

// Ref head rules are identified by their ref, e.g. "unset a[0]" and
// "unset foo.bar.baz".
func TestUnsetRefHeadRule(t *testing.T) {
	ctx := t.Context()
	var buffer bytes.Buffer
	repl := newRepl(inmem.New(), &buffer)

	for _, stmt := range []string{`a[0] := 1`, `a[1] := 2`, `foo.bar.baz if true`} {
		if err := repl.OneShot(ctx, stmt); err != nil {
			t.Fatalf("%q: unexpected error: %v", stmt, err)
		}
	}

	// Unsetting a key leaves the other keys of the document in place.
	buffer.Reset()
	if err := repl.OneShot(ctx, `unset a[0]`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `a`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "{\n  \"1\": 2\n}\n")

	// Unsetting a prefix of the ref removes all rules under it.
	buffer.Reset()
	if err := repl.OneShot(ctx, `unset a`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `unset foo.bar.baz`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `show`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "package repl\n")

	buffer.Reset()
	if err := repl.OneShot(ctx, `unset foo.bar.baz`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "warning: no matching rules in current module\n")

	if err := repl.OneShot(ctx, `unset a[x]`); err == nil {
		t.Fatal("Expected error for non-ground ref")
	}
}

func TestOneShotBufferedExpr(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "data.a[i].b.c[j] = "); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "")
	if err := repl.OneShot(ctx, "2"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "")
	if err := repl.OneShot(ctx, ""); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), `┌───┬───┐
│ i │ j │
├───┼───┤
│ 0 │ 1 │
└───┴───┘
`)
}

func TestOneShotBufferedRule(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	// We import rego.v1 to ensure we're compatible with both v0 and v1 as default rego-version.
	if err := repl.OneShot(ctx, "import rego.v1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, "p contains x if { "); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "")
	if err := repl.OneShot(ctx, "data.a[i].b.c[1]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "")
	if err := repl.OneShot(ctx, " = "); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "")
	if err := repl.OneShot(ctx, "x"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "")
	if err := repl.OneShot(ctx, "}"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "")
	if err := repl.OneShot(ctx, ""); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "Rule 'p' defined in package repl. Type 'show' to see rules.\n")
	buffer.Reset()
	if err := repl.OneShot(ctx, "p[2]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectOutput(t, buffer.String(), "2\n")
}

func TestOneShotJSON(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.outputFormat = "json"
	if err := repl.OneShot(ctx, "data.a[i] = x"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	var expected any
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

	var result any

	if err := util.UnmarshalJSON(buffer.Bytes(), &result); err != nil {
		t.Errorf("Unexpected output format: %v", err)
		return
	}

	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Expected %v but got: %v", expected, buffer.String())
	}
}

func TestOneShot_DefaultRegoVersion(t *testing.T) {
	type action struct {
		line      string
		expOutput string
		expErrs   []string
	}

	tests := []struct {
		note    string
		actions []action
	}{
		{
			note: "v1 keywords used",
			actions: []action{
				{
					line:      "a contains 2 if { true }",
					expOutput: "Rule 'a' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note: "v1 keywords not used",
			actions: []action{
				{
					line: "a[2] { true }",
					expErrs: []string{
						"rego_parse_error: `if` keyword is required before rule body",
						"rego_parse_error: `contains` keyword is required for partial set rules",
					},
				},
			},
		},
		{
			note: "v1 keywords imported",
			actions: []action{
				{
					line: "import future.keywords",
				},
				{
					line:      "a contains 2 if { true }",
					expOutput: "Rule 'a' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note: "v1 compile-time violation",
			actions: []action{
				{
					line: "b if { data := 1; data == 1 }",
					expErrs: []string{
						"rego_compile_error: variables must not shadow data (use a different variable name)",
					},
				},
			},
		},
		{
			note: "rego.v1 imported",
			actions: []action{
				{
					line: "import rego.v1",
				},
				{
					line:      "a contains 2 if { true }",
					expOutput: "Rule 'a' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note: "v1 keywords",
			actions: []action{
				{
					line:      "a contains 2 if { true }",
					expOutput: "Rule 'a' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := t.Context()
			store := newTestStore()
			var buffer bytes.Buffer
			repl := newRepl(store, &buffer)

			for _, action := range tc.actions {
				err := repl.OneShot(ctx, action.line)

				if len(action.expErrs) != 0 {
					if err == nil {
						t.Fatalf("Expected error but got: %s", buffer.String())
					}

					for _, e := range action.expErrs {
						if !strings.Contains(err.Error(), e) {
							t.Fatalf("Expected error to contain:\n\n%q\n\nbut got:\n\n%v", e, err)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}
					expectOutput(t, buffer.String(), action.expOutput)
				}
			}
		})
	}
}

func TestOneShot_RegoVersion(t *testing.T) {
	type action struct {
		line      string
		expOutput string
		expErrs   []string
	}
	tests := []struct {
		note        string
		actions     []action
		regoVersion ast.RegoVersion
	}{
		{
			note:        "v0, keywords used",
			regoVersion: ast.RegoV0,
			actions: []action{
				{
					line:    "a contains 2 if { true }",
					expErrs: []string{"rego_unsafe_var_error: var a is unsafe"},
				},
			},
		},
		{
			note:        "v0, keywords not used",
			regoVersion: ast.RegoV0,
			actions: []action{
				{
					line:      "a[2] { true }",
					expOutput: "Rule 'a' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note:        "v0, keywords imported",
			regoVersion: ast.RegoV0,
			actions: []action{
				{
					line: "import future.keywords",
				},
				{
					line:      "a contains 2 if { true }",
					expOutput: "Rule 'a' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note:        "v0, rego.v1 imported",
			regoVersion: ast.RegoV0,
			actions: []action{
				{
					line: "import rego.v1",
				},
				{
					line:      "a contains 2 if { true }",
					expOutput: "Rule 'a' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note:        "v0, v1 compile-time violation",
			regoVersion: ast.RegoV0,
			actions: []action{
				{
					line:      "b { data := 1; data == 1 }",
					expOutput: "Rule 'b' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note:        "v1, keywords not used",
			regoVersion: ast.RegoV1,
			actions: []action{
				{
					line: "a[2] { true }",
					expErrs: []string{
						"rego_parse_error: `if` keyword is required before rule body",
						"rego_parse_error: `contains` keyword is required for partial set rules",
					},
				},
			},
		},
		{
			note:        "v1, keywords used, not imported",
			regoVersion: ast.RegoV1,
			actions: []action{
				{
					line:      "a contains 2 if { true }",
					expOutput: "Rule 'a' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note:        "v1, keywords used, keywords imported",
			regoVersion: ast.RegoV1,
			actions: []action{
				{
					line: "import future.keywords",
				},
				{
					line:      "a contains 2 if { true }",
					expOutput: "Rule 'a' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note:        "v1, keywords used, rego.v1 imported",
			regoVersion: ast.RegoV1,
			actions: []action{
				{
					line: "import rego.v1",
				},
				{
					line:      "a contains 2 if { true }",
					expOutput: "Rule 'a' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note:        "v1 compile-time violation",
			regoVersion: ast.RegoV1,
			actions: []action{
				{
					line: "b if { data := 1; data == 1 }",
					expErrs: []string{
						"rego_compile_error: variables must not shadow data (use a different variable name)",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := t.Context()
			store := newTestStore()
			var buffer bytes.Buffer
			repl := newRepl(store, &buffer).
				WithRegoVersion(tc.regoVersion)

			for _, action := range tc.actions {
				err := repl.OneShot(ctx, action.line)

				if len(action.expErrs) != 0 {
					if err == nil {
						t.Fatalf("Expected error but got: %s", buffer.String())
					}

					for _, e := range action.expErrs {
						if !strings.Contains(err.Error(), e) {
							t.Fatalf("Expected error to contain:\n\n%q\n\nbut got:\n\n%v", e, err)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}
					expectOutput(t, buffer.String(), action.expOutput)
				}
			}
		})
	}
}

func TestStoredModule_RegoVersion(t *testing.T) {
	tests := []struct {
		note        string
		regoVersion ast.RegoVersion
		module      string
		line        string
		expOutput   string
		expErrs     []string
	}{
		{
			note:        "v0 keywords not used",
			regoVersion: ast.RegoV0,
			module: `package example
p[2] { 1 == 1 }`,
			line:      "data.example.p",
			expOutput: "[\n  2\n]\n",
		},
		{
			note:        "v0, keywords not imported but used",
			regoVersion: ast.RegoV0,
			module: `package example
p contains 2 if { 1 == 1 }`,
			line: "data.example.p",
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note:        "v0, keywords imported",
			regoVersion: ast.RegoV0,
			module: `package example
import future.keywords
p contains 2 if { 1 == 1 }`,
			line:      "data.example.p",
			expOutput: "[\n  2\n]\n",
		},
		{
			note:        "v0, rego.v1 imported",
			regoVersion: ast.RegoV0,
			module: `package example
import rego.v1
p contains 2 if { 1 == 1 }`,
			line:      "data.example.p",
			expOutput: "[\n  2\n]\n",
		},
		{
			note:        "v0, v1 compile-time violation",
			regoVersion: ast.RegoV0,
			module: `package example
p { data := 1; data == 1 }`,
			line:      "data.example.p",
			expOutput: "true\n",
		},
		{
			note:        "v1, keywords not used",
			regoVersion: ast.RegoV1,
			module: `package example
p[2] { 1 == 1 }`,
			line: "data.example.p",
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:        "v1, keywords not imported",
			regoVersion: ast.RegoV1,
			module: `package example
p contains 2 if { 1 == 1 }`,
			line:      "data.example.p",
			expOutput: "[\n  2\n]\n",
		},
		{
			note:        "v1, keywords imported",
			regoVersion: ast.RegoV1,
			module: `package example
import future.keywords
p contains 2 if { 1 == 1 }`,
			line:      "data.example.p",
			expOutput: "[\n  2\n]\n",
		},
		{
			note:        "v1, rego.v1 imported",
			regoVersion: ast.RegoV1,
			module: `package example
import rego.v1
p contains 2 if { 1 == 1 }`,
			line:      "data.example.p",
			expOutput: "[\n  2\n]\n",
		},
		{
			note:        "v1, v1 compile-time violation",
			regoVersion: ast.RegoV1,
			module: `package example
p if { data := 1; data == 1 }`,
			line: "data.example.p",
			expErrs: []string{
				"rego_compile_error: variables must not shadow data (use a different variable name)",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := t.Context()
			store := newTestStore()

			txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
			if err := store.UpsertPolicy(ctx, txn, "policy", []byte(tc.module)); err != nil {
				t.Fatalf("Unexpected error upserting policy: %v", err)
			}

			if err := store.Commit(ctx, txn); err != nil {
				t.Fatalf("Unexpected error committing store transaction: %v", err)
			}

			var buffer bytes.Buffer
			repl := newRepl(store, &buffer).
				WithRegoVersion(tc.regoVersion)

			err := repl.OneShot(ctx, tc.line)

			if len(tc.expErrs) != 0 {
				if err == nil {
					t.Fatalf("Expected error but got: %s", buffer.String())
				}

				for _, e := range tc.expErrs {
					if !strings.Contains(err.Error(), e) {
						t.Fatalf("Expected error to contain:\n\n%q\n\nbut got:\n\n%v", e, err)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				expectOutput(t, buffer.String(), tc.expOutput)
			}
		})
	}
}

func TestEvalData(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	testMod := []byte(`package ex
import rego.v1

p = [1, 2, 3] if { true }`)

	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	if err := store.UpsertPolicy(ctx, txn, "test", testMod); err != nil {
		panic(err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	if err := repl.OneShot(ctx, "data"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

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
	data := result.(map[string]any)
	delete(data, "repl")

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected:\n%v\n\nGot:\n%v", expected, result)
	}
}

func TestEvalFalse(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "false"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result := buffer.String()
	if result != "false\n" {
		t.Errorf("Expected result to be false but got: %v", result)
	}
}

func TestEvalConstantRule(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "pi = 3.14"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result := buffer.String()
	if result != "Rule 'pi' defined in package repl. Type 'show' to see rules.\n" {
		t.Errorf("Expected rule to be defined but got: %v", result)
		return
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "pi"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result = buffer.String()
	expected := "3.14\n"
	if result != expected {
		t.Errorf("Expected pi to evaluate to 3.14 but got: %v", result)
		return
	}
	buffer.Reset()
	err := repl.OneShot(ctx, "pi.deadbeef")
	result = buffer.String()
	expected = "undefined ref: data.repl.pi.deadbeef"
	if err == nil {
		t.Fatalf("Expected OneShot to return error %v but got: %v", expected, err)
	}
	if result != "" || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Expected pi.deadbeef to fail/error but got:\nresult: %q\nerr: %v", result, err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "pi > 3"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result = buffer.String()
	if result != "true\n" {
		t.Errorf("Expected pi > 3 to be true but got: %v", result)
		return
	}
}

func TestEvalBooleanFlags(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "flags = [true, true]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "flags[_]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expected := strings.TrimSpace(`
Rule 'flags' defined in package repl. Type 'show' to see rules.
┌──────────┐
│ flags[_] │
├──────────┤
│ true     │
│ true     │
└──────────┘`)
	result := strings.TrimSpace(buffer.String())
	if result != expected {
		t.Errorf("Expected a single column with boolean output but got:\n%v", result)
	}
	buffer.Reset()

	if err := repl.OneShot(ctx, `flags2 = [true, "x", 1]`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "flags2[_]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expected = strings.TrimSpace(`
Rule 'flags2' defined in package repl. Type 'show' to see rules.
┌───────────┐
│ flags2[_] │
├───────────┤
│ true      │
│ "x"       │
│ 1         │
└───────────┘`)
	result = strings.TrimSpace(buffer.String())
	if result != expected {
		t.Errorf("Expected a single column with boolean output but got:\n%v", result)
	}
}

func TestEvalConstantRuleDefaultRootDoc(t *testing.T) {
	// The 'input' document may only be shadowed in rego v0.

	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer).
		WithRegoVersion(ast.RegoV0)
	if err := repl.OneShot(ctx, "input = 1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "input = 2"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	assertREPLText(t, buffer, "undefined\n")
	buffer.Reset()
	if err := repl.OneShot(ctx, "input = 1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	assertREPLText(t, buffer, "true\n")
}

func TestEvalConstantRuleAssignment(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer

	defined := "Rule 'x' defined in package repl. Type 'show' to see rules.\n"
	redefined := "Rule 'x' re-defined in package repl. Type 'show' to see rules.\n"

	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "x = 1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	assertREPLText(t, buffer, defined)
	buffer.Reset()
	if err := repl.OneShot(ctx, "x := 2"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	assertREPLText(t, buffer, redefined)
	buffer.Reset()

	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	assertREPLText(t, buffer, `package repl

x := 2
`)
	buffer.Reset()

	if err := repl.OneShot(ctx, "x := 3"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	assertREPLText(t, buffer, redefined)
	buffer.Reset()
	if err := repl.OneShot(ctx, "x"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result := buffer.String()
	if result != "3\n" {
		t.Fatalf("Expected 3 but got: %v", result)
	}

	buffer.Reset()
	if err := repl.OneShot(ctx, "x = 3"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result = buffer.String()
	if result != "true\n" {
		t.Fatalf("Expected true but got: %v", result)
	}

	buffer.Reset()
	err := repl.OneShot(ctx, "assign()")
	if err == nil || !strings.Contains(err.Error(), "rego_type_error: assign: arity mismatch\n\thave: ()\n\twant: (any, any)") {
		t.Fatal("Expected type check error but got:", err)
	}
}
func TestEvalConstantRuleAssignmentInputDocument(t *testing.T) {
	// input is only allowed to be overridden in rego v0, so we only assert the following when that's the active version.

	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer).
		WithRegoVersion(ast.RegoV0)

	definedInput := "Rule 'input' defined in package repl. Type 'show' to see rules.\n"
	redefinedInput := "Rule 'input' re-defined in package repl. Type 'show' to see rules.\n"

	buffer.Reset()
	if err := repl.OneShot(ctx, "input = 0"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	assertREPLText(t, buffer, definedInput)
	buffer.Reset()
	if err := repl.OneShot(ctx, "input := 1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	assertREPLText(t, buffer, redefinedInput)
	buffer.Reset()
	if err := repl.OneShot(ctx, "input"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result := buffer.String()
	if result != "1\n" {
		t.Fatalf("Expected 1 but got: %v", result)
	}
}

func TestEvalSingleTermMultiValue(t *testing.T) {
	ctx := t.Context()
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

	var expected any
	if err := util.UnmarshalJSON([]byte(input), &expected); err != nil {
		panic(err)
	}

	if err := repl.OneShot(ctx, "data.a[i].b.c[_]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	var result any
	if err := util.UnmarshalJSON(buffer.Bytes(), &result); err != nil {
		t.Errorf("Expected valid JSON document: %v: %v", err, buffer.String())
		return
	}

	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Expected %v but got: %v", expected, buffer.String())
		return
	}

	buffer.Reset()

	// We import rego.v1 to ensure we're compatible with both v0 and v1 as default rego-version.
	if err := repl.OneShot(ctx, "import rego.v1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, "data.deadbeef[x]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	s := buffer.String()
	if s != "{}\n" {
		t.Errorf("Expected undefined from reference but got: %v", s)
		return
	}

	buffer.Reset()

	if err := repl.OneShot(ctx, `p contains x if { a = [1, 2, 3, 4]; a[_] = x }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "p[x]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

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
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.outputFormat = "json"

	// We import rego.v1 to ensure we're compatible with both v0 and v1 as default rego-version.
	if err := repl.OneShot(ctx, "import rego.v1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, `p contains 1 if { true }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `p contains 2 if { true }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `q = {3, 4} if { true }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `r = [x, y] if { x = {5, 6}; y = [7, 8] }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, "p[x]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	if err := repl.OneShot(ctx, "q[x]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	if err := repl.OneShot(ctx, "r[_][x]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	// We import rego.v1 to ensure we're compatible with both v0 and v1 as default rego-version.
	if err := repl.OneShot(ctx, "import rego.v1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	err := repl.OneShot(ctx, `p contains x if { true }`)
	expected := "x is unsafe"
	if err == nil {
		t.Fatalf("Expected OneShot to return error %v but got: %v", expected, err)
	}
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("Expected error to contain %v but got: %v (err: %v)", expected, buffer.String(), err)
		return
	}
	buffer.Reset()
	err = repl.OneShot(ctx, `p = true if { true }`)
	result := buffer.String()
	if err != nil || result != "Rule 'p' defined in package repl. Type 'show' to see rules.\n" {
		t.Errorf("Expected valid rule to compile (because state should be unaffected) but got: %v (err: %v)", result, err)
	}
}

func TestEvalBodyCompileError(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.outputFormat = "json"
	err := repl.OneShot(ctx, `x = 1; y > x`)
	if _, ok := err.(ast.Errors); !ok {
		t.Fatalf("Expected error message in output but got`: %v", buffer.String())
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, `x = 1; y = 2; y > x`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "data.a[_].b.c[_] = x"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expected := strings.TrimSpace(`
┌───────┐
│   x   │
├───────┤
│ true  │
│ 2     │
│ false │
│ false │
│ true  │
│ 1     │
└───────┘`)
	result := strings.TrimSpace(buffer.String())
	if result != expected {
		t.Errorf("Expected only a single column of output but got:\n%v", result)
	}

}

func TestEvalBodyInput(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer).
		WithRegoVersion(ast.RegoV0)

	if err := repl.OneShot(ctx, `package repl`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `input["foo.bar"] = "hello" { true }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `input["baz"] = data.a[0].b.c[2] { true }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `package test`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "import input.baz"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `p = true { input["foo.bar"] = "hello"; baz = false }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "p"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result := buffer.String()
	if result != "true\n" {
		t.Fatalf("expected true but got: %v", result)
	}
}

func TestEvalBodyInputComplete(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer).
		WithRegoVersion(ast.RegoV0)

	// Test that input can be defined completely:
	// https://github.com/open-policy-agent/opa/issues/231
	if err := repl.OneShot(ctx, `package repl`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `input = 1`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, `input`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result := buffer.String()
	if result != "1\n" {
		t.Fatalf("Expected 1 but got: %v", result)
	}

	buffer.Reset()

	// Test that input is as expected
	if err := repl.OneShot(ctx, `package ex1`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `x = input`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, `x`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result = buffer.String()
	if result != "1\n" {
		t.Fatalf("Expected 1 but got: %v", result)
	}

	// Test that local input replaces other inputs
	if err := repl.OneShot(ctx, `package ex2`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `input = 2`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, `input`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result = buffer.String()

	if result != "2\n" {
		t.Fatalf("Expected 2 but got: %v", result)
	}

	buffer.Reset()

	// Test that original input is intact
	if err := repl.OneShot(ctx, `package ex3`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `input`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result = buffer.String()

	if result != "1\n" {
		t.Fatalf("Expected 1 but got: %v", result)
	}

	// Test that deferencing undefined input results in undefined
	buffer.Reset()

	repl = newRepl(store, &buffer)
	if err := repl.OneShot(ctx, `input.p`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result = buffer.String()
	if result != "undefined\n" {
		t.Fatalf("Expected undefined but got: %v", result)
	}

	buffer.Reset()
	if err := repl.OneShot(ctx, `input.p = false`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result = buffer.String()
	if result != "undefined\n" {
		t.Fatalf("Expected undefined but got: %v", result)
	}

}

func TestEvalBodyWith(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	// We import rego.v1 to ensure we're compatible with both v0 and v1 as default rego-version.
	if err := repl.OneShot(ctx, "import rego.v1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, `p = true if { input.foo = "bar" }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "p"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if buffer.String() != "undefined\n" {
		t.Fatalf("Expected undefined but got: %v", buffer.String())
	}

	buffer.Reset()

	if err := repl.OneShot(ctx, `p with input.foo as "bar"`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result := buffer.String()
	expected := "true\n"

	if result != expected {
		t.Fatalf("Expected true but got: %v", result)
	}
}

func TestEvalBodyRewrittenBuiltin(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "json"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// We import rego.v1 to ensure we're compatible with both v0 and v1 as default rego-version.
	if err := repl.OneShot(ctx, "import rego.v1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, `p contains x if { a[x]; a = [1,2,3,4] }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "p[x] > 1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "json"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `i = 1`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `data.a[0].b.c[i]`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	if err := repl.OneShot(ctx, "p = {1,2,3}"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "p"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	if err := repl.OneShot(ctx, "p[x]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "json"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "some x; x = 1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "import data.a"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(buffer.Bytes()) != 0 {
		t.Errorf("Expected no output but got: %v", buffer.String())
		return
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "a[0].b.c[0] = true"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result := buffer.String()
	expected := "true\n"
	if result != expected {
		t.Errorf("Expected expression to evaluate successfully but got: %v", result)
		return
	}

	// https://github.com/open-policy-agent/opa/issues/158 - re-run query to
	// make sure import is not lost
	buffer.Reset()
	if err := repl.OneShot(ctx, "a[0].b.c[0] = true"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result = buffer.String()
	expected = "true\n"
	if result != expected {
		t.Fatalf("Expected expression to evaluate successfully but got: %v", result)
	}
}

func TestEvalImportFutureKeywords(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer).
		WithRegoVersion(ast.RegoV0)

	err := repl.OneShot(ctx, "1 in [1]")
	if err == nil {
		t.Fatal("Expected error got nil")
	}
	expected := "rego_unsafe_var_error: var in is unsafe (hint: `import future.keywords.in` to import a future keyword)"
	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("Expected error to contain %q but got: %v", expected, err)
	}
	buffer.Reset()

	// future keywords import
	if err := repl.OneShot(ctx, "import future.keywords"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(buffer.Bytes()) != 0 {
		t.Errorf("Expected no output but got: %v", buffer.String())
		return
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "1 in [1,2,3]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result := buffer.String()
	expected = "true\n"
	if result != expected {
		t.Errorf("Expected expression to evaluate successfully but got: %v", result)
		return
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	act := buffer.String()
	exp := `package repl

import future.keywords
`
	if act != exp {
		t.Errorf("expected %q, got: %q", exp, act)
		return
	}

	buffer.Reset()
	if err := repl.OneShot(ctx, `package foo.bar`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "import future.keywords.in"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(buffer.Bytes()) != 0 {
		t.Errorf("Expected no output but got: %v", buffer.String())
		return
	}
	if err := repl.OneShot(ctx, `p = true { 1 in [1,2,3] }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// ignore "rule p defined" message
	buffer.Reset()
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	act = buffer.String()
	exp = `package foo.bar

import future.keywords.in

p {
	1 in [1, 2, 3]
}
`
	if act != exp {
		t.Errorf("expected %q, got: %q", exp, act)
		return
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "p"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result = buffer.String()
	expected = "true\n"
	if result != expected {
		t.Errorf("Expected expression to evaluate successfully but got: %v", result)
		return
	}
}

type replAction struct {
	line      string
	expOutput string
	expErrs   []string
}

func runReplActions(t *testing.T, repl *REPL, buffer *bytes.Buffer, actions []replAction) {
	t.Helper()

	ctx := t.Context()

	for _, action := range actions {
		buffer.Reset()
		err := repl.OneShot(ctx, action.line)

		if len(action.expErrs) != 0 {
			if err == nil {
				t.Fatalf("%q: expected error but got: %s", action.line, buffer.String())
			}
			for _, e := range action.expErrs {
				if !strings.Contains(err.Error(), e) {
					t.Fatalf("%q: expected error to contain:\n\n%q\n\nbut got:\n\n%v", action.line, e, err)
				}
			}
		} else {
			if err != nil {
				t.Fatalf("%q: unexpected error: %v", action.line, err)
			}
			expectOutput(t, buffer.String(), action.expOutput)
		}
	}
}

func TestEvalLogicalKeywords(t *testing.T) {
	tests := []struct {
		note    string
		actions []replAction
	}{
		{
			note: "and, implicit operands",
			actions: []replAction{
				{line: "import future.keywords.and"},
				{line: "1 == 1 and 2 == 2", expOutput: "true\n"},
			},
		},
		{
			note: "and, lhs undefined",
			actions: []replAction{
				{line: "import future.keywords.and"},
				{line: "1 == 2 and 2 == 2", expOutput: "undefined\n"},
			},
		},
		{
			note: "and, explicit operands",
			actions: []replAction{
				{line: "import future.keywords.and"},
				{line: "{x := 1; x == 1} and {y := 2; y == 2}", expOutput: "true\n"},
			},
		},
		{
			note: "and, implicit operands binding vars",
			actions: []replAction{
				{line: "import future.keywords.and"},
				{line: "x = 1 and y = 2", expErrs: []string{"var x is unsafe", "var y is unsafe"}},
			},
		},
		{
			note: "or, lhs undefined",
			actions: []replAction{
				{line: "import future.keywords.or"},
				{line: "1 == 2 or 1 == 1", expOutput: "true\n"},
			},
		},
		{
			note: "or, both operands undefined",
			actions: []replAction{
				{line: "import future.keywords.or"},
				{line: "1 == 2 or 1 == 3", expOutput: "undefined\n"},
			},
		},
		{
			note: "or, nested through explicit body",
			actions: []replAction{
				{line: "import future.keywords"},
				{line: "1 == 2 or {1 == 1 and {not false}}", expOutput: "true\n"},
			},
		},
		{
			note: "wildcard future import",
			actions: []replAction{
				{line: "import future.keywords"},
				{line: "false or 1 == 1 and 2 == 2", expOutput: "true\n"},
			},
		},
		{
			note: "import and expression in same submission",
			actions: []replAction{
				{line: "import future.keywords.and\n1 == 1 and 2 == 2", expOutput: "true\n"},
			},
		},
		{
			note: "keyword not imported",
			actions: []replAction{
				{
					line: "1 == 1 and 2 == 2",
					expErrs: []string{
						"var and is unsafe (hint: `import future.keywords.and` to import a future keyword)",
					},
				},
			},
		},
		{
			note: "and inside every body",
			actions: []replAction{
				{line: "import future.keywords"},
				{line: "every x in [1, 2] { x > 0 and x < 3 }", expOutput: "true\n"},
			},
		},
	}

	for _, tc := range tests {
		for _, regoVersion := range []ast.RegoVersion{ast.RegoV0, ast.RegoV1} {
			t.Run(regoVersion.String()+", "+tc.note, func(t *testing.T) {
				var buffer bytes.Buffer
				repl := newRepl(newTestStore(), &buffer).WithRegoVersion(regoVersion)
				runReplActions(t, repl, &buffer, tc.actions)
			})
		}
	}
}

func TestEvalLogicalKeywordsRules(t *testing.T) {
	tests := []struct {
		note        string
		regoVersion ast.RegoVersion
		actions     []replAction
	}{
		{
			note:        "v0, rule with or, show, unset",
			regoVersion: ast.RegoV0,
			actions: []replAction{
				{line: "import future.keywords.or"},
				{
					line:      "p { 1 == 2 or 1 == 1 }",
					expOutput: "Rule 'p' defined in package repl. Type 'show' to see rules.\n",
				},
				{line: "p", expOutput: "true\n"},
				{line: "show", expOutput: `package repl

import future.keywords.or

p {
	1 == 2 or 1 == 1
}
`},
				{line: "unset p"},
				{line: "p", expErrs: []string{"var p is unsafe"}},
			},
		},
		{
			note:        "v1, rule with or, show, unset",
			regoVersion: ast.RegoV1,
			actions: []replAction{
				{line: "import future.keywords.or"},
				{
					line:      "p if { 1 == 2 or 1 == 1 }",
					expOutput: "Rule 'p' defined in package repl. Type 'show' to see rules.\n",
				},
				{line: "p", expOutput: "true\n"},
				{line: "show", expOutput: `package repl

import future.keywords.or

p if 1 == 2 or 1 == 1
`},
				{line: "unset p"},
				{line: "p", expErrs: []string{"var p is unsafe"}},
			},
		},
		{
			note:        "v1, keyword import dropped on package switch",
			regoVersion: ast.RegoV1,
			actions: []replAction{
				{line: "import future.keywords.and"},
				{line: "package foo"},
				{
					line: "1 == 1 and 2 == 2",
					expErrs: []string{
						"var and is unsafe (hint: `import future.keywords.and` to import a future keyword)",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var buffer bytes.Buffer
			repl := newRepl(newTestStore(), &buffer).WithRegoVersion(tc.regoVersion)
			runReplActions(t, repl, &buffer, tc.actions)
		})
	}
}

// Parse errors are buffered as (potentially) incomplete input, which is why an
// or expression missing its keyword import produces no output at all.
func TestEvalLogicalKeywordsParseErrorBuffering(t *testing.T) {
	tests := []struct {
		note           string
		bufferDisabled bool
		expErrs        []string
	}{
		{
			note:           "buffering enabled",
			bufferDisabled: false,
		},
		{
			note:           "buffering disabled",
			bufferDisabled: true,
			expErrs:        []string{"rego_parse_error"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var buffer bytes.Buffer
			repl := newRepl(newTestStore(), &buffer).
				WithRegoVersion(ast.RegoV1).
				DisableMultiLineBuffering(tc.bufferDisabled)

			runReplActions(t, repl, &buffer, []replAction{
				{line: "p if { 1 == 2 or 1 == 1 }", expErrs: tc.expErrs},
			})
		})
	}
}

func TestEvalLogicalKeywordsPartial(t *testing.T) {
	tests := []struct {
		note    string
		actions []replAction
	}{
		{
			note: "or query",
			actions: []replAction{
				{line: "import future.keywords.or"},
				{line: "unknown input"},
				{line: "input.x == 1 or input.y == 2", expOutput: `┌─────────┬────────────────────────────┐
│ Query 1 │ input.x = 1 or input.y = 2 │
└─────────┴────────────────────────────┘
`},
			},
		},
		{
			note: "and query",
			actions: []replAction{
				{line: "import future.keywords.and"},
				{line: "unknown input"},
				{line: "input.x == 1 and input.y == 2", expOutput: `┌─────────┬─────────────────────────────┐
│ Query 1 │ input.x = 1 and input.y = 2 │
└─────────┴─────────────────────────────┘
`},
			},
		},
		{
			note: "and through rule",
			actions: []replAction{
				{line: "import future.keywords"},
				{line: "unknown input"},
				{
					line:      "p if { input.a and input.b }",
					expOutput: "Rule 'p' defined in package repl. Type 'show' to see rules.\n",
				},
				{line: "p", expOutput: `┌─────────┬─────────────────────┐
│ Query 1 │ input.a and input.b │
└─────────┴─────────────────────┘
`},
			},
		},
	}

	for _, tc := range tests {
		for _, regoVersion := range []ast.RegoVersion{ast.RegoV0, ast.RegoV1} {
			t.Run(regoVersion.String()+", "+tc.note, func(t *testing.T) {
				var buffer bytes.Buffer
				repl := newRepl(inmem.New(), &buffer).WithRegoVersion(regoVersion)
				runReplActions(t, repl, &buffer, tc.actions)
			})
		}
	}
}

func TestEvalLogicalKeywordsTrace(t *testing.T) {
	for _, regoVersion := range []ast.RegoVersion{ast.RegoV0, ast.RegoV1} {
		t.Run(regoVersion.String(), func(t *testing.T) {
			var buffer bytes.Buffer
			repl := newRepl(newTestStore(), &buffer).WithRegoVersion(regoVersion)

			runReplActions(t, repl, &buffer, []replAction{
				{line: "import future.keywords.or"},
				{line: "trace"},
				{line: "1 == 2 or 1 == 1", expOutput: `query:1     Enter equal(1, 2) or equal(1, 1)
query:1     | Eval equal(1, 2) or equal(1, 1)
query:1     | Enter equal(1, 2)
query:1     | | Eval equal(1, 2)
query:1     | | Fail equal(1, 2)
query:1     | Enter equal(1, 1)
query:1     | | Eval equal(1, 1)
query:1     | | Exit equal(1, 1) early
query:1     | Redo equal(1, 1)
query:1     | | Redo equal(1, 1)
query:1     | Exit equal(1, 2) or equal(1, 1)
query:1     Redo equal(1, 2) or equal(1, 1)
query:1     | Redo equal(1, 2) or equal(1, 1)
true
`},
			})
		})
	}
}

func TestEvalNotBodyRegoV1(t *testing.T) {
	var buffer bytes.Buffer
	repl := newRepl(newTestStore(), &buffer).WithRegoVersion(ast.RegoV1)

	runReplActions(t, repl, &buffer, []replAction{
		{line: "import future.keywords.not"},
		{line: "not {1 == 2}", expOutput: "true\n"},
	})
}

func TestEvalPackage(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, `package foo.bar`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// We import rego.v1 to ensure we're compatible with both v0 and v1 as default rego-version.
	if err := repl.OneShot(ctx, "import rego.v1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, `p = true if { true }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `package baz.qux`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	err := repl.OneShot(ctx, "p")
	expected := "p is unsafe"
	if err == nil {
		t.Fatalf("Expected OneShot to return error %v but got: %v", expected, err)
	}
	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("Expected unsafe variable error but got: %v", err)
	}
	if err := repl.OneShot(ctx, "import data.foo.bar.p"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "p"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if buffer.String() != "true\n" {
		t.Errorf("Expected expression to eval successfully but got: %v", buffer.String())
		return
	}
}

func TestMetrics(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer

	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "a = {[1,2], [3,4]}"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "metrics"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `[x | a[x]]`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(buffer.String(), "timer_rego_query_compile_ns") {
		t.Fatal("Expected output to contain well known metric key but got:", buffer.String())
	}

	buffer.Reset()
	if err := repl.OneShot(ctx, `[x | a[x]]`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(buffer.String(), "timer_rego_query_compile_ns") {
		t.Fatal("Expected output to contain well known metric key but got:", buffer.String())
	}

	buffer.Reset()
	if err := repl.OneShot(ctx, "metrics"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `[x | a[x]]`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

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
	ctx := t.Context()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	const numLines = 21

	mod2 := []byte(`package rbac
	import rego.v1

	inp := {
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

	roles := [
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

	allow if {
		user_has_role[role_name]
		role_has_permission[role_name]
	}

	user_has_role contains role_name if {
		binding := bindings[_]
		binding.user = inp.subject
		role_name := binding.roles[_]
	}

	role_has_permission contains role_name if {
		role := roles[_]
		role_name := role.name
		perm := role.permissions[_]
		perm.resource = inp.resource
		perm.action = inp.action
	}`)

	if err := store.UpsertPolicy(ctx, txn, "mod2", mod2); err != nil {
		panic(err)
	}
	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "profile"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "data.rbac.allow"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result := buffer.String()
	lines := strings.Split(result, "\n")
	if len(lines) != numLines {
		t.Fatal("Expected 21 lines, got :", len(lines))
	}
	buffer.Reset()
}

func TestStrictBuiltinErrors(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer

	repl := newRepl(store, &buffer)

	if err := repl.OneShot(ctx, "1/0"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result := buffer.String()

	if !strings.Contains(result, "undefined") {
		t.Fatal("expected undefined")
	}

	buffer.Reset()

	if err := repl.OneShot(ctx, "strict-builtin-errors"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "1/0"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result = buffer.String()

	if !strings.Contains(result, "divide by zero") {
		t.Fatal("expected divide by zero error")
	}
}

func TestInstrument(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer

	repl := newRepl(store, &buffer)

	// Turn on instrumentation w/o turning on metrics.
	if err := repl.OneShot(ctx, "instrument"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "true"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result := buffer.String()

	if !strings.Contains(result, "histogram_eval_op_plug") {
		t.Fatal("Expected plug histogram in output but got:", result)
	}

	buffer.Reset()

	// Turn off instrumentation.
	if err := repl.OneShot(ctx, "instrument"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "true"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result = buffer.String()

	if strings.Contains(result, "histogram_eval_op_plug") {
		t.Fatal("Expected instrumentation to be turned off but got:", result)
	}

	buffer.Reset()

	// Turn on metrics and then turn on instrumentation.
	if err := repl.OneShot(ctx, "metrics"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "true"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result = buffer.String()

	if strings.Contains(result, "histogram_eval_op_plug") {
		t.Fatal("Expected instrumentation to be turned off but got:", result)
	}

	if !strings.Contains(result, "timer_rego_query_eval_ns") {
		t.Fatal("Expected metrics to be turned on but got:", result)
	}

	buffer.Reset()

	if err := repl.OneShot(ctx, "instrument"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "true"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result = buffer.String()

	if !strings.Contains(result, "histogram_eval_op_plug") {
		t.Fatal("Expected instrumentation to be turned on but got:", result)
	}

	if !strings.Contains(result, "timer_rego_query_eval_ns") {
		t.Fatal("Expected metrics to be turned on but got:", result)
	}

}

func TestEvalTrace(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "trace"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `data.a[i].b.c[j] = x; data.a[k].b.c[x] = 1`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expected := strings.TrimSpace(`
query:1     Enter data.a[i].b.c[j] = x; data.a[k].b.c[x] = 1
query:1     | Eval data.a[i].b.c[j] = x
query:1     | Eval data.a[k].b.c[x] = 1
query:1     | Fail data.a[k].b.c[x] = 1
query:1     | Redo data.a[i].b.c[j] = x
query:1     | Eval data.a[k].b.c[x] = 1
query:1     | Exit data.a[i].b.c[j] = x; data.a[k].b.c[x] = 1
query:1     Redo data.a[i].b.c[j] = x; data.a[k].b.c[x] = 1
query:1     | Redo data.a[k].b.c[x] = 1
query:1     | Redo data.a[i].b.c[j] = x
query:1     | Eval data.a[k].b.c[x] = 1
query:1     | Fail data.a[k].b.c[x] = 1
query:1     | Redo data.a[i].b.c[j] = x
query:1     | Eval data.a[k].b.c[x] = 1
query:1     | Fail data.a[k].b.c[x] = 1
query:1     | Redo data.a[i].b.c[j] = x
query:1     | Eval data.a[k].b.c[x] = 1
query:1     | Fail data.a[k].b.c[x] = 1
query:1     | Redo data.a[i].b.c[j] = x
query:1     | Eval data.a[k].b.c[x] = 1
query:1     | Fail data.a[k].b.c[x] = 1
query:1     | Redo data.a[i].b.c[j] = x
┌───┬───┬───┬───┐
│ i │ j │ k │ x │
├───┼───┼───┼───┤
│ 0 │ 1 │ 1 │ 2 │
└───┴───┴───┴───┘`)
	expected += "\n"

	if expected != buffer.String() {
		t.Fatalf("Expected output to be exactly:\n%v\n\nGot:\n\n%v\n", expected, buffer.String())
	}
}

func TestEvalNotes(t *testing.T) {
	ctx := t.Context()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	// We import rego.v1 to ensure we're compatible with both v0 and v1 as default rego-version.
	if err := repl.OneShot(ctx, "import rego.v1"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := repl.OneShot(ctx, `p if { a = [1,2,3]; a[i] = x; x > 1; trace(sprintf("x = %d", [x])) }`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "notes"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	buffer.Reset()
	if err := repl.OneShot(ctx, "p"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expected := strings.TrimSpace(`query:1     Enter data.repl.p = _
query:1     | Enter data.repl.p
query:1     | | Note "x = 2"
true`)
	expected += "\n"
	if expected != buffer.String() {
		t.Fatalf("Expected output to be exactly:\n%v\n\nGot:\n\n%v\n", expected, buffer.String())
	}
}

func TestTruncatePrettyOutput(t *testing.T) {
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	repl.prettyLimit = 1000 // crank up limit to test repl command
	if err := repl.OneShot(ctx, "pretty-limit 80"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "data[x]"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	for line := range strings.SplitSeq(buffer.String(), "\n") {
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

func TestUnsetPackage(t *testing.T) {
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	if err := repl.OneShot(ctx, "package a"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `unset-package 5`); err == nil {
		t.Fatalf("Expected package-unset error for bad package but got: %v", buffer.String())
	}

	buffer.Reset()

	if err := repl.OneShot(ctx, "package a"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "unset-package b"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if buffer.String() != "warning: no matching package\n" {
		t.Fatalf("Expected unset-package warning no matching package but got: %v", buffer.String())
	}

	buffer.Reset()

	if err := repl.OneShot(ctx, `package a`); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, `unset-package b`); err != nil {
		t.Fatalf("Expected unset-package to succeed for input: %v", err)
	}

	buffer.Reset()

	if err := repl.OneShot(ctx, "package a"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "unset-package a"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if buffer.String() != "no rules defined\n" {
		t.Fatalf("Expected unset-package to return to default but got: %v", buffer.String())
	}
}

func TestCapabilities(t *testing.T) {
	capabilities := ast.CapabilitiesForThisVersion()
	allowedBuiltins := []*ast.Builtin{}
	for _, builtin := range capabilities.Builtins {
		if builtin.Name != "http.send" {
			allowedBuiltins = append(allowedBuiltins, builtin)
		}
	}
	capabilities.Builtins = allowedBuiltins
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer).WithCapabilities(capabilities)
	if err := repl.OneShot(ctx, `http.send({"url": "http://example.com", "method": "GET"})`); err != nil {
		if !strings.Contains(err.Error(), "undefined function http.send") {
			t.Fatalf("Unexpected error: %v", err)
		}
	} else {
		t.Fatalf("Expected error on http.send")
	}
}

func TestTraceArgument(t *testing.T) {
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "trace debug"); err != nil {
		t.Fatal(err)
	}
	if err := repl.OneShot(ctx, "show debug"); err != nil {
		t.Fatal(err)
	}
	output := buffer.String()
	expected := `"explain": "debug"`
	if !strings.Contains(output, expected) {
		t.Fatalf("Expected output to contain %s but got %s", expected, output)
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
	return New(store, "", buffer, "", 0, "").WithStderrWriter(buffer)
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
	var data map[string]any
	err := util.UnmarshalJSON([]byte(input), &data)
	if err != nil {
		panic(err)
	}
	return inmem.NewFromObject(data)
}

func parseJSON(s string) any {
	var v any
	if err := util.UnmarshalJSON([]byte(s), &v); err != nil {
		panic(err)
	}
	return v
}

func TestWith(t *testing.T) {
	ctx := t.Context()
	store := inmem.New()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)
	if err := repl.OneShot(ctx, "package main"); err != nil {
		t.Fatal(err)
	}
	if err := repl.OneShot(ctx, "n = 5"); err != nil {
		t.Fatal(err)
	}

	// add invalid expression using with in rule head
	expectedErr := "expressions using with keyword cannot be used for rule head"
	err := repl.OneShot(ctx, "even := n % 2 == 0 with n as 4")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != expectedErr {
		t.Fatalf("expected error: %q but got %q", expectedErr, err.Error())
	}

	// add valid with expression used in body
	if err := repl.OneShot(ctx, "even if {\n z := n % 2 == 0 with n as 4 \n z }"); err != nil {
		t.Fatal(err)
	}

	buffer.Reset()

	if err := repl.OneShot(ctx, "show"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := `package main

n := 5

even if {
	z := (n % 2) == 0 with n as 4
	z
}` + "\n"

	assertREPLText(t, buffer, expected)
}
