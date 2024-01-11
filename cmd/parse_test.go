package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

func TestParseExit0(t *testing.T) {

	files := map[string]string{
		"x.rego": `package x
		
		p = 1
		`,
	}
	errc, stdout, stderr, _ := testParse(t, files, &configuredParseParams)
	if errc != 0 {
		t.Fatalf("Expected exit code 0, got %v", errc)
	}
	if len(stderr) > 0 {
		t.Fatalf("Expected no stderr output, got:\n%s\n", string(stderr))
	}

	expectedOutput := `module
 package
  ref
   data
   "x"
 rule
  head
   ref
    p
   1
  body
   expr index=0
    true
`

	if got, want := string(stdout), expectedOutput; got != want {
		t.Fatalf("Expected output\n%v\n, got\n%v", want, got)
	}
}

func TestParseExit1(t *testing.T) {

	files := map[string]string{
		"x.rego": `???`,
	}
	errc, _, stderr, _ := testParse(t, files, &configuredParseParams)
	if errc != 1 {
		t.Fatalf("Expected exit code 1, got %v", errc)
	}
	if len(stderr) == 0 {
		t.Fatalf("Expected output in stderr")
	}
}

func TestParseJSONOutput(t *testing.T) {

	files := map[string]string{
		"x.rego": `package x
		
		p = 1
		`,
	}
	errc, stdout, stderr, _ := testParse(t, files, &parseParams{
		format: util.NewEnumFlag(parseFormatJSON, []string{parseFormatPretty, parseFormatJSON}),
	})
	if errc != 0 {
		t.Fatalf("Expected exit code 0, got %v", errc)
	}
	if len(stderr) > 0 {
		t.Fatalf("Expected no stderr output, got:\n%s\n", string(stderr))
	}

	expectedOutput := `{
  "package": {
    "path": [
      {
        "type": "var",
        "value": "data"
      },
      {
        "type": "string",
        "value": "x"
      }
    ]
  },
  "rules": [
    {
      "body": [
        {
          "index": 0,
          "terms": {
            "type": "boolean",
            "value": true
          }
        }
      ],
      "head": {
        "name": "p",
        "value": {
          "type": "number",
          "value": 1
        },
        "ref": [
          {
            "type": "var",
            "value": "p"
          }
        ]
      }
    }
  ]
}
`

	if got, want := string(stdout), expectedOutput; got != want {
		t.Fatalf("Expected output\n%v\n, got\n%v", want, got)
	}
}

func TestParseJSONOutputWithLocations(t *testing.T) {

	files := map[string]string{
		"x.rego": `package x

p = 1
`,
	}
	errc, stdout, stderr, tempDirPath := testParse(t, files, &parseParams{
		format:      util.NewEnumFlag(parseFormatJSON, []string{parseFormatPretty, parseFormatJSON}),
		jsonInclude: "locations",
	})
	if errc != 0 {
		t.Fatalf("Expected exit code 0, got %v", errc)
	}
	if len(stderr) > 0 {
		t.Fatalf("Expected no stderr output, got:\n%s\n", string(stderr))
	}

	expectedOutput := strings.Replace(`{
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
          "text": "eA=="
        },
        "type": "var",
        "value": "data"
      },
      {
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 1,
          "col": 9,
          "text": "eA=="
        },
        "type": "string",
        "value": "x"
      }
    ]
  },
  "rules": [
    {
      "body": [
        {
          "index": 0,
          "location": {
            "file": "TEMPDIR/x.rego",
            "row": 3,
            "col": 5,
            "text": "MQ=="
          },
          "terms": {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 3,
              "col": 5,
              "text": "MQ=="
            },
            "type": "boolean",
            "value": true
          }
        }
      ],
      "head": {
        "name": "p",
        "value": {
          "location": {
            "file": "TEMPDIR/x.rego",
            "row": 3,
            "col": 5,
            "text": "MQ=="
          },
          "type": "number",
          "value": 1
        },
        "ref": [
          {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 3,
              "col": 1,
              "text": "cA=="
            },
            "type": "var",
            "value": "p"
          }
        ],
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 3,
          "col": 1,
          "text": "cCA9IDE="
        }
      },
      "location": {
        "file": "TEMPDIR/x.rego",
        "row": 3,
        "col": 1,
        "text": "cCA9IDE="
      }
    }
  ]
}
`, "TEMPDIR", tempDirPath, -1)

	gotLines := strings.Split(string(stdout), "\n")
	wantLines := strings.Split(expectedOutput, "\n")
	min := len(gotLines)
	if len(wantLines) < min {
		min = len(wantLines)
	}

	for i := 0; i < min; i++ {
		if gotLines[i] != wantLines[i] {
			t.Fatalf("Expected line %d to be\n%v\n, got\n%v", i, wantLines[i], gotLines[i])
		}
	}

	if len(gotLines) != len(wantLines) {
		t.Fatalf("Expected %d lines, got %d", len(wantLines), len(gotLines))
	}
}

