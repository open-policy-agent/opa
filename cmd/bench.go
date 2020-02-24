// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

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
	benchMem bool
	count    int
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
		},
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

The optional "gobench" output format conforms to the Go Benchmark Data Format.
`,

		PreRunE: func(_ *cobra.Command, args []string) error {
			return validateEvalParams(&params.evalCommandParams, args)
		},
		Run: func(_ *cobra.Command, args []string) {
			os.Exit(benchMain(args, params, os.Stdout, &goBenchRunner{}))
		},
	}

	// Sub-set of the standard `opa eval ..` flags
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

	// Shared benchmark flags
	addCountFlag(benchCommand.Flags(), &params.count, "benchmark")
	addBenchmemFlag(benchCommand.Flags(), &params.benchMem, true)

	RootCommand.AddCommand(benchCommand)
}

type benchRunner interface {
	run(ctx context.Context, ectx *evalContext, pq rego.PreparedEvalQuery, params benchmarkCommandParams) (testing.BenchmarkResult, error)
}

func benchMain(args []string, params benchmarkCommandParams, w io.Writer, r benchRunner) int {
	ectx, err := setupEval(args, params.evalCommandParams)
	if err != nil {
		renderBenchmarkError(params, err, w)
		return 1
	}

	ctx := context.Background()

	// Take the eval context and prepare anything else we possible can before benchmarking the evaluation
	pq, err := ectx.r.PrepareForEval(ctx)
	if err != nil {
		renderBenchmarkError(params, err, w)
		return 1
	}

	// Run the benchmark as many times as specified, re-use the prepared objects for each
	for i := 0; i < params.count; i++ {
		br, err := r.run(ctx, ectx, pq, params)
		if err != nil {
			renderBenchmarkError(params, err, w)
			return 1
		}
		renderBenchmarkResult(params, br, w)
	}

	return 0
}

type goBenchRunner struct {
}

func (r *goBenchRunner) run(ctx context.Context, ectx *evalContext, pq rego.PreparedEvalQuery, params benchmarkCommandParams) (testing.BenchmarkResult, error) {

	var hist metrics.Metrics
	if params.metrics {
		hist = metrics.New()
	}

	var m metrics.Metrics
	if params.metrics {
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
			rs, err := pq.Eval(ctx, ectx.evalArgs...)

			// Stop the timer while processing the results
			b.StopTimer()
			if err != nil {
				benchErr = err
				b.FailNow()
			}

			if len(rs) == 0 && params.fail {
				benchErr = errors.New("undefined result")
				b.FailNow()
			}

			// Add metrics for that evaluation into the top level histogram
			if params.metrics {
				for name, metric := range m.All() {
					// Note: We only support int64 metrics right now, this should cover pretty
					// much all of the ones we would care about (timers and counters).
					switch v := metric.(type) {
					case int64:
						hist.Histogram(name).Update(v)
					}
				}
				m.Clear()
			}
		}

		if params.metrics {
			// For each histogram add their values to the benchmark results.
			// Note: If there are many metrics this gets super verbose.
			for histName, metric := range hist.All() {
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
	})

	return br, benchErr
}

func renderBenchmarkResult(params benchmarkCommandParams, br testing.BenchmarkResult, w io.Writer) {
	switch params.outputFormat.String() {
	case evalJSONOutput:
		presentation.JSON(w, br)
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
