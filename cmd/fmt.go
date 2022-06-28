// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/format"
	fileurl "github.com/open-policy-agent/opa/internal/file/url"
)

type fmtCommandParams struct {
	overwrite bool
	list      bool
	diff      bool
	fail      bool
}

var fmtParams = fmtCommandParams{}

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
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(opaFmt(args))
	},
}

func opaFmt(args []string) int {

	if len(args) == 0 {
		if err := formatStdin(os.Stdin, os.Stdout); err != nil {
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

	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return newError("failed to open file: %v", err)
	}

	formatted, err := format.Source(filename, contents)
	if err != nil {
		return newError("failed to parse Rego source file: %v", err)
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
			stdout, stderr, err := doDiff(contents, formatted)
			if err != nil && stdout.Len() == 0 {
				fmt.Fprintln(os.Stderr, stderr.String())
				return newError("failed to diff formatting: %v", err)
			}

			fmt.Fprintln(out, stdout.String())

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

func formatStdin(r io.Reader, w io.Writer) error {

	contents, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	formatted, err := format.Source("stdin", contents)
	if err != nil {
		return err
	}

	_, err = w.Write(formatted)
	return err
}

func doDiff(old, new []byte) (stdout, stderr bytes.Buffer, err error) {
	o, err := ioutil.TempFile("", ".opafmt")
	if err != nil {
		return stdout, stderr, err
	}
	defer os.Remove(o.Name())
	defer o.Close()

	n, err := ioutil.TempFile("", ".opafmt")
	if err != nil {
		return stdout, stderr, err
	}
	defer os.Remove(n.Name())
	defer n.Close()

	_, err = o.Write(old)
	if err != nil {
		return stdout, stderr, err
	}
	_, err = n.Write(new)
	if err != nil {
		return stdout, stderr, err
	}

	cmd := exec.Command("diff", "-u", o.Name(), n.Name())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	return stdout, stderr, cmd.Run()
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
	RootCommand.AddCommand(formatCommand)
}
