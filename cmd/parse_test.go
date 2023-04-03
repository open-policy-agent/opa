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
      "col": 1
    },
    "path": [
      {
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 1,
          "col": 9
        },
        "type": "var",
        "value": "data"
      },
      {
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 1,
          "col": 9
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
            "col": 7
          },
          "terms": {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 3,
              "col": 7
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
            "col": 7
          },
          "type": "number",
          "value": 1
        },
        "ref": [
          {
            "type": "var",
            "value": "p"
          }
        ],
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 3,
          "col": 3
        }
      },
      "location": {
        "file": "TEMPDIR/x.rego",
        "row": 3,
        "col": 3
      }
    }
  ]
}
`, "TEMPDIR", tempDirPath, -1)

	if got, want := string(stdout), expectedOutput; got != want {
		t.Fatalf("Expected output\n%v\n, got\n%v", want, got)
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
      "col": 1
    },
    "path": [
      {
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 1,
          "col": 9
        },
        "type": "var",
        "value": "data"
      },
      {
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 1,
          "col": 9
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
            "col": 3
          },
          "terms": {
            "location": {
              "file": "TEMPDIR/x.rego",
              "row": 3,
              "col": 3
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
            "col": 19
          },
          "type": "boolean",
          "value": false
        },
        "ref": [
          {
            "type": "var",
            "value": "allow"
          }
        ],
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 3,
          "col": 11
        }
      },
      "location": {
        "file": "TEMPDIR/x.rego",
        "row": 3,
        "col": 3
      }
    },
    {
      "body": [
        {
          "index": 0,
          "location": {
            "file": "TEMPDIR/x.rego",
            "row": 5,
            "col": 7
          },
          "terms": [
            {
              "location": {
                "file": "TEMPDIR/x.rego",
                "row": 5,
                "col": 20
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 5,
                    "col": 20
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
                "col": 7
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 5,
                    "col": 7
                  },
                  "type": "var",
                  "value": "input"
                },
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 5,
                    "col": 13
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
                "col": 23
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
            "col": 7
          },
          "terms": [
            {
              "location": {
                "file": "TEMPDIR/x.rego",
                "row": 6,
                "col": 18
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 6,
                    "col": 18
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
                "col": 7
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 6,
                    "col": 7
                  },
                  "type": "var",
                  "value": "input"
                },
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 6,
                    "col": 13
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
                "col": 20
              },
              "type": "array",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 6,
                    "col": 21
                  },
                  "type": "string",
                  "value": "getUser"
                },
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 6,
                    "col": 32
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
            "col": 7
          },
          "terms": [
            {
              "location": {
                "file": "TEMPDIR/x.rego",
                "row": 7,
                "col": 18
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 7,
                    "col": 18
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
                "col": 7
              },
              "type": "ref",
              "value": [
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 7,
                    "col": 7
                  },
                  "type": "var",
                  "value": "input"
                },
                {
                  "location": {
                    "file": "TEMPDIR/x.rego",
                    "row": 7,
                    "col": 13
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
                "col": 21
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
            "col": 13
          },
          "type": "boolean",
          "value": true
        },
        "ref": [
          {
            "type": "var",
            "value": "allow"
          }
        ],
        "location": {
          "file": "TEMPDIR/x.rego",
          "row": 4,
          "col": 5
        }
      },
      "location": {
        "file": "TEMPDIR/x.rego",
        "row": 4,
        "col": 5
      }
    }
  ]
}
`, "TEMPDIR", tempDirPath, -1)

	if got, want := string(stdout), expectedOutput; got != want {
		t.Fatalf("Expected output\n%v\n, got\n%v", want, got)
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
