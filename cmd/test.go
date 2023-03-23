// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/compile"
	"github.com/open-policy-agent/opa/cover"
	"github.com/open-policy-agent/opa/filewatcher"
	"github.com/open-policy-agent/opa/internal/runtime"
	initload "github.com/open-policy-agent/opa/internal/runtime/init"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/tester"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/lineage"
	"github.com/open-policy-agent/opa/util"
)

const (
	testPrettyOutput = "pretty"
	testJSONOutput   = "json"
)

type testCommandParams struct {
	verbose      bool
	explain      *util.EnumFlag
	errLimit     int
	outputFormat *util.EnumFlag
	coverage     bool
	threshold    float64
	timeout      time.Duration
	ignore       []string
	bundleMode   bool
	benchmark    bool
	benchMem     bool
	runRegex     string
	count        int
	target       *util.EnumFlag
	skipExitZero bool
	capabilities *capabilitiesFlag
	watch        bool
	output       io.Writer
	killChan     chan os.Signal
}

func newTestCommandParams() *testCommandParams {
	return &testCommandParams{
		outputFormat: util.NewEnumFlag(testPrettyOutput, []string{testPrettyOutput, testJSONOutput, benchmarkGoBenchOutput}),
		explain:      newExplainFlag([]string{explainModeFails, explainModeFull, explainModeNotes, explainModeDebug}),
		target:       util.NewEnumFlag(compile.TargetRego, []string{compile.TargetRego, compile.TargetWasm}),
		capabilities: newcapabilitiesFlag(),
		output:       os.Stdout,
		killChan:     make(chan os.Signal, 1),
	}
}

var testParams = newTestCommandParams()

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

Test cases under development may be prefixed "todo_" in order to skip their execution,
while still getting marked as skipped in the test results.

Example policy (example/authz.rego):

	package authz

	import future.keywords.if

	allow if {
		input.path == ["users"]
		input.method == "POST"
	}

	allow if {
		input.path == ["users", input.user_id]
		input.method == "GET"
	}

Example test (example/authz_test.rego):

	package authz_test

	import data.authz.allow

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

	todo_test_user_allowed_http_client_data {
		false # Remember to test this later!
	}

Example test run:

	$ opa test ./example/

If used with the '--bench' option then tests will be benchmarked.

Example benchmark run:

	$ opa test --bench ./example/

