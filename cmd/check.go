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
	format       *util.EnumFlag
	errLimit     int
	ignore       []string
	bundleMode   bool
	capabilities *capabilitiesFlag
	schema       *schemaFlags
	strict       bool
}{
	format: util.NewEnumFlag(checkFormatPretty, []string{
		checkFormatPretty, checkFormatJSON,
	}),
	capabilities: newcapabilitiesFlag(),
	schema:       &schemaFlags{},
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

	ss, err := loader.Schemas(checkParams.schema.path)
	if err != nil {
		outputErrors(err)
		return 1
	}

	if checkParams.bundleMode {
		for _, path := range args {
			b, err := loader.NewFileLoader().
				WithSkipBundleVerification(true).
				WithProcessAnnotation(ss != nil).
				AsBundle(path)
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

		result, err := loader.NewFileLoader().
			WithProcessAnnotation(ss != nil).
			Filtered(args, f.Apply)
		if err != nil {
			outputErrors(err)
			return 1
		}

		for _, m := range result.Modules {
			modules[m.Name] = m.Parsed
		}
	}
	var capabilities *ast.Capabilities
	// if capabilities are not provided as a cmd flag,
	// then ast.CapabilitiesForThisVersion must be called
	// within checkModules to ensure custom builtins are properly captured
	if checkParams.capabilities.C != nil {
		capabilities = checkParams.capabilities.C
	} else {
		capabilities = ast.CapabilitiesForThisVersion()
	}
	compiler := ast.NewCompiler().
		SetErrorLimit(checkParams.errLimit).
		WithCapabilities(capabilities).
		WithSchemas(ss).
		WithEnablePrintStatements(true).
		WithStrict(checkParams.strict)

	compiler.Compile(modules)

	if !compiler.Failed() {
		return 0
	}

	outputErrors(compiler.Errors)

	return 1
}

func outputErrors(err error) {
	var out io.Writer
	if err != nil {
		out = os.Stderr
	} else {
		out = os.Stdout
	}

	switch checkParams.format.String() {
	case checkFormatJSON:
		result := pr.Output{
			Errors: pr.NewOutputErrors(err),
		}
		err := pr.JSON(out, result)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
	default:
		fmt.Fprintln(out, err)
	}
}

func init() {
	addMaxErrorsFlag(checkCommand.Flags(), &checkParams.errLimit)
	addIgnoreFlag(checkCommand.Flags(), &checkParams.ignore)
	checkCommand.Flags().VarP(checkParams.format, "format", "f", "set output format")
	addBundleModeFlag(checkCommand.Flags(), &checkParams.bundleMode, false)
	addCapabilitiesFlag(checkCommand.Flags(), checkParams.capabilities)
	addSchemaFlags(checkCommand.Flags(), checkParams.schema)
	addStrictFlag(checkCommand.Flags(), &checkParams.strict, false)
	RootCommand.AddCommand(checkCommand)
}
