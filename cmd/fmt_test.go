package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/format"
	"github.com/open-policy-agent/opa/v1/util/test"
)

const formattedV0 = `package test

p {
	a == 1
	true
	1 + 3
}
`

const formattedV1 = `package test

p if {
	a == 1
	true
	1 + 3
}
`

const unformattedV0 = `
        package test

        p { a == 1; true
                1 +    3
        }


`

const unformattedV1 = `
        package test

        p if{ a == 1; true
                1 +    3
        }


`

const singleWrongArity = `package test
import rego.v1

p if {
	a := 1
	b := 2
	plus(a, b, c) == 3
}
`

const MultipleWrongArity = `package test
import rego.v1

p if {
	x:=5
	y:=7
	z:=6
	plus([x, y]) == 3
	and(true, false, false) == false
	plus(a, x, y, z)
}
`

const ComprehensionCommentShouldNotMoveFormatted = `package test

f(x) := [x |
	some v in x

	# regal ignore:external-reference
	x in data.foo
][0]
`

const ComprehensionCommentShouldNotMoveUnformatted = `package test

f(x) := [x |
	some v in x
	# regal ignore:external-reference
	x in data.foo
][0]
`

type errorWriter struct {
	ErrMsg string
}

func (ew errorWriter) Write([]byte) (int, error) {
	return 0, errors.New(ew.ErrMsg)
}

func TestFmtFormatFile(t *testing.T) {
	cases := []struct {
		note        string
		params      fmtCommandParams
		unformatted string
		formatted   string
	}{
		{
			note:        "v0",
			params:      fmtCommandParams{v0Compatible: true},
			unformatted: unformattedV0,
			formatted:   formattedV0,
		},
		{
			note:        "v1",
			params:      fmtCommandParams{},
			unformatted: unformattedV1,
			formatted:   formattedV1,
		},
		{
			note:        "comment in comprehension",
			params:      fmtCommandParams{},
			unformatted: ComprehensionCommentShouldNotMoveUnformatted,
			formatted:   ComprehensionCommentShouldNotMoveFormatted,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			var stdout bytes.Buffer

			files := map[string]string{
				"policy.rego": tc.unformatted,
			}

			test.WithTempFS(files, func(path string) {
				policyFile := filepath.Join(path, "policy.rego")
				info, err := os.Stat(policyFile)
				err = formatFile(&tc.params, &stdout, policyFile, info, err)
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}

				actual := stdout.String()
				if actual != tc.formatted {
					t.Fatalf("Expected:\n%s\n\nGot:\n%s\n\n", tc.formatted, actual)
				}
			})
		})
	}
}

func TestFmtFormatFileFailToReadFile(t *testing.T) {

	params := fmtCommandParams{
		diff: true,
	}

	var stdout = bytes.Buffer{}

	files := map[string]string{
		"policy.rego": unformattedV0,
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
	cases := []struct {
		note   string
		params fmtCommandParams
		module string
	}{
		{
			note:   "v0",
			params: fmtCommandParams{v0Compatible: true},
			module: formattedV0,
		},
		{
			note:   "v1",
			params: fmtCommandParams{},
			module: formattedV1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			var stdout bytes.Buffer

			files := map[string]string{
				"policy.rego": tc.module,
			}

			test.WithTempFS(files, func(path string) {
				policyFile := filepath.Join(path, "policy.rego")
				info, err := os.Stat(policyFile)
				err = formatFile(&tc.params, &stdout, policyFile, info, err)
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}

				actual := stdout.String()
				if actual != tc.module {
					t.Fatalf("Expected:%s\n\nGot:\n%s\n\n", tc.module, actual)
				}
			})
		})
	}
}

func TestFmtFailFormatFileNoChanges(t *testing.T) {
	cases := []struct {
		note   string
		params fmtCommandParams
		module string
	}{
		{
			note: "v0",
			params: fmtCommandParams{
				v0Compatible: true,
				fail:         true,
				diff:         true,
			},
			module: formattedV0,
		},
		{
			note: "v1",
			params: fmtCommandParams{
				fail: true,
				diff: true,
			},
			module: formattedV1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			var stdout bytes.Buffer

			files := map[string]string{
				"policy.rego": tc.module,
			}

			test.WithTempFS(files, func(path string) {
				policyFile := filepath.Join(path, "policy.rego")
				info, err := os.Stat(policyFile)
				err = formatFile(&tc.params, &stdout, policyFile, info, err)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				actual := stdout.String()
				if len(actual) > 0 {
					t.Fatalf("Expected no output, got:\n%v\n\n", actual)
				}
			})
		})
	}
}

