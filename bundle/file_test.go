package bundle

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"

	"github.com/open-policy-agent/opa/util/test"
)

const testReadLimit = (1024 * 1024) + 1

var archiveFiles = map[string]string{
	"/a.json":                          "a",
	"/a/b.json":                        "b",
	"/a/b/c.json":                      "c",
	"/some.txt":                        "text",
	"/policy.rego":                     "package foo\n p = 1",
	"/deeper/dir/path/than/others/foo": "bar",
}

func TestTarballLoader(t *testing.T) {

	files := map[string]string{
		"/archive.tar.gz": "",
	}

	test.WithTempFS(files, func(rootDir string) {
		tarballFile := filepath.Join(rootDir, "archive.tar.gz")
		f, err := os.Create(tarballFile)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		var gzFiles [][2]string
		for name, content := range archiveFiles {
			gzFiles = append(gzFiles, [2]string{name, content})
		}

		_, err = f.Write(archive.MustWriteTarGz(gzFiles).Bytes())
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		f.Close()

		f, err = os.Open(tarballFile)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		loader := NewTarballLoaderWithBaseURL(f, tarballFile)
		defer f.Close()

		testLoader(t, loader, tarballFile, archiveFiles)

	})

}

func TestDirectoryLoader(t *testing.T) {
	test.WithTempFS(archiveFiles, func(rootDir string) {
		loader := NewDirectoryLoader(rootDir)

		testLoader(t, loader, rootDir, archiveFiles)
	})
}

func testLoader(t *testing.T, loader DirectoryLoader, baseURL string, expectedFiles map[string]string) {
	t.Helper()

	fileCount := 0
	for {
		f, err := loader.NextFile()
		if err != nil && err != io.EOF {
			t.Fatalf("Unexpected error: %s", err)
		} else if err == io.EOF {
			break
		}

		expPath := strings.TrimPrefix(f.URL(), baseURL)
		if f.Path() != expPath {
			t.Fatalf("Expected path to be %v but got %v", expPath, f.Path())
		}

		var buf bytes.Buffer
		n, err := f.Read(&buf, testReadLimit)
		f.Close() // always close, even on error
		if err != nil && err != io.EOF {
			t.Fatalf("Unexpected error: %s", err)
		} else if err == nil && n >= testReadLimit {
			t.Fatalf("Attempted to read too much data")
		}

		expectedContent, found := expectedFiles[f.Path()]
		if !found {
			t.Fatalf("Found unexpected file %s", f.Path())
		}
		if expectedContent != buf.String() {
			t.Fatalf("Content mismatch for file %s\n\nExpected:\n%s\n\nActual:\n%s\n\n",
				f.Path(), expectedContent, buf.String())
		}

		fileCount++
	}

	if fileCount != len(expectedFiles) {
		t.Fatalf("Expected to read %d files, read %d", len(expectedFiles), fileCount)
	}
}

func TestNewDirectoryLoaderNormalizedRoot(t *testing.T) {
	cases := []struct {
		note     string
		root     string
		expected string
	}{
		{
			note:     "abs",
			root:     "/a/b/c",
			expected: "/a/b/c",
		},
		{
			note:     "trailing slash",
			root:     "/a/b/c/",
			expected: "/a/b/c/",
		},
		{
			note:     "empty",
			root:     "",
			expected: "",
		},
		{
			note:     "single abs",
			root:     "/",
			expected: "/",
		},
		{
			note:     "single relative",
			root:     "foo",
			expected: "foo",
		},
		{
			note:     "single relative dot",
			root:     ".",
			expected: ".",
		},
		{
			note:     "single relative dot slash",
			root:     "./",
			expected: ".",
		},
		{
			note:     "relative leading dot slash",
			root:     "./a/b/c",
			expected: "a/b/c",
		},
		{
			note:     "relative",
			root:     "a/b/c",
			expected: "a/b/c",
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			l := NewDirectoryLoader(tc.root)
			actual := l.(*dirLoader).root
			if actual != tc.expected {
				t.Fatalf("Expected root %s got %s", tc.expected, actual)
			}
		})
	}
}
