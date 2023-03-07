package bundle

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	opaBundle "github.com/open-policy-agent/opa/bundle"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBundleFromDisk_Legacy(t *testing.T) {
	var err error

	tempDir := t.TempDir()

	bundle := opaBundle.Bundle{
		Manifest: opaBundle.Manifest{
			Revision: "rev1",
		},
		Data: map[string]interface{}{
			"foo": "bar",
		},
	}

	var buf bytes.Buffer
	err = opaBundle.NewWriter(&buf).Write(bundle)
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

	loadedBundle, err := LoadBundleFromDisk(tempDir, "", nil)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !loadedBundle.Equal(bundle) {
		t.Fatalf("unexpected bundle: %#v", loadedBundle)
	}
}

func TestLoadBundleFromDisk(t *testing.T) {
	var err error

	tempDir := t.TempDir()

	bundle := opaBundle.Bundle{
		Manifest: opaBundle.Manifest{
			Revision: "rev1",
		},
		Data: map[string]interface{}{
			"foo": "bar",
		},
	}

	var buf bytes.Buffer
	err = opaBundle.NewWriter(&buf).Write(bundle)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	bundlePackagePath := filepath.Join(tempDir, "bundlePackage.tar.gz")
	f, err := os.Create(bundlePackagePath)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	packageData := bundlePackage{
		Etag:   "123",
		Bundle: buf.Bytes(),
	}

	jsonPackageData, err := json.Marshal(packageData)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	zw := gzip.NewWriter(f)
	zw.Name = "bundlePackage.tar.gz"

	_, err = zw.Write(jsonPackageData)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = zw.Close()
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	loadedBundle, err := LoadBundleFromDisk(tempDir, "", nil)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !loadedBundle.Equal(bundle) {
		t.Fatalf("unexpected bundle: %#v", loadedBundle)
	}

	if loadedBundle.Etag != "123" {
		t.Fatalf("unexpected etag: %s", bundle.Etag)
	}
}

func TestSaveBundleToDisk(t *testing.T) {
	var err error

	tempDir := t.TempDir()

	bundle := opaBundle.Bundle{
		Manifest: opaBundle.Manifest{
			Revision: "rev1",
		},
		Data: map[string]interface{}{
			"foo": "bar",
		},
	}

	var buf bytes.Buffer
	err = opaBundle.NewWriter(&buf).Write(bundle)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = SaveBundlePackageToDisk(tempDir, &buf, &SaveOptions{Etag: "123"})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	loadedBundle, err := LoadBundleFromDisk(tempDir, "", nil)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !loadedBundle.Equal(bundle) {
		t.Fatalf("unexpected bundle: %#v", loadedBundle)
	}

	if loadedBundle.Etag != "123" {
		t.Fatalf("unexpected etag: %s", bundle.Etag)
	}
}
