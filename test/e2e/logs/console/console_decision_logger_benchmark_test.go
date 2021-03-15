// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build noisy

package console

import (
	"testing"

	"github.com/open-policy-agent/opa/test/e2e/logs"
)

func BenchmarkRESTConsoleDecisionLogger(b *testing.B) {
	logs.RunDecisionLoggerBenchmark(b, testRuntime)
}
