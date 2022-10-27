// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package cases contains utilities for evaluation test cases.
package cases

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/open-policy-agent/opa/util"
)

// Set represents a collection of test cases.
type Set struct {
	Cases []TestCase `json:"cases"`
}

// Sorted returns a sorted copy of s.
func (s Set) Sorted() Set {
	cpy := make([]TestCase, len(s.Cases))
	copy(cpy, s.Cases)
	sort.Slice(cpy, func(i, j int) bool {
		return cpy[i].Note < cpy[j].Note
	})
	return Set{Cases: cpy}
}

// TestCase represents a single test case.
type TestCase struct {
	Filename      string                    `json:"-"`                         // name of file that case was loaded from
	Note          string                    `json:"note"`                      // globally unique identifier for this test case
	Query         string                    `json:"query"`                     // policy query to execute
	Modules       []string                  `json:"modules,omitempty"`         // policies to test against
	Data          *map[string]interface{}   `json:"data,omitempty"`            // data to test against
	Input         *interface{}              `json:"input,omitempty"`           // parsed input data to use
	InputTerm     *string                   `json:"input_term,omitempty"`      // raw input data (serialized as a string, overrides input)
	WantDefined   *bool                     `json:"want_defined,omitempty"`    // expect query result to be defined (or not)
	WantResult    *[]map[string]interface{} `json:"want_result,omitempty"`     // expect query result (overrides defined)
	WantErrorCode *string                   `json:"want_error_code,omitempty"` // expect query error code (overrides result)
	WantError     *string                   `json:"want_error,omitempty"`      // expect query error message (overrides error code)
	SortBindings  bool                      `json:"sort_bindings,omitempty"`   // indicates that binding values should be treated as sets
	StrictError   bool                      `json:"strict_error,omitempty"`    // indicates that the error depends on strict builtin error mode
}

// Load returns a set of built-in test cases.
func Load(path string) (Set, error) {
	return loadRecursive(path)
}

// MustLoad returns a set of built-in test cases or panics if an error occurs.
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
