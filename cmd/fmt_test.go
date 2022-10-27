package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
