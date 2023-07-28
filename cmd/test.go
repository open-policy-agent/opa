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

	"github.com/fsnotify/fsnotify"
	"github.com/open-policy-agent/opa/internal/pathwatcher"
	initload "github.com/open-policy-agent/opa/internal/runtime/init"
	"github.com/open-policy-agent/opa/loader"
	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/compile"
	"github.com/open-policy-agent/opa/cover"
	"github.com/open-policy-agent/opa/internal/runtime"
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
	schema       *schemaFlags
	watch        bool
	stopChan     chan os.Signal
	output       io.Writer
	errOutput    io.Writer
}

func newTestCommandParams() testCommandParams {
	return testCommandParams{
		outputFormat: util.NewEnumFlag(testPrettyOutput, []string{testPrettyOutput, testJSONOutput, benchmarkGoBenchOutput}),
		explain:      newExplainFlag([]string{explainModeFails, explainModeFull, explainModeNotes, explainModeDebug}),
		target:       util.NewEnumFlag(compile.TargetRego, []string{compile.TargetRego, compile.TargetWasm}),
		capabilities: newcapabilitiesFlag(),
		schema:       &schemaFlags{},
		output:       os.Stdout,
		errOutput:    os.Stderr,
		stopChan:     make(chan os.Signal, 1),
	}
}

func opaTest(args []string, testParams testCommandParams) (int, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	if testParams.outputFormat.String() == benchmarkGoBenchOutput && !testParams.benchmark {
		errMsg := "cannot use output format %s without running benchmarks (--bench)\n"
		fmt.Fprintf(testParams.errOutput, errMsg, benchmarkGoBenchOutput)
		return 0, fmt.Errorf(errMsg, benchmarkGoBenchOutput)
	}

	if !isThresholdValid(testParams.threshold) {
		fmt.Fprintln(testParams.errOutput, "Code coverage threshold must be between 0 and 100")
		return 1, err
	}

	filter := loaderFilter{
		Ignore: testParams.ignore,
	}

	var modules map[string]*ast.Module
	var bundles map[string]*bundle.Bundle
	var store storage.Store

	if testParams.bundleMode {
		bundles, err = tester.LoadBundles(args, filter.Apply)
		store = inmem.NewWithOpts(inmem.OptRoundTripOnWrite(false))
	} else {
		modules, store, err = tester.Load(args, filter.Apply)
	}

	if err != nil {
		fmt.Fprintln(testParams.errOutput, err)
		return 1, err
	}

	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		fmt.Fprintln(testParams.errOutput, err)
		return 1, err
	}

	runner, reporter, err := compileAndSetupTests(ctx, testParams, store, txn, modules, bundles)
	if err != nil {
		store.Abort(ctx, txn)
		fmt.Fprintln(testParams.errOutput, err)
		return 1, err
	}

	success := true
	for i := 0; i < testParams.count; i++ {
		exitCode, err := runTests(ctx, txn, runner, reporter, testParams)
		if exitCode != 0 {
			success = false
			store.Abort(ctx, txn)
			if testParams.watch {
				break
			}
			return exitCode, err
		}
	}

	if success {
		store.Abort(ctx, txn)
	}

	if !testParams.watch {
		return 0, nil
	}

	done := make(chan struct{})
	go startWatcher(ctx, testParams, args, inmem.NewWithOpts(inmem.OptRoundTripOnWrite(false)), done)

	signal.Notify(testParams.stopChan, syscall.SIGINT, syscall.SIGTERM)

	<-testParams.stopChan
	done <- struct{}{}
	return 0, nil
}

func runTests(ctx context.Context, txn storage.Transaction, runner *tester.Runner, reporter tester.Reporter, testParams testCommandParams) (int, error) {
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
		fmt.Fprintln(testParams.errOutput, err)
		return 1, err
	}

	exitCode := 0
	dup := make(chan *tester.Result)

	go func() {
		defer close(dup)
		for tr := range ch {
			if !tr.Pass() {
				if !(tr.Skip && testParams.skipExitZero) {
					exitCode = 2
				}
			}
			tr.Trace = filterTrace(&testParams, tr.Trace)
			dup <- tr
		}
	}()

	if err := reporter.Report(dup); err != nil {
		fmt.Fprintln(testParams.errOutput, err)
		if !testParams.benchmark {
			if _, ok := err.(*cover.CoverageThresholdError); ok {
				return 2, err
			}
		}
		return 1, err
	}

	return exitCode, err
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

func startWatcher(ctx context.Context, testParams testCommandParams, paths []string, store storage.Store, done chan struct{}) {
	watcher, err := pathwatcher.CreatePathWatcher(paths)
	if err != nil {
		fmt.Fprintln(testParams.errOutput, "Error creating path watcher: ", err)
		os.Exit(1)
	}
	readWatcher(ctx, testParams, watcher, paths, store, done)
}

func readWatcher(ctx context.Context, testParams testCommandParams, watcher *fsnotify.Watcher, paths []string, store storage.Store, done chan struct{}) {
	for {

		fmt.Fprintln(testParams.output, strings.Repeat("*", 80))
		fmt.Fprintln(testParams.output, "Watching for changes ...")
		select {
		case evt := <-watcher.Events:
			removalMask := fsnotify.Remove | fsnotify.Rename
			mask := fsnotify.Create | fsnotify.Write | removalMask
			if (evt.Op & mask) != 0 {
				removed := ""
				if (evt.Op & removalMask) != 0 {
					removed = evt.Name
				}
				processWatcherUpdate(ctx, testParams, paths, removed, store)
			}
		case <-done:
			watcher.Close()
			return
		}
	}
}

