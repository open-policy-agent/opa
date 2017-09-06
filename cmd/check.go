// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/loader"
	"github.com/spf13/cobra"
)

var errLimit int

var checkCommand = &cobra.Command{
	Use:   "check",
	Short: "Check Rego source files",
	Long: `Check Rego source files for parse and compilation errors.

If the 'check' command succeeds in parsing and compiling the source file(s), no output
is produced. If the parsing or compiling fails, 'check' will output the errors
and exit with a non-zero exit code.`,
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(checkModules(args))
	},
}

func checkModules(args []string) int {

	result, err := loader.AllRegos(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	modules := map[string]*ast.Module{}

	for _, m := range result.Modules {
		modules[m.Name] = m.Parsed
	}

	compiler := ast.NewCompiler().SetErrorLimit(errLimit)

	if compiler.Compile(modules); compiler.Failed() {
		for _, err := range compiler.Errors {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}

	return 0
}

func init() {
	setMaxErrors(checkCommand.Flags(), &errLimit)
	RootCommand.AddCommand(checkCommand)
}