func TestParseRefsJSONOutput(t *testing.T) {

	files := map[string]string{
		"x.rego": `package x
		
    a.b.c := true
		`,
	}
	errc, stdout, stderr, _ := testParse(t, files, &parseParams{
		format: util.NewEnumFlag(parseFormatJSON, []string{parseFormatPretty, parseFormatJSON}),
	})
	if errc != 0 {
		t.Fatalf("Expected exit code 0, got %v", errc)
	}
	if len(stderr) > 0 {
		t.Fatalf("Expected no stderr output, got:\n%s\n", string(stderr))
	}

	expectedOutput := `{
  "package": {
    "path": [
      {
        "type": "var",
        "value": "data"
      },
      {
        "type": "string",
        "value": "x"
      }
    ]
  },
  "rules": [
    {
      "body": [
        {
          "index": 0,
          "terms": {
            "type": "boolean",
            "value": true
          }
        }
      ],
      "head": {
        "value": {
          "type": "boolean",
          "value": true
        },
        "assign": true,
        "ref": [
          {
            "type": "var",
            "value": "a"
          },
          {
            "type": "string",
            "value": "b"
          },
          {
            "type": "string",
            "value": "c"
          }
        ]
      }
    }
  ]
}
`

	if got, want := string(stdout), expectedOutput; got != want {
		t.Fatalf("Expected output\n%v\n, got\n%v", want, got)
	}
}

