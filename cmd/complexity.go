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
	query        string
}

func newComplexityCommandParams() complexityCommandParams {
	return complexityCommandParams{
		outputFormat: util.NewEnumFlag(complexityPrettyOutput, []string{complexityPrettyOutput, complexityJSONOutput}),
	}
}

func analyzeModules(args []string, params complexityCommandParams, w io.Writer) int {
	modules := map[string]*ast.Module{}

	f := loaderFilter{}

	result, err := loader.NewFileLoader().Filtered(args, f.Apply)
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
		fmt.Fprintln(w, compiler.Errors)
		return 1
	}

	complexityCalculator := complexity.New().WithCompiler(compiler).WithQuery(params.query)
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
		Use:    "complexity",
		Short:  "Compute runtime complexity of a Rego query. Command is under active development and currently hidden",
		Hidden: true,

		Run: func(cmd *cobra.Command, args []string) {
			os.Exit(analyzeModules(args, params, os.Stdout))
		},
	}

	complexityCommand.Flags().VarP(params.outputFormat, "format", "f", "set output format")
	complexityCommand.Flags().StringVarP(&params.query, "query", "q", "data", "set a Rego query to calculate runtime of a rule")
	RootCommand.AddCommand(complexityCommand)
}
