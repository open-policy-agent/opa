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

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/cover"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// Reporter defines the interface for reporting test results.
type Reporter interface {

	// Report is called with a channel that will contain test results.
	Report(chan *Result) error
}

// PrettyReporter reports test results in a simple human readable format.
type PrettyReporter struct {
	Output                   io.Writer
	Verbose                  bool
	FailureLine              bool
	LocalVars                bool
	BenchmarkResults         bool
	BenchMarkShowAllocations bool
	BenchMarkGoBenchFormat   bool
}

func (r PrettyReporter) println(a ...any) {
	_, _ = fmt.Fprintln(r.Output, a...)
}

// Report prints the test report to the reporter's output.
func (r PrettyReporter) Report(ch chan *Result) error {

	dirty := false
	var pass, fail, skip, errs int
	results := make([]*Result, 0, len(ch))
	var failures []*Result

	for tr := range ch {
		if tr.Skip {
			skip++
		} else if tr.Error != nil {
			errs++
		} else {
			if tr.Fail {
				failures = append(failures, tr)
			}

			if len(tr.SubResults) > 0 {
				for _, sr := range tr.SubResults.Iter {
					if len(sr.SubResults) == 0 {
						// Only count leaf results
						if sr.Fail {
							fail++
						} else {
							pass++
						}
					}
				}
			} else {
				if tr.Pass() {
					pass++
				} else if tr.Fail {
					fail++
				}
			}
		}
		results = append(results, tr)
	}

	if fail > 0 && (r.Verbose || r.FailureLine) {
		r.println("FAILURES")
		r.hl()

		for _, failure := range failures {
			_, _ = fmt.Fprint(r.Output, failure.string(false))
			r.println()

			if len(failure.SubResults) > 0 {
				// Print trace collectively for all sub-results.
				if err := printFailure(r.Output, failure.Trace, r.Verbose, false, r.LocalVars); err != nil {
					return err
				}

				if r.Verbose || r.FailureLine {
					r.println()
				}

				for fullName, sr := range failure.SubResults.Iter {
					w := newIndentingWriter(r.Output)

					if sr.Fail {
						if len(sr.SubResults) == 0 {
							// Print full test-case lineage for every failed leaf sub-result for readability.
							for _, n := range fullName {
								_, _ = fmt.Fprintf(w, "%s: %s\n", n, sr.outcome())
								w = newIndentingWriter(w)
							}

							if err := printFailure(w, sr.Trace, false, r.FailureLine, r.LocalVars); err != nil {
								return err
							}
						}
					}
				}
			} else {
				if err := printFailure(r.Output, failure.Trace, r.Verbose, r.FailureLine, r.LocalVars); err != nil {
					return err
				}
			}

			r.println()
		}

		r.println("SUMMARY")
		r.hl()
	}

	// Report individual tests.
	var lastFile string
	for _, tr := range results {

		if tr.Pass() && r.BenchmarkResults {
			dirty = true
			r.println(r.fmtBenchmark(tr))
		} else if r.Verbose || !tr.Pass() {
			if tr.Location != nil && tr.Location.File != lastFile {
				if lastFile != "" {
					r.println("")
				}
				_, _ = fmt.Fprintf(r.Output, "%s:\n", tr.Location.File)
				lastFile = tr.Location.File
			}

			dirty = true
			r.println(tr.string(false))

			w := newIndentingWriter(r.Output)
			if srs := tr.SubResults; len(srs) > 0 {
				for fullName, sr := range srs.Iter {
					if sr.Fail || r.Verbose {
						_, _ = fmt.Fprintf(w, "%s%s\n",
							strings.Repeat("  ", len(fullName)-1),
							sr.String(),
						)
					}
				}
			}

			if len(tr.Output) > 0 {
				r.println()
				_, _ = fmt.Fprintln(newIndentingWriter(r.Output), strings.TrimSpace(string(tr.Output)))
				r.println()
			}
		}
		if tr.Error != nil {
			_, _ = fmt.Fprintf(r.Output, "  %v\n", tr.Error)
		}
	}

	// Report summary of test.
	if dirty {
		r.hl()
	}

	total := pass + fail + skip + errs

	if pass != 0 {
		r.println("PASS:", fmt.Sprintf("%d/%d", pass, total))
	}

	if fail != 0 {
		r.println("FAIL:", fmt.Sprintf("%d/%d", fail, total))
	}

	if skip != 0 {
		r.println("SKIPPED:", fmt.Sprintf("%d/%d", skip, total))
	}

	if errs != 0 {
		r.println("ERROR:", fmt.Sprintf("%d/%d", errs, total))
	}

	return nil
}

func printFailure(w io.Writer, trace []*topdown.Event, verbose bool, failureLine bool, localVars bool) error {
	if verbose {
		_, _ = fmt.Fprintln(w)
		topdown.PrettyTraceWithOpts(newIndentingWriter(w), trace, topdown.PrettyTraceOptions{
			Locations:     true,
			ExprVariables: localVars,
		})
	}

	if failureLine {
		_, _ = fmt.Fprintln(w)
		for i := len(trace) - 1; i >= 0; i-- {
			e := trace[i]
			if e.Op == topdown.FailOp && e.Location != nil && e.QueryID != 0 {
				if expr, isExpr := e.Node.(*ast.Expr); isExpr {
					if _, isEvery := expr.Terms.(*ast.Every); isEvery {
						// We're interested in the failing expression inside the every body.
						continue
					}
				}
				_, _ = fmt.Fprintf(newIndentingWriter(w), "%s:%d:\n", e.Location.File, e.Location.Row)
				if err := topdown.PrettyEvent(newIndentingWriter(w, 4), e, topdown.PrettyEventOpts{PrettyVars: localVars}); err != nil {
					return err
				}
				_, _ = fmt.Fprintln(w)
				break
			}
		}
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
		for _, part := range strings.Split(strings.ReplaceAll(name, "_", "."), ".") {
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
	Verbose   bool
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
		err := cover.CoverageThresholdError{
			Coverage:  report.Coverage,
			Threshold: r.Threshold,
		}

		if r.Verbose {
			err.Report = &report
		}

		return &err
	}

	encoder := json.NewEncoder(r.Output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

type indentingWriter struct {
	w      io.Writer
	indent int
}

func newIndentingWriter(w io.Writer, indent ...int) indentingWriter {
	i := 2
	if len(indent) > 0 {
		i = indent[0]
	}

	if iw, ok := w.(indentingWriter); ok {
		i += iw.indent
		w = iw.w
	}

	return indentingWriter{
		w:      w,
		indent: i,
	}
}

func (w indentingWriter) Write(bs []byte) (int, error) {
	var written int
	// insert indentation at the start of every line.
	indent := true
	for _, b := range bs {
		if indent {
			wrote, err := w.w.Write([]byte(strings.Repeat(" ", w.indent)))
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
