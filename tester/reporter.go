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

	"github.com/open-policy-agent/opa/topdown"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/cover"
)

// Reporter defines the interface for reporting test results.
type Reporter interface {

	// Report is called with a channel that will contain test results.
	Report(ch chan *Result) error
}

// PrettyReporter reports test results in a simple human readable format.
type PrettyReporter struct {
	Output  io.Writer
	Verbose bool
}

// Report prints the test report to the reporter's output.
func (r PrettyReporter) Report(ch chan *Result) error {

	dirty := false
	var pass, fail, errs int

	var results, failures []*Result
	for tr := range ch {
		if tr.Pass() {
			pass++
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
			topdown.PrettyTrace(newIndentingWriter(r.Output), failure.Trace)
			fmt.Fprintln(r.Output)
		}

		fmt.Fprintln(r.Output, "SUMMARY")
		r.hl()
	}

	// Report individual tests.
	for _, tr := range results {
		if !tr.Pass() || r.Verbose {
			fmt.Fprintln(r.Output, tr)
			dirty = true
		}
		if tr.Error != nil {
			fmt.Fprintf(r.Output, "  %v\n", tr.Error)
		}
	}

	// Report summary of test.
	if dirty {
		r.hl()
	}

	total := pass + fail + errs

	if pass != 0 {
		fmt.Fprintln(r.Output, "PASS:", fmt.Sprintf("%d/%d", pass, total))
	}

	if fail != 0 {
		fmt.Fprintln(r.Output, "FAIL:", fmt.Sprintf("%d/%d", fail, total))
	}

	if errs != 0 {
		fmt.Fprintln(r.Output, "ERROR:", fmt.Sprintf("%d/%d", errs, total))
	}

	return nil
}

func (r PrettyReporter) hl() {
	fmt.Fprintln(r.Output, strings.Repeat("-", 80))
}

// JSONReporter reports test results as array of JSON objects.
type JSONReporter struct {
	Output io.Writer
}

// Report prints the test report to the reporter's output.
func (r JSONReporter) Report(ch chan *Result) error {
	var report []*Result
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
