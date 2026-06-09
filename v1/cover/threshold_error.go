// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cover

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/v1/util"
)

// CoverageThresholdError represents an error raised when the global
// code coverage percentage is lower than the specified threshold.
type CoverageThresholdError struct {
	Coverage  float64
	Threshold float64
	Report    *Report
}

func (e *CoverageThresholdError) Error() string {
	sb := &strings.Builder{}
	fmt.Fprintf(sb,
		"Code coverage threshold not met: got %.2f instead of %.2f",
		e.Coverage,
		e.Threshold,
	)

	if e.Report != nil && len(e.Report.Files) > 0 {
		sb.WriteString("\nLines not covered:")

		for _, file := range util.KeysSorted(e.Report.Files) {
			report := e.Report.Files[file]
			for _, span := range rowSpans(report.NotCovered) {
				if span[0] == span[1] {
					fmt.Fprintf(sb, "\n\t%s:%d", file, span[0])
				} else {
					fmt.Fprintf(sb, "\n\t%s:%d-%d", file, span[0], span[1])
				}
			}
		}
	}

	return sb.String()
}
