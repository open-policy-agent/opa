// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package console

import (
	"encoding/json"
	"flag"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/plugins/logs"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
	"os"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/logging/test"
	"github.com/open-policy-agent/opa/runtime"
	"github.com/open-policy-agent/opa/test/e2e"
)

var testRuntime *e2e.TestRuntime

func TestMain(m *testing.M) {
	flag.Parse()
	testServerParams := e2e.NewAPIServerTestParams()

	testServerParams.ConfigOverrides = []string{
		"decision_logs.console=true",
	}
	// Ensure decisions are logged regardless of regular log level
	testServerParams.Logging = runtime.LoggingConfig{Level: "error"}
	consoleLogger := test.New()
	testServerParams.ConsoleLogger = consoleLogger

	var err error
	testRuntime, err = e2e.NewTestRuntime(testServerParams)
	if err != nil {
		os.Exit(1)
	}

	testRuntime.ConsoleLogger = consoleLogger
	os.Exit(testRuntime.RunTests(m))
}

func TestConsoleDecisionLogWithInput(t *testing.T) {

	// Setup a test hook on the console logger (what the console decision logger uses)

	policy := `
	package test

	default allow = false

	allow {
		input.x == 1
	}
	`

	err := testRuntime.UploadPolicy(t.Name(), strings.NewReader(policy))
	if err != nil {
		t.Fatal(err)
	}

	input := map[string]int{
		"x": 1,
	}

	expected := true

	resultJSON, err := testRuntime.GetDataWithInput("test/allow", input)
	if err != nil {
		t.Fatal(err)
	}

	parsedBody := struct {
		Result bool `json:"result"`
	}{}

	err = json.Unmarshal(resultJSON, &parsedBody)
	if err != nil {
		t.Fatalf("Failed to parse body: \n\nActual: %s\n\nExpected: {\"result\": BOOL}\n\nerr = %s ", string(resultJSON), err)
	}

	if parsedBody.Result != expected {
		t.Fatalf("Unexpected result: %v", parsedBody.Result)
	}

	// Check for some important fields
	expectedFields := map[string]*struct {
		found bool
		match func(*testing.T, string)
	}{
		"labels":      {},
		"decision_id": {},
		"path":        {},
		"input":       {},
		"result":      {},
		"timestamp":   {},
		"type": {match: func(t *testing.T, actual string) {
			if actual != "openpolicyagent.org/decision_logs" {
				t.Fatalf("Expected field 'type' to be 'openpolicyagent.org/decision_logs'")
			}
		}},
	}

	var entry test.LogEntry
	var found bool

	for _, entry = range testRuntime.ConsoleLogger.Entries() {
		if entry.Message == "Decision Log" {
			found = true
		}
	}

	if !found {
		t.Fatalf("Did not find 'Decision Log' event in captured log entries")
	}

	// Ensure expected fields exist
	for fieldName, rawField := range entry.Fields {
		if fd, ok := expectedFields[fieldName]; ok {
			if fieldValue, ok := rawField.(string); ok && fd.match != nil {
				fd.match(t, fieldValue)
			}
			fd.found = true
		}
	}

	for field, fd := range expectedFields {
		if !fd.found {
			t.Errorf("Missing expected field in decision log: %s\n\nEntry: %+v\n\n", field, entry)
		}
	}
}

func TestConsoleLogDecisionWithExtraFields(t *testing.T) {

	rego.RegisterBuiltin1(
		&rego.Function{
			Name: "hello",
			Decl: types.NewFunction(types.Args(types.S), types.S),
		},
		func(bctx rego.BuiltinContext, a *ast.Term) (*ast.Term, error) {
			logs.SetExtra(bctx.Context, "foo", "bar")
			if str, ok := a.Value.(ast.String); ok {
				return ast.StringTerm("hello, " + string(str)), nil
			}
			return nil, nil
		},
	)

	policy := `
	package test_extra

	msg := hello(input.term)
	`

	err := testRuntime.UploadPolicy(t.Name(), strings.NewReader(policy))
	if err != nil {
		t.Fatal(err)
	}

	input := map[string]string{
		"term": "world",
	}

	expected := "hello, world"

	resultJSON, err := testRuntime.GetDataWithInput("test_extra/msg", input)
	if err != nil {
		t.Fatal(err)
	}

	parsedBody := struct {
		Result string `json:"result"`
	}{}

	err = json.Unmarshal(resultJSON, &parsedBody)
	if err != nil {
		t.Fatalf("Failed to parse body: \n\nActual: %s\n\nExpected: {\"result\": \"STRING\"}\n\nerr = %s ", string(resultJSON), err)
	}

	if parsedBody.Result != expected {
		t.Fatalf("Unexpected result: %v", parsedBody.Result)
	}

	var entry test.LogEntry
	var found bool

	for _, entry = range testRuntime.ConsoleLogger.Entries() {
		if entry.Message == "Decision Log" {
			found = true
		}
	}

	if !found {
		t.Fatalf("Did not find 'Decision Log' event in captured log entries")
	}

	extra, ok := entry.Fields["extra"]
	if !ok {
		t.Errorf("Did not find the 'extra' field")
	}

	extraMap, ok := extra.(map[string]interface{})
	if !ok {
		t.Errorf("Expected extra to be a map, got: %v\n", extra)
	}

	if extraMap["foo"] != "bar" {
		t.Errorf("Unexpected value: %v", extraMap["foo"])
	}
}
