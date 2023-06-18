// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-policy-agent/opa/util"

	"github.com/open-policy-agent/opa/util/test"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
	"github.com/open-policy-agent/opa/internal/jwx/jws"
)

func TestGenerateSignedToken(t *testing.T) {

	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
		{"/a/b/c/data.json", "[1,2,3]"},
		{"/a/b/d/data.json", "true"},
		{"/a/b/y/data.yaml", `foo: 1`},
		{"/example/example.rego", `package example`},
		{"/policy.wasm", `modules-compiled-as-wasm-binary`},
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}`},
	}

	input := []FileInfo{}

	expDigests := make([]string, len(files))
	expDigests[0] = "a005c38a509dc2d5a7407b9494efb2ad"
	expDigests[1] = "60f7b5dc86ded48785436192a08dbfd04894d7f1b417c4f8d3714679a7f78cb3c833f16a8559a1cf1f32968747dc1d95ef34826263dacf125ded8f5c374be4c0"
	expDigests[2] = "b326b5062b2f0e69046810717534cb09"
	expDigests[3] = "20f27a640a233e6524fe7d138898583cd43475724806feb26be7f214e1d10b29edf6a0d3cb08f82107a45686b61b8fdabab6406cf4e70efe134f42238dbd70ab"
	expDigests[4] = "655578028abb7b9006e93aff9dda8620"
	expDigests[5] = "6347e9be8e3051dc054fbbd3db72fb3f7ae7051c4ef6353e29895aa495452179e10e434fb4a60316e06916464bcc5d4ecabbb2797e04c0213943cf8e69f4c0ae"
	expDigests[6] = "36669864a622563256817033b1fc53db"

	for i, f := range files {
		file := FileInfo{
			Name: f[0],
			Hash: expDigests[i],
		}

		if i%2 == 0 {
			file.Algorithm = MD5.String()
		} else {
			file.Algorithm = SHA512.String()
		}

		input = append(input, file)
	}

	sc := NewSigningConfig("secret", "HS256", "")
	token, err := GenerateSignedToken(input, sc, "")
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// verify the signed token
	_, err = jws.Verify([]byte(token), jwa.SignatureAlgorithm("HS256"), []byte("secret"))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
}

func TestGenerateSignedTokenWithClaims(t *testing.T) {

	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
		{"/a/b/d/data.json", "true"},
		{"/example/example.rego", `package example`},
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}}`},
	}

	input := []FileInfo{}

	expDigests := make([]string, len(files))
	expDigests[0] = "a005c38a509dc2d5a7407b9494efb2ad"
	expDigests[1] = "b326b5062b2f0e69046810717534cb09"
	expDigests[2] = "655578028abb7b9006e93aff9dda8620"
	expDigests[3] = "36669864a622563256817033b1fc53db"

	for i, f := range files {
		file := FileInfo{
			Name:      f[0],
			Hash:      expDigests[i],
			Algorithm: MD5.String(),
		}
		input = append(input, file)
	}

	test.WithTempFS(map[string]string{}, func(rootDir string) {
		claims := make(map[string]interface{})
		claims["scope"] = "read"

		claimBytes, err := json.Marshal(claims)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		// create claims file
		claimsFile := filepath.Join(rootDir, "claims.json")
		if err := os.WriteFile(claimsFile, claimBytes, 0644); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		keyid := "foo"

		sc := NewSigningConfig("secret", "HS256", filepath.Join(rootDir, "claims.json"))

		token, err := GenerateSignedToken(input, sc, keyid)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		// verify the signed token
		_, err = jws.Verify([]byte(token), jwa.SignatureAlgorithm("HS256"), []byte("secret"))
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		// check the kid is in the header
		m, err := jws.ParseString(token)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		if v, ok := m.GetSignatures()[0].ProtectedHeaders().Get(jws.KeyIDKey); !ok || v != keyid {
			t.Errorf("key id not set")
		}
	})
}

func TestGeneratePayload(t *testing.T) {

	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
	}

	input := []FileInfo{}

	file := FileInfo{
		Name:      files[0][0],
		Hash:      "a005c38a509dc2d5a7407b9494efb2ad",
		Algorithm: MD5.String(),
	}
	input = append(input, file)

	sc := NewSigningConfig("secret", "HS256", "")
	keyID := "default"

	// non-empty key id
	bytes, err := generatePayload(input, sc, keyID)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	payload := make(map[string]interface{})
	if err := util.UnmarshalJSON(bytes, &payload); err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	if _, ok := payload["keyid"]; !ok {
		t.Fatal("Expected claim \"keyid\" in token")
	}

	if payload["keyid"] != keyID {
		t.Fatalf("Expected key id %v but got %v", keyID, payload["keyid"])
	}

	// empty key id
	bytes, err = generatePayload(input, sc, "")
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	payload = make(map[string]interface{})
	err = util.UnmarshalJSON(bytes, &payload)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	if _, ok := payload["keyid"]; ok {
		t.Fatal("Unexpected claim \"keyid\" in token")
	}
}

type CustomSigner struct{}

func (*CustomSigner) GenerateSignedToken(files []FileInfo, sc *SigningConfig, keyID string) (string, error) {
	return "", nil
}

func TestCustomSigner(t *testing.T) {
	custom := &CustomSigner{}
	err := RegisterSigner(defaultSignerID, custom)
	if err == nil {
		t.Fatalf("Expected error when registering with default ID")
	}
	if err := RegisterSigner("_test", custom); err != nil {
		t.Fatal(err)
	}
	defaultSigner, err := GetSigner(defaultSignerID)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if _, isDefault := defaultSigner.(*DefaultSigner); !isDefault {
		t.Fatalf("Expected DefaultSigner to be registered at key %s", defaultSignerID)
	}
	customSigner, err := GetSigner("_test")
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if _, isCustom := customSigner.(*CustomSigner); !isCustom {
		t.Fatalf("Expected CustomSigner to be registered at key _test")
	}
	if _, err = GetSigner("_unregistered"); err == nil {
		t.Fatalf("Expected error when no Signer exists at provided key")
	}
}
