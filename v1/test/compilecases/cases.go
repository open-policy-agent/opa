// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package compilecases contains utilities for compiler diagnostic test cases
package compilecases

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/open-policy-agent/opa/v1/util"
)

// Set represents a collection of test cases.
type Set struct {
	Cases []TestCase `json:"cases"`
}

// Sorted returns a sorted copy of s.
func (s Set) Sorted() Set {
	cpy := make([]TestCase, len(s.Cases))
	copy(cpy, s.Cases)
	slices.SortFunc(cpy, func(a, b TestCase) int {
		return strings.Compare(a.Note, b.Note)
	})
	return Set{Cases: cpy}
}

// TestCase represents a single test case: a set of modules that must fail to
// compile, and the diagnostics that failure must produce.
type TestCase struct {
	Filename             string   `json:"-"                                yaml:"-"`                               // name of file that case was loaded from
	Note                 string   `json:"note"                             yaml:"note"`                            // globally unique identifier for this test case
	Modules              []string `json:"modules"                          yaml:"modules"`                         // policies to compile, named test-0.rego, test-1.rego, ...
	RegoVersion          string   `json:"rego_version,omitempty"           yaml:"rego_version,omitempty"`          // rego version to parse the modules as: v0, v1 (default), or v0-compat-v1
	Strict               bool     `json:"strict,omitempty"                 yaml:"strict,omitempty"`                // enable the compiler's strict mode
	ExperimentalKeywords bool     `json:"experimental_keywords,omitempty"  yaml:"experimental_keywords,omitempty"` // opt-in to experimental future keywords
	PrintStatements      bool     `json:"print_statements,omitempty"       yaml:"print_statements,omitempty"`      // keep print() calls instead of erasing them, as required to reach diagnostics about their operands
	WantErrors           []Error  `json:"want_errors"                      yaml:"want_errors"`                     // diagnostics the compilation must produce
	Exhaustive           bool     `json:"exhaustive,omitempty"             yaml:"exhaustive,omitempty"`            // require want_errors to be the complete set, not a subset
}

// Error is one expected diagnostic. Row and Col are 1-based positions in the
// module named by Module; Message is the error sentence, without the position
// and code an implementation may prefix it with when rendering.
type Error struct {
	Module  string `json:"module,omitempty"  yaml:"module,omitempty"` // module the error is reported against, defaults to test-0.rego
	Code    string `json:"code"              yaml:"code"`
	Row     int    `json:"row"               yaml:"row"`
	Col     int    `json:"col,omitempty"     yaml:"col,omitempty"` // asserted when non-zero
	Message string `json:"message"           yaml:"message"`
}

func (e Error) String() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s", e.ModuleOrDefault(), e.Row, e.Col, e.Code, e.Message)
}

// ModuleOrDefault returns the module the error is reported against.
func (e Error) ModuleOrDefault() string {
	if e.Module == "" {
		return DefaultModuleName
	}
	return e.Module
}

// DefaultModuleName is the name given to the first module of a case, and the
// module errors are reported against unless stated otherwise.
const DefaultModuleName = "test-0.rego"

// ModuleName returns the name given to the i-th module of a case.
func ModuleName(i int) string {
	return fmt.Sprintf("test-%d.rego", i)
}

// Load returns the set of test cases under path.
func Load(path string) (Set, error) {
	return loadRecursive(path)
}

// MustLoad returns the set of test cases under path or panics if an error occurs.
func MustLoad(path string) Set {
	result, err := Load(path)
	if err != nil {
		panic(err)
	}
	return result
}

func loadRecursive(dirpath string) (Set, error) {
	result := Set{}

	err := filepath.Walk(dirpath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if ext := filepath.Ext(path); ext != ".yaml" && ext != ".yml" {
			return nil
		}

		bs, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		var x Set
		if err := util.Unmarshal(bs, &x); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		for i := range x.Cases {
			x.Cases[i].Filename = path
		}

		result.Cases = append(result.Cases, x.Cases...)
		return nil
	})

	return result, err
}
