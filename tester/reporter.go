// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tester

import (
	"fmt"
	"io"
	"strings"
)

// PrettyReporter reports test results in a simple human readable format.
type PrettyReporter struct {
	Output  io.Writer
	Verbose bool
}

// Report prints the test report to the reporter's output.
func (r PrettyReporter) Report(ch chan *Result) error {

	dirty := false
	var pass, fail, errs int

	// Report individual tests.
	for tr := range ch {
		if tr.Pass() {
			pass++
		} else if tr.Error != nil {
			errs++
		} else if tr.Fail != nil {
			fail++
		}
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
		fmt.Fprintln(r.Output, strings.Repeat("-", 80))
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
