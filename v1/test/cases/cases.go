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

	"github.com/open-policy-agent/opa/v1/util"
)

// Create v1 test cases from v0 test cases.
// //go:generate ../../build/gen-run-go.sh internal/fmtcases/main.go testdata/v0 0 testdata/v1 1
//go:generate ../../build/gen-run-go.sh internal/fmtcases/main.go testdata/v1 0 testdata/v1_2 1

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
	WantErrorCode       *string           `json:"want_error_code,omitempty" yaml:"want_error_code,omitempty"`
	WantError           *string           `json:"want_error,omitempty"      yaml:"want_error,omitempty"`
	Env                 map[string]string `json:"env,omitempty"             yaml:"env,omitempty"`
	WantDefined         *bool             `json:"want_defined,omitempty"    yaml:"want_defined,omitempty"`
	Data                *map[string]any   `json:"data,omitempty"            yaml:"data,omitempty"`
	Input               *any              `json:"input,omitempty"           yaml:"input,omitempty"`
	InputTerm           *string           `json:"input_term,omitempty"      yaml:"input_term,omitempty"`
	WantResult          *[]map[string]any `json:"want_result,omitempty"     yaml:"want_result,omitempty"`
	Note                string            `json:"note"                      yaml:"note"`
	Filename            string            `json:"-"                         yaml:"-"`
	Query               string            `json:"query"                     yaml:"query"`
	Modules             []string          `json:"modules,omitempty"         yaml:"modules,omitempty"`
	SortBindings        bool              `json:"sort_bindings,omitempty"   yaml:"sort_bindings,omitempty"`
	IgnoreGeneratedVars bool              `json:"ignore_generated_vars"     yaml:"ignore_generated_vars"`
	StrictError         bool              `json:"strict_error,omitempty"    yaml:"strict_error,omitempty"`
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
