// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util/test"
)

func TestAll_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		expErrs []string
	}{
		{
			note: "v0", // v0 is the default rego-version
			module: `package test

p[x] {
	x := "a"
}`,
		},
		{
			note: "rego.v1 import",
			module: `package test
import rego.v1

p contains x if {
	x := "a"
}`,
		},
		{
			note: "v1",
			module: `package test

p contains x if {
	x := "a"
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: var cannot be used for rule name",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"/test.rego": tc.module}

			test.WithTempFS(files, func(rootDir string) {
				moduleFile := filepath.Join(rootDir, "test.rego")
				loaded, err := All([]string{moduleFile})

				if len(tc.expErrs) > 0 {
					if err == nil {
						t.Fatalf("Expected errors but got nil")
					}

					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("Expected error to contain:\n\n%s\n\nbut got:\n\n%s", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}
					expected := ast.MustParseModule(files["/test.rego"])
					if !expected.Equal(loaded.Modules[CleanPath(moduleFile)].Parsed) {
						t.Fatalf("Expected:\n%v\n\nGot:\n%v", expected, loaded.Modules[moduleFile])
					}
				}
			})
		})
	}
}

func TestFiltered_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		expErrs []string
	}{
		{
			note: "v0", // v0 is the default rego-version
			module: `package test

p[x] {
	x := "a"
}`,
		},
		{
			note: "rego.v1 import",
			module: `package test
import rego.v1

p contains x if {
	x := "a"
}`,
		},
		{
			note: "v1",
			module: `package test

p contains x if {
	x := "a"
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: var cannot be used for rule name",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"/test.rego": tc.module}

			test.WithTempFS(files, func(rootDir string) {
				moduleFile := filepath.Join(rootDir, "test.rego")
				filter := func(string, os.FileInfo, int) bool {
					return false
				}

				loaded, err := Filtered([]string{moduleFile}, filter)

				if len(tc.expErrs) > 0 {
					if err == nil {
						t.Fatalf("Expected errors but got nil")
					}

					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("Expected error to contain:\n\n%s\n\nbut got:\n\n%s", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}
					expected := ast.MustParseModule(files["/test.rego"])
					if !expected.Equal(loaded.Modules[CleanPath(moduleFile)].Parsed) {
						t.Fatalf("Expected:\n%v\n\nGot:\n%v", expected, loaded.Modules[moduleFile])
					}
				}
			})
		})
	}
}

func TestRego_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		expErrs []string
	}{
		{
			note: "v0", // v0 is the default rego-version
			module: `package test

p[x] {
	x := "a"
}`,
		},
		{
			note: "rego.v1 import",
			module: `package test
import rego.v1

p contains x if {
	x := "a"
}`,
		},
		{
			note: "v1",
			module: `package test

p contains x if {
	x := "a"
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: var cannot be used for rule name",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"/test.rego": tc.module}

			test.WithTempFS(files, func(rootDir string) {
				moduleFile := filepath.Join(rootDir, "test.rego")
				loaded, err := Rego(moduleFile)

				if len(tc.expErrs) > 0 {
					if err == nil {
						t.Fatalf("Expected errors but got nil")
					}

					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("Expected error to contain:\n\n%s\n\nbut got:\n\n%s", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}
					expected := ast.MustParseModule(files["/test.rego"])
					if !expected.Equal(loaded.Parsed) {
						t.Fatalf("Expected:\n%v\n\nGot:\n%v", expected, loaded.Parsed)
					}
				}
			})
		})
	}
}

func TestAllRegos_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		expErrs []string
	}{
		{
			note: "v0", // v0 is the default rego-version
			module: `package test

p[x] {
	x := "a"
}`,
		},
		{
			note: "rego.v1 import",
			module: `package test
import rego.v1

p contains x if {
	x := "a"
}`,
		},
		{
			note: "v1",
			module: `package test

p contains x if {
	x := "a"
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: var cannot be used for rule name",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"/test.rego": tc.module}

			test.WithTempFS(files, func(rootDir string) {
				moduleFile := filepath.Join(rootDir, "test.rego")
				loaded, err := AllRegos([]string{moduleFile})

				if len(tc.expErrs) > 0 {
					if err == nil {
						t.Fatalf("Expected errors but got nil")
					}

					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("Expected error to contain:\n\n%s\n\nbut got:\n\n%s", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}
					expected := ast.MustParseModule(files["/test.rego"])
					if !expected.Equal(loaded.Modules[CleanPath(moduleFile)].Parsed) {
						t.Fatalf("Expected:\n%v\n\nGot:\n%v", expected, loaded.Modules[moduleFile])
					}
				}
			})
		})
	}
}

func TestLoadRego_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		expErrs []string
	}{
		{
			note: "v0", // v0 is the default rego-version
			module: `package test

p[x] {
	x := "a"
}`,
		},
		{
			note: "rego.v1 import",
			module: `package test
import rego.v1

p contains x if {
	x := "a"
}`,
		},
		{
			note: "v1",
			module: `package test

p contains x if {
	x := "a"
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: var cannot be used for rule name",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"/test.rego": tc.module}

			test.WithTempFS(files, func(rootDir string) {
				moduleFile := filepath.Join(rootDir, "test.rego")
				loaded, err := NewFileLoader().All([]string{moduleFile})

				if len(tc.expErrs) > 0 {
					if err == nil {
						t.Fatalf("Expected errors but got nil")
					}

					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("Expected error to contain:\n\n%s\n\nbut got:\n\n%s", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}
					expected := ast.MustParseModule(files["/test.rego"])
					if !expected.Equal(loaded.Modules[CleanPath(moduleFile)].Parsed) {
						t.Fatalf("Expected:\n%v\n\nGot:\n%v", expected, loaded.Modules[moduleFile])
					}
				}
			})
		})
	}
}
