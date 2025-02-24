package exec

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/sdk"

	"github.com/open-policy-agent/opa/v1/util/test"
)

/*
Most of cmd/internal/exec/exec.go is tested indirectly in cmd/exec_test.go.
This file tests internal functions that are not as easily accessed, particularly
their edge and error cases.
*/

func TestParse(t *testing.T) {
	files := map[string]string{
		"valid.json":   `{"this": "that"}`,
		"invalid.json": `{[a",!`,
	}

	test.WithTempFS(files, func(rootDir string) {
		if val, err := parse(filepath.Join(rootDir, "no parser")); val != nil || err != nil {
			t.Fatalf("return values should have been nil passing a file path with no matching extension, got val: %v; err: %s", val, err.Error())
		}
		if _, err := parse(filepath.Join(rootDir, "nonexistent.json")); err == nil {
			t.Fatalf("should have received an error for passing a file path that does not exist")
		}
		if _, err := parse(filepath.Join(rootDir, "invalid.json")); err == nil {
			t.Fatalf("should have received an error for passing a file with invalid json")
		}
		if val, err := parse(filepath.Join(rootDir, "valid.json")); err != nil {
			t.Fatalf("unexpected error when passing file wiith valid json: %q", err.Error())
		} else {
			v := *val
			that, ok := v.(map[string]interface{})["this"]
			if !ok {
				t.Fatalf("expected parsed data to have key %q with value %q, found none", "this", "that")
			}
			if that.(string) != "that" {
				t.Fatalf("expected parsed data to have key %q with value %q, instead got value %v", "this", "that", that)
			}
		}
	})
}

func TestListAllPaths(t *testing.T) {
	files := map[string]string{
		"file.json": `{"this": "that"}`,
	}

	test.WithTempFS(files, func(rootDir string) {
		notFound := "./test/error"
		ch := listAllPaths([]string{rootDir, notFound})
		for item := range ch {
			if strings.Contains(item.Path, rootDir) {
				if item.Error != nil {
					t.Errorf("unexpected error for mock file: %q", item.Error)
				}
			} else if strings.Contains(item.Path, notFound) {
				if item.Error == nil {
					t.Errorf("expected error for tempDir, found none")
				}
			}
		}
	})
}

func TestExec(t *testing.T) {
	tests := []struct {
		description string
		files       map[string]string
		stdIn       bool
		input       string
		assertion   func(t *testing.T, buf string, err error)
	}{
		{
			description: "should read from valid JSON file and not raise an error",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system

		test_fun := x {
			x = false
			x
		}

		undefined_test {
			test_fun
		}`,
			},
			assertion: func(t *testing.T, _ string, err error) {
				if err != nil {
					t.Fatalf("unexpected error raised: %q", err.Error())
				}
			},
		},
		{
			description: "should raise error count if invalid json is found",
			files: map[string]string{
				"files/test.json": `{[foo":`,
				"bundle/x.rego": `package system

		test_fun := x {
			x = false
			x
		}

		undefined_test {
			test_fun
		}`,
			},
			assertion: func(t *testing.T, _ string, err error) {
				if err == nil {
					t.Fatalf("expected error, found none")
				}
				if r.errorCount != 1 {
					t.Fatalf("expected r.errorCount to be 1, got %d", r.errorCount)
				}
			},
		},
		{
			description: "should read from stdin-input if flag is set",
			files: map[string]string{
				"bundle/x.rego": `package system

		test_fun := x {
			x = false
			x
		}

		undefined_test {
			test_fun
		}`,
			},
			stdIn: true,
			input: `{"foo": 7}`,
			assertion: func(t *testing.T, _ string, err error) {
				if err != nil {
					t.Fatalf("unexpected error raised: %q", err.Error())
				}
			},
		},
		{
			description: "should read from files and stdin-input if flag is set",
			files: map[string]string{
				"files/test.json": `{"foo": 8}`,
				"bundle/x.rego": `package system

		test_fun := x {
			x = false
			x
		}

		undefined_test {
			test_fun
		}`,
			},
			stdIn: true,
			input: `{"foo": 7}`,
			assertion: func(t *testing.T, output string, err error) {
				if err != nil {
					t.Fatalf("unexpected error raised: %q", err.Error())
				}

				exp := `{
  "result": [
    {
      "path": "--stdin-input",
      "error": {
        "code": "opa_undefined_error",
        "message": "/system/main decision was undefined"
      }
    },
    {
      "path": "%ROOT%/files/test.json",
      "error": {
        "code": "opa_undefined_error",
        "message": "/system/main decision was undefined"
      }
    }
  ]
}
`
				if output != exp {
					t.Fatalf("expected output to be:\n\n%s\n\ngot:\n\n%s", exp, output)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			test.WithTempFS(tt.files, func(dir string) {
				var buf bytes.Buffer
				params := NewParams(&buf)
				_ = params.OutputFormat.Set("json")
				params.BundlePaths = []string{dir + "/bundle/"}
				params.FailNonEmpty = true
				if tt.stdIn {
					params.StdIn = true
					tempFile, err := os.CreateTemp(t.TempDir(), "test")
					if err != nil {
						t.Fatalf("unexpected error creating temp file: %q", err.Error())
					}
					if _, err := tempFile.WriteString(tt.input); err != nil {
						t.Fatalf("unexpeced error when writing to temp file: %q", err.Error())
					}
					if _, err := tempFile.Seek(0, 0); err != nil {
						t.Fatalf("unexpected error when rewinding temp file: %q", err.Error())
					}
					oldStdin := os.Stdin
					defer func() {
						os.Stdin = oldStdin
						os.Remove(tempFile.Name())
					}()
					os.Stdin = tempFile
				}

				if _, ok := tt.files["files/test.json"]; ok {
					params.Paths = append(params.Paths, dir+"/files/")
				}

				ctx := context.Background()
				opa, _ := sdk.New(ctx, sdk.Options{
					Config:        bytes.NewReader([]byte{}),
					Logger:        logging.NewNoOpLogger(),
					ConsoleLogger: logging.NewNoOpLogger(),
					Ready:         make(chan struct{}),
					V1Compatible:  params.V1Compatible,
				})

				err := Exec(ctx, opa, params)
				output := strings.ReplaceAll(buf.String(), dir, "%ROOT%")
				tt.assertion(t, output, err)
			})
		})
	}
}
