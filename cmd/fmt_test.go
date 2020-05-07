package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/util/test"
)

func TestFmtFormatFile(t *testing.T) {
	params := fmtCommandParams{}
	var stdout bytes.Buffer

	unformatted := `
	package test
	
	p { a == 1; true
		1 +    3
	}

	
	`

	formatted := `package test

p {
	a == 1
	true
	1 + 3
}
`

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

	policyContent := `package test

p {
	a == 1
	true
	1 + 3
}
`

	files := map[string]string{
		"policy.rego": policyContent,
	}

	test.WithTempFS(files, func(path string) {
		policyFile := filepath.Join(path, "policy.rego")
		info, err := os.Stat(policyFile)
		err = formatFile(&params, &stdout, policyFile, info, err)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		actual := stdout.String()
		if actual != policyContent {
			t.Fatalf("Expected:%s\n\nGot:\n%s\n\n", policyContent, actual)
		}
	})
}

func TestFmtFormatFileDiff(t *testing.T) {
	params := fmtCommandParams{
		diff: true,
	}
	var stdout bytes.Buffer

	policyContent := `package test

p {
	a == 1
	true
	1 + 3
}
`

	files := map[string]string{
		"policy.rego": policyContent,
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

	policyContent := `package test

p {
	a == 1
	true
	1 + 3
}
`

	files := map[string]string{
		"policy.rego": policyContent,
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
