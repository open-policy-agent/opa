// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/cmd/formats"
	"github.com/open-policy-agent/opa/cmd/internal/env"
	pr "github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/v1/ast"
	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/loader"
	"github.com/open-policy-agent/opa/v1/util"
)

type parseParams struct {
	format       *util.EnumFlag
	jsonInclude  string
	v0Compatible bool
	v1Compatible bool
}

func (p *parseParams) regoVersion() ast.RegoVersion {
	// the '--v0--compatible' flag takes precedence over the '--v1-compatible' flag
	if p.v0Compatible {
		return ast.RegoV0
	}
	if p.v1Compatible {
		return ast.RegoV1
	}
	return ast.DefaultRegoVersion
}

var configuredParseParams = parseParams{
	format:      formats.Flag(formats.Pretty, formats.JSON),
	jsonInclude: "",
}

var parseCommand = &cobra.Command{
	Use:   "parse <path>",
	Short: "Parse Rego source file",
	Long:  `Parse Rego source file and print AST.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("no source file specified")
		}
		return env.CmdFlags.CheckEnvironmentVariables(cmd)
	},
	Run: func(_ *cobra.Command, args []string) {
		os.Exit(parse(args, &configuredParseParams, os.Stdout, os.Stderr))
	},
}

func parse(args []string, params *parseParams, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		return 0
	}

	exposeLocation := false
	exposeComments := true
	for _, opt := range strings.Split(params.jsonInclude, ",") {
		value := !strings.HasPrefix(opt, "-")

		if strings.HasSuffix(opt, "locations") {
			exposeLocation = value
		}
		if strings.HasSuffix(opt, "comments") {
			exposeComments = value
		}
	}

	parserOpts := ast.ParserOptions{
		ProcessAnnotation: true,
		RegoVersion:       params.regoVersion(),
	}
	if exposeLocation {
		astJSON.SetOptions(astJSON.Options{
			MarshalOptions: astJSON.MarshalOptions{
				IncludeLocationText: true,
				IncludeLocation: astJSON.NodeToggle{
					Term:           true,
					Package:        true,
					Comment:        true,
					Import:         true,
					Rule:           true,
					Head:           true,
					Expr:           true,
					SomeDecl:       true,
					Every:          true,
					With:           true,
					Annotations:    true,
					AnnotationsRef: true,
				},
			},
		})
		defer astJSON.SetOptions(astJSON.Defaults())
	}

	result, err := loader.RegoWithOpts(args[0], parserOpts)
	if err != nil {
		_ = pr.JSON(stderr, pr.Output{Errors: pr.NewOutputErrors(err)})
		return 1
	}

	if !exposeComments {
		result.Parsed.Comments = nil
	}

	switch params.format.String() {
	case formats.JSON:
		bs, err := json.MarshalIndent(result.Parsed, "", "  ")
		if err != nil {
			_ = pr.JSON(stderr, pr.Output{Errors: pr.NewOutputErrors(err)})
			return 1
		}

		_, _ = fmt.Fprint(stdout, string(bs)+"\n")
	default:
		ast.Pretty(stdout, result.Parsed)
	}

	return 0
}

func init() {
	addOutputFormat(parseCommand.Flags(), configuredParseParams.format)
	parseCommand.Flags().StringVarP(&configuredParseParams.jsonInclude, "json-include", "", "", "include or exclude optional elements. By default comments are included. Current options: locations, comments. E.g. --json-include locations,-comments will include locations and exclude comments.")
	addV1CompatibleFlag(parseCommand.Flags(), &configuredParseParams.v1Compatible, false)
	addV0CompatibleFlag(parseCommand.Flags(), &configuredParseParams.v0Compatible, false)

	RootCommand.AddCommand(parseCommand)
}