func TestFmtFormatFileDiff(t *testing.T) {
	cases := []struct {
		note   string
		params fmtCommandParams
		module string
	}{
		{
			note: "v0",
			params: fmtCommandParams{
				v0Compatible: true,
				diff:         true,
			},
			module: formattedV0,
		},
		{
			note: "v1",
			params: fmtCommandParams{
				diff: true,
			},
			module: formattedV1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			var stdout bytes.Buffer

			files := map[string]string{
				"policy.rego": tc.module,
			}

			test.WithTempFS(files, func(path string) {
				policyFile := filepath.Join(path, "policy.rego")
				info, err := os.Stat(policyFile)
				err = formatFile(&tc.params, &stdout, policyFile, info, err)
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}

				actual := stdout.String()

				if len(actual) > 0 {
					t.Fatalf("Expected no output, got:\n%s\n\n", actual)
				}
			})
		})
	}
}

func TestFmtFormatFileFailToPrintDiff(t *testing.T) {
	cases := []struct {
		note   string
		params fmtCommandParams
		module string
	}{
		{
			note: "v0",
			params: fmtCommandParams{
				v0Compatible: true,
				diff:         true,
			},
			module: unformattedV0,
		},
		{
			note: "v1",
			params: fmtCommandParams{
				diff: true,
			},
			module: unformattedV1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			errMsg := "io.Write error"
			var stdout = errorWriter{ErrMsg: errMsg}

			files := map[string]string{
				"policy.rego": tc.module,
			}

			test.WithTempFS(files, func(path string) {
				policyFile := filepath.Join(path, "policy.rego")
				info, err := os.Stat(policyFile)
				err = formatFile(&tc.params, &stdout, policyFile, info, err)
				if err == nil {
					t.Fatalf("Expected error, found none")
				}

				actual := err.Error()

				if !strings.Contains(actual, errMsg) {
					t.Fatalf("Expected error message to include %s, got:\n%s\n\n", errMsg, actual)
				}
			})
		})
	}
}

func TestFmtFormatFileList(t *testing.T) {
	cases := []struct {
		note   string
		params fmtCommandParams
		module string
	}{
		{
			note: "v0",
			params: fmtCommandParams{
				v0Compatible: true,
				list:         true,
			},
			module: formattedV0,
		},
		{
			note: "v1",
			params: fmtCommandParams{
				list: true,
			},
			module: formattedV1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			var stdout bytes.Buffer

			files := map[string]string{
				"policy.rego": tc.module,
			}

			test.WithTempFS(files, func(path string) {
				policyFile := filepath.Join(path, "policy.rego")
				info, err := os.Stat(policyFile)
				err = formatFile(&tc.params, &stdout, policyFile, info, err)
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}

				actual := strings.TrimSpace(stdout.String())

				if len(actual) > 0 {
					t.Fatalf("Expected no output, got:\n%s\n\n", actual)
				}
			})
		})
	}
}

func TestFmtFailFormatFileList(t *testing.T) {
	cases := []struct {
		note   string
		params fmtCommandParams
		module string
	}{
		{
			note: "v0",
			params: fmtCommandParams{
				v0Compatible: true,
				fail:         true,
				list:         true,
			},
			module: formattedV0,
		},
		{
			note: "v1",
			params: fmtCommandParams{
				fail: true,
				list: true,
			},
			module: formattedV1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			var stdout bytes.Buffer

			files := map[string]string{
				"policy.rego": tc.module,
			}

			test.WithTempFS(files, func(path string) {
				policyFile := filepath.Join(path, "policy.rego")
				info, err := os.Stat(policyFile)
				err = formatFile(&tc.params, &stdout, policyFile, info, err)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				actual := strings.TrimSpace(stdout.String())
				if len(actual) > 0 {
					t.Fatalf("Expected no output, got:\n%v\n\n", actual)
				}
			})
		})
	}
}

func TestFmtFailFormatFileChangesList(t *testing.T) {
	cases := []struct {
		note   string
		params fmtCommandParams
		module string
	}{
		{
			note: "v0",
			params: fmtCommandParams{
				v0Compatible: true,
				fail:         true,
				list:         true,
			},
			module: unformattedV0,
		},
		{
			note: "v1",
			params: fmtCommandParams{
				fail: true,
				list: true,
			},
			module: unformattedV1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			var stdout bytes.Buffer

			files := map[string]string{
				"policy.rego": tc.module,
			}

			test.WithTempFS(files, func(path string) {
				policyFile := filepath.Join(path, "policy.rego")
				info, err := os.Stat(policyFile)
				err = formatFile(&tc.params, &stdout, policyFile, info, err)
				if err == nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				actual := strings.TrimSpace(stdout.String())
				if len(actual) == 0 {
					t.Fatalf("Expected output, got:\n%v\n\n", actual)
				}
			})
		})
	}
}

