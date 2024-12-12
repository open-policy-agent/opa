//go:build go1.16
// +build go1.16

package bundle

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func TestFSLoader(t *testing.T) {
	archiveFS := make(fstest.MapFS)
	for k, v := range archiveFiles {
		file := strings.TrimPrefix(k, "/")
		archiveFS[file] = &fstest.MapFile{
			Data: []byte(v),
		}
	}

	loader, err := NewFSLoader(archiveFS)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	testLoader(t, loader, "", archiveFiles)
}

func TestFSLoaderWithFilter(t *testing.T) {
	files := map[string]string{
		"/a/data.json":            `{"foo": "not-bar"}`,
		"/policy.rego":            "package foo\n p = 1",
		"/policy_test.rego":       "package foo\n test_p { p }",
		"/a/b/c/policy.rego":      "package bar\n q = 1",
		"/a/b/c/policy_test.rego": "package bar\n test_q { q }",
	}

	expectedFiles := map[string]string{
		"/a/data.json":       `{"foo": "not-bar"}`,
		"/policy.rego":       "package foo\n p = 1",
		"/a/b/c/policy.rego": "package bar\n q = 1",
	}

	archiveFS := make(fstest.MapFS)

	for k, v := range files {
		file := strings.TrimPrefix(k, "/")
		archiveFS[file] = &fstest.MapFile{
			Data: []byte(v),
		}
	}

	loader, err := NewFSLoader(archiveFS)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	loader.WithFilter(func(abspath string, info os.FileInfo, depth int) bool {
		return getFilter("*_test.rego", 1)(abspath, info, depth)
	})

	testLoader(t, loader, "", expectedFiles)
}
