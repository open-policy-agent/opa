// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
)

//go:generate go run main.go

func main() {
	templateFile := "test-keywords-in-ref_v0.yaml.template"
	templateContent, err := os.ReadFile(templateFile)
	if err != nil {
		fmt.Printf("Error reading v0 template file: %v\n", err)
		os.Exit(1)
	}

	err = generate(ast.KeywordsV0[:], "../../testdata/v0/keywordrefs", string(templateContent))
	if err != nil {
		fmt.Printf("Error:%v\n", err)
		os.Exit(1)
	}

	templateFile = "test-keywords-in-ref_v1.yaml.template"
	templateContent, err = os.ReadFile(templateFile)
	if err != nil {
		fmt.Printf("Error reading v1 template file: %v\n", err)
		os.Exit(1)
	}

	err = generate(ast.KeywordsV1[:], "../../testdata/v1/keywordrefs", string(templateContent))
	if err != nil {
		fmt.Printf("Error:%v\n", err)
		os.Exit(1)
	}
}

func generate(keywords []string, root string, template string) error {
	err := os.MkdirAll(root, os.ModePerm)
	if err != nil {
		return fmt.Errorf("error creating directory %s: %v", root, err)
	}

	// Generate a YAML file for each keyword
	for _, keyword := range keywords {
		file := fmt.Sprintf("%s/test-keyword-%s.yaml", root, keyword)

		outputContent := strings.ReplaceAll(template, "%{KW}", keyword)

		err := os.WriteFile(file, []byte(outputContent), 0644)
		if err != nil {
			return fmt.Errorf("error writing file %s: %v", file, err)
		}
		fmt.Printf("Generated file: %s\n", file)
	}

	return nil
}
