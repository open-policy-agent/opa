// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/open-policy-agent/opa/format"

	"github.com/spf13/cobra"
)

var (
	overwrite bool
	list      bool
	diff      bool
)

var formatCommand = &cobra.Command{
	Use:   "fmt",
	Short: "Format Rego source code",
	Long: `Format Rego source code.

The 'fmt' command takes a Rego source file and outputs a reformatted version.
The format of the output is not defined specifically; whatever this tool outputs
is considered correct format (with the exception of bugs).

If the '-w' option is supplied, the 'fmt' command with overwrite the source file
instead of printing to stdout.

If the '-d' option is supplied, the 'fmt' command will output a diff between the
original and formatted source.

If the '-l' option is suppled, the 'fmt' command will output the names of files
that would change if formatted. The '-l' option will suppress any other output
to stdout from the 'fmt' command.`,
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(opaFmt(args))
	},
}

func opaFmt(args []string) int {
	for _, filename := range args {
		if err := filepath.Walk(filename, formatFile); err != nil {
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

func formatFile(filename string, info os.FileInfo, err error) error {
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

	if bytes.Equal(formatted, contents) {
		return nil
	}

	var out io.Writer = os.Stdout
	if list {
		fmt.Fprintln(out, filename)
		out = ioutil.Discard
	}

	if diff {
		stdout, stderr, err := doDiff(contents, formatted)
		if err != nil && stdout.Len() == 0 {
			fmt.Fprintln(os.Stderr, stderr.String())
			return newError("failed to diff formatting: %v", err)
		}

		fmt.Fprintln(out, stdout.String())

		// If we called diff, we shouldn't output to stdout.
		out = ioutil.Discard
	}

	if overwrite {
		backupName := filename + ".bak"
		if _, err := os.Stat(backupName); err == nil || !os.IsNotExist(err) {
			backupName, err = requestBackupName(backupName)
			if err != nil {
				return newError("failed to read user input: %v", err)
			}
		}

		bak, err := os.OpenFile(backupName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return newError("failed to open backup file for writing: %v", err)
		}
		defer bak.Close()

		if _, err := bak.Write(contents); err != nil {
			return newError("failed to write to backup file: %v", err)
		}

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

func requestBackupName(old string) (string, error) {
	r := bufio.NewReader(os.Stdin)
	fmt.Printf("Backup file (%s) already exists. Enter a name for the backup file (or blank to overwrite): ", old)
	resp, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}

	resp = strings.TrimSpace(resp)
	if resp == "" {
		return old, nil
	}
	return resp, nil
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

	o.Write(old)
	n.Write(new)

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
	formatCommand.Flags().BoolVarP(&overwrite, "write", "w", false, "overwrite the original source file")
	formatCommand.Flags().BoolVarP(&list, "list", "l", false, "list all files who would change when formatted")
	formatCommand.Flags().BoolVarP(&diff, "diff", "d", false, "only display a diff of the changes")
	RootCommand.AddCommand(formatCommand)
}
