// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/dependencies"
	"github.com/open-policy-agent/opa/internal/presentation"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/util"
)

type depsCommandParams struct {
	dataPaths   repeatedStringFlag
	format      *util.EnumFlag
	ignore      []string
	bundlePaths repeatedStringFlag
}

const (
	depsFormatPretty = "pretty"
	depsFormatJSON   = "json"
)

func init() {

	var params depsCommandParams

	params.format = util.NewEnumFlag(depsFormatPretty, []string{
		depsFormatPretty, depsFormatJSON,
	})

	depsCommand := &cobra.Command{
		Use:   "deps <query>",
		Short: "Analyze Rego query dependencies",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("specify exactly one query argument")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			if err := deps(args, params); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}

	depsCommand.Flags().VarP(params.format, "format", "f", "set output format")
	depsCommand.Flags().VarP(&params.dataPaths, "data", "d", "set policy or data file(s). This flag can be repeated.")
	depsCommand.Flags().VarP(&params.bundlePaths, "bundle", "b", "set bundle file(s) or directory path(s). This flag can be repeated.")
	addIgnoreFlag(depsCommand.Flags(), &params.ignore)

	RootCommand.AddCommand(depsCommand)
}

func deps(args []string, params depsCommandParams) error {

	query, err := ast.ParseBody(args[0])
	if err != nil {
		return err
	}

	modules := map[string]*ast.Module{}

	if len(params.dataPaths.v) > 0 {
		f := loaderFilter{
			Ignore: params.ignore,
		}

		result, err := loader.NewFileLoader().Filtered(params.dataPaths.v, f.Apply)
		if err != nil {
			return err
		}

		for _, m := range result.Modules {
			modules[m.Name] = m.Parsed
		}
	}

	if len(params.bundlePaths.v) > 0 {
		for _, path := range params.bundlePaths.v {
			b, err := loader.NewFileLoader().WithSkipBundleVerification(true).AsBundle(path)
			if err != nil {
				return err
			}

			for name, mod := range b.ParsedModules(path) {
				modules[name] = mod
			}
		}
	}

	compiler := ast.NewCompiler()
	compiler.Compile(modules)

	if compiler.Failed() {
		return compiler.Errors
	}

	brs, err := dependencies.Base(compiler, query)
	if err != nil {
		return err
	}

	vrs, err := dependencies.Virtual(compiler, query)
	if err != nil {
		return err
	}

	output := presentation.DepAnalysisOutput{
		Base:    brs,
		Virtual: vrs,
	}

	switch params.format.String() {
	case depsFormatJSON:
		return presentation.JSON(os.Stdout, output)
	default:
		return output.Pretty(os.Stdout)
	}
}
