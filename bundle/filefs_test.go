//go:build go1.16
// +build go1.16

package bundle

import (
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
