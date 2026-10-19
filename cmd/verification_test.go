package cmd

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
)

// writeSignatureTestBundle writes a bundle tarball at <dir>/bundle.tar.gz, signed
// with HS256/"secret" under key id keyID when signed is true, and returns its path.
func writeSignatureTestBundle(t *testing.T, dir string, signed bool, keyID string) string {
	t.Helper()

	mod := `package test

p := 7`

	bundleFile := filepath.Join(dir, "bundle.tar.gz")

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Roots: &[]string{"test"}, Revision: "123"},
		Data:     map[string]any{},
		Modules: []bundle.ModuleFile{
			{
				URL:    path.Join(bundleFile, "policy.rego"),
				Path:   "/policy.rego",
				Raw:    []byte(mod),
				Parsed: ast.MustParseModule(mod),
			},
		},
	}

	if signed {
		if err := b.GenerateSignature(bundle.NewSigningConfig("secret", "HS256", ""), keyID, true); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	f, err := os.Create(bundleFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := bundle.Write(f, b); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	return bundleFile
}

// TestBuildVerifiesInputBundleSignature covers `opa build -b --verification-key`,
// which verifies its *input* bundle before recompiling it.
func TestBuildVerifiesInputBundleSignature(t *testing.T) {
	tests := []struct {
		note     string
		signed   bool
		pubKeyID string
		expErr   string
	}{
		{
			// the documented CLI default: --verification-key-id defaults to "default"
			note:     "default key id rejects unsigned bundle",
			pubKeyID: defaultPublicKeyID,
			expErr:   "bundle missing .signatures.json file",
		},
		{
			note:     "empty key id rejects unsigned bundle",
			pubKeyID: "",
			expErr:   "bundle missing .signatures.json file",
		},
		{
			note:     "default key id accepts signed bundle",
			signed:   true,
			pubKeyID: defaultPublicKeyID,
		},
		// Note: there is no "empty key id accepts signed bundle" case. Signing
		// requires a non-empty key id, and an empty --verification-key-id cannot
		// resolve any JWT `kid`, so that flag value can only ever fail open on an
		// unsigned bundle, and it can never verify a real signature.
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			root := t.TempDir()
			bundleFile := writeSignatureTestBundle(t, root, tc.signed, tc.pubKeyID)

			params := newBuildParams()
			params.bundleMode = true
			params.outputFile = filepath.Join(root, "out.tar.gz")
			params.pubKey = "secret"
			params.pubKeyID = tc.pubKeyID
			params.algorithm = "HS256"

			err := dobuild(params, []string{bundleFile})

			if tc.expErr == "" {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !strings.Contains(err.Error(), tc.expErr) {
				t.Fatalf("Expected error to contain %q but got %v", tc.expErr, err)
			}
		})
	}
}

// TestInitRuntimeVerifiesBundleSignature covers `opa run -b --verification-key`.
func TestInitRuntimeVerifiesBundleSignature(t *testing.T) {
	tests := []struct {
		note       string
		signed     bool
		pubKeyID   string
		skipVerify bool
		expErr     string
	}{
		{
			note:     "default key id rejects unsigned bundle",
			pubKeyID: defaultPublicKeyID,
			expErr:   "bundle missing .signatures.json file",
		},
		{
			note:     "empty key id rejects unsigned bundle",
			pubKeyID: "",
			expErr:   "bundle missing .signatures.json file",
		},
		{
			note:     "default key id accepts signed bundle",
			signed:   true,
			pubKeyID: defaultPublicKeyID,
		},
		{
			note:       "skip verify accepts unsigned bundle",
			pubKeyID:   defaultPublicKeyID,
			skipVerify: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			root := t.TempDir()
			bundleFile := writeSignatureTestBundle(t, root, tc.signed, tc.pubKeyID)

			params := newTestRunParams()
			params.serverMode = false
			params.rt.BundleMode = true
			params.pubKey = "secret"
			params.pubKeyID = tc.pubKeyID
			params.algorithm = "HS256"
			params.skipBundleVerify = tc.skipVerify

			_, err := initRuntime(t.Context(), params, []string{bundleFile}, false)

			if tc.expErr == "" {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !strings.Contains(err.Error(), tc.expErr) {
				t.Fatalf("Expected error to contain %q but got %v", tc.expErr, err)
			}
		})
	}
}
