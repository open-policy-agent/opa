// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/internal/oracle"
	"github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/loader"
)

type findDefinitionParams struct {
	stdinBuffer bool
	bundlePaths repeatedStringFlag
}

func init() {

	var findDefinitionParams findDefinitionParams

	var oracleCommand = &cobra.Command{
		Use:    "oracle",
		Short:  "Answer questions about Rego",
		Long:   "Answer questions about Rego.",
		Hidden: true,
	}

	var findDefinitionCommand = &cobra.Command{
		Use:   "find-definition",
		Short: "Find the location of a definition",
		Long: `Find the location of a definition.

The 'find-definition' command outputs the location of the definition of the symbol
or value referred to by the location passed as a positional argument. The location
should be of the form:

	<filename>:<offset>

The offset can be specified as a decimal or hexadecimal number. The output format
specifies the file, row, and column of the definition:

	{
		"result": {
			"file": "/path/to/some/policy.rego",
			"row": 18,
			"col": 1
		}
	}

If the 'find-definition' command cannot find a location it will print an error
reason. The exit status will be zero in this case:

	{
		"error": "no match found"
	}

If an unexpected error occurs (e.g., a file read error) the subcommand will print
the error reason to stderr and exit with a non-zero status code.

If the --stdin-buffer flag is supplied the 'find-definition' subcommand will
consume stdin and treat the bytes read as the content of the file referenced
by the input location.`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("expected exactly one position <filename>:<offset>")
			}
			_, _, err := parseFilenameOffset(args[0])
			return err
		},
		Run: func(cmd *cobra.Command, args []string) {
			if err := dofindDefinition(findDefinitionParams, os.Stdin, os.Stdout, args); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
		},
	}

	findDefinitionCommand.Flags().BoolVarP(&findDefinitionParams.stdinBuffer, "stdin-buffer", "", false, "read buffer from stdin")
	addBundleFlag(findDefinitionCommand.Flags(), &findDefinitionParams.bundlePaths)
	oracleCommand.AddCommand(findDefinitionCommand)
	RootCommand.AddCommand(oracleCommand)
}

func dofindDefinition(params findDefinitionParams, stdin io.Reader, stdout io.Writer, args []string) error {

	filename, offset, err := parseFilenameOffset(args[0])
	if err != nil {
		return err
	}

	var b *bundle.Bundle

	if len(params.bundlePaths.v) != 0 {
		if len(params.bundlePaths.v) > 1 {
			return errors.New("not implemented: multiple bundle paths")
		}
		b, err = loader.NewFileLoader().
			WithSkipBundleVerification(true).
			WithFilter(func(abspath string, info os.FileInfo, depth int) bool {
				// While directories may contain other things of interest for OPA (json, yaml..),
				// only .rego will work reliably for the purpose of finding definitions
				return strings.HasPrefix(info.Name(), ".rego")
			}).
			AsBundle(params.bundlePaths.v[0])
		if err != nil {
			return err
		}
	}

	modules := map[string]*ast.Module{}

	if b != nil {
		for _, mf := range b.Modules {
			modules[mf.Path] = mf.Parsed
		}
	}

	var bs []byte

	if params.stdinBuffer {
		stat, err := os.Stdin.Stat()
		if err != nil {
			return err
		}
		// Only read from stdin when there is something actually there
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			bs, err = io.ReadAll(stdin)
			if err != nil {
				return err
			}
		}
	}

	result, err := oracle.New().FindDefinition(oracle.DefinitionQuery{
		Buffer:   bs,
		Filename: filename,
		Pos:      offset,
		Modules:  modules,
	})

	if err != nil {
		return presentation.JSON(stdout, map[string]interface{}{
			"error": err,
		})
	}

	return presentation.JSON(stdout, result)
}

func parseFilenameOffset(s string) (string, int, error) {
	s = strings.TrimPrefix(s, "file://")

	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "", 0, errors.New("expected <filename>:<offset> argument")
	}

	base := 10
	str := parts[1]
	if strings.HasPrefix(parts[1], "0x") {
		base = 16
		str = parts[1][2:]
	}

	offset, err := strconv.ParseInt(str, base, 32)
	if err != nil {
		return "", 0, err
	}

	return parts[0], int(offset), nil
}