func processWatcherUpdate(ctx context.Context, testParams testCommandParams, paths []string, removed string, store storage.Store) {
	filter := loaderFilter{
		Ignore: testParams.ignore,
	}

	var loadResult *initload.LoadPathsResult

	err := pathwatcher.ProcessWatcherUpdate(ctx, paths, removed, store, filter.Apply, testParams.bundleMode,
		func(ctx context.Context, txn storage.Transaction, loaded *initload.LoadPathsResult) error {
			if len(loaded.Files.Documents) > 0 || removed != "" {
				if err := store.Write(ctx, txn, storage.AddOp, storage.Path{}, loaded.Files.Documents); err != nil {
					return fmt.Errorf("storage error: %w", err)
				}
			}

			loadResult = loaded

			return nil
		})

	if err != nil {
		fmt.Fprintln(testParams.output, err)
		return
	}

	modules := map[string]*ast.Module{}
	for id, module := range loadResult.Files.Modules {
		modules[id] = module.Parsed
	}

	err = storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		runner, reporter, err := compileAndSetupTests(ctx, testParams, store, txn, modules, loadResult.Bundles)
		if err != nil {
			return err
		}

		for i := 0; i < testParams.count; i++ {
			exitCode, err := runTests(ctx, txn, runner, reporter, testParams)
			if exitCode != 0 {
				return err
			}
		}
		return nil
	})

	if err != nil {
		fmt.Fprintln(testParams.output, err)
	}
}

func compileAndSetupTests(ctx context.Context, testParams testCommandParams, store storage.Store, txn storage.Transaction, modules map[string]*ast.Module, bundles map[string]*bundle.Bundle) (*tester.Runner, tester.Reporter, error) {

	var capabilities *ast.Capabilities
	// if capabilities are not provided as a cmd flag,
	// then ast.CapabilitiesForThisVersion must be called
	// within checkModules to ensure custom builtins are properly captured
	if testParams.capabilities.C != nil {
		capabilities = testParams.capabilities.C
	} else {
		capabilities = ast.CapabilitiesForThisVersion()
	}

	//	-s {file} (one input schema file)
	//	-s {directory} (one schema directory with input and data schema files)
	schemaSet, err := loader.Schemas(testParams.schema.path)
	if err != nil {
		return nil, nil, err
	}

	compiler := ast.NewCompiler().
		SetErrorLimit(testParams.errLimit).
		WithPathConflictsCheck(storage.NonEmpty(ctx, store, txn)).
		WithEnablePrintStatements(!testParams.benchmark).
		WithCapabilities(capabilities).
		WithSchemas(schemaSet).
		WithUseTypeCheckAnnotations(true)

	info, err := runtime.Term(runtime.Params{})
	if err != nil {
		return nil, nil, err
	}

	if testParams.threshold > 0 && !testParams.coverage {
		testParams.coverage = true
	}

	var cov *cover.Cover
	var coverTracer topdown.QueryTracer

	if testParams.coverage {
		if testParams.benchmark {
			errMsg := "coverage reporting is not supported when benchmarking tests"
			fmt.Fprintln(testParams.errOutput, errMsg)
			return nil, nil, fmt.Errorf(errMsg)
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
				Output: testParams.output,
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

	return runner, reporter, nil
}

func init() {
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

The --watch flag can be used to monitor policy and data file-system changes. When a change is detected, OPA reloads
the policy and data and then re-runs the tests. Watching individual files (rather than directories) is generally not
recommended as some updates might cause them to be dropped by OPA.
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
			exitCode, _ := opaTest(args, testParams)
			os.Exit(exitCode)
		},
	}

	// Test specific flags
	testCommand.Flags().BoolVarP(&testParams.skipExitZero, "exit-zero-on-skipped", "z", false, "skipped tests return status 0")
	testCommand.Flags().BoolVarP(&testParams.verbose, "verbose", "v", false, "set verbose reporting mode")
	testCommand.Flags().DurationVar(&testParams.timeout, "timeout", 0, "set test timeout (default 5s, 30s when benchmarking)")
	testCommand.Flags().VarP(testParams.outputFormat, "format", "f", "set output format")
	testCommand.Flags().BoolVarP(&testParams.coverage, "coverage", "c", false, "report coverage (overrides debug tracing)")
	testCommand.Flags().Float64VarP(&testParams.threshold, "threshold", "", 0, "set coverage threshold and exit with non-zero status if coverage is less than threshold %")
	testCommand.Flags().BoolVar(&testParams.benchmark, "bench", false, "benchmark the unit tests")
	testCommand.Flags().StringVarP(&testParams.runRegex, "run", "r", "", "run only test cases matching the regular expression.")
	testCommand.Flags().BoolVarP(&testParams.watch, "watch", "w", false, "watch command line files for changes")

	// Shared flags
	addBundleModeFlag(testCommand.Flags(), &testParams.bundleMode, false)
	addBenchmemFlag(testCommand.Flags(), &testParams.benchMem, true)
	addCountFlag(testCommand.Flags(), &testParams.count, "test")
	addMaxErrorsFlag(testCommand.Flags(), &testParams.errLimit)
	addIgnoreFlag(testCommand.Flags(), &testParams.ignore)
	setExplainFlag(testCommand.Flags(), testParams.explain)
	addTargetFlag(testCommand.Flags(), testParams.target)
	addCapabilitiesFlag(testCommand.Flags(), testParams.capabilities)
	addSchemaFlags(testCommand.Flags(), testParams.schema)

	RootCommand.AddCommand(testCommand)
}
