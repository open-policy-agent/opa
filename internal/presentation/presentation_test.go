// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package presentation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util/test"
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

func validateJSONOutput(t *testing.T, testErr error, expected string) {
	t.Helper()
	output := Output{Errors: NewOutputErrors(testErr)}
	var buf bytes.Buffer
	err := JSON(&buf, output)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if buf.String() != expected {
		t.Fatalf("Unexpected marshalled error value.\n Expected (len=%d):\n>>>\n%s\n<<<\n\nActual (len=%d):\n>>>>\n%s\n<<<<\n",
			len(expected), expected, len(buf.String()), buf.String())
	}
}

func TestOutputJSONErrorUnstructured(t *testing.T) {
	err := errors.New("some text")
	expected := `{
  "errors": [
    {
      "message": "some text"
    }
  ]
}
`

	validateJSONOutput(t, err, expected)
}

func TestOutputJSONErrorCustomMarshaller(t *testing.T) {
	err := &testErrorWithMarshaller{
		msg: "custom message",
	}
	expected := `{
  "errors": [
    {
      "message": "custom message"
    }
  ]
}
`

	validateJSONOutput(t, err, expected)
}

func TestOutputJSONErrorStructuredASTErr(t *testing.T) {
	err := &ast.Error{
		Code:    "1",
		Message: "error message",
	}
	expected := `{
  "errors": [
    {
      "message": "error message",
      "code": "1"
    }
  ]
}
`

	validateJSONOutput(t, err, expected)
}

func TestOutputJSONErrorStructuredStorageErr(t *testing.T) {
	store := inmem.New()
	txn := storage.NewTransactionOrDie(context.Background(), store)
	err := store.Write(context.Background(), txn, storage.AddOp, storage.Path{}, map[string]interface{}{"foo": 1})
	expected := `{
  "errors": [
    {
      "message": "data write during read transaction",
      "code": "storage_invalid_txn_error"
    }
  ]
}
`

	validateJSONOutput(t, err, expected)
}

func TestOutputJSONErrorStructuredTopdownErr(t *testing.T) {
	mod := `
		package test

		p(x) = y {
			y = x[_]
		}

		z := p([1, 2, 3])
		`

	_, err := rego.New(
		rego.Module("test.rego", mod),
		rego.Query("data.test.z"),
	).Eval(context.Background())

	expected := `{
  "errors": [
    {
      "message": "functions must not produce multiple outputs for same inputs",
      "code": "eval_conflict_error",
      "location": {
        "file": "test.rego",
        "row": 4,
        "col": 3
      }
    }
  ]
}
`

	validateJSONOutput(t, err, expected)
}

func TestOutputJSONErrorStructuredAstErr(t *testing.T) {
	_, err := rego.New(rego.Query("count(0)")).Eval(context.Background())
	expected := `{
  "errors": [
    {
      "message": "count: invalid argument(s)",
      "code": "rego_type_error",
      "location": {
        "file": "",
        "row": 1,
        "col": 1
      },
      "details": {
        "have": [
          {
            "type": "number"
          },
          null
        ],
        "want": [
          {
            "of": [
              {
                "of": {
                  "of": [],
                  "type": "any"
                },
                "type": "set"
              },
              {
                "dynamic": {
                  "of": [],
                  "type": "any"
                },
                "type": "array"
              },
              {
                "dynamic": {
                  "key": {
                    "of": [],
                    "type": "any"
                  },
                  "value": {
                    "of": [],
                    "type": "any"
                  }
                },
                "type": "object"
              },
              {
                "type": "string"
              }
            ],
            "type": "any"
          },
          {
            "type": "number"
          }
        ]
      }
    }
  ]
}
`

	validateJSONOutput(t, err, expected)
}

func TestOutputJSONErrorStructuredAstParseErr(t *testing.T) {
	_, err := rego.New(
		rego.Module("parse-err.rego", "!!!"),
		rego.Query("!!!"),
	).Eval(context.Background())

	expected := `{
  "errors": [
    {
      "message": "no match found",
      "code": "rego_parse_error",
      "location": {
        "file": "parse-err.rego",
        "row": 1,
        "col": 1
      },
      "details": {
        "line": "!!!",
        "idx": 0
      }
    }
  ]
}
`

	validateJSONOutput(t, err, expected)
}

func TestOutputJSONErrorStructuredASTErrList(t *testing.T) {
	c := ast.NewCompiler()
	c.Compile(map[string]*ast.Module{
		"error.rego": ast.MustParseModule(`
package test

q {
	bad[reference]
}
`)})
	c.Errors.Sort()
	err := c.Errors

	expected := `{
  "errors": [
    {
      "message": "var bad is unsafe",
      "code": "rego_unsafe_var_error",
      "location": {
        "file": "",
        "row": 5,
        "col": 2
      }
    },
    {
      "message": "var reference is unsafe",
      "code": "rego_unsafe_var_error",
      "location": {
        "file": "",
        "row": 5,
        "col": 2
      }
    }
  ]
}
`

	validateJSONOutput(t, err, expected)
}

func TestOutputJSONErrorStructuredLoaderErrList(t *testing.T) {
	files := map[string]string{
		// bundle a
		"a/data.json": "{{{",
		"b/data.json": "...",
	}

	var err error
	var tmpPath string
	test.WithTempFS(files, func(path string) {
		tmpPath = path
		_, err = loader.All([]string{path})
	})

	expected := fmt.Sprintf(`{
  "errors": [
    {
      "message": "%s/a/data.json: invalid character '{' looking for beginning of object key string"
    },
    {
      "message": "%s/b/data.json: invalid character '.' looking for beginning of value"
    }
  ]
}
`, tmpPath, tmpPath)

	validateJSONOutput(t, err, expected)
}

func TestOutputJSONErrorStructuredRegoErrList(t *testing.T) {
	mod := `
package test

p {
	bad_func1()
}

q {
	bad_func2()
}
`
	_, err := rego.New(
		rego.Module("error.rego", mod),
		rego.Query("data"),
	).PrepareForEval(context.Background())

	expected := `{
  "errors": [
    {
      "message": "undefined function bad_func1",
      "code": "rego_type_error",
      "location": {
        "file": "error.rego",
        "row": 5,
        "col": 2
      }
    },
    {
      "message": "undefined function bad_func2",
      "code": "rego_type_error",
      "location": {
        "file": "error.rego",
        "row": 9,
        "col": 2
      }
    }
  ]
}
`

	validateJSONOutput(t, err, expected)
}

func TestSource(t *testing.T) {

	buf := new(bytes.Buffer)

	Source(buf, Output{
		Partial: &rego.PartialQueries{
			Queries: []ast.Body{
				ast.MustParseBody("a = 1; b = 2"),
			},
			Support: []*ast.Module{
				ast.MustParseModule(`
            package test
            p = 1
        `),
			},
		},
	})

	exp := `# Query 1
a = 1
b = 2

# Module 1
package test

p = 1
`

	if buf.String() != exp {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp, buf.String())
	}

}
