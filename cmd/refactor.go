// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/format"
	fileurl "github.com/open-policy-agent/opa/internal/file/url"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/refactor"
)

type moveCommandParams struct {
	mapping   repeatedStringFlag
	ignore    []string
	overwrite bool
}

func init() {

	var moveCommandParams moveCommandParams

	var refactorCommand = &cobra.Command{
		Use:    "refactor",
		Short:  "Refactor Rego file(s)",
		Hidden: true,
	}

	var moveCommand = &cobra.Command{
		Use:   "move [file-path [...]]",
		Short: "Rename packages and their references in Rego file(s)",
		Long: `Rename packages and their references in Rego file(s).

The 'move' command takes one or more Rego source file(s) and rewrites package paths and other references in them as per
the mapping defined by the '-p' option. At least one mapping should be provided and should be of the form:

	<from>:<to>

The 'move' command formats the Rego modules after renaming packages, etc. and prints the formatted modules to stdout by default.
If the '-w' option is supplied, the 'move' command will overwrite the source file instead.

Example:
--------

"policy.rego" contains the below policy:
 _ _ _ _ _ _ _ _ _ _ _ _ _
| package lib.foo         |
|                         |
| default allow = false   |
| _ _ _ _ _ _ _ _ _ _ _ _ |     
	
	$ opa refactor move -p data.lib.foo:data.baz.bar policy.rego

The 'move' command outputs the below policy to stdout with the package name rewritten as per the mapping:

 _ _ _ _ _ _ _ _ _ _ _ _ _
| package baz.bar         |
|                         |
| default allow = false   |
| _ _ _ _ _ _ _ _ _ _ _ _ | 
`,
		PreRunE: func(_ *cobra.Command, args []string) error {
			return validateMoveArgs(args)
		},
		Run: func(cmd *cobra.Command, args []string) {
			if err := doMove(moveCommandParams, args, os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
		},
	}

	moveCommand.Flags().VarP(&moveCommandParams.mapping, "path", "p", "set the mapping that defines how references should be rewritten (ie. <from>:<to>). This flag can be repeated.")
	moveCommand.Flags().BoolVarP(&moveCommandParams.overwrite, "write", "w", false, "overwrite the original source file")
	addIgnoreFlag(moveCommand.Flags(), &moveCommandParams.ignore)
	refactorCommand.AddCommand(moveCommand)
	RootCommand.AddCommand(refactorCommand)
}

func doMove(params moveCommandParams, args []string, out io.Writer) error {
	if len(params.mapping.v) == 0 {
		return errors.New("specify at least one mapping of the form <from>:<to>")
	}

	srcDstMap, err := parseSrcDstMap(params.mapping.v)
	if err != nil {
		return err
	}

	modules := map[string]*ast.Module{}

	f := loaderFilter{
		Ignore: params.ignore,
	}

	result, err := loader.NewFileLoader().Filtered(args, f.Apply)
	if err != nil {
		return err
	}

	for _, m := range result.Modules {
		modules[m.Name] = m.Parsed
	}

	mq := refactor.MoveQuery{
		Modules:       modules,
		SrcDstMapping: srcDstMap,
	}.WithValidation(true)

	movedModules, err := refactor.New().Move(mq)
	if err != nil {
		return err
	}

	for filename, mod := range movedModules.Result {
		filename, err = fileurl.Clean(filename)
		if err != nil {
			return err
		}

		formatted, err := format.Ast(mod)
		if err != nil {
			return newError("failed to parse Rego source file: %v", err)
		}

		if params.overwrite {
			info, err := os.Stat(filename)
			if err != nil {
				return err
			}

			outfile, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, info.Mode())
			if err != nil {
				return newError("failed to open file for writing: %v", err)
			}
			defer outfile.Close()
			out = outfile
		}

		_, err = out.Write(formatted)
		if err != nil {
			return newError("failed writing formatted contents: %v", err)
		}
	}

	return nil
}

func parseSrcDstMap(data []string) (map[string]string, error) {
	result := map[string]string{}

	for _, d := range data {
		parts := strings.Split(d, ":")
		if len(parts) != 2 {
			return nil, errors.New("expected mapping of the form <from>:<to>")
		}

		result[parts[0]] = parts[1]
	}
	return result, nil
}

func validateMoveArgs(args []string) error {
	if len(args) == 0 {
		return errors.New("specify at least one path containing policy files")
	}
	return nil
}
