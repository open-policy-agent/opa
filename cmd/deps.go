// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"

	"github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/v1/dependencies"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/cmd/formats"
	"github.com/open-policy-agent/opa/cmd/internal/env"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/loader"
	"github.com/open-policy-agent/opa/v1/util"
)

type depsCommandParams struct {
	dataPaths    repeatedStringFlag
	outputFormat *util.EnumFlag
	ignore       []string
	bundlePaths  repeatedStringFlag
	v0Compatible bool
	v1Compatible bool
}

func (p *depsCommandParams) regoVersion() ast.RegoVersion {
	// The '--v0-compatible' flag takes precedence over the '--v1-compatible' flag.
	if p.v0Compatible {
		return ast.RegoV0
	}
	if p.v1Compatible {
		return ast.RegoV1
	}
	return ast.DefaultRegoVersion
}

func newDepsCommandParams() depsCommandParams {
	return depsCommandParams{
		outputFormat: formats.Flag(formats.Pretty, formats.JSON),
	}
}

func init() {
	params := newDepsCommandParams()

	depsCommand := &cobra.Command{
		Use:   "deps <query>",
		Short: "Analyze Rego query dependencies",
		Long: `Print dependencies of provided query.

Dependencies are categorized as either base documents, which is any data loaded
from the outside world, or virtual documents, i.e values that are computed from rules.
`,

		Example: `
Given a policy like this:

	package policy

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
		Run: func(_ *cobra.Command, args []string) {
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

	var modules map[string]*ast.Module

	if len(params.dataPaths.v) > 0 {
		result, err := loader.NewFileLoader().
			WithBundleLazyLoadingMode(bundle.HasExtension()).
			WithRegoVersion(params.regoVersion()).
			Filtered(params.dataPaths.v, ignored(params.ignore).Apply)
		if err != nil {
			return err
		}

		modules = result.ParsedModules()
	}

	if len(params.bundlePaths.v) > 0 {
		modules = make(map[string]*ast.Module, len(params.bundlePaths.v))
		for _, path := range params.bundlePaths.v {
			b, err := loader.NewFileLoader().WithBundleLazyLoadingMode(bundle.HasExtension()).WithSkipBundleVerification(true).AsBundle(path)
			if err != nil {
				return err
			}

			maps.Copy(modules, b.ParsedModules(path))
		}
	}

	compiler := ast.NewCompiler()
	if compiler.Compile(modules); compiler.Failed() {
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
	case formats.JSON:
		return presentation.JSON(w, output)
	default:
		return output.Pretty(w)
	}
}
