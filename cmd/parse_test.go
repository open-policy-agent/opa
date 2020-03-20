package cmd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/open-policy-agent/opa/util/test"
)

func TestParseExit0(t *testing.T) {

	files := map[string]string{
		"x.rego": `package x
		
		p = 1
		`,
	}
	errc, _, stderr := testParse(t, files)
	if errc != 0 {
		t.Fatalf("Expected exit code 0, got %v", errc)
	}
	if len(stderr) > 0 {
		t.Fatalf("Expected no stderr output, got:\n%s\n", string(stderr))
	}
}

func TestParseExit1(t *testing.T) {

	files := map[string]string{
		"x.rego": `???`,
	}
	errc, _, stderr := testParse(t, files)
	if errc != 1 {
		t.Fatalf("Expected exit code 1, got %v", errc)
	}
	if len(stderr) == 0 {
		t.Fatalf("Expected output in stderr")
	}
}

// Runs parse and returns the exit code, stdout, and stderr contents
func testParse(t *testing.T, files map[string]string) (int, []byte, []byte) {
	t.Helper()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	var errc int

	test.WithTempFS(files, func(path string) {
		var args []string
		for file := range files {
			args = append(args, filepath.Join(path, file))
		}
		errc = parse(args, stdout, stderr)
	})

	return errc, stdout.Bytes(), stderr.Bytes()
}
