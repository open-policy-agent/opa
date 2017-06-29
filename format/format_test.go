// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package format

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestFormatNilLocation(t *testing.T) {
	rule := ast.MustParseRule(`r = y { y = "foo" }`)
	rule.Head.Location = nil

	_, err := Ast(rule)
	if err == nil {
		t.Fatal("Expected error for rule with nil Location in head")
	}

	if _, ok := err.(nilLocationErr); !ok {
		t.Fatalf("Expected nilLocationErr, got %v", err)
	}
}

func TestFormatSourceError(t *testing.T) {
	rego := "testfiles/test.rego.error"
	contents, err := ioutil.ReadFile(rego)
	if err != nil {
		t.Fatalf("Failed to read rego source: %v", err)
	}

	_, err = Source(rego, contents)
	if err == nil {
		t.Fatal("Expected parsing error, not nil")
	}

	exp := "1 error occurred: testfiles/test.rego.error:25: rego_parse_error: no match found, unexpected '{'"
	if err.Error() != exp {
		t.Fatalf("Expected error message '%s', got '%s'", exp, err.Error())
	}
}

func TestFormatSource(t *testing.T) {
	regoFiles, err := filepath.Glob("testfiles/*.rego")
	if err != nil {
		panic(err)
	}

	for _, rego := range regoFiles {
		t.Run(rego, func(t *testing.T) {
			contents, err := ioutil.ReadFile(rego)
			if err != nil {
				t.Fatalf("Failed to read rego source: %v", err)
			}

			expected, err := ioutil.ReadFile(rego + ".formatted")
			if err != nil {
				t.Fatalf("Failed to read expected rego source: %v", err)
			}

			formatted, err := Source(rego, contents)
			if err != nil {
				t.Fatalf("Failed to format file: %v", err)
			}

			if !bytes.Equal(expected, formatted) {
				t.Fatalf("Formatted bytes did not match expected:\n%s", string(formatted))
			}

			if _, err := ast.ParseModule(rego+".tmp", string(formatted)); err != nil {
				t.Fatalf("Failed to parse formatted bytes: %v", err)
			}

			formatted, err = Source(rego, formatted)
			if err != nil {
				t.Fatalf("Failed to double format file")
			}

			if !bytes.Equal(expected, formatted) {
				t.Fatal("Formatted bytes did not match expected")
			}

		})
	}
}