func TestParseRefsJSONOutputWithLocations(t *testing.T) {

	files := map[string]string{
		"x.rego": `package x

a.b.c := true
`,
	}
	errc, stdout, stderr, tempDirPath := testParse(t, files, &parseParams{
		format:      util.NewEnumFlag(parseFormatJSON, []string{parseFormatPretty, parseFormatJSON}),
		jsonInclude: "locations",
	})
	if errc != 0 {
		t.Fatalf("Expected exit code 0, got %v", errc)
	}
	if len(stderr) > 0 {
		t.Fatalf("Expected no stderr output, got:\n%s\n", string(stderr))
	}

	expectedOutput := strings.Replace(`{
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
          "text": "eA=="
        },
        "type": "var",
        "value": "data"
      },
      {
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 1,
          "col": 9,
          "text": "eA=="
        },
        "type": "string",
        "value": "x"
      }
    ]
  },
  "rules": [
    {
      "body": [
        {
          "index": 0,
          "location": {
            "file": "TEMPDIR/x.rego",
            "row": 3,
            "col": 10,
            "text": "dHJ1ZQ=="
          },
          "terms": {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 3,
              "col": 10,
              "text": "dHJ1ZQ=="
            },
            "type": "boolean",
            "value": true
          }
        }
      ],
      "head": {
        "value": {
          "location": {
            "file": "TEMPDIR/x.rego",
            "row": 3,
            "col": 10,
            "text": "dHJ1ZQ=="
          },
          "type": "boolean",
          "value": true
        },
        "assign": true,
        "ref": [
          {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 3,
              "col": 1,
              "text": "YQ=="
            },
            "type": "var",
            "value": "a"
          },
          {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 3,
              "col": 3,
              "text": "Yg=="
            },
            "type": "string",
            "value": "b"
          },
          {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 3,
              "col": 5,
              "text": "Yw=="
            },
            "type": "string",
            "value": "c"
          }
        ],
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 3,
          "col": 1,
          "text": "YS5iLmMgOj0gdHJ1ZQ=="
        }
      },
      "location": {
        "file": "TEMPDIR/x.rego",
        "row": 3,
        "col": 1,
        "text": "YS5iLmMgOj0gdHJ1ZQ=="
      }
    }
  ]
}
`, "TEMPDIR", tempDirPath, -1)

	gotLines := strings.Split(string(stdout), "\n")
	wantLines := strings.Split(expectedOutput, "\n")
	min := len(gotLines)
	if len(wantLines) < min {
		min = len(wantLines)
	}

	for i := 0; i < min; i++ {
		if gotLines[i] != wantLines[i] {
			t.Fatalf("Expected line %d to be\n%v\n, got\n%v", i, wantLines[i], gotLines[i])
		}
	}

	if len(gotLines) != len(wantLines) {
		t.Fatalf("Expected %d lines, got %d", len(wantLines), len(gotLines))
	}
}
func TestParseRulesBlockJSONOutputWithLocations(t *testing.T) {

	files := map[string]string{
		"x.rego": `package x

default allow = false
allow = true {
  input.method == "GET"
  input.path = ["getUser", user]
  input.user == user
}
`,
	}
	errc, stdout, stderr, tempDirPath := testParse(t, files, &parseParams{
		format:      util.NewEnumFlag(parseFormatJSON, []string{parseFormatPretty, parseFormatJSON}),
		jsonInclude: "locations",
	})
	if errc != 0 {
		t.Fatalf("Expected exit code 0, got %v", errc)
	}
	if len(stderr) > 0 {
		t.Fatalf("Expected no stderr output, got:\n%s\n", string(stderr))
	}

	expectedOutput := strings.Replace(`{
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
          "text": "eA=="
        },
        "type": "var",
        "value": "data"
      },
      {
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 1,
          "col": 9,
          "text": "eA=="
        },
        "type": "string",
        "value": "x"
      }
    ]
  },
  "rules": [
    {
      "body": [
        {
          "index": 0,
          "location": {
            "file": "TEMPDIR/x.rego",
            "row": 3,
            "col": 1,
            "text": "ZGVmYXVsdA=="
          },
          "terms": {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 3,
              "col": 1,
              "text": "ZGVmYXVsdA=="
            },
            "type": "boolean",
            "value": true
          }
        }
      ],
      "default": true,
      "head": {
        "name": "allow",
        "value": {
          "location": {
            "file": "TEMPDIR/x.rego",
            "row": 3,
            "col": 17,
            "text": "ZmFsc2U="
          },
          "type": "boolean",
          "value": false
        },
        "ref": [
          {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 3,
              "col": 9,
              "text": "YWxsb3c="
            },
            "type": "var",
            "value": "allow"
          }
        ],
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 3,
          "col": 9,
          "text": "YWxsb3cgPSBmYWxzZQ=="
        }
      },
      "location": {
        "file": "TEMPDIR/x.rego",
        "row": 3,
        "col": 1,
        "text": "ZGVmYXVsdA=="
      }
    },
    {
      "body": [
        {
          "index": 0,
          "location": {
            "file": "TEMPDIR/x.rego",
            "row": 5,
            "col": 3,
            "text": "aW5wdXQubWV0aG9kID09ICJHRVQi"
          },
          "terms": [
            {
              "location": {
                "file": "TEMPDIR/x.rego",
                "row": 5,
                "col": 16,
                "text": "PT0="
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 5,
                    "col": 16,
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
                "row": 5,
                "col": 3,
                "text": "aW5wdXQubWV0aG9k"
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 5,
                    "col": 3,
                    "text": "aW5wdXQ="
                  },
                  "type": "var",
                  "value": "input"
                },
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 5,
                    "col": 9,
                    "text": "bWV0aG9k"
                  },
                  "type": "string",
                  "value": "method"
                }
              ]
            },
            {
              "location": {
                "file": "TEMPDIR/x.rego",
                "row": 5,
                "col": 19,
                "text": "IkdFVCI="
              },
              "type": "string",
              "value": "GET"
            }
          ]
        },
        {
          "index": 1,
          "location": {
            "file": "TEMPDIR/x.rego",
            "row": 6,
            "col": 3,
            "text": "aW5wdXQucGF0aCA9IFsiZ2V0VXNlciIsIHVzZXJd"
          },
          "terms": [
            {
              "location": {
                "file": "TEMPDIR/x.rego",
                "row": 6,
                "col": 14,
                "text": "PQ=="
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 6,
                    "col": 14,
                    "text": "PQ=="
                  },
                  "type": "var",
                  "value": "eq"
                }
              ]
            },
            {
              "location": {
                "file": "TEMPDIR/x.rego",
                "row": 6,
                "col": 3,
                "text": "aW5wdXQucGF0aA=="
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 6,
                    "col": 3,
                    "text": "aW5wdXQ="
                  },
                  "type": "var",
                  "value": "input"
                },
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 6,
                    "col": 9,
                    "text": "cGF0aA=="
                  },
                  "type": "string",
                  "value": "path"
                }
              ]
            },
            {
              "location": {
                "file": "TEMPDIR/x.rego",
                "row": 6,
                "col": 16,
                "text": "WyJnZXRVc2VyIiwgdXNlcl0="
              },
              "type": "array",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 6,
                    "col": 17,
                    "text": "ImdldFVzZXIi"
                  },
                  "type": "string",
                  "value": "getUser"
                },
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 6,
                    "col": 28,
                    "text": "dXNlcg=="
                  },
                  "type": "var",
                  "value": "user"
                }
              ]
            }
          ]
        },
        {
          "index": 2,
          "location": {
            "file": "TEMPDIR/x.rego",
            "row": 7,
            "col": 3,
            "text": "aW5wdXQudXNlciA9PSB1c2Vy"
          },
          "terms": [
            {
              "location": {
                "file": "TEMPDIR/x.rego",
                "row": 7,
                "col": 14,
                "text": "PT0="
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 7,
                    "col": 14,
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
                "row": 7,
                "col": 3,
                "text": "aW5wdXQudXNlcg=="
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 7,
                    "col": 3,
                    "text": "aW5wdXQ="
                  },
                  "type": "var",
                  "value": "input"
                },
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 7,
                    "col": 9,
                    "text": "dXNlcg=="
                  },
                  "type": "string",
                  "value": "user"
                }
              ]
            },
            {
              "location": {
                "file": "TEMPDIR/x.rego",
                "row": 7,
                "col": 17,
                "text": "dXNlcg=="
              },
              "type": "var",
              "value": "user"
            }
          ]
        }
      ],
      "head": {
        "name": "allow",
        "value": {
          "location": {
            "file": "TEMPDIR/x.rego",
            "row": 4,
            "col": 9,
            "text": "dHJ1ZQ=="
          },
          "type": "boolean",
          "value": true
        },
        "ref": [
          {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 4,
              "col": 1,
              "text": "YWxsb3c="
            },
            "type": "var",
            "value": "allow"
          }
        ],
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 4,
          "col": 1,
          "text": "YWxsb3cgPSB0cnVl"
        }
      },
      "location": {
        "file": "TEMPDIR/x.rego",
        "row": 4,
        "col": 1,
        "text": "YWxsb3cgPSB0cnVlIHsKICBpbnB1dC5tZXRob2QgPT0gIkdFVCIKICBpbnB1dC5wYXRoID0gWyJnZXRVc2VyIiwgdXNlcl0KICBpbnB1dC51c2VyID09IHVzZXIKfQ=="
      }
    }
  ]
}
`, "TEMPDIR", tempDirPath, -1)

	gotLines := strings.Split(string(stdout), "\n")
	wantLines := strings.Split(expectedOutput, "\n")
	min := len(gotLines)
	if len(wantLines) < min {
		min = len(wantLines)
	}

	for i := 0; i < min; i++ {
		if gotLines[i] != wantLines[i] {
			t.Fatalf("Expected line %d to be\n%v\n, got\n%v", i, wantLines[i], gotLines[i])
		}
	}

	if len(gotLines) != len(wantLines) {
		t.Fatalf("Expected %d lines, got %d", len(wantLines), len(gotLines))
	}
}

