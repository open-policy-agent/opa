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
	dataPaths    repeatedStringFlag
	outputFormat *util.EnumFlag
	ignore       []string
	bundlePaths  repeatedStringFlag
}

const (
	depsFormatPretty = "pretty"
	depsFormatJSON   = "json"
)

func init() {

	var params depsCommandParams

	params.outputFormat = util.NewEnumFlag(depsFormatPretty, []string{
		depsFormatPretty, depsFormatJSON,
	})

	depsCommand := &cobra.Command{
		Use:   "deps <query>",
		Short: "Analyze Rego query dependencies",
		Long: `Print dependencies of provided query.

Dependencies are categorized as either base documents, which is any data loaded
from the outside world, or virtual documents, i.e values that are computed from rules.

Example
-------
Given a policy like this:

	package policy

	import future.keywords.if
	import future.keywords.in

	allow if is_admin

	is_admin if "admin" in input.user.roles

To evaluate the dependencies of a simple query (e.g. data.policy.allow),
we'd run opa deps like demonstrated below:

	$ opa deps --data policy.rego data.policy.allow
	+------------------+----------------------+
	|  BASE DOCUMENTS  |  VIRTUAL DOCUMENTS   |
	+------------------+----------------------+
	| input.user.roles | data.policy.allow    |
	|                  | data.policy.is_admin |
	+------------------+----------------------+

From the output we're able to determine that the allow rule depends on
the input.user.roles base document, as well as the virtual document (rule)
data.policy.is_admin.
`,
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

	addIgnoreFlag(depsCommand.Flags(), &params.ignore)
	addDataFlag(depsCommand.Flags(), &params.dataPaths)
	addBundleFlag(depsCommand.Flags(), &params.bundlePaths)
	addOutputFormat(depsCommand.Flags(), params.outputFormat)

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

	switch params.outputFormat.String() {
	case depsFormatJSON:
		return presentation.JSON(os.Stdout, output)
	default:
		return output.Pretty(os.Stdout)
	}
}
