// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/ast"
	pr "github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/util"
)

var checkParams = struct {
	format     *util.EnumFlag
	errLimit   int
	ignore     []string
	bundleMode bool
}{
	format: util.NewEnumFlag(checkFormatPretty, []string{
		checkFormatPretty, checkFormatJSON,
	}),
}

const (
	checkFormatPretty = "pretty"
	checkFormatJSON   = "json"
)

var checkCommand = &cobra.Command{
	Use:   "check <path> [path [...]]",
	Short: "Check Rego source files",
	Long: `Check Rego source files for parse and compilation errors.

If the 'check' command succeeds in parsing and compiling the source file(s), no output
is produced. If the parsing or compiling fails, 'check' will output the errors
and exit with a non-zero exit code.`,

	PreRunE: func(Cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("specify at least one file")
		}
		return nil
	},

	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(checkModules(args))
	},
}

func checkModules(args []string) int {
	modules := map[string]*ast.Module{}

	if checkParams.bundleMode {
		for _, path := range args {
			b, err := loader.AsBundle(path)
			if err != nil {
				outputErrors(err)
				return 1
			}
			for name, mod := range b.ParsedModules(path) {
				modules[name] = mod
			}
		}
	} else {
		f := loaderFilter{
			Ignore: checkParams.ignore,
		}

		result, err := loader.Filtered(args, f.Apply)
		if err != nil {
			outputErrors(err)
			return 1
		}

		for _, m := range result.Modules {
			modules[m.Name] = m.Parsed
		}
	}

	compiler := ast.NewCompiler().SetErrorLimit(checkParams.errLimit)

	compiler.Compile(modules)

	if !compiler.Failed() {
		return 0
	}

	outputErrors(compiler.Errors)

	return 1
}

func outputErrors(err error) {
	switch checkParams.format.String() {
	case checkFormatJSON:
		result := pr.Output{
			Errors: pr.NewOutputErrors(err),
		}
		var out io.Writer
		if err != nil {
			out = os.Stderr
		} else {
			out = os.Stdout
		}
		err := pr.JSON(out, result)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
	default:
		fmt.Fprintln(os.Stdout, err)
	}
}

func init() {
	setMaxErrors(checkCommand.Flags(), &checkParams.errLimit)
	setIgnore(checkCommand.Flags(), &checkParams.ignore)
	checkCommand.Flags().VarP(checkParams.format, "format", "f", "set output format")
	checkCommand.Flags().BoolVarP(&checkParams.bundleMode, "bundle", "b", false, "load paths as bundle files or root directories")
	RootCommand.AddCommand(checkCommand)
}
