// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package format

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestFormatNilLocation(t *testing.T) {
	rule := ast.MustParseRule(`r = y { y = "foo" }`)
	rule.Head.Location = nil

	bs, err := Ast(rule)
	if err != nil {
		t.Fatal(err)
	}

	exp := strings.Trim(`
r = y {
	y = "foo"
}`, " \n")

	if string(bs) != exp {
		t.Fatalf("Expected %q but got %q", exp, string(bs))
	}
}

func TestFormatNilLocationEmptyBody(t *testing.T) {
	b := ast.NewBody()
	x, err := Ast(b)
	if len(x) != 0 || err != nil {
		t.Fatalf("Expected empty result but got: %q, err: %v", string(x), err)
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

	exp := "1 error occurred: testfiles/test.rego.error:27: rego_parse_error: no match found"

	if !strings.HasPrefix(err.Error(), exp) {
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

			if ln, at := differsAt(formatted, expected); ln != 0 {
				t.Fatalf("Expected formatted bytes to equal expected bytes but differed near line %d / byte %d (got: %q, expected: %q):\n%s", ln, at, formatted[at], expected[at], prefixWithLineNumbers(formatted))
			}

			if _, err := ast.ParseModule(rego+".tmp", string(formatted)); err != nil {
				t.Fatalf("Failed to parse formatted bytes: %v", err)
			}

			formatted, err = Source(rego, formatted)
			if err != nil {
				t.Fatalf("Failed to double format file")
			}

			if ln, at := differsAt(formatted, expected); ln != 0 {
				t.Fatalf("Expected roundtripped bytes to equal expected bytes but differed near line %d / byte %d:\n%s", ln, at, prefixWithLineNumbers(formatted))
			}

		})
	}
}

func differsAt(a, b []byte) (int, int) {
	if bytes.Equal(a, b) {
		return 0, 0
	}
	minLen := len(a)
	if minLen > len(b) {
		minLen = len(b)
	}
	ln := 1
	for i := 0; i < minLen; i++ {
		if a[i] == '\n' {
			ln++
		}
		if a[i] != b[i] {
			return ln, i
		}
	}
	return ln, minLen
}

func prefixWithLineNumbers(bs []byte) []byte {
	raw := string(bs)
	lines := strings.Split(raw, "\n")
	format := fmt.Sprintf("%%%dd %%s", len(fmt.Sprint(len(lines)+1)))
	for i, line := range lines {
		lines[i] = fmt.Sprintf(format, i+1, line)
	}
	return []byte(strings.Join(lines, "\n"))
}
