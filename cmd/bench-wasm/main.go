// Copyright 2024 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// bench-wasm is a command-line tool for benchmarking OPA WASM performance.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-policy-agent/opa/internal/wasm/benchmark"
	"github.com/open-policy-agent/opa/v1/util"
)

func main() {
	var (
		iterations = flag.Int("iterations", 10, "number of iterations per benchmark")
		target     = flag.String("target", "wasm", "target to benchmark (wasm or rego)")
		baseline   = flag.String("baseline", "", "baseline results file for comparison")
		output     = flag.String("output", "", "output file for results (default: stdout)")
		verbose    = flag.Bool("verbose", false, "verbose output")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <policy-files...>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -iterations 100 policies/*.rego\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -baseline baseline.json -output results.json policies/*.rego\n", os.Args[0])
	}

	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	// Load policies
	suite := benchmark.NewSuite()

	for _, path := range flag.Args() {
		matches, err := filepath.Glob(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error globbing %s: %v\n", path, err)
			os.Exit(1)
		}

		for _, file := range matches {
			if err := loadPolicy(suite, file); err != nil {
				fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", file, err)
				os.Exit(1)
			}
		}
	}

	// Run benchmarks
	ctx := context.Background()
	results, err := suite.Run(ctx, *iterations, *target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Benchmark failed: %v\n", err)
		os.Exit(1)
	}

	// Print results
	if *verbose {
		printVerboseResults(results)
	} else {
		printSummaryResults(results)
	}

	// Compare with baseline if provided
	if *baseline != "" {
		baselineResults, err := loadBaseline(*baseline)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading baseline: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n" + benchmark.Compare(baselineResults, results))
	}

	// Save results if output file specified
	if *output != "" {
		if err := saveResults(*output, results); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving results: %v\n", err)
			os.Exit(1)
		}
	}
}

func loadPolicy(suite *benchmark.Suite, path string) error {
	bs, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Extract test name from filename
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	// Look for accompanying test data
	dataPath := strings.TrimSuffix(path, filepath.Ext(path)) + "_data.json"
	inputPath := strings.TrimSuffix(path, filepath.Ext(path)) + "_input.json"

	var data, input interface{}

	if _, err := os.Stat(dataPath); err == nil {
		dataBytes, err := os.ReadFile(dataPath)
		if err != nil {
			return fmt.Errorf("read data file: %w", err)
		}
		if err := util.UnmarshalJSON(dataBytes, &data); err != nil {
			return fmt.Errorf("parse data file: %w", err)
		}
	}

	if _, err := os.Stat(inputPath); err == nil {
		inputBytes, err := os.ReadFile(inputPath)
		if err != nil {
			return fmt.Errorf("read input file: %w", err)
		}
		if err := util.UnmarshalJSON(inputBytes, &input); err != nil {
			return fmt.Errorf("parse input file: %w", err)
		}
	}

	suite.Add(benchmark.Test{
		Name:   name,
		Policy: string(bs),
		Data:   data,
		Input:  input,
	})

	return nil
}

func printVerboseResults(results []benchmark.Result) {
	fmt.Println("Benchmark Results")
	fmt.Println("=================")

	for _, r := range results {
		fmt.Printf("\n%s:\n", r.Name)
		fmt.Printf("  Iterations: %d\n", r.Iterations)
		fmt.Printf("  Total Time: %v\n", r.TotalTime)
		fmt.Printf("  Average:    %v\n", r.AvgTime)
		fmt.Printf("  Min:        %v\n", r.MinTime)
		fmt.Printf("  Max:        %v\n", r.MaxTime)
		fmt.Printf("  Throughput: %.1f ops/sec\n", float64(r.Iterations)/r.TotalTime.Seconds())
	}
}

func printSummaryResults(results []benchmark.Result) {
	fmt.Println("Benchmark Summary")
	fmt.Println("=================")
	fmt.Printf("%-30s %15s %15s\n", "Test", "Avg Time", "Ops/sec")
	fmt.Println(strings.Repeat("-", 62))

	for _, r := range results {
		throughput := float64(r.Iterations) / r.TotalTime.Seconds()
		fmt.Printf("%-30s %15v %15.1f\n", r.Name, r.AvgTime, throughput)
	}
}

func loadBaseline(path string) ([]benchmark.Result, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var results []benchmark.Result
	if err := json.Unmarshal(bs, &results); err != nil {
		return nil, err
	}

	return results, nil
}

func saveResults(path string, results []benchmark.Result) error {
	bs, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, bs, 0644)
}

