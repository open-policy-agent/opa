// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"

	"github.com/open-policy-agent/opa/server/types"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/runtime"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/compile"
	"github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/util"
)

// benchmarkCommandParams are a superset of evalCommandParams
// but not all eval options are exposed with flags. Only the
// ones compatible with running a benchmark.
type benchmarkCommandParams struct {
	evalCommandParams
	benchMem               bool
	count                  int
	e2e                    bool
	gracefulShutdownPeriod int
	shutdownWaitPeriod     int
	configFile             string
}

const (
	benchmarkGoBenchOutput = "gobench"
)

func newBenchmarkEvalParams() benchmarkCommandParams {
	return benchmarkCommandParams{
		evalCommandParams: evalCommandParams{
			outputFormat: util.NewEnumFlag(evalPrettyOutput, []string{
				evalJSONOutput,
				evalPrettyOutput,
				benchmarkGoBenchOutput,
			}),
			target: util.NewEnumFlag(compile.TargetRego, []string{compile.TargetRego, compile.TargetWasm}),
			schema: &schemaFlags{},
		},
		gracefulShutdownPeriod: 10,
	}
}

func init() {
	params := newBenchmarkEvalParams()

	benchCommand := &cobra.Command{
		Use:   "bench <query>",
		Short: "Benchmark a Rego query",
		Long: `Benchmark a Rego query and print the results.

The benchmark command works very similar to 'eval' and will evaluate the query in the same fashion. The
evaluation will be repeated a number of times and performance results will be returned.

Example with bundle and input data:

	opa bench -b ./policy-bundle -i input.json 'data.authz.allow'

To enable more detailed analysis use the --metrics and --benchmem flags.

To run benchmarks against a running OPA server to evaluate server overhead use the --e2e flag.

The optional "gobench" output format conforms to the Go Benchmark Data Format.
`,

		PreRunE: func(_ *cobra.Command, args []string) error {
			return validateEvalParams(&params.evalCommandParams, args)
		},
		Run: func(_ *cobra.Command, args []string) {
			exit, err := benchMain(args, params, os.Stdout, &goBenchRunner{})
			if err != nil {
				// NOTE: err should only be non-nil if a (highly unlikely)
				// presentation error occurs.
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			os.Exit(exit)
		},
	}

	// Sub-set of the standard `opa eval ..` flags
	addPartialFlag(benchCommand.Flags(), &params.partial, false)
	addUnknownsFlag(benchCommand.Flags(), &params.unknowns, []string{"input"})
	addFailFlag(benchCommand.Flags(), &params.fail, true)
	addDataFlag(benchCommand.Flags(), &params.dataPaths)
	addBundleFlag(benchCommand.Flags(), &params.bundlePaths)
	addInputFlag(benchCommand.Flags(), &params.inputPath)
	addImportFlag(benchCommand.Flags(), &params.imports)
	addPackageFlag(benchCommand.Flags(), &params.pkg)
	addQueryStdinFlag(benchCommand.Flags(), &params.stdin)
	addInputStdinFlag(benchCommand.Flags(), &params.stdinInput)
	addMetricsFlag(benchCommand.Flags(), &params.metrics, true)
	addOutputFormat(benchCommand.Flags(), params.outputFormat)
	addIgnoreFlag(benchCommand.Flags(), &params.ignore)
	addSchemaFlags(benchCommand.Flags(), params.schema)
	addTargetFlag(benchCommand.Flags(), params.target)

	// Shared benchmark flags
	addCountFlag(benchCommand.Flags(), &params.count, "benchmark")
	addBenchmemFlag(benchCommand.Flags(), &params.benchMem, true)

	addE2EFlag(benchCommand.Flags(), &params.e2e, false)
	addConfigFileFlag(benchCommand.Flags(), &params.configFile)

	benchCommand.Flags().IntVar(&params.gracefulShutdownPeriod, "shutdown-grace-period", 10, "set the time (in seconds) that the server will wait to gracefully shut down. This flag is valid in 'e2e' mode only.")
	benchCommand.Flags().IntVar(&params.shutdownWaitPeriod, "shutdown-wait-period", 0, "set the time (in seconds) that the server will wait before initiating shutdown. This flag is valid in 'e2e' mode only.")

	RootCommand.AddCommand(benchCommand)
}

type benchRunner interface {
	run(ctx context.Context, ectx *evalContext, params benchmarkCommandParams, f func(context.Context, ...rego.EvalOption) error) (testing.BenchmarkResult, error)
}

func benchMain(args []string, params benchmarkCommandParams, w io.Writer, r benchRunner) (int, error) {

	ctx := context.Background()

	if params.e2e {
		err := benchE2E(ctx, args, params, w)
		if err != nil {
			errRender := renderBenchmarkError(params, err, w)
			return 1, errRender
		}
		return 0, nil
	}

	ectx, err := setupEval(args, params.evalCommandParams)
	if err != nil {
		errRender := renderBenchmarkError(params, err, w)
		return 1, errRender
	}

	var benchFunc func(context.Context, ...rego.EvalOption) error
	rg := rego.New(ectx.regoArgs...)

	if !params.partial {
		// Take the eval context and prepare anything else we possible can before benchmarking the evaluation
		pq, err := rg.PrepareForEval(ctx)
		if err != nil {
			errRender := renderBenchmarkError(params, err, w)
			return 1, errRender
		}

		benchFunc = func(ctx context.Context, opts ...rego.EvalOption) error {
			result, err := pq.Eval(ctx, opts...)
			if err != nil {
				return err
			} else if len(result) == 0 && params.fail {
				return fmt.Errorf("undefined result")
			}
			return nil
		}
	} else {
		// As with normal evaluation, prepare as much as possible up front.
		pq, err := rg.PrepareForPartial(ctx)
		if err != nil {
			errRender := renderBenchmarkError(params, err, w)
			return 1, errRender
		}

		benchFunc = func(ctx context.Context, opts ...rego.EvalOption) error {
			result, err := pq.Partial(ctx, opts...)
			if err != nil {
				return err
			} else if len(result.Queries) == 0 && params.fail {
				return fmt.Errorf("undefined result")
			}
			return nil
		}
	}

	// Run the benchmark as many times as specified, re-use the prepared objects for each
	for i := 0; i < params.count; i++ {
		br, err := r.run(ctx, ectx, params, benchFunc)
		if err != nil {
			errRender := renderBenchmarkError(params, err, w)
			return 1, errRender
		}
		renderBenchmarkResult(params, br, w)
	}

	return 0, nil
}

type goBenchRunner struct {
}

func (r *goBenchRunner) run(ctx context.Context, ectx *evalContext, params benchmarkCommandParams, f func(context.Context, ...rego.EvalOption) error) (testing.BenchmarkResult, error) {

	var hist, m metrics.Metrics
	if params.metrics {
		hist = metrics.New()
		m = metrics.New()
	}

	ectx.evalArgs = append(ectx.evalArgs, rego.EvalMetrics(m))

	var benchErr error

	br := testing.Benchmark(func(b *testing.B) {

		// Track memory allocations, if enabled
		if params.benchMem {
			b.ReportAllocs()
		}

		// Reset the histogram for each invocation of the bench function
		hist.Clear()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {

			// Start the timer (might already be started, but that's ok)
			b.StartTimer()

			// Perform the evaluation
			err := f(ctx, ectx.evalArgs...)

			// Stop the timer while processing the results
			b.StopTimer()
			if err != nil {
				benchErr = err
				b.FailNow()
			}

			// Add metrics for that evaluation into the top level histogram
			if params.metrics {
				for name, metric := range m.All() {
					// Note: We only support int64 metrics right now, this should cover pretty
					// much all the ones we would care about (timers and counters).
					switch v := metric.(type) {
					case int64:
						hist.Histogram(name).Update(v)
					}
				}
				m.Clear()
			}
		}

		if params.metrics {
			reportMetrics(b, hist.All())
		}
	})

	return br, benchErr
}

func benchE2E(ctx context.Context, args []string, params benchmarkCommandParams, w io.Writer) error {
	host := "localhost"
	port := 0

	logger := logging.New()
	logger.SetLevel(logging.Error)

	paths := params.dataPaths.v
	if len(params.bundlePaths.v) > 0 {
		paths = append(paths, params.bundlePaths.v...)
	}

	// Because of test concurrency, several instances of this function can be
	// running simultaneously, which will result in occasional collisions when
	// two goroutines wish to bind the same port for the runtime.
	// We fix the issue here by binding port 0; this will result in the OS
	// allocating us an open port.
	rtParams := runtime.Params{
		Addrs:                  &[]string{fmt.Sprintf("%s:0", host)},
		Paths:                  paths,
		Logger:                 logger,
		BundleMode:             params.bundlePaths.isFlagSet(),
		SkipBundleVerification: true,
		EnableVersionCheck:     false,
		GracefulShutdownPeriod: params.gracefulShutdownPeriod,
		ShutdownWaitPeriod:     params.shutdownWaitPeriod,
		ConfigFile:             params.configFile,
	}

	rt, err := runtime.NewRuntime(ctx, rtParams)
	if err != nil {
		return err
	}

	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	initChannel := rt.Manager.ServerInitializedChannel()

	done := make(chan error)
	go func() {
		done <- rt.Serve(cctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-initChannel:
		break
	}

	// Busy loop until server has truly come online to recover the bound port.
	// We do this with exponential backoff for wait times, since the server
	// typically comes online very quickly.
	baseDelay := time.Duration(100) * time.Millisecond
	maxDelay := time.Duration(60) * time.Second
	retries := 3 // Max of around 1 minute total wait time.
	for i := 0; i < retries; i++ {
		if len(rt.Addrs()) == 0 {
			delay := util.DefaultBackoff(float64(baseDelay), float64(maxDelay), i)
			time.Sleep(delay)
			continue
		}
		// We have an address to parse the port from.
		port, err = strconv.Atoi(strings.Split(rt.Addrs()[0], ":")[1])
		if err != nil {
			return err
		}
		break
	}
	// Check for port still being unbound after retry loop.
	if port == 0 {
		return fmt.Errorf("unable to bind a port for bench testing")
	}

	query, err := readQuery(params, args)
	if err != nil {
		return err
	}

	input, err := readInputBytes(params.evalCommandParams)
	if err != nil {
		return err
	}

	// Wrap input in "input" attribute
	inp := make(map[string]interface{})

	if input != nil {
		if err = util.Unmarshal(input, &inp); err != nil {
			return err
		}
	}

	body := map[string]interface{}{"input": inp}

	var path string
	if params.partial {
		path = "compile"
		body["query"] = query
		if len(params.unknowns) > 0 {
			body["unknowns"] = params.unknowns
		}
	} else {
		_, err := ast.ParseBody(query)
		if err != nil {
			return fmt.Errorf("error occurred while parsing query")
		}

		if strings.HasPrefix(query, "data.") {
			path = strings.ReplaceAll(query, ".", "/")
		} else {
			path = "query"
			body["query"] = query
		}
	}

	url := fmt.Sprintf("http://%s:%d/v1/%v", host, port, path)
	if params.metrics {
		url += "?metrics=true"
	}

	for i := 0; i < params.count; i++ {
		br, err := runE2E(params, url, body)
		if err != nil {
			return err
		}
		renderBenchmarkResult(params, br, w)
	}
	return nil
}

func runE2E(params benchmarkCommandParams, url string, input map[string]interface{}) (testing.BenchmarkResult, error) {
	hist := metrics.New()

	var benchErr error

	br := testing.Benchmark(func(b *testing.B) {

		// Track memory allocations, if enabled
		if params.benchMem {
			b.ReportAllocs()
		}

		// Reset the histogram for each invocation of the bench function
		hist.Clear()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {

			// Start the timer
			b.StartTimer()

			// Execute the query API call
			m, err := e2eQuery(params, url, input)

			// Stop the timer while processing the results
			b.StopTimer()
			if err != nil {
				benchErr = err
				b.FailNow()
			}

			// Add metrics for that evaluation into the top level histogram
			if params.metrics {
				for name, metric := range m {
					switch v := metric.(type) {
					case json.Number:
						num, err := v.Int64()
						if err != nil {
							benchErr = err
							b.FailNow()
						}
						hist.Histogram(name).Update(num)

					}
				}
			}
		}

		if params.metrics {
			reportMetrics(b, hist.All())
		}
	})

	return br, benchErr
}

func e2eQuery(params benchmarkCommandParams, url string, input map[string]interface{}) (types.MetricsV1, error) {

	reqBody, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("unexpected error: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		var e map[string]interface{}
		if err = util.Unmarshal(body, &e); err != nil {
			return nil, err
		}

		if _, ok := e["errors"]; !ok {
			return nil, fmt.Errorf("request failed, OPA server replied with HTTP %v: %v", resp.StatusCode, e["message"])
		}

		bs, err := json.Marshal(e["errors"])
		if err != nil {
			return nil, err
		}

		var astErrs ast.Errors
		if err = util.Unmarshal(bs, &astErrs); err != nil {
			// ignore err
			return nil, fmt.Errorf("request failed, OPA server replied with HTTP %v: %v", resp.StatusCode, e["message"])
		}

		return nil, astErrs
	}

	if !params.partial {
		var result types.DataResponseV1
		if err = util.Unmarshal(body, &result); err != nil {
			return nil, err
		}

		if result.Result == nil && params.fail {
			return nil, fmt.Errorf("undefined result")
		}

		return result.Metrics, nil
	}

	var result types.CompileResponseV1
	if err = util.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if params.fail {
		if result.Result == nil {
			return nil, fmt.Errorf("undefined result")
		}

		i := *result.Result

		peResult, ok := i.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid result for compile response")
		}

		if len(peResult) == 0 {
			return nil, fmt.Errorf("undefined result")
		}

		if val, ok := peResult["queries"]; ok {
			queries, ok := val.([]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid result for output of partial evaluation")
			}

			if len(queries) == 0 {
				return nil, fmt.Errorf("undefined result")
			}
		} else {
			return nil, fmt.Errorf("invalid result for output of partial evaluation")
		}
	}

	return result.Metrics, nil
}

func readQuery(params benchmarkCommandParams, args []string) (string, error) {
	var query string
	if params.stdin {
		bs, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		query = string(bs)
	} else {
		query = args[0]
	}
	return query, nil
}

func renderBenchmarkResult(params benchmarkCommandParams, br testing.BenchmarkResult, w io.Writer) {
	switch params.outputFormat.String() {
	case evalJSONOutput:
		_ = presentation.JSON(w, br)
	case benchmarkGoBenchOutput:
		fmt.Fprintf(w, "BenchmarkOPAEval\t%s", br.String())
		if params.benchMem {
			fmt.Fprintf(w, "\t%s", br.MemString())
		}
		fmt.Fprintf(w, "\n")
	default:
		data := [][]string{
			{"samples", fmt.Sprintf("%d", br.N)},
			{"ns/op", prettyFormatFloat(float64(br.T.Nanoseconds()) / float64(br.N))},
		}
		if params.benchMem {
			data = append(data, []string{
				"B/op", fmt.Sprintf("%d", br.AllocedBytesPerOp()),
			}, []string{
				"allocs/op", fmt.Sprintf("%d", br.AllocsPerOp()),
			})
		}

		var keys []string
		for k := range br.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			data = append(data, []string{k, prettyFormatFloat(br.Extra[k])})
		}

		table := tablewriter.NewWriter(w)
		table.AppendBulk(data)
		table.Render()
	}
}

