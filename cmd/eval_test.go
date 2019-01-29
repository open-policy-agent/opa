// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

func TestEvalExitCode(t *testing.T) {
	params := newEvalCommandParams()
	params.fail = true

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

func TestEvalWithCoverage(t *testing.T) {

	files := map[string]string{
		"x.rego": `package x

p = 1`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.coverage = true
		params.dataPaths = newrepeatedStringFlag([]string{path})

		var buf bytes.Buffer

		code, err := eval([]string{"data"}, params, &buf)
		if code != 0 || err != nil {
			t.Fatalf("Unexpected exit code (%d) or error: %v", code, err)
		}

		var output presentation.Output

		if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
			t.Fatal(err)
		}

		if output.Coverage == nil || output.Coverage.Coverage != 100.0 {
			t.Fatalf("Expected coverage in output but got: %v", buf.String())
		}
	})
}
