// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/cmd/complexity"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/util"
)

const (
	complexityPrettyOutput = "pretty"
	complexityJSONOutput   = "json"
)

type complexityCommandParams struct {
	outputFormat *util.EnumFlag
	dataPaths    repeatedStringFlag
	ignore       []string
	bundlePaths  repeatedStringFlag
}

func newComplexityCommandParams() complexityCommandParams {
	return complexityCommandParams{
		outputFormat: util.NewEnumFlag(complexityPrettyOutput, []string{complexityPrettyOutput, complexityJSONOutput}),
	}
}

func analyzeModules(query string, params complexityCommandParams, w io.Writer) int {

	modules := map[string]*ast.Module{}

	if len(params.dataPaths.v) > 0 {
		f := loaderFilter{
			Ignore: checkParams.ignore,
		}

		result, err := loader.NewFileLoader().Filtered(params.dataPaths.v, f.Apply)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

		if result != nil {
			for _, m := range result.Modules {
				modules[m.Name] = m.Parsed
			}
		}
	}

	if params.bundlePaths.isFlagSet() {
		for _, bundleDir := range params.bundlePaths.v {
			result, err := loader.NewFileLoader().AsBundle(bundleDir)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}

			if result != nil {
				for _, m := range result.Modules {
					modules[m.Path] = m.Parsed
				}
			}
		}
	}

	// compile
	compiler := ast.NewCompiler()
	compiler.Compile(modules)

	if compiler.Failed() {
		fmt.Fprintln(w, compiler.Errors)
		return 1
	}

	complexityCalculator := complexity.New().WithCompiler(compiler).WithQuery(query)
	report, err := complexityCalculator.Calculate()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	switch params.outputFormat.String() {
	case complexityJSONOutput:
		err := report.JSON(w)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
	default:
		fmt.Fprintln(w, report.String())
	}
	return 0
}

func init() {
	params := newComplexityCommandParams()

	complexityCommand := &cobra.Command{
		Use:   "complexity <query>",
		Short: "Compute runtime complexity of a Rego query. Command is under active development and currently hidden",
		Long: `Compute runtime complexity of a Rego query and print the result. Command is under active development and currently hidden.

Examples
--------

To compute runtime of a simple query:

	$ opa complexity '1 == 1'

To compute runtime of a query defined in a Rego file:

	$ opa complexity --data policy.rego 'data.authz.foo  == true'

To compute runtime of a query defined in a Rego file inside a bundle:

	$ opa complexity --bundle /some/path 'data.authz.foo  == true'

Where /some/path contains:

	foo/
	  +-- policy.rego
`,
		Hidden: true,

		PreRunE: func(Cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("specify exactly one query argument")
			}
			return nil
		},

		Run: func(cmd *cobra.Command, args []string) {
			os.Exit(analyzeModules(args[0], params, os.Stdout))
		},
	}

	complexityCommand.Flags().VarP(params.outputFormat, "format", "f", "set output format")
	addDataFlag(complexityCommand.Flags(), &params.dataPaths)
	addIgnoreFlag(complexityCommand.Flags(), &params.ignore)
	addBundleFlag(complexityCommand.Flags(), &params.bundlePaths)
	RootCommand.AddCommand(complexityCommand)
}
