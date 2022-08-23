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
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
	"github.com/open-policy-agent/opa/util"
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

type testErrorWithDetails struct{}

func (*testErrorWithDetails) Error() string   { return "something went wrong" }
func (*testErrorWithDetails) Lines() []string { return []string{"oh", "so", "wrong"} }

func validateJSONOutput(t *testing.T, testErr error, expected string) {
	t.Helper()
	output := Output{Errors: NewOutputErrors(testErr)}
	var buf bytes.Buffer
	err := JSON(&buf, output)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	result := util.MustUnmarshalJSON(buf.Bytes())
	exp := util.MustUnmarshalJSON([]byte(expected))

	if !reflect.DeepEqual(result, exp) {
		t.Fatal("expected:", exp, "got:", result)
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

func TestOutputJSONErrorWithDetails(t *testing.T) {
	err := &testErrorWithDetails{}
	expected := `{
  "errors": [
    {
      "message": "something went wrong",
      "details": "oh\nso\nwrong"
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
			  "want": {
				"args": [
				  {
					"description": "the set/array/object/string to be counted",
					"name": "collection",
					"of": [
					  {
						"type": "string"
					  },
					  {
						"dynamic": {
						  "type": "any"
						},
						"type": "array"
					  },
					  {
						"dynamic": {
						  "key": {
							"type": "any"
						  },
						  "value": {
							"type": "any"
						  }
						},
						"type": "object"
					  },
					  {
						"of": {
						  "type": "any"
						},
						"type": "set"
					  }
					],
					"type": "any"
				  },
				  {
					"description": "the count of elements, key/val pairs, or characters, respectively.",
					"name": "n",
					"type": "number"
				  }
				]
			  }
			}
		  }
		]
	  }`

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
      "message": "illegal ! character",
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
		_, err = loader.NewFileLoader().All([]string{path})
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

	err := Source(buf, Output{
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
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

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

func TestRaw(t *testing.T) {
	tests := []struct {
		note   string
		output Output
		want   string
	}{
		{
			note: "simple single string",
			output: Output{
				Result: []rego.Result{
					{
						Expressions: []*rego.ExpressionValue{
							{Value: "Hello world"},
						},
					},
				},
			},
			want: "Hello world\n",
		},
		{
			note: "table format",
			output: Output{
				Result: []rego.Result{
					{
						Expressions: []*rego.ExpressionValue{
							{Value: "one"},
							{Value: 1},
						},
					},
					{
						Expressions: []*rego.ExpressionValue{
							{Value: "two"},
							{Value: 2},
						},
					},
				},
			},
			want: "one 1\ntwo 2\n",
		},
		{
			note: "compound values",
			output: Output{
				Result: []rego.Result{
					{
						Expressions: []*rego.ExpressionValue{
							{Value: []interface{}{"one"}},
							{Value: map[string]interface{}{
								"key": []interface{}{},
							}},
						},
					},
				},
			},
			want: "[\"one\"] {\"key\":[]}\n",
		},
		{
			note: "error",
			output: Output{
				Errors: NewOutputErrors(fmt.Errorf("boom")),
			},
			want: "1 error occurred: boom\n",
		},
		{
			// NOTE(sr): The presentation package outputs whatever Error() on
			// the errors it is given yields. So even though NewOutputErrors
			// will pick up the error details, they won't be output, as they
			// are not included in the error's Error() string.
			note: "error with details",
			output: Output{
				Errors: NewOutputErrors(&testErrorWithDetails{}),
			},
			want: "1 error occurred: something went wrong\noh\nso\nwrong\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			buf := new(bytes.Buffer)
			err := Raw(buf, tc.output)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if buf.String() != tc.want {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", tc.want, buf.String())
			}
		})
	}
}

func TestDepsAnalysisPrettyOutput(t *testing.T) {
	tests := []struct {
		note   string
		output DepAnalysisOutput
		want   []string
	}{
		{
			note:   "base document",
			output: DepAnalysisOutput{Base: []ast.Ref{ast.InputRootRef}},
			want: []string{
				"+----------------+",
				"| BASE DOCUMENTS |",
				"+----------------+",
				"| input          |",
				"+----------------+",
			},
		},
		{
			note: "virtual document",
			output: DepAnalysisOutput{
				Virtual: []ast.Ref{[]*ast.Term{
					ast.VarTerm("data"), ast.StringTerm("policy"), ast.StringTerm("allow")},
				},
			},
			want: []string{
				"+-------------------+",
				"| VIRTUAL DOCUMENTS |",
				"+-------------------+",
				"| data.policy.allow |",
				"+-------------------+",
			},
		},
		{
			note: "base document and virtual document",
			output: DepAnalysisOutput{
				Base: []ast.Ref{ast.InputRootRef},
				Virtual: []ast.Ref{[]*ast.Term{
					ast.VarTerm("data"), ast.StringTerm("policy"), ast.StringTerm("allow")},
				},
			},
			want: []string{
				"+----------------+-------------------+",
				"| BASE DOCUMENTS | VIRTUAL DOCUMENTS |",
				"+----------------+-------------------+",
				"| input          | data.policy.allow |",
				"+----------------+-------------------+",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			buf := new(bytes.Buffer)
			if err := tc.output.Pretty(buf); err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			expected := strings.Join(tc.want, "\n") + "\n"
			if buf.String() != expected {
				t.Errorf("expected %v, got %v", expected, buf.String())
			}
		})
	}
}