The optional "gobench" output format conforms to the Go Benchmark Data Format.
`,
	PreRunE: func(Cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("specify at least one file")
		}

		// If an --explain flag was set, turn on verbose output
		if testParams.explain.IsSet() {
			testParams.verbose = true
		}

		return nil
	},

	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(opaTest(args))
	},
}

func newOnReload(c chan int) filewatcher.OnReload {

	onReload := func(ctx context.Context, txn storage.Transaction, d time.Duration, s storage.Store, l *initload.LoadPathsResult, err error) {
		notify := func() {
			c <- 1
		}
		defer notify()

		if err != nil {
			fmt.Printf("error reloading files: %v\n", err)
			return
		}

		// FIXME: We don't detect when data files are removed.
		if len(l.Files.Documents) > 0 {
			if err := s.Write(ctx, txn, storage.AddOp, storage.Path{}, l.Files.Documents); err != nil {
				fmt.Printf("storage error: %v\n", err)
				return
			}
		}

		modules := map[string]*ast.Module{}
		for id, module := range l.Files.Modules {
			modules[id] = module.Parsed
		}

		compileAndRunTests(ctx, txn, s, modules, l.Bundles)
	}

	return onReload
}

func watchTests(ctx context.Context, paths []string, filter loader.Filter, bundleMode bool, store storage.Store) int {
	reloadChan := make(chan int)
	onReload := newOnReload(reloadChan)

	signal.Notify(testParams.killChan, syscall.SIGINT, syscall.SIGTERM)

	logger := logging.New()

	w := filewatcher.NewFileWatcher(paths, filter, bundleMode, store, onReload, logger)
	err := w.Start(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error", err)
		return 1
	}

	for {
		fmt.Fprintln(testParams.output, strings.Repeat("*", 80))
		fmt.Fprintln(testParams.output, "Watching for changes ...")
		select {
		case <-testParams.killChan:
			return 0
		case <-reloadChan:
			break
		}
	}
}

func opaTest(args []string) int {
	if testParams.outputFormat.String() == benchmarkGoBenchOutput && !testParams.benchmark {
		fmt.Fprintf(os.Stderr, "cannot use output format %s without running benchmarks (--bench)\n", benchmarkGoBenchOutput)
		return 0
	}

	if !isThresholdValid(testParams.threshold) {
		fmt.Fprintln(os.Stderr, "Code coverage threshold must be between 0 and 100")
		return 1
	}

	filter := loaderFilter{
		Ignore: testParams.ignore,
	}

	var store storage.Store
	var err error

	result, err := initload.LoadPaths(args, filter.Apply, testParams.bundleMode, nil, true, false, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	store = inmem.NewFromObjectWithOpts(result.Files.Documents, inmem.OptRoundTripOnWrite(false))

	modules := map[string]*ast.Module{}
	for _, m := range result.Files.Modules {
		modules[m.Name] = m.Parsed
	}

	bundles := result.Bundles

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if testParams.watch {
		compileAndRunTests(ctx, txn, store, modules, bundles)
		store.Commit(ctx, txn)
		return watchTests(ctx, args, filter.Apply, testParams.bundleMode, store)
	} else {
		defer store.Abort(ctx, txn)
		return compileAndRunTests(ctx, txn, store, modules, bundles)
	}
}

func compileAndRunTests(ctx context.Context, txn storage.Transaction, store storage.Store, modules map[string]*ast.Module, bundles map[string]*bundle.Bundle) int {

	var capabilities *ast.Capabilities
	// if capabilities are not provided as a cmd flag,
	// then ast.CapabilitiesForThisVersion must be called
	// within checkModules to ensure custom builtins are properly captured
	if testParams.capabilities.C != nil {
		capabilities = testParams.capabilities.C
	} else {
		capabilities = ast.CapabilitiesForThisVersion()
	}

	compiler := ast.NewCompiler().
		SetErrorLimit(testParams.errLimit).
		WithPathConflictsCheck(storage.NonEmpty(ctx, store, txn)).
		WithEnablePrintStatements(!testParams.benchmark).
		WithCapabilities(capabilities)

	info, err := runtime.Term(runtime.Params{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if testParams.threshold > 0 && !testParams.coverage {
		testParams.coverage = true
	}

	var cov *cover.Cover
	var coverTracer topdown.QueryTracer

	if testParams.coverage {
		if testParams.benchmark {
			fmt.Fprintln(os.Stderr, "coverage reporting is not supported when benchmarking tests")
			return 1
		}
		cov = cover.New()
		coverTracer = cov
	}

	timeout := testParams.timeout
	if timeout == 0 { // unset
		timeout = 5 * time.Second
		if testParams.benchmark {
			timeout = 30 * time.Second
		}
	}

	runner := tester.NewRunner().
		SetCompiler(compiler).
		SetStore(store).
		CapturePrintOutput(true).
		EnableTracing(testParams.verbose).
		SetCoverageQueryTracer(coverTracer).
		SetRuntime(info).
		SetModules(modules).
		SetBundles(bundles).
		SetTimeout(timeout).
		Filter(testParams.runRegex).
		Target(testParams.target.String())

	var reporter tester.Reporter

	goBench := false

	if !testParams.coverage {
		switch testParams.outputFormat.String() {
		case testJSONOutput:
			reporter = tester.JSONReporter{
				Output: os.Stdout,
			}
		case benchmarkGoBenchOutput:
			goBench = true
			fallthrough
		default:
			reporter = tester.PrettyReporter{
				Verbose:                  testParams.verbose,
				Output:                   testParams.output,
				BenchmarkResults:         testParams.benchmark,
				BenchMarkShowAllocations: testParams.benchMem,
				BenchMarkGoBenchFormat:   goBench,
			}
		}
	} else {
		reporter = tester.JSONCoverageReporter{
			Cover:     cov,
			Modules:   modules,
			Output:    testParams.output,
			Threshold: testParams.threshold,
		}
	}

	for i := 0; i < testParams.count; i++ {
		exitCode := runTests(ctx, txn, runner, reporter)
		if exitCode != 0 {
			return exitCode
		}
	}

	return 0
}

func runTests(ctx context.Context, txn storage.Transaction, runner *tester.Runner, reporter tester.Reporter) int {
	var err error
	var ch chan *tester.Result
	if testParams.benchmark {
		benchOpts := tester.BenchmarkOptions{
			ReportAllocations: testParams.benchMem,
		}
		ch, err = runner.RunBenchmarks(ctx, txn, benchOpts)
	} else {
		ch, err = runner.RunTests(ctx, txn)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	exitCode := 0
	dup := make(chan *tester.Result)

	go func() {
		defer close(dup)
		for tr := range ch {
			if !tr.Pass() && !testParams.skipExitZero {
				exitCode = 2
			}
			if tr.Skip && exitCode == 0 && testParams.skipExitZero {
				// there is a skipped test, adding the flag -z exits 0 if there are no failures
				exitCode = 0
			}
			tr.Trace = filterTrace(testParams, tr.Trace)
			dup <- tr
		}
	}()

	if err := reporter.Report(dup); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if !testParams.benchmark {
			if _, ok := err.(*cover.CoverageThresholdError); ok {
				return 2
			}
		}
		return 1
	}

	return exitCode
}

func filterTrace(params *testCommandParams, trace []*topdown.Event) []*topdown.Event {
	// If an explain mode was specified, filter based
	// on the mode. If no explain mode was specified,
	// default to show both notes and fail events
	showDefault := !params.explain.IsSet() && params.verbose
	if showDefault {
		return lineage.Filter(trace, func(event *topdown.Event) bool {
			return event.Op == topdown.NoteOp || event.Op == topdown.FailOp
		})
	}

	mode := params.explain.String()
	switch mode {
	case explainModeNotes:
		return lineage.Notes(trace)
	case explainModeFull:
		return lineage.Full(trace)
	case explainModeFails:
		return lineage.Fails(trace)
	case explainModeDebug:
		return lineage.Debug(trace)
	default:
		return nil
	}
}

func isThresholdValid(t float64) bool {
	return 0 <= t && t <= 100
}

func init() {
	testCommand.Flags().BoolVarP(&testParams.skipExitZero, "exit-zero-on-skipped", "z", false, "skipped tests return status 0")
	testCommand.Flags().BoolVarP(&testParams.verbose, "verbose", "v", false, "set verbose reporting mode")
	testCommand.Flags().DurationVar(&testParams.timeout, "timeout", 0, "set test timeout (default 5s, 30s when benchmarking)")
	testCommand.Flags().VarP(testParams.outputFormat, "format", "f", "set output format")
	testCommand.Flags().BoolVarP(&testParams.coverage, "coverage", "c", false, "report coverage (overrides debug tracing)")
	testCommand.Flags().Float64VarP(&testParams.threshold, "threshold", "", 0, "set coverage threshold and exit with non-zero status if coverage is less than threshold %")
	testCommand.Flags().BoolVar(&testParams.benchmark, "bench", false, "benchmark the unit tests")
	testCommand.Flags().StringVarP(&testParams.runRegex, "run", "r", "", "run only test cases matching the regular expression.")
	addBundleModeFlag(testCommand.Flags(), &testParams.bundleMode, false)
	addBenchmemFlag(testCommand.Flags(), &testParams.benchMem, true)
	addCountFlag(testCommand.Flags(), &testParams.count, "test")
	addMaxErrorsFlag(testCommand.Flags(), &testParams.errLimit)
	addIgnoreFlag(testCommand.Flags(), &testParams.ignore)
	setExplainFlag(testCommand.Flags(), testParams.explain)
	addTargetFlag(testCommand.Flags(), testParams.target)
	addCapabilitiesFlag(testCommand.Flags(), testParams.capabilities)
	testCommand.Flags().BoolVarP(&testParams.watch, "watch", "w", false, "watch for file changes and re-run tests")
	RootCommand.AddCommand(testCommand)
}
