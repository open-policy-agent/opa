// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/sirupsen/logrus/hooks/test"

	"github.com/open-policy-agent/opa/test/e2e"
)

var testRuntime *e2e.TestRuntime

func TestMain(m *testing.M) {
	flag.Parse()
	testServerParams := e2e.NewAPIServerTestParams()

	testServerParams.ConfigOverrides = []string{
		"decision_logs.console=true",
	}

	var err error
	testRuntime, err = e2e.NewTestRuntime(testServerParams)
	if err != nil {
		os.Exit(1)
	}

	os.Exit(testRuntime.RunAPIServerTests(m))
}

func TestDecisionLogWithInput(t *testing.T) {

	// Setup a test hook on the global logrus logger (what
	// the console decision logger uses)
	hook := test.NewGlobal()

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
	var entry *logrus.Entry
	for _, e := range hook.AllEntries() {
		if e.Message == "Decision Log" {
			entry = e
		}
	}

	if entry == nil {
		t.Fatalf("Did not find 'Decision Log' event in captured logrus entries")
	}

	// Ensure expected fields exist
	for fieldName, rawField := range entry.Data {
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
