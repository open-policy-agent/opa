// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tester

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/cover"
	"github.com/open-policy-agent/opa/topdown"
)

// Reporter defines the interface for reporting test results.
type Reporter interface {

	// Report is called with a channel that will contain test results.
	Report(ch chan *Result) error
}

// PrettyReporter reports test results in a simple human readable format.
type PrettyReporter struct {
	Output                   io.Writer
	Verbose                  bool
	FailureLine              bool
	BenchmarkResults         bool
	BenchMarkShowAllocations bool
	BenchMarkGoBenchFormat   bool
}

// Report prints the test report to the reporter's output.
func (r PrettyReporter) Report(ch chan *Result) error {

	dirty := false
	var pass, fail, skip, errs int
	results := make([]*Result, 0, len(ch))
	var failures []*Result

	for tr := range ch {
		if tr.Pass() {
			pass++
		} else if tr.Skip {
			skip++
		} else if tr.Error != nil {
			errs++
		} else if tr.Fail {
			fail++
			failures = append(failures, tr)
		}
		results = append(results, tr)
	}

	if fail > 0 && r.Verbose {
		fmt.Fprintln(r.Output, "FAILURES")
		r.hl()

		for _, failure := range failures {
			fmt.Fprintln(r.Output, failure)
			fmt.Fprintln(r.Output)
			topdown.PrettyTraceWithLocation(newIndentingWriter(r.Output), failure.Trace)
			fmt.Fprintln(r.Output)
		}

		fmt.Fprintln(r.Output, "SUMMARY")
		r.hl()
	}

	// Report individual tests.
	var lastFile string
	for _, tr := range results {

		if tr.Pass() && r.BenchmarkResults {
			dirty = true
			fmt.Fprintln(r.Output, r.fmtBenchmark(tr))
		} else if r.Verbose || !tr.Pass() {
			if tr.Location != nil && tr.Location.File != lastFile {
				if lastFile != "" {
					fmt.Fprintln(r.Output, "")
				}
				fmt.Fprintf(r.Output, "%s:\n", tr.Location.File)
				lastFile = tr.Location.File
			}
			dirty = true
			fmt.Fprintln(r.Output, tr)
			if len(tr.Output) > 0 {
				fmt.Fprintln(r.Output)
				fmt.Fprintln(newIndentingWriter(r.Output), strings.TrimSpace(string(tr.Output)))
				fmt.Fprintln(r.Output)
			}
		}
		if tr.Error != nil {
			fmt.Fprintf(r.Output, "  %v\n", tr.Error)
		}
	}

	// Report summary of test.
	if dirty {
		r.hl()
	}

	total := pass + fail + skip + errs

	if pass != 0 {
		fmt.Fprintln(r.Output, "PASS:", fmt.Sprintf("%d/%d", pass, total))
	}

	if fail != 0 {
		fmt.Fprintln(r.Output, "FAIL:", fmt.Sprintf("%d/%d", fail, total))
	}

	if skip != 0 {
		fmt.Fprintln(r.Output, "SKIPPED:", fmt.Sprintf("%d/%d", skip, total))
	}

	if errs != 0 {
		fmt.Fprintln(r.Output, "ERROR:", fmt.Sprintf("%d/%d", errs, total))
	}

	return nil
}

func (r PrettyReporter) hl() {
	fmt.Fprintln(r.Output, strings.Repeat("-", 80))
}

func (r PrettyReporter) fmtBenchmark(tr *Result) string {
	if tr.BenchmarkResult == nil {
		return ""
	}
	name := fmt.Sprintf("%v.%v", tr.Package, tr.Name)
	if r.BenchMarkGoBenchFormat {
		// The Golang benchmark data format requires the line start with "Benchmark" and then
		// the next letter needs to be capitalized.
		// https://go.googlesource.com/proposal/+/master/design/14313-benchmark-format.md
		//
		// This converts the test case name like data.foo.bar.test_auth to be more
		// like BenchmarkDataFooBarTestAuth.
		camelCaseName := ""
		for _, part := range strings.Split(strings.Replace(name, "_", ".", -1), ".") {
			camelCaseName += strings.Title(part) //nolint:staticcheck // SA1019, no unicode here
		}
		name = "Benchmark" + camelCaseName
	}

	result := fmt.Sprintf("%s\t%s", name, tr.BenchmarkResult.String())
	if r.BenchMarkShowAllocations {
		result += "\t" + tr.BenchmarkResult.MemString()
	}

	return result
}

// JSONReporter reports test results as array of JSON objects.
type JSONReporter struct {
	Output io.Writer
}

// Report prints the test report to the reporter's output.
func (r JSONReporter) Report(ch chan *Result) error {
	report := make([]*Result, 0, len(ch))
	for tr := range ch {
		report = append(report, tr)
	}

	bs, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(r.Output, string(bs))
	return nil
}

// JSONCoverageReporter reports coverage as a JSON structure.
type JSONCoverageReporter struct {
	Cover     *cover.Cover
	Modules   map[string]*ast.Module
	Output    io.Writer
	Threshold float64
}

// Report prints the test report to the reporter's output. If any tests fail or
// encounter errors, this function returns an error.
func (r JSONCoverageReporter) Report(ch chan *Result) error {
	for tr := range ch {
		if !tr.Pass() {
			if tr.Error != nil {
				return tr.Error
			}
			return errors.New(tr.String())
		}
	}
	report := r.Cover.Report(r.Modules)

	if report.Coverage < r.Threshold {
		return &cover.CoverageThresholdError{
			Coverage:  report.Coverage,
			Threshold: r.Threshold,
		}
	}

	encoder := json.NewEncoder(r.Output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

type indentingWriter struct {
	w io.Writer
}

func newIndentingWriter(w io.Writer) indentingWriter {
	return indentingWriter{
		w: w,
	}
}

func (w indentingWriter) Write(bs []byte) (int, error) {
	var written int
	// insert indentation at the start of every line.
	indent := true
	for _, b := range bs {
		if indent {
			wrote, err := w.w.Write([]byte("  "))
			if err != nil {
				return written, err
			}
			written += wrote
		}
		wrote, err := w.w.Write([]byte{b})
		if err != nil {
			return written, err
		}
		written += wrote
		indent = b == '\n'
	}
	return written, nil
}
