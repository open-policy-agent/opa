// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/open-policy-agent/opa/util"
)

func TestEvalExitCode(t *testing.T) {
	params := evalCommandParams{
		fail:         true,
		explain:      util.NewEnumFlag(explainModeOff, []string{explainModeFull}),
		outputFormat: util.NewEnumFlag(evalJSONOutput, []string{evalJSONOutput}),
	}
	tests := []struct {
		note         string
		query        string
		expectedCode int
	}{
		{"defined result", "true=true", 0},
		{"undefined result", "true = false", 1},
		{"on error", "x = 1/0", 2},
	}

	var b bytes.Buffer
	writer := bufio.NewWriter(&b)
	for _, tc := range tests {
		code, err := eval([]string{tc.query}, params, writer)
		if err != nil {
			t.Fatalf("%v: Unexpected error %v", tc.note, err)
		}
		if code != tc.expectedCode {
			t.Fatalf("%v: Expected code %v, got %v", tc.note, tc.expectedCode, code)
		}
	}
}
