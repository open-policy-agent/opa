// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logging

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestPrettyFormatterNoFields(t *testing.T) {
	fmtr := prettyFormatter{}

	e := logrus.NewEntry(logrus.StandardLogger())
	e.Message = "test"
	e.Level = logrus.InfoLevel

	out, err := fmtr.Format(e)
	if err != nil {
		t.Fatalf("Unexpected error formatting log entry: %s", err.Error())
	}

	actualStr := string(out)

	expectedLvl := strings.ToUpper(e.Level.String())
	if !strings.Contains(actualStr, expectedLvl) {
		t.Errorf("Expected log message to have level %s:\n%s", expectedLvl, actualStr)
	}

	if !strings.Contains(actualStr, "test") {
		t.Errorf("Expected log message to have the entry message '%s':\n%s", "test", actualStr)
	}
}

func TestPrettyFormatterBasicFields(t *testing.T) {
	fmtr := prettyFormatter{}

	e := logrus.WithFields(logrus.Fields{
		"number": 5,
		"string": "field_string",
		"nil":    nil,
		"error":  errors.New("field_error").Error(),
	})

	e.Message = "test"
	e.Level = logrus.InfoLevel

	out, err := fmtr.Format(e)
	if err != nil {
		t.Fatalf("Unexpected error formatting log entry: %s", err.Error())
	}

	actualStr := string(out)

	expectedLvl := strings.ToUpper(e.Level.String())
	if !strings.Contains(actualStr, expectedLvl) {
		t.Errorf("Expected log message to have level %s:\n%s", expectedLvl, actualStr)
	}

	if !strings.Contains(actualStr, "test\n") {
		t.Errorf("Expected log message to have the entry message '%s':\n%s", "test", actualStr)
	}

	if !strings.Contains(actualStr, "number = 5\n") {
		t.Errorf("Expected to have the number field in message")
	}

	if !strings.Contains(actualStr, "string = \"field_string\"\n") {
		t.Errorf("Expected to have the string field in message")
	}

	if !strings.Contains(actualStr, "nil = null\n") {
		t.Errorf("Expected to have the nil field in message")
	}

	if !strings.Contains(actualStr, "error = \"field_error\"\n") {
		t.Errorf("Expected to have the nil field in message")
	}

	expectedLines := 7 // one for the message, 4 fields (one line each), and two trailing \n
	actualLines := len(strings.Split(actualStr, "\n"))
	if actualLines != expectedLines {
		t.Errorf("Expected %d lines in output, found %d\n Output: \n%s\n", expectedLines, actualLines, actualStr)
	}
}

func TestPrettyFormatterMultilineStringFields(t *testing.T) {
	fmtr := prettyFormatter{}

	mlStr := `
package opa.examples

import data.servers
import data.networks
import data.ports

public_servers[server] {
	server := servers[_]
	server.ports[_] == ports[k].id
	ports[k].networks[_] == networks[m].id
	networks[m].public == true
}
`

	e := logrus.WithFields(logrus.Fields{
		"multi_line": mlStr,
	})

	e.Message = "test"
	e.Level = logrus.InfoLevel

	out, err := fmtr.Format(e)
	if err != nil {
		t.Fatalf("Unexpected error formatting log entry: %s", err.Error())
	}

	actualStr := string(out)

	expectedLvl := strings.ToUpper(e.Level.String())
	if !strings.Contains(actualStr, expectedLvl) {
		t.Errorf("Expected log message to have level %s:\n%s", expectedLvl, actualStr)
	}

	if !strings.Contains(actualStr, "test") {
		t.Errorf("Expected log message to have the entry message '%s':\n%s", "test", actualStr)
	}

	for _, line := range strings.Split(mlStr, "\n") {
		// The lines will get prefixed with some padding but should always
		// still have their real newlines, and not be encoded.
		expectedStr := line + "\n"
		if !strings.Contains(actualStr, expectedStr) {
			t.Errorf("Expected to find line in message:\n\n%s\n\nactual:\n\n%s\n", expectedStr, actualStr)
		}
	}
}

func TestPrettyFormatterMultilineJSONFields(t *testing.T) {
	fmtr := prettyFormatter{}

	obj := map[string]interface{}{
		"a": 123,
		"b": nil,
		"d": "abc",
		"e": map[string]interface{}{
			"test": []string{
				"aa",
				"bb",
				"cc",
			},
		},
	}

	e := logrus.WithFields(logrus.Fields{
		"json_string": obj,
	})

	e.Message = "test"
	e.Level = logrus.InfoLevel

	out, err := fmtr.Format(e)
	if err != nil {
		t.Fatalf("Unexpected error formatting log entry: %s", err.Error())
	}

	actualStr := string(out)

	expectedLvl := strings.ToUpper(e.Level.String())
	if !strings.Contains(actualStr, expectedLvl) {
		t.Errorf("Expected log message to have level %s:\n%s", expectedLvl, actualStr)
	}

	if !strings.Contains(actualStr, "test") {
		t.Errorf("Expected log message to have the entry message 'test':\n%s", actualStr)
	}

	expectedJSON, err := json.MarshalIndent(&obj, "      ", "  ")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(actualStr, string(expectedJSON)) {
		t.Errorf("Expected JSON to be formatted and included in message:\n\nExpected:\n%s\n\nActual:\n%s\n\n", string(expectedJSON), actualStr)
	}
}
