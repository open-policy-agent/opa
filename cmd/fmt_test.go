package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/format"
	"github.com/open-policy-agent/opa/util/test"
)

const formatted = `package test

p {
	a == 1
	true
	1 + 3
}
`

const unformatted = `
        package test

        p { a == 1; true
                1 +    3
        }


`

const singleWrongArity = `package test

p {
	a := 1
	b := 2
	plus(a, b, c) == 3
}
`

const MultipleWrongArity = `package test

p {
	x:=5
	y:=7
	z:=6
	plus([x, y]) == 3
	and(true, false, false) == false
	plus(a, x, y, z)
}
`

type errorWriter struct {
	ErrMsg string
}

func (ew errorWriter) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf(ew.ErrMsg)
}

func TestFmtFormatFile(t *testing.T) {
	params := fmtCommandParams{}
	var stdout bytes.Buffer

	files := map[string]string{
		"policy.rego": unformatted,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, &stdout, policyFile, info, err)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		actual := stdout.String()
		if actual != formatted {
			t.Fatalf("Expected:%s\n\nGot:\n%s\n\n", formatted, actual)
		}
	})
}

func TestFmtFormatFileFailToReadFile(t *testing.T) {

	params := fmtCommandParams{
		diff: true,
	}

	var stdout = bytes.Buffer{}

	files := map[string]string{
		"policy.rego": unformatted,
	}

	notThere := "notThere.rego"

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, &stdout, notThere, info, err)
		if err == nil {
			t.Fatalf("Expected error, found none")
		}

		actual := err.Error()

		if !strings.Contains(actual, notThere) {
			t.Fatalf("Expected error message to include %s, got:\n%s\n\n", notThere, actual)
		}
	})
}

func TestFmtFormatFileNoChanges(t *testing.T) {
	params := fmtCommandParams{}
	var stdout bytes.Buffer

	files := map[string]string{
		"policy.rego": formatted,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, &stdout, policyFile, info, err)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		actual := stdout.String()
		if actual != formatted {
			t.Fatalf("Expected:%s\n\nGot:\n%s\n\n", formatted, actual)
		}
	})
}

func TestFmtFailFormatFileNoChanges(t *testing.T) {
	params := fmtCommandParams{
		fail: true,
		diff: true,
	}
	var stdout bytes.Buffer

	files := map[string]string{
		"policy.rego": formatted,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, &stdout, policyFile, info, err)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		actual := stdout.String()
		if len(actual) > 0 {
			t.Fatalf("Expected no output, got:\n%v\n\n", actual)
		}
	})
}

func TestFmtFormatFileDiff(t *testing.T) {
	params := fmtCommandParams{
		diff: true,
	}
	var stdout bytes.Buffer

	files := map[string]string{
		"policy.rego": formatted,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, &stdout, policyFile, info, err)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		actual := stdout.String()

		if len(actual) > 0 {
			t.Fatalf("Expected no output, got:\n%s\n\n", actual)
		}
	})
}

func TestFmtFormatFileFailToPrintDiff(t *testing.T) {

	params := fmtCommandParams{
		diff: true,
	}

	errMsg := "io.Write error"
	var stdout = errorWriter{ErrMsg: errMsg}

	files := map[string]string{
		"policy.rego": unformatted,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, &stdout, policyFile, info, err)
		if err == nil {
			t.Fatalf("Expected error, found none")
		}

		actual := err.Error()

		if !strings.Contains(actual, errMsg) {
			t.Fatalf("Expected error message to include %s, got:\n%s\n\n", errMsg, actual)
		}
	})
}

func TestFmtFormatFileList(t *testing.T) {
	params := fmtCommandParams{
		list: true,
	}
	var stdout bytes.Buffer

	files := map[string]string{
		"policy.rego": formatted,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, &stdout, policyFile, info, err)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		actual := strings.TrimSpace(stdout.String())

		if len(actual) > 0 {
			t.Fatalf("Expected no output, got:\n%s\n\n", actual)
		}
	})
}

func TestFmtFailFormatFileList(t *testing.T) {
	params := fmtCommandParams{
		fail: true,
		list: true,
	}
	var stdout bytes.Buffer

	files := map[string]string{
		"policy.rego": formatted,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, &stdout, policyFile, info, err)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		actual := strings.TrimSpace(stdout.String())
		if len(actual) > 0 {
			t.Fatalf("Expected no output, got:\n%v\n\n", actual)
		}
	})
}

func TestFmtFailFormatFileChangesList(t *testing.T) {
	params := fmtCommandParams{
		fail: true,
		list: true,
	}
	var stdout bytes.Buffer

	files := map[string]string{
		"policy.rego": unformatted,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, &stdout, policyFile, info, err)
		if err == nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		actual := strings.TrimSpace(stdout.String())
		if len(actual) == 0 {
			t.Fatalf("Expected output, got:\n%v\n\n", actual)
		}
	})
}

