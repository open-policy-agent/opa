package bundle

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/v1/util"

	"github.com/open-policy-agent/opa/v1/util/test"
)

var benchTestArchiveFiles = map[string]string{
	"/a.json":                          `"a"`,
	"/a/b.json":                        `"b"`,
	"/a/b/c.json":                      `"c"`,
	"/a/b/d/data.json":                 `"hello"`,
	"/a/c/data.yaml":                   "12",
	"/some.txt":                        "text",
	"/policy.rego":                     "package foo\n p = 1",
	"/roles/policy.rego":               "package bar\n p = 1",
	"/deeper/dir/path/than/others/foo": "bar",
}

func BenchmarkTarballLoader(b *testing.B) {
	files := map[string]string{
		"/archive.tar.gz": "",
	}
	sizes := []int{1000, 10000, 100000, 250000}

	for _, n := range sizes {
		expectedFiles := make(map[string]string, len(benchTestArchiveFiles)+1)
		for k, v := range benchTestArchiveFiles {
			expectedFiles[k] = v
		}
		expectedFiles["/x/data.json"] = benchTestGetFlatDataJSON(n)

		// We generate the tarball once in the tempfs, and then reuse it many
		// times in the benchmark.
		test.WithTempFS(files, func(rootDir string) {
			tarballFile := filepath.Join(rootDir, "archive.tar.gz")
			benchTestCreateTarballFile(b, rootDir, expectedFiles)

			b.ResetTimer()

			f, err := os.Open(tarballFile)
			if err != nil {
				b.Fatalf("Unexpected error: %s", err)
			}
			defer f.Close()

			b.Run(strconv.Itoa(n), func(b *testing.B) {
				// Reset the file reader.
				if _, err := f.Seek(0, 0); err != nil {
					b.Fatalf("Unexpected error: %s", err)
				}
				loader := NewTarballLoaderWithBaseURL(f, tarballFile)
				benchTestLoader(b, loader)
			})
		})
	}
}

func BenchmarkDirectoryLoader(b *testing.B) {
	sizes := []int{10000, 100000, 250000, 500000}

	for _, n := range sizes {
		expectedFiles := make(map[string]string, len(benchTestArchiveFiles)+1)
		for k, v := range benchTestArchiveFiles {
			expectedFiles[k] = v
		}
		expectedFiles["/x/data.json"] = benchTestGetFlatDataJSON(n)

		test.WithTempFS(expectedFiles, func(rootDir string) {
			b.ResetTimer()

			b.Run(strconv.Itoa(n), func(b *testing.B) {
				loader := NewDirectoryLoader(rootDir)
				benchTestLoader(b, loader)
			})
		})
	}
}

// Creates a flat JSON object of configurable size.
func benchTestGetFlatDataJSON(numKeys int) string {
	largeFile := make(map[string]string, numKeys)
	for i := range numKeys {
		largeFile[strconv.FormatInt(int64(i), 10)] = strings.Repeat("A", 1024)
	}
	return string(util.MustMarshalJSON(largeFile))
}

// Generates a tarball with a data.json of variable size.
func benchTestCreateTarballFile(b *testing.B, root string, filesToWrite map[string]string) {
	b.Helper()

	tarballFile := filepath.Join(root, "archive.tar.gz")
	f, err := os.Create(tarballFile)
	if err != nil {
		b.Fatalf("Unexpected error: %s", err)
	}

	gzFiles := make([][2]string, 0, len(filesToWrite))
	for name, content := range filesToWrite {
		gzFiles = append(gzFiles, [2]string{name, content})
	}

	_, err = f.Write(archive.MustWriteTarGz(gzFiles).Bytes())
	if err != nil {
		b.Fatalf("Unexpected error: %s", err)
	}
	f.Close()
}

// We specifically invoke the loader through the bundle reader to mimic
// real-world usage.
func benchTestLoader(b *testing.B, loader DirectoryLoader) {
	b.Helper()

	br := NewCustomReader(loader).WithLazyLoadingMode(true)
	bundle, err := br.Read()
	if err != nil {
		b.Fatal(err)
	}

	if len(bundle.Raw) == 0 {
		b.Fatal("bundle.Raw is unexpectedly empty")
	}
}