func TestFmtFailFileNoChanges(t *testing.T) {
	cases := []struct {
		note   string
		params fmtCommandParams
		module string
	}{
		{
			note: "v0",
			params: fmtCommandParams{
				v0Compatible: true,
				fail:         true,
			},
			module: formattedV0,
		},
		{
			note: "v1",
			params: fmtCommandParams{
				fail: true,
			},
			module: formattedV1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"policy.rego": tc.module,
			}

			test.WithTempFS(files, func(path string) {
				policyFile := filepath.Join(path, "policy.rego")
				info, err := os.Stat(policyFile)
				err = formatFile(&tc.params, io.Discard, policyFile, info, err)
				if err != nil {
					t.Fatalf("Expected error but did not receive one")
				}
			})
		})
	}
}

func TestFmtFailFileChanges(t *testing.T) {
	cases := []struct {
		note   string
		params fmtCommandParams
		module string
	}{
		{
			note: "v0",
			params: fmtCommandParams{
				v0Compatible: true,
				fail:         true,
			},
			module: unformattedV0,
		},
		{
			note: "v1",
			params: fmtCommandParams{
				fail: true,
			},
			module: unformattedV1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"policy.rego": tc.module,
			}

			test.WithTempFS(files, func(path string) {
				policyFile := filepath.Join(path, "policy.rego")
				info, err := os.Stat(policyFile)
				err = formatFile(&tc.params, io.Discard, policyFile, info, err)
				if err == nil {
					t.Fatalf("Unexpected error: %s", err)
				}
			})
		})
	}
}

func TestFmtFailFileChangesDiff(t *testing.T) {
	cases := []struct {
		note   string
		params fmtCommandParams
		module string
	}{
		{
			note: "v0",
			params: fmtCommandParams{
				v0Compatible: true,
				diff:         true,
				fail:         true,
			},
			module: unformattedV0,
		},
		{
			note: "v1",
			params: fmtCommandParams{
				diff: true,
				fail: true,
			},
			module: unformattedV1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			var stdout bytes.Buffer

			files := map[string]string{
				"policy.rego": tc.module,
			}

			test.WithTempFS(files, func(path string) {
				policyFile := filepath.Join(path, "policy.rego")
				info, err := os.Stat(policyFile)
				err = formatFile(&tc.params, &stdout, policyFile, info, err)
				if err == nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				actual := strings.TrimSpace(stdout.String())
				if len(actual) == 0 {
					t.Fatalf("Expected output, got:\n%v\n\n", actual)
				}
			})
		})
	}
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

		loc := ast.Location{File: policyFile, Row: 7}
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
			{File: policyFile, Row: 8},
			{File: policyFile, Row: 9},
			{File: policyFile, Row: 10},
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
		for i := range 3 {
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
		note         string
		v1Compatible bool
		input        string
		expected     string
		expectedErr  string
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
		{
			note:         "v1 module",
			v1Compatible: true,
			input: `package test

p contains x if {
	some x in ["a", "b", "c"]
}`,
			expected: `package test

import rego.v1

p contains x if {
	some x in ["a", "b", "c"]
}
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			params := fmtCommandParams{
				regoV1:       true,
				v1Compatible: tc.v1Compatible,
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

func TestFmt_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note          string
		dropV0Imports bool
		input         string
		expected      string
		expectedErrs  []string
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
			// NOTE: We keep the future imports to create the broadest possible compatibility surface
			expected: `package test

import future.keywords

p if {
	input.x == 1
}

q contains "foo" if {
	input.x == 2
}
`,
		},
		{
			note: "future imports, drop v0 imports",
			input: `package test
import future.keywords.if
import future.keywords.contains
p if {
	input.x == 1
}

q contains "foo" if {
	input.x == 2
}
`,
			expected: `package test

import future.keywords.contains
import future.keywords.if

p if {
	input.x == 1
}

q contains "foo" if {
	input.x == 2
}
`,
		},
		{
			note: "rego.v1 import",
			input: `package test
import rego.v1
p if {
	input.x == 1
}

q contains "foo" if {
	input.x == 2
}
`,
			// NOTE: We keep the rego.v1 import to create the broadest possible compatibility surface
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
			note:          "rego.v1 import, drop v0 imports",
			dropV0Imports: true,
			input: `package test
import rego.v1
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
			params := fmtCommandParams{}
			params.dropV0Imports = tc.dropV0Imports

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
