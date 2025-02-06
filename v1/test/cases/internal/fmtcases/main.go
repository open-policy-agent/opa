// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/format"
	"github.com/open-policy-agent/opa/v1/test/cases"
	"github.com/open-policy-agent/opa/v1/util"
	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) < 5 {
		fmt.Println("Usage: main <source-dir> <source-version> <target-dir> <target-version>")
		os.Exit(1)
	}

	s := os.Args[1]
	sv, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("Version must be an integer")
		os.Exit(1)
	}
	t := os.Args[3]
	tv, err := strconv.Atoi(os.Args[4])
	if err != nil {
		fmt.Println("Version must be an integer")
		os.Exit(1)
	}
	sourceRegoVersion := ast.RegoVersionFromInt(sv)
	targetRegoVersion := ast.RegoVersionFromInt(tv)

	fmt.Printf("Formatting test cases '%s'->'%s' to rego-version %s\n", s, t, targetRegoVersion)

	es, err := os.ReadDir(s)
	if err != nil {
		fmt.Println("Error reading source directory:", err)
		os.Exit(1)
	}
	for _, e := range es {
		if err := copyEntry(s, sourceRegoVersion, e, t, targetRegoVersion); err != nil {
			fmt.Println("Error handling source entry:", err)
			os.Exit(1)
		}
	}
}

func copyEntry(sourceRoot string, sourceRegoVersion ast.RegoVersion, e os.DirEntry, targetRoot string, targetRegoVersion ast.RegoVersion) error {
	i, err := e.Info()
	if err != nil {
		return err
	}

	if i.IsDir() {
		err = os.MkdirAll(filepath.Join(targetRoot, e.Name()), i.Mode())
		if err != nil {
			return err
		}
		childSourceRoot := filepath.Join(sourceRoot, e.Name())
		childTargetRoot := filepath.Join(targetRoot, e.Name())
		es, err := os.ReadDir(childSourceRoot)
		if err != nil {
			return err
		}
		for _, c := range es {
			if err := copyEntry(childSourceRoot, sourceRegoVersion, c, childTargetRoot, targetRegoVersion); err != nil {
				return err
			}
		}
	} else {
		path := filepath.Join(sourceRoot, i.Name())
		bs, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var testCases cases.Set
		if err := util.Unmarshal(bs, &testCases); err != nil {
			return err
		}

		// Format test modules
		for _, testCase := range testCases.Cases {
			for i, module := range testCase.Modules {
				bs, err := format.SourceWithOpts(fmt.Sprintf("mod%d.rego", i), []byte(module),
					format.Opts{
						ParserOptions: &ast.ParserOptions{
							RegoVersion: sourceRegoVersion,
						},
						RegoVersion: targetRegoVersion,
					})
				if err != nil {
					fmt.Printf("Error formatting module %s %s:%d: %v\n", path, testCase.Note, i, err)
				} else {
					testCase.Modules[i] = string(bs)
				}
			}
		}

		// Write formatted test cases to target directory
		targetPath := filepath.Join(targetRoot, i.Name())
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(testCases); err != nil {
			return err
		}

		text := "---\n" + buf.String()
		if err := os.WriteFile(targetPath, []byte(text), i.Mode()); err != nil {
			return err
		}
	}
	return nil
}
