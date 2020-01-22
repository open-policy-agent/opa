// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/cmd/complexity"
	pr "github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/loader"
)

var analyzeCommand = &cobra.Command{
	Use:    "complexity <path>",
	Short:  "Compute runtime complexity of Rego source code",
	Hidden: true,

	PreRunE: func(Cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("specify at least one file")
		}
		return nil
	},

	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(analyzeModules(args))
	},
}

func analyzeModules(args []string) int {
	modules := map[string]*ast.Module{}

	f := loaderFilter{}

	result, err := loader.Filtered(args, f.Apply)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	for _, m := range result.Modules {
		modules[m.Name] = m.Parsed
	}

	// compile
	compiler := ast.NewCompiler()
	compiler.Compile(modules)

	if compiler.Failed() {
		fmt.Println(compiler.Errors)
		return 1
	}

	for _, module := range compiler.Modules {
		result := complexity.CalculateRuntimeComplexity(module)

		fmt.Println()
		if len(result.Result) != 0 {
			fmt.Printf("Time Complexity Results for rules in %v:\n", module.Package.Location.File)
			pr.JSON(os.Stdout, result.Result)
		}

		// Missing/Unhandled rules
		if len(result.Missing) != 0 {
			fmt.Printf("\nRules with unhandled expressions in %v:\n", module.Package.Location.File)
			pr.JSON(os.Stdout, result.Missing)
		}
	}
	return 0
}

func init() {
	RootCommand.AddCommand(analyzeCommand)
}
