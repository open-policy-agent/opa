// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/cmd/formats"
	"github.com/open-policy-agent/opa/cmd/internal/env"
	pr "github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/loader"
	"github.com/open-policy-agent/opa/v1/util"
)

type checkParams struct {
	format       *util.EnumFlag
	errLimit     int
	ignore       []string
	bundleMode   bool
	capabilities *capabilitiesFlag
	schema       *schemaFlags
	strict       bool
	regoV1       bool
	v0Compatible bool
	v1Compatible bool
}

func newCheckParams() checkParams {
	return checkParams{
		format:       formats.Flag(formats.Pretty, formats.JSON),
		capabilities: newCapabilitiesFlag(),
		schema:       &schemaFlags{},
	}
}

func (p *checkParams) regoVersion() ast.RegoVersion {
	// The '--rego-v1' flag takes precedence over the '--v1-compatible' flag.
	if p.regoV1 {
		return ast.RegoV0CompatV1
	}
	// The '--v0-compatible' flag takes precedence over the '--v1-compatible' flag.
	if p.v0Compatible {
		return ast.RegoV0
	}
	if p.v1Compatible {
		return ast.RegoV1
	}
	return ast.DefaultRegoVersion
}

func checkModules(params checkParams, args []string) error {
	// ensure custom builtins are properly captured
	capabilities := params.capabilities.C
	if capabilities == nil {
		capabilities = ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(params.regoVersion()))
	}

	l := loader.NewFileLoader().
		WithRegoVersion(params.regoVersion()).
		WithProcessAnnotation(true).
		WithCapabilities(capabilities)

	ss, err := loader.Schemas(params.schema.path)
	if err != nil {
		return err
	}

	compiler := ast.NewCompiler().
		SetErrorLimit(params.errLimit).
		WithCapabilities(capabilities).
		WithSchemas(ss).
		WithEnablePrintStatements(true).
		WithStrict(params.strict).
		WithUseTypeCheckAnnotations(true)

	var modules map[string]*ast.Module

	if params.bundleMode {
		bundles := make([]*bundle.Bundle, 0, len(args))
		for _, path := range args {
			b, err := l.WithSkipBundleVerification(true).WithFilter(filterFromPaths(params.ignore)).AsBundle(path)
			if err != nil {
				return err
			}
			bundles = append(bundles, b)
		}
		b, err := bundle.Merge(bundles)
		if err != nil {
			return err
		}

		modules = maps.Clone(b.ParsedModules(""))
		if len(b.Data) > 0 {
			compiler = compiler.WithPathConflictsCheck(mapFinder(b.Data))
		}
	} else {
		result, err := l.Filtered(args, ignoredOnlyRego(params.ignore).Apply)
		if err != nil {
			return err
		}

		modules = result.ParsedModules()
	}

	if compiler.Compile(modules); compiler.Failed() {
		return compiler.Errors
	}

	return nil
}

// emulate storage.NonEmpty without having to create storage / transaction
// returned function returns false, nil when m or path is empty
func mapFinder(m map[string]any) func(path []string) (bool, error) {
	if len(m) == 0 {
		return emptyMapFinder
	}
	return func(path []string) (bool, error) {
		if len(path) == 0 {
			return false, nil
		}

		node := m
		for _, key := range path {
			if val, ok := node[key]; ok {
				if subMap, ok := val.(map[string]any); ok {
					node = subMap
				} else {
					return true, nil
				}
			} else {
				return false, nil
			}
		}
		return true, nil
	}
}

func emptyMapFinder(path []string) (bool, error) {
	return false, nil
}

func filterFromPaths(paths []string) loader.Filter {
	return func(abspath string, info fs.FileInfo, depth int) bool {
		return ignored(paths).Apply(abspath, info, depth)
	}
}

func outputErrors(format string, err error) {
	out := os.Stdout
	if err != nil {
		out = os.Stderr
	}

	switch format {
	case formats.JSON:
		if err := pr.JSON(out, pr.Output{Errors: pr.NewOutputErrors(err)}); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
	default:
		fmt.Fprintln(out, err)
	}
}

func init() {
	checkParams := newCheckParams()

	checkCommand := &cobra.Command{
		Use:   "check <path> [path [...]]",
		Short: "Check Rego source files",
		Long: `Check Rego source files for parse and compilation errors.
	
If the 'check' command succeeds in parsing and compiling the source file(s), no output
is produced. If the parsing or compiling fails, 'check' will output the errors
and exit with a non-zero exit code.`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("specify at least one file")
			}
			return env.CmdFlags.CheckEnvironmentVariables(cmd)
		},

		Run: func(_ *cobra.Command, args []string) {
			if err := checkModules(checkParams, args); err != nil {
				outputErrors(checkParams.format.String(), err)
				os.Exit(1)
			}
		},
	}

	addMaxErrorsFlag(checkCommand.Flags(), &checkParams.errLimit)
	addIgnoreFlag(checkCommand.Flags(), &checkParams.ignore)
	addOutputFormat(checkCommand.Flags(), checkParams.format)
	addBundleModeFlag(checkCommand.Flags(), &checkParams.bundleMode, false)
	addCapabilitiesFlag(checkCommand.Flags(), checkParams.capabilities)
	addSchemaFlags(checkCommand.Flags(), checkParams.schema)
	addStrictFlag(checkCommand.Flags(), &checkParams.strict, false)
	addRegoV0V1FlagWithDescription(checkCommand.Flags(), &checkParams.regoV1, false,
		"check for Rego v0 and v1 compatibility (policies must be compatible with both Rego versions)")
	addV0CompatibleFlag(checkCommand.Flags(), &checkParams.v0Compatible, false)
	addV1CompatibleFlag(checkCommand.Flags(), &checkParams.v1Compatible, false)
	RootCommand.AddCommand(checkCommand)
}
