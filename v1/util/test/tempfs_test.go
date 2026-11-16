package test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMakeTempFS(t *testing.T) {
	t.Run("creates files and cleanup removes them", func(t *testing.T) {
		root := t.TempDir()

		dir, cleanup, err := MakeTempFS(root, "opa_test", map[string]string{"a/b.txt": "hello"})
		if err != nil {
			t.Fatal(err)
		}

		bs, err := os.ReadFile(filepath.Join(dir, "a", "b.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if string(bs) != "hello" {
			t.Fatalf("unexpected content %q", bs)
		}

		cleanup()

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("expected %v to be removed, got: %v", dir, err)
		}
	})

	t.Run("failure removes the directory and returns no cleanup", func(t *testing.T) {
		root := t.TempDir()

		// "a" is a file in one entry and a directory in the other, so whichever is created second fails.
		_, cleanup, err := MakeTempFS(root, "opa_test", map[string]string{"a": "1", "a/b": "2"})
		if err == nil {
			t.Fatal("expected error")
		}
		if cleanup != nil {
			t.Fatal("expected nil cleanup on error")
		}

		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected temp dir to be removed, found %v", entries)
		}
	})
}