func TestFmtFailFileNoChanges(t *testing.T) {
	params := fmtCommandParams{
		fail: true,
	}

	files := map[string]string{
		"policy.rego": formatted,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, io.Discard, policyFile, info, err)
		if err != nil {
			t.Fatalf("Expected error but did not receive one")
		}
	})
}

func TestFmtFailFileChanges(t *testing.T) {
	params := fmtCommandParams{
		fail: true,
	}

	files := map[string]string{
		"policy.rego": unformatted,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, io.Discard, policyFile, info, err)
		if err == nil {
			t.Fatalf("Unexpected error: %s", err)
		}
	})
}

func TestFmtFailFileChangesDiff(t *testing.T) {
	params := fmtCommandParams{
		diff: true,
		fail: true,
	}
	var stdout bytes.Buffer

	files := map[string]string{
		"policy.rego": unformatted,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, &stdout, policyFile, info, err)
		if err == nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		actual := strings.TrimSpace(stdout.String())
		if len(actual) == 0 {
			t.Fatalf("Expected output, got:\n%v\n\n", actual)
		}
	})
}

func TestFmtSingleWrongArityError(t *testing.T) {
	params := fmtCommandParams{}
	var stdout bytes.Buffer

	files := map[string]string{
		"policy.rego": singleWrongArity,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, &stdout, policyFile, info, err)
		if err == nil {
			t.Fatalf("Expected error but did not receive one")
		}

		loc := ast.Location{File: policyFile, Row: 6}
		errExp := ast.NewError(ast.TypeErr, &loc, "%s: %s", "plus", "arity mismatch")
		errExp.Details = &format.ArityFormatErrDetail{
			Have: []string{"var", "var", "var"},
			Want: []string{"number", "number"},
		}
		expectedErrs := ast.Errors(make([]*ast.Error, 1))
		expectedErrs[0] = errExp
		expectedSingleWrongArityErr := newError("failed to format Rego source file: %v", fmt.Errorf("%s: %v", policyFile, expectedErrs))

		if err != expectedSingleWrongArityErr {
			t.Fatalf("Expected:%s\n\nGot:%s\n\n", expectedSingleWrongArityErr, err)
		}
	})
}

func TestFmtMultipleWrongArityError(t *testing.T) {
	params := fmtCommandParams{}
	var stdout bytes.Buffer

	files := map[string]string{
		"policy.rego": MultipleWrongArity,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, &stdout, policyFile, info, err)
		if err == nil {
			t.Fatalf("Expected error but did not receive one")
		}

		locations := []ast.Location{
			{File: policyFile, Row: 7},
			{File: policyFile, Row: 8},
			{File: policyFile, Row: 9},
		}
		haveStrings := [][]string{
			{"array"},
			{"boolean", "boolean", "boolean"},
			{"var", "var", "var", "var"},
		}
		wantStrings := [][]string{
			{"number", "number"},
			{"set[any]", "set[any]"},
			{"number", "number"},
		}
		operators := []string{
			"plus",
			"and",
			"plus",
		}
		expectedErrs := ast.Errors(make([]*ast.Error, 3))
		for i := 0; i < 3; i++ {
			loc := locations[i]
			errExp := ast.NewError(ast.TypeErr, &loc, "%s: %s", operators[i], "arity mismatch")
			errExp.Details = &format.ArityFormatErrDetail{
				Have: haveStrings[i],
				Want: wantStrings[i],
			}
			expectedErrs[i] = errExp
		}
		expectedMultipleWrongArityErr := newError("failed to format Rego source file: %v", fmt.Errorf("%s: %v", policyFile, expectedErrs))

		if err != expectedMultipleWrongArityErr {
			t.Fatalf("Expected:%s\n\nGot:%s\n\n", expectedMultipleWrongArityErr, err)
		}
	})
}

