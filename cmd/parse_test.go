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
          "terms": {
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
        ]
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
