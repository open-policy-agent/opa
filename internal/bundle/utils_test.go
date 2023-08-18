package bundle

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-policy-agent/opa/bundle"
)

func TestLoadBundleFromDiskLegacy(t *testing.T) {
	var err error

	tempDir := t.TempDir()

	sourceBundle := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "rev1",
		},
		Data: map[string]interface{}{
			"foo": "bar",
		},
	}

	var buf bytes.Buffer
	err = bundle.NewWriter(&buf).Write(sourceBundle)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	bundlePath := filepath.Join(tempDir, "bundle.tar.gz")
	f, err := os.Create(bundlePath)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	_, err = io.Copy(f, &buf)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	loadedBundle, err := LoadBundleFromDiskWithOptions(tempDir, &LoadOptions{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !loadedBundle.Equal(sourceBundle) {
		t.Fatalf("unexpected bundle: %#v", loadedBundle)
	}
}

func TestLoadBundleFromDiskBundlePackage(t *testing.T) {
	var err error

	tempDir := t.TempDir()

	sourceBundle := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "rev1",
		},
		Data: map[string]interface{}{
			"foo": "bar",
		},
	}

	var buf bytes.Buffer
	err = bundle.NewWriter(&buf).Write(sourceBundle)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = SaveBundleToDiskWithOptions(tempDir, &buf, &SaveOptions{Etag: "123"})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	loadedBundle, err := LoadBundleFromDiskWithOptions(tempDir, &LoadOptions{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !loadedBundle.Equal(sourceBundle) {
		t.Fatalf("unexpected bundle: %#v", loadedBundle)
	}

	if loadedBundle.Etag != "123" {
		t.Fatalf("unexpected etag: %s", sourceBundle.Etag)
	}
}

func TestSaveBundleToDisk(t *testing.T) {
	var err error

	tempDir := t.TempDir()

	sourceBundle := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "rev1",
		},
		Data: map[string]interface{}{
			"foo": "bar",
		},
	}

	var buf bytes.Buffer
	err = bundle.NewWriter(&buf).Write(sourceBundle)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = SaveBundleToDiskWithOptions(tempDir, &buf, &SaveOptions{Etag: "123"})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	loadedBundle, err := LoadBundleFromDiskWithOptions(tempDir, &LoadOptions{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !loadedBundle.Equal(sourceBundle) {
		t.Fatalf("unexpected bundle: %#v", loadedBundle)
	}

	if loadedBundle.Etag != "123" {
		t.Fatalf("unexpected etag: %s", loadedBundle.Etag)
	}
}

func TestSaveBundleToDiskOverwrite(t *testing.T) {
	var err error

	tempDir := t.TempDir()

	sourceBundle1 := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "rev1",
		},
		Data: map[string]interface{}{
			"foo": "bar",
		},
	}
	sourceBundle2 := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "rev2",
		},
		Data: map[string]interface{}{
			"foo": "baz",
		},
	}

	var buf1 bytes.Buffer
	err = bundle.NewWriter(&buf1).Write(sourceBundle1)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	var buf2 bytes.Buffer
	err = bundle.NewWriter(&buf2).Write(sourceBundle2)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	sourceBundle1ETag := "123"
	sourceBundle2Etag := "456"

	// write the first version of the bundle to disk
	err = SaveBundleToDiskWithOptions(tempDir, &buf1, &SaveOptions{Etag: sourceBundle1ETag})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	// load the bundle and validate it
	loadedBundle1, err := LoadBundleFromDiskWithOptions(tempDir, &LoadOptions{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !loadedBundle1.Equal(sourceBundle1) {
		t.Fatalf("unexpected bundle: %#v", loadedBundle1)
	}

	if loadedBundle1.Etag != sourceBundle1ETag {
		t.Fatalf("unexpected etag: %s", loadedBundle1.Etag)
	}

	// overwrite the bundle
	err = SaveBundleToDiskWithOptions(tempDir, &buf2, &SaveOptions{Etag: sourceBundle2Etag})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	// load the new bundle and validate it
	loadedBundle2, err := LoadBundleFromDiskWithOptions(tempDir, &LoadOptions{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !loadedBundle2.Equal(sourceBundle2) {
		t.Fatalf("unexpected bundle: %#v", loadedBundle1)
	}

	if loadedBundle2.Etag != sourceBundle2Etag {
		t.Fatalf("unexpected etag: %s", loadedBundle1.Etag)
	}
}

func TestSaveBundleToDiskNewPath(t *testing.T) {
	var err error

	tempDir := t.TempDir()

	sourceBundle := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "rev1",
		},
		Data: map[string]interface{}{
			"foo": "bar",
		},
	}

	var buf bytes.Buffer
	err = bundle.NewWriter(&buf).Write(sourceBundle)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	bundlePath := filepath.Join(tempDir, "foo", "bar", "example1")

	err = SaveBundleToDiskWithOptions(bundlePath, &buf, &SaveOptions{Etag: "123"})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	loadedBundle, err := LoadBundleFromDiskWithOptions(bundlePath, &LoadOptions{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !loadedBundle.Equal(sourceBundle) {
		t.Fatalf("unexpected bundle: %#v", loadedBundle)
	}

	if loadedBundle.Etag != "123" {
		t.Fatalf("unexpected etag: %s", sourceBundle.Etag)
	}
}

func TestSaveBundleToDiskNil(t *testing.T) {
	var err error
	srcDir := t.TempDir()

	err = SaveBundleToDiskWithOptions(srcDir, nil, &SaveOptions{})
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	expErrMsg := "no raw bundle bytes to persist to disk"
	if err.Error() != expErrMsg {
		t.Fatalf("expected error: %v but got: %v", expErrMsg, err)
	}
}
