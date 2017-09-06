// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/tester"
	"github.com/spf13/cobra"
)

var testParams = struct {
	verbose  bool
	errLimit int
	timeout  time.Duration
}{}

var testCommand = &cobra.Command{
	Use:   "test",
	Short: "Execute Rego test cases",
	Long: `Execute Rego test cases.

The 'test' command takes a file or directory path as input and executes all
test cases discovered in matching files. Test cases are rules whose names have the prefix "test_".

Example policy (example/authz.rego):

	package authz

	allow {
		input.path = ["users"]
		input.method = "POST"
	}

	allow {
		input.path = ["users", profile_id]
		input.method = "GET"
		profile_id = input.user_id
	}

Example test (example/authz_test.rego):

	package authz

	test_post_allowed {
		allow with input as {"path": ["users"], "method": "POST"}
	}

	test_get_denied {
		not allow with input as {"path": ["users"], "method": "GET"}
	}

	test_get_user_allowed {
		allow with input as {"path": ["users", "bob"], "method": "GET", "user_id": "bob"}
	}

	test_get_another_user_denied {
		not allow with input as {"path": ["users", "bob"], "method": "GET", "user_id": "alice"}
	}

Example test run:

	$ opa test ./example/
`,
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(opaTest(args))
	},
}

func opaTest(args []string) int {

	compiler := ast.NewCompiler().SetErrorLimit(testParams.errLimit)

	ctx, cancel := context.WithTimeout(context.Background(), testParams.timeout)
	defer cancel()

	ch, err := tester.NewRunner().SetCompiler(compiler).Paths(ctx, args...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	reporter := tester.PrettyReporter{
		Verbose: testParams.verbose,
		Output:  os.Stdout,
	}

	exitCode := 0
	dup := make(chan *tester.Result)

	go func() {
		defer close(dup)
		for tr := range ch {
			if !tr.Pass() {
				exitCode = 1
			}
			dup <- tr
		}
	}()

	if err := reporter.Report(dup); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return exitCode
}

func init() {
	testCommand.Flags().BoolVarP(&testParams.verbose, "verbose", "v", false, "set verbose reporting mode")
	testCommand.Flags().DurationVarP(&testParams.timeout, "timeout", "t", time.Second*5, "set test timeout")
	setMaxErrors(testCommand.Flags(), &testParams.errLimit)
	RootCommand.AddCommand(testCommand)
}
