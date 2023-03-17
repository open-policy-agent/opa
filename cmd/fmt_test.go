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
		expectedSingleWrongArityErr := newError("failed to parse Rego source file: %v", fmt.Errorf("%s: %v", policyFile, expectedErrs))

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
		expectedMultipleWrongArityErr := newError("failed to parse Rego source file: %v", fmt.Errorf("%s: %v", policyFile, expectedErrs))

		if err != expectedMultipleWrongArityErr {
			t.Fatalf("Expected:%s\n\nGot:%s\n\n", expectedMultipleWrongArityErr, err)
		}
	})
}
