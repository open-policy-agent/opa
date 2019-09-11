// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package presentation

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

type testErrorWithMarshaller struct {
	msg string
}

func (t *testErrorWithMarshaller) Error() string {
	return t.msg
}

func (t *testErrorWithMarshaller) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Text string `json:"text"`
	}{
		Text: t.msg,
	})
}

func TestOutputJSONErrors(t *testing.T) {
	cases := []struct {
		note     string
		err      error
		expected string
	}{
		{
			note: "unstructured error",
			err:  errors.New("some text"),
			expected: `{
  "error": "some text"
}
`,
		},
		{
			note: "structured error",
			err: &ast.Error{
				Code:    "1",
				Message: "error message",
			},
			expected: `{
  "error": {
    "code": "1",
    "message": "error message"
  }
}
`,
		},
		{
			note: "structured error list",
			err: ast.Errors{&ast.Error{
				Code:    "1",
				Message: "error message",
			}},
			expected: `{
  "error": [
    {
      "code": "1",
      "message": "error message"
    }
  ]
}
`,
		},
		{
			note: "custom marshaller",
			err: &testErrorWithMarshaller{
				msg: "custom message",
			},
			expected: `{
  "error": {
    "text": "custom message"
  }
}
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			output := Output{Error: tc.err}
			var buf bytes.Buffer
			err := JSON(&buf, output)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			if buf.String() != tc.expected {
				t.Fatalf("Unexpected marshalled error value.\n Expected:\n\n%s\n\nActual:\n\n%s\n\n", tc.expected, buf.String())
			}
		})
	}
}
