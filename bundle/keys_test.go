package bundle

import (
	"bytes"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// TestSignatureRequiredThroughShim checks the v0 API surface: the root bundle
// package type-aliases v1.VerificationConfig, so the strictness default and both
// opt-outs must be reachable here without a separate shim.
func TestSignatureRequiredThroughShim(t *testing.T) {
	keyConfigs := map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}

	if vc := NewVerificationConfig(keyConfigs, "", "", nil); !vc.SignatureRequired() {
		t.Fatal("expected NewVerificationConfig to require a signature by default")
	}

	if vc := NewVerificationConfig(keyConfigs, "", "", nil).WithOptionalSignature(); vc.SignatureRequired() {
		t.Fatal("expected WithOptionalSignature to make the signature optional")
	}

	vc := NewVerificationConfig(keyConfigs, "", "", nil)
	if vc.UnsignedDeltaBundlesAllowed() {
		t.Fatal("expected NewVerificationConfig to reject unsigned delta bundles by default")
	}

	vc.AllowUnsignedDeltaBundles = true
	if !vc.UnsignedDeltaBundlesAllowed() {
		t.Fatal("expected AllowUnsignedDeltaBundles to allow unsigned delta bundles")
	}

	if !vc.SignatureRequired() {
		t.Fatal("expected AllowUnsignedDeltaBundles to leave snapshot verification alone")
	}
}

func TestReadUnsignedBundleThroughShim(t *testing.T) {
	keyConfigs := map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}

	mod := `package test

p := 7`

	tests := []struct {
		note   string
		signed bool
		vc     *VerificationConfig
		expErr string
	}{
		{
			note:   "strict config rejects unsigned bundle",
			vc:     NewVerificationConfig(keyConfigs, "foo", "", nil),
			expErr: "bundle missing .signatures.json file",
		},
		{
			note:   "strict config with no keyid rejects unsigned bundle",
			vc:     NewVerificationConfig(keyConfigs, "", "", nil),
			expErr: "bundle missing .signatures.json file",
		},
		{
			note:   "strict config accepts signed bundle",
			signed: true,
			vc:     NewVerificationConfig(keyConfigs, "foo", "", nil),
		},
		{
			note: "optional signature config accepts unsigned bundle",
			vc:   NewVerificationConfig(keyConfigs, "", "", nil).WithOptionalSignature(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			b := Bundle{
				Manifest: Manifest{Roots: &[]string{"test"}, Revision: "123"},
				Data:     map[string]any{},
				Modules: []ModuleFile{
					{
						URL:    "/policy.rego",
						Path:   "/policy.rego",
						Raw:    []byte(mod),
						Parsed: ast.MustParseModule(mod),
					},
				},
			}

			if tc.signed {
				if err := b.GenerateSignature(NewSigningConfig("secret", "HS256", ""), "foo", true); err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			}

			var buf bytes.Buffer
			if err := Write(&buf, b); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			_, err := NewReader(&buf).WithBundleVerificationConfig(tc.vc).Read()

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