func TestParseJSONOutputComments(t *testing.T) {

	files := map[string]string{
		"x.rego": `package x
		
		# comment
		p = 1
		`,
	}
	errc, stdout, stderr, _ := testParse(t, files, &parseParams{
		format:      util.NewEnumFlag(parseFormatJSON, []string{parseFormatPretty, parseFormatJSON}),
		jsonInclude: "comments",
	})
	if errc != 0 {
		t.Fatalf("Expected exit code 0, got %v", errc)
	}
	if len(stderr) > 0 {
		t.Fatalf("Expected no stderr output, got:\n%s\n", string(stderr))
	}

	expectedCommentTextValue := "IGNvbW1lbnQ="

	if !strings.Contains(string(stdout), expectedCommentTextValue) {
		t.Fatalf("Comment text value %q missing in output: %s", expectedCommentTextValue, string(stdout))
	}
}

func TestParseV1Compatible(t *testing.T) {
	tests := []struct {
		note         string
		v1Compatible bool
		policy       string
		expErrs      []string
	}{
		{
			note: "v0.x, keywords not used",
			policy: `package test
p[v] { 
	v := input.x
}`,
		},
		{
			note: "v0.x, keywords not imported",
			policy: `package test
p contains v if { 
	v := input.x
}`,
			expErrs: []string{
				"var cannot be used for rule name",
			},
		},
		{
			note: "v0.x, keywords imported",
			policy: `package test
import future.keywords
p contains v if { 
	v := input.x
}`,
		},
		{
			note: "v0.x, rego.v1 imported",
			policy: `package test
import rego.v1
p contains v if { 
	v := input.x
}`,
		},

		{
			note:         "v1.0, keywords not used",
			v1Compatible: true,
			policy: `package test
p[v] { 
	v := input.x
}`,
			expErrs: []string{
				"`if` keyword is required before rule body",
				"`contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v1.0, keywords not imported",
			v1Compatible: true,
			policy: `package test
p contains v if { 
	v := input.x
}`,
		},
		{
			note:         "v1.0, keywords imported",
			v1Compatible: true,
			policy: `package test
import future.keywords
p contains v if { 
	v := input.x
}`,
		},
		{
			note:         "v1.0, rego.v1 imported",
			v1Compatible: true,
			policy: `package test
import rego.v1
p contains v if { 
	v := input.x
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"policy.rego": tc.policy,
			}

			_, _, stderr, _ := testParse(t, files, &parseParams{
				format:       util.NewEnumFlag(parseFormatPretty, []string{parseFormatPretty, parseFormatJSON}),
				v1Compatible: tc.v1Compatible,
			})

			if len(tc.expErrs) > 0 {
				errs := string(stderr)
				for _, expErr := range tc.expErrs {
					if !strings.Contains(errs, expErr) {
						t.Fatalf("Expected error:\n\n%q\n\ngot:\n\n%s", expErr, errs)
					}
				}
			} else {
				if len(stderr) > 0 {
					t.Fatalf("Expected no stderr output, got:\n%s\n", string(stderr))
				}
			}
		})
	}
}

// Runs parse and returns the exit code, stdout, and stderr contents
func testParse(t *testing.T, files map[string]string, params *parseParams) (int, []byte, []byte, string) {
	t.Helper()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	var errc int

	var tempDirUsed string
	test.WithTempFS(files, func(path string) {
		var args []string
		for file := range files {
			args = append(args, filepath.Join(path, file))
		}
		errc = parse(args, params, stdout, stderr)

		tempDirUsed = path
	})

	return errc, stdout.Bytes(), stderr.Bytes(), tempDirUsed
}
