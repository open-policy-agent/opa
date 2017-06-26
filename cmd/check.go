// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/runtime"

	"github.com/spf13/cobra"
)

var checkCommand = &cobra.Command{
	Use:   "check",
	Short: "Check policies for errors",
	Long: `Check that policy source files can be parsed and compiled.

If the 'check' command succeeds in parsing and compiling the source file(s), no output
is produced. If the parsing or compiling fails, 'check' will output the errors
and exit with a non-zero exit code.`,
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(checkModules(args))
	},
}

func checkModules(args []string) int {
	var errors []string
	modules := map[string]*ast.Module{}
	checkModule := func(filename string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if filepath.Ext(filename) != ".rego" {
			return nil
		}

		loaded, err := runtime.RegoLoad(filename)
		if err != nil {
			errors = append(errors, err.Error())
			return nil
		}

		modules[filename] = loaded.Parsed
		return nil
	}

	for _, filename := range args {
		if err := filepath.Walk(filename, checkModule); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 2
		}
	}

	if len(errors) == 0 {
		c := ast.NewCompiler()
		if c.Compile(modules); c.Failed() {
			errors = append(errors, c.Errors.Error())
		}
	}

	for _, err := range errors {
		fmt.Fprintln(os.Stderr, err)
	}
	return len(errors)
}

func init() {
	RootCommand.AddCommand(checkCommand)
}