func renderBenchmarkError(params benchmarkCommandParams, err error, w io.Writer) error {
	o := presentation.Output{
		Errors: presentation.NewOutputErrors(err),
	}

	switch params.outputFormat.String() {
	case evalJSONOutput:
		return presentation.JSON(w, o)
	default:
		return presentation.Pretty(w, o)
	}
}

// Same format used by testing/benchmark.go to format floating point output strings
// Using this keeps the results consistent between the "pretty" and "gobench" outputs.
func prettyFormatFloat(x float64) string {
	// Print all numbers with 10 places before the decimal point
	// and small numbers with three sig figs.
	var format string
	switch y := math.Abs(x); {
	case y == 0 || y >= 99.95:
		format = "%10.0f"
	case y >= 9.995:
		format = "%12.1f"
	case y >= 0.9995:
		format = "%13.2f"
	case y >= 0.09995:
		format = "%14.3f"
	case y >= 0.009995:
		format = "%15.4f"
	case y >= 0.0009995:
		format = "%16.5f"
	default:
		format = "%17.6f"
	}
	return fmt.Sprintf(format, x)
}

func reportMetrics(b *testing.B, m map[string]interface{}) {
	// For each histogram add their values to the benchmark results.
	// Note: If there are many metrics this gets super verbose.
	for histName, metric := range m {
		histValues, ok := metric.(map[string]interface{})
		if !ok {
			continue
		}
		for metricName, rawValue := range histValues {
			unit := fmt.Sprintf("%s_%s", histName, metricName)

			// Only support histogram metrics that are a float64 or int64,
			// this covers the stock implementation in metrics.Metrics
			switch v := rawValue.(type) {
			case int64:
				b.ReportMetric(float64(v), unit)
			case float64:
				b.ReportMetric(v, unit)
			}
		}
	}
}