func TestFmtRegoV1(t *testing.T) {
	tests := []struct {
		note        string
		input       string
		expected    string
		expectedErr string
	}{
		{
			note: "no future imports",
			input: `package test
p {
	input.x == 1
}

q.foo {
	input.x == 2
}
`,
			expected: `package test

import rego.v1

p if {
	input.x == 1
}

q contains "foo" if {
	input.x == 2
}
`,
		},
		{
			note: "future imports",
			input: `package test
import future.keywords
p if {
	input.x == 1
}

q contains "foo" {
	input.x == 2
}
`,
			expected: `package test

import rego.v1

p if {
	input.x == 1
}

q contains "foo" if {
	input.x == 2
}
`,
		},
		{
			note: "duplicate imports",
			input: `package test
import data.foo
import data.bar as foo
`,
			expectedErr: `failed to format Rego source file: 1 error occurred: %ROOT%/policy.rego:3: rego_compile_error: import must not shadow import data.foo`,
		},
		{
			note: "root document overrides",
			input: `package test
input {
	1 == 1
}

p {
	data := 2
}`,
			expectedErr: `failed to format Rego source file: 2 errors occurred:
%ROOT%/policy.rego:2: rego_compile_error: rules must not shadow input (use a different rule name)
%ROOT%/policy.rego:7: rego_compile_error: variables must not shadow data (use a different variable name)`,
		},
		{
			note: "deprecated built-in",
			input: `package test
p {
	any([true, false])
}

q := all([true, false])
`,
			expectedErr: `failed to format Rego source file: 2 errors occurred:
%ROOT%/policy.rego:3: rego_type_error: deprecated built-in function calls in expression: any
%ROOT%/policy.rego:6: rego_type_error: deprecated built-in function calls in expression: all`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			params := fmtCommandParams{
				regoV1: true,
			}

			files := map[string]string{
				"policy.rego": tc.input,
			}

			var stdout bytes.Buffer

			test.WithTempFS(files, func(root string) {
				policyFile := filepath.Join(root, "policy.rego")
				info, err := os.Stat(policyFile)
				err = formatFile(&params, &stdout, policyFile, info, err)

				if tc.expectedErr != "" {
					if err == nil {
						t.Fatalf("Expected error but got: %s", stdout.String())
					}
					expectedErr := strings.ReplaceAll(tc.expectedErr, "%ROOT%", root)
					var actualErr string
					switch err := err.(type) {
					case fmtError:
						actualErr = err.msg
					default:
						actualErr = err.Error()
					}
					if actualErr != expectedErr {
						t.Fatalf("Expected error:\n\n%s\n\nGot error:\n\n%s\n\n", expectedErr, actualErr)
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %s", err)
					}
					actual := stdout.String()
					if actual != tc.expected {
						t.Fatalf("Expected:%s\n\nGot:\n%s\n\n", tc.expected, actual)
					}
				}
			})
		})
	}
}

func TestFmtV1Compatible(t *testing.T) {
	tests := []struct {
		note         string
		input        string
		expected     string
		expectedErrs []string
	}{
		{
			note: "no keywords used",
			input: `package test
p {
	input.x == 1
}

q.foo {
	input.x == 2
}
`,
			expectedErrs: []string{
				"policy.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"policy.rego:6: rego_parse_error: `if` keyword is required before rule body",
				"policy.rego:6: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "no imports",
			input: `package test
p if {
	input.x == 1
}

q contains "foo" if {
	input.x == 2
}
`,
			expected: `package test

p if {
	input.x == 1
}

q contains "foo" if {
	input.x == 2
}
`,
		},
		{
			note: "future imports",
			input: `package test
import future.keywords
p if {
	input.x == 1
}

q contains "foo" if {
	input.x == 2
}
`,
			expected: `package test

p if {
	input.x == 1
}

q contains "foo" if {
	input.x == 2
}
`,
		},
		{
			note: "duplicate imports",
			input: `package test
import data.foo
import data.bar as foo
`,
			expectedErrs: []string{
				`policy.rego:3: rego_compile_error: import must not shadow import data.foo`,
			},
		},
		{
			note: "root document overrides",
			input: `package test
input if {
	1 == 1
}

p if {
	data := 2
}`,
			expectedErrs: []string{
				`policy.rego:2: rego_compile_error: rules must not shadow input (use a different rule name)`,
				`policy.rego:7: rego_compile_error: variables must not shadow data (use a different variable name)`,
			},
		},
		{
			note: "deprecated built-in",
			input: `package test
p if {
	any([true, false])
}

q := all([true, false])
`,
			expectedErrs: []string{
				`policy.rego:3: rego_type_error: deprecated built-in function calls in expression: any`,
				`policy.rego:6: rego_type_error: deprecated built-in function calls in expression: all`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			params := fmtCommandParams{
				v1Compatible: true,
			}

			files := map[string]string{
				"policy.rego": tc.input,
			}

			var stdout bytes.Buffer

			test.WithTempFS(files, func(root string) {
				policyFile := filepath.Join(root, "policy.rego")
				info, err := os.Stat(policyFile)
				err = formatFile(&params, &stdout, policyFile, info, err)

				if len(tc.expectedErrs) > 0 {
					if err == nil {
						t.Fatalf("Expected error but got: %s", stdout.String())
					}

					for _, expectedErr := range tc.expectedErrs {
						var actualErr string
						switch err := err.(type) {
						case fmtError:
							actualErr = err.msg
						default:
							actualErr = err.Error()
						}
						if !strings.Contains(actualErr, expectedErr) {
							t.Fatalf("Expected error to contain:\n\n%s\n\nGot error:\n\n%s\n\n", expectedErr, actualErr)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %s", err)
					}
					actual := stdout.String()
					if actual != tc.expected {
						t.Fatalf("Expected:%s\n\nGot:\n%s\n\n", tc.expected, actual)
					}
				}
			})
		})
	}
}
