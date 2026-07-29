//go:build go1.27

package cmd

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/open-policy-agent/opa/cmd/formats"
)

// On go1.27 the jsonv2 encoders write object keys in a different order than the
// v1 marshallers, so util.JsonEqual ignores key order and the TestParse*JSON*
// assertions in parse_test.go no longer compare bytes. For the module below:
//
//   - imports[0]: 1.26 puts location before path, 1.27 puts it after.
//   - rules[0]: 1.26 puts body before head, 1.27 puts head first.
//   - rules[0].head: 1.26 orders name, value, ref, location; 1.27 orders name,
//     ref, value, location.
//
// This covers one module rather than every parse output shape.
//
// TODO: Once only the jsonv2 implementation is used, port the parse_test.go
// assertions, and those using JsonEqual in v1/ast/policy_test.go and
// policy_logical_test.go, to byte compares against the 1.27 key order, then
// delete util.JsonEqual and this file.
func TestParseJSONOutputBytes(t *testing.T) {
	files := map[string]string{
		"x.rego": `package test

import data.foo

allow if {
	foo.bar == 1
}
`,
	}
	errc, stdout, stderr, tempDirPath := testParse(t, files, &parseParams{
		format:      formats.Flag(formats.JSON, formats.Pretty),
		jsonInclude: "locations",
	})
	if errc != 0 {
		t.Fatalf("Expected exit code 0, got %v", errc)
	}
	if len(stderr) > 0 {
		t.Fatalf("Expected no stderr output, got:\n%s\n", string(stderr))
	}

	expectedOutput := strings.ReplaceAll(`{
  "package": {
    "location": {
      "file": "TEMPDIR/x.rego",
      "row": 1,
      "col": 1,
      "text": "cGFja2FnZQ=="
    },
    "path": [
      {
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 1,
          "col": 9,
          "text": "dGVzdA=="
        },
        "type": "var",
        "value": "data"
      },
      {
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 1,
          "col": 9,
          "text": "dGVzdA=="
        },
        "type": "string",
        "value": "test"
      }
    ]
  },
  "imports": [
    {
      "path": {
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 3,
          "col": 8,
          "text": "ZGF0YS5mb28="
        },
        "type": "ref",
        "value": [
          {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 3,
              "col": 8,
              "text": "ZGF0YQ=="
            },
            "type": "var",
            "value": "data"
          },
          {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 3,
              "col": 13,
              "text": "Zm9v"
            },
            "type": "string",
            "value": "foo"
          }
        ]
      },
      "location": {
        "file": "TEMPDIR/x.rego",
        "row": 3,
        "col": 1,
        "text": "aW1wb3J0"
      }
    }
  ],
  "rules": [
    {
      "head": {
        "name": "allow",
        "ref": [
          {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 5,
              "col": 1,
              "text": "YWxsb3c="
            },
            "type": "var",
            "value": "allow"
          }
        ],
        "value": {
          "type": "boolean",
          "value": true
        },
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 5,
          "col": 1,
          "text": "YWxsb3c="
        }
      },
      "body": [
        {
          "index": 0,
          "location": {
            "file": "TEMPDIR/x.rego",
            "row": 6,
            "col": 2,
            "text": "Zm9vLmJhciA9PSAx"
          },
          "terms": [
            {
              "location": {
                "file": "TEMPDIR/x.rego",
                "row": 6,
                "col": 10,
                "text": "PT0="
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 6,
                    "col": 10,
                    "text": "PT0="
                  },
                  "type": "var",
                  "value": "equal"
                }
              ]
            },
            {
              "location": {
                "file": "TEMPDIR/x.rego",
                "row": 6,
                "col": 2,
                "text": "Zm9vLmJhcg=="
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 6,
                    "col": 2,
                    "text": "Zm9v"
                  },
                  "type": "var",
                  "value": "foo"
                },
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 6,
                    "col": 6,
                    "text": "YmFy"
                  },
                  "type": "string",
                  "value": "bar"
                }
              ]
            },
            {
              "location": {
                "file": "TEMPDIR/x.rego",
                "row": 6,
                "col": 13,
                "text": "MQ=="
              },
              "type": "number",
              "value": 1
            }
          ]
        }
      ],
      "location": {
        "file": "TEMPDIR/x.rego",
        "row": 5,
        "col": 1,
        "text": "YWxsb3cgaWYgewoJZm9vLmJhciA9PSAxCn0="
      }
    }
  ]
}
`, "TEMPDIR", tempDirPath)

	if diff := cmp.Diff(expectedOutput, string(stdout)); diff != "" {
		t.Errorf("unexpected result (-want, +got):\n%s", diff)
	}
}
