// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/open-policy-agent/opa/dependencies"
	"github.com/open-policy-agent/opa/internal/presentation"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/cmd/internal/env"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/util"
)

type depsCommandParams struct {
	dataPaths    repeatedStringFlag
	outputFormat *util.EnumFlag
	ignore       []string
	bundlePaths  repeatedStringFlag
	v1Compatible bool
}

func (p *depsCommandParams) regoVersion() ast.RegoVersion {
	if p.v1Compatible {
		return ast.RegoV1
	}
	return ast.RegoV0
}

const (
	depsFormatPretty = "pretty"
	depsFormatJSON   = "json"
)

func newDepsCommandParams() depsCommandParams {
	var params depsCommandParams

	params.outputFormat = util.NewEnumFlag(depsFormatPretty, []string{
		depsFormatPretty, depsFormatJSON,
	})

	return params
}

func init() {

	params := newDepsCommandParams()

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

	import rego.v1

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
			return env.CmdFlags.CheckEnvironmentVariables(cmd)
		},
		Run: func(cmd *cobra.Command, args []string) {
			if err := deps(args, params, os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}

	addIgnoreFlag(depsCommand.Flags(), &params.ignore)
	addDataFlag(depsCommand.Flags(), &params.dataPaths)
	addBundleFlag(depsCommand.Flags(), &params.bundlePaths)
	addOutputFormat(depsCommand.Flags(), params.outputFormat)
	addV1CompatibleFlag(depsCommand.Flags(), &params.v1Compatible, false)

	RootCommand.AddCommand(depsCommand)
}

func deps(args []string, params depsCommandParams, w io.Writer) error {

	query, err := ast.ParseBody(args[0])
	if err != nil {
		return err
	}

	modules := map[string]*ast.Module{}

	if len(params.dataPaths.v) > 0 {
		f := loaderFilter{
			Ignore: params.ignore,
		}

		result, err := loader.NewFileLoader().
			WithRegoVersion(params.regoVersion()).
			Filtered(params.dataPaths.v, f.Apply)
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
		return presentation.JSON(w, output)
	default:
		return output.Pretty(w)
	}
}
