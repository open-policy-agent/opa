// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/open-policy-agent/opa/storage/inmem"

	"github.com/open-policy-agent/opa/bundle"

	"github.com/open-policy-agent/opa/internal/runtime"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown/lineage"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/cover"
	"github.com/open-policy-agent/opa/tester"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
	"github.com/spf13/cobra"
)

const (
	testPrettyOutput = "pretty"
	testJSONOutput   = "json"
)

var testParams = struct {
	verbose      bool
	explain      *util.EnumFlag
	errLimit     int
	outputFormat *util.EnumFlag
	coverage     bool
	threshold    float64
	timeout      time.Duration
	ignore       []string
	failureLine  bool
	bundleMode   bool
}{
	outputFormat: util.NewEnumFlag(testPrettyOutput, []string{testPrettyOutput, testJSONOutput}),
	explain:      newExplainFlag([]string{explainModeFails, explainModeFull, explainModeNotes}),
}

var testCommand = &cobra.Command{
	Use:   "test <path> [path [...]]",
	Short: "Execute Rego test cases",
	Long: `Execute Rego test cases.

The 'test' command takes a file or directory path as input and executes all
test cases discovered in matching files. Test cases are rules whose names have the prefix "test_".

If the '--bundle' option is specified the paths will be treated as policy bundles
and loaded following standard bundle conventions. The path can be a compressed archive
file or a directory which will be treated as a bundle. Without the '--bundle' flag OPA
will recursively load ALL *.rego, *.json, and *.yaml files for evaluating the test cases.
	
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
	PreRunE: func(Cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("specify at least one file")
		}
		return nil
	},

	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(opaTest(args))
	},
}

func opaTest(args []string) int {
	ctx, cancel := context.WithTimeout(context.Background(), testParams.timeout)
	defer cancel()

	filter := loaderFilter{
		Ignore: testParams.ignore,
	}

	var modules map[string]*ast.Module
	var bundles map[string]*bundle.Bundle
	var store storage.Store
	var err error

	if testParams.bundleMode {
		bundles, err = tester.LoadBundles(args, filter.Apply)
		store = inmem.New()
	} else {
		modules, store, err = tester.Load(args, filter.Apply)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	defer store.Abort(ctx, txn)

	compiler := ast.NewCompiler().
		SetErrorLimit(testParams.errLimit).
		WithPathConflictsCheck(storage.NonEmpty(ctx, store, txn))

	info, err := runtime.Term(runtime.Params{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if testParams.threshold > 0 && !testParams.coverage {
		testParams.coverage = true
	}

	var cov *cover.Cover
	var coverTracer topdown.Tracer

	if testParams.coverage {
		cov = cover.New()
		coverTracer = cov
	}

	runner := tester.NewRunner().
		SetCompiler(compiler).
		SetStore(store).
		EnableTracing(testParams.verbose).
		SetCoverageTracer(coverTracer).
		EnableFailureLine(testParams.failureLine).
		SetRuntime(info).
		SetModules(modules).
		SetBundles(bundles)

	ch, err := runner.RunTests(ctx, txn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	var reporter tester.Reporter

	if !testParams.coverage {
		switch testParams.outputFormat.String() {
		case testJSONOutput:
			reporter = tester.JSONReporter{
				Output: os.Stdout,
			}
		default:
			reporter = tester.PrettyReporter{
				Verbose:     testParams.verbose,
				FailureLine: testParams.failureLine,
				Output:      os.Stdout,
			}
		}
	} else {
		reporter = tester.JSONCoverageReporter{
			Cover:     cov,
			Modules:   modules,
			Output:    os.Stdout,
			Threshold: testParams.threshold,
		}
	}

	exitCode := 0
	dup := make(chan *tester.Result)

	go func() {
		defer close(dup)
		for tr := range ch {
			if !tr.Pass() {
				exitCode = 2
			}
			switch testParams.explain.String() {
			case explainModeNotes:
				tr.Trace = lineage.Notes(tr.Trace)
			case explainModeFails:
				tr.Trace = lineage.Fails(tr.Trace)
			}
			dup <- tr
		}
	}()

	if err := reporter.Report(dup); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if _, ok := err.(*cover.CoverageThresholdError); ok {
			return 2
		}
		return 1
	}

	return exitCode
}

func init() {
	testCommand.Flags().BoolVarP(&testParams.verbose, "verbose", "v", false, "set verbose reporting mode")
	testCommand.Flags().BoolVarP(&testParams.failureLine, "show-failure-line", "l", false, "show test failure line")
	testCommand.Flags().DurationVarP(&testParams.timeout, "timeout", "t", time.Second*5, "set test timeout")
	testCommand.Flags().VarP(testParams.outputFormat, "format", "f", "set output format")
	testCommand.Flags().BoolVarP(&testParams.coverage, "coverage", "c", false, "report coverage (overrides debug tracing)")
	testCommand.Flags().Float64VarP(&testParams.threshold, "threshold", "", 0, "set coverage threshold and exit with non-zero status if coverage is less than threshold %")
	testCommand.Flags().BoolVarP(&testParams.bundleMode, "bundle", "b", false, "load paths as bundle files or root directories")
	setMaxErrors(testCommand.Flags(), &testParams.errLimit)
	setIgnore(testCommand.Flags(), &testParams.ignore)
	setExplain(testCommand.Flags(), testParams.explain)
	RootCommand.AddCommand(testCommand)
}
