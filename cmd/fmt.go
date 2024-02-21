// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/cmd/internal/env"
	"github.com/open-policy-agent/opa/format"
	fileurl "github.com/open-policy-agent/opa/internal/file/url"
)

type fmtCommandParams struct {
	overwrite    bool
	list         bool
	diff         bool
	fail         bool
	regoV1       bool
	v1Compatible bool
}

var fmtParams = fmtCommandParams{}

func (p *fmtCommandParams) regoVersion() ast.RegoVersion {
	// The '--rego-v1' flag takes precedence over the '--v1-compatible' flag.
	if p.regoV1 {
		return ast.RegoV0CompatV1
	}
	if p.v1Compatible {
		return ast.RegoV1
	}
	return ast.RegoV0
}

var formatCommand = &cobra.Command{
	Use:   "fmt [path [...]]",
	Short: "Format Rego source files",
	Long: `Format Rego source files.

The 'fmt' command takes a Rego source file and outputs a reformatted version. If no file path
is provided - this tool will use stdin.
The format of the output is not defined specifically; whatever this tool outputs
is considered correct format (with the exception of bugs).

If the '-w' option is supplied, the 'fmt' command with overwrite the source file
instead of printing to stdout.

If the '-d' option is supplied, the 'fmt' command will output a diff between the
original and formatted source.

If the '-l' option is supplied, the 'fmt' command will output the names of files
that would change if formatted. The '-l' option will suppress any other output
to stdout from the 'fmt' command.

If the '--fail' option is supplied, the 'fmt' command will return a non zero exit
code if a file would be reformatted.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return env.CmdFlags.CheckEnvironmentVariables(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(opaFmt(args))
	},
}

func opaFmt(args []string) int {

	if len(args) == 0 {
		if err := formatStdin(&fmtParams, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	for _, filename := range args {

		var err error
		filename, err = fileurl.Clean(filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		err = filepath.Walk(filename, func(path string, info os.FileInfo, err error) error {
			return formatFile(&fmtParams, os.Stdout, path, info, err)
		})
		if err != nil {
			switch err := err.(type) {
			case fmtError:
				fmt.Fprintln(os.Stderr, err.msg)
				return err.code
			default:
				fmt.Fprintln(os.Stderr, err.Error())
				return 1
			}
		}
	}

	return 0
}

func formatFile(params *fmtCommandParams, out io.Writer, filename string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if info.IsDir() {
		return nil
	}

	if filepath.Ext(filename) != ".rego" {
		return nil
	}

	contents, err := os.ReadFile(filename)
	if err != nil {
		return newError("failed to open file: %v", err)
	}

	opts := format.Opts{}
	opts.RegoVersion = params.regoVersion()
	formatted, err := format.SourceWithOpts(filename, contents, opts)
	if err != nil {
		return newError("failed to format Rego source file: %v", err)
	}

	changed := !bytes.Equal(contents, formatted)

	if params.fail && !params.list && !params.diff {
		if changed {
			return newError("unexpected diff")
		}
	}

	if params.list {
		if changed {
			fmt.Fprintln(out, filename)

			if params.fail {
				return newError("unexpected diff")
			}
		}
		return nil
	}

	if params.diff {
		if changed {
			diffString := doDiff(contents, formatted)
			if _, err := fmt.Fprintln(out, diffString); err != nil {
				return newError("failed to print contents: %v", err)
			}
			if params.fail {
				return newError("unexpected diff")
			}
		}
		return nil
	}

	if params.overwrite {
		outfile, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
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

	return nil
}

func formatStdin(params *fmtCommandParams, r io.Reader, w io.Writer) error {

	contents, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	opts := format.Opts{}
	opts.RegoVersion = params.regoVersion()
	formatted, err := format.SourceWithOpts("stdin", contents, opts)
	if err != nil {
		return err
	}

	_, err = w.Write(formatted)
	return err
}

func doDiff(old, new []byte) (diffString string) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(old), string(new), false)
	return dmp.DiffPrettyText(diffs)
}

type fmtError struct {
	msg  string
	code int
}

func (e fmtError) Error() string {
	return fmt.Sprintf("%s (%d)", e.msg, e.code)
}

func newError(msg string, a ...interface{}) fmtError {
	return fmtError{
		msg:  fmt.Sprintf(msg, a...),
		code: 2,
	}
}

func init() {
	formatCommand.Flags().BoolVarP(&fmtParams.overwrite, "write", "w", false, "overwrite the original source file")
	formatCommand.Flags().BoolVarP(&fmtParams.list, "list", "l", false, "list all files who would change when formatted")
	formatCommand.Flags().BoolVarP(&fmtParams.diff, "diff", "d", false, "only display a diff of the changes")
	formatCommand.Flags().BoolVar(&fmtParams.fail, "fail", false, "non zero exit code on reformat")
	addRegoV1FlagWithDescription(formatCommand.Flags(), &fmtParams.regoV1, false, "format module(s) to be compatible with both Rego v1 and current OPA version)")
	addV1CompatibleFlag(formatCommand.Flags(), &fmtParams.v1Compatible, false)

	RootCommand.AddCommand(formatCommand)
}
