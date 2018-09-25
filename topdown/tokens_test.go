package topdown

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"github.com/open-policy-agent/opa/ast"
	"testing"
)

func TestParseTokenConstraints(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		var constraints tokenConstraints
		var err error
		c := ast.NewObject()
		constraints, err = parseTokenConstraints(c)
		if err != nil {
			t.Fatalf("parseTokenConstraints: %v", err)
		}
		if constraints.alg != "" {
			t.Errorf("alg: %v", constraints.alg)
		}
		if constraints.key != nil {
			t.Errorf("key: %v", constraints.key)
		}
	})
	t.Run("Alg", func(t *testing.T) {
		var constraints tokenConstraints
		var err error
		c := ast.NewObject()
		c.Insert(ast.StringTerm("alg"), ast.StringTerm("RS256"))
		constraints, err = parseTokenConstraints(c)
		if err != nil {
			t.Fatalf("parseTokenConstraints: %v", err)
		}
		if constraints.alg != "RS256" {
			t.Errorf("alg: %v", constraints.alg)
		}
	})
	t.Run("Cert", func(t *testing.T) {
		var constraints tokenConstraints
		var err error
		c := ast.NewObject()
		c.Insert(ast.StringTerm("cert"), ast.StringTerm(`-----BEGIN CERTIFICATE-----
MIIBcDCCARagAwIBAgIJAMZmuGSIfvgzMAoGCCqGSM49BAMCMBMxETAPBgNVBAMM
CHdoYXRldmVyMB4XDTE4MDgxMDE0Mjg1NFoXDTE4MDkwOTE0Mjg1NFowEzERMA8G
A1UEAwwId2hhdGV2ZXIwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATPwn3WCEXL
mjp/bFniDwuwsfu7bASlPae2PyWhqGeWwe23Xlyx+tSqxlkXYe4pZ23BkAAscpGj
yn5gXHExyDlKo1MwUTAdBgNVHQ4EFgQUElRjSoVgKjUqY5AXz2o74cLzzS8wHwYD
VR0jBBgwFoAUElRjSoVgKjUqY5AXz2o74cLzzS8wDwYDVR0TAQH/BAUwAwEB/zAK
BggqhkjOPQQDAgNIADBFAiEA4yQ/88ZrUX68c6kOe9G11u8NUaUzd8pLOtkKhniN
OHoCIHmNX37JOqTcTzGn2u9+c8NlnvZ0uDvsd1BmKPaUmjmm
-----END CERTIFICATE-----`))
		constraints, err = parseTokenConstraints(c)
		if err != nil {
			t.Fatalf("parseTokenConstraints: %v", err)
		}
		pubKey := constraints.key.(*ecdsa.PublicKey)
		if pubKey.Curve != elliptic.P256() {
			t.Errorf("curve: %v", pubKey.Curve)
		}
		if pubKey.X.Text(16) != "cfc27dd60845cb9a3a7f6c59e20f0bb0b1fbbb6c04a53da7b63f25a1a86796c1" {
			t.Errorf("x: %x", pubKey.X)
		}
		if pubKey.Y.Text(16) != "edb75e5cb1fad4aac6591761ee29676dc190002c7291a3ca7e605c7131c8394a" {
			t.Errorf("y: %x", pubKey.Y)
		}
	})
	t.Run("Unrecognized", func(t *testing.T) {
		var err error
		c := ast.NewObject()
		c.Insert(ast.StringTerm("hatever"), ast.StringTerm("junk"))
		_, err = parseTokenConstraints(c)
		if err == nil {
			t.Fatalf("parseTokenConstraints: %v", err)
		}
	})
	t.Run("IllFormed", func(t *testing.T) {
		var err error
		c := ast.Array{ast.StringTerm("alg")}
		_, err = parseTokenConstraints(c)
		if err == nil {
			t.Fatalf("parseTokenConstraints: %v", err)
		}
	})
}

func TestParseTokenHeader(t *testing.T) {
	t.Run("Errors", func(t *testing.T) {
		token := &JSONWebToken{
			header: "",
		}
		var err error
		if err = token.decodeHeader(); err == nil {
			t.Fatalf("token.decodeHeader: %v", err)
		}
		token.header = "###"
		if err = token.decodeHeader(); err == nil {
			t.Fatalf("token.decodeHeader: %v", err)
		}
		token.header = base64.RawURLEncoding.EncodeToString([]byte(`{`))
		if err = token.decodeHeader(); err == nil {
			t.Fatalf("token.decodeHeader: %v", err)
		}
		token.header = base64.RawURLEncoding.EncodeToString([]byte(`{}`))
		if err = token.decodeHeader(); err != nil {
			t.Fatalf("token.decodeHeader: %v", err)
		}
		var header tokenHeader
		header, err = parseTokenHeader(token)
		if err != nil {
			t.Fatalf("parseTokenHeader: %v", err)
		}
		if header.valid() {
			t.Fatalf("tokenHeader valid")
		}
	})
	t.Run("Alg", func(t *testing.T) {
		token := &JSONWebToken{
			header: base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`)),
		}
		var err error
		if err = token.decodeHeader(); err != nil {
			t.Fatalf("token.decodeHeader: %v", err)
		}
		var header tokenHeader
		header, err = parseTokenHeader(token)
		if err != nil {
			t.Fatalf("parseTokenHeader: %v", err)
		}
		if !header.valid() {
			t.Fatalf("tokenHeader !valid")
		}
		if header.alg != "RS256" {
			t.Fatalf("alg: %s", header.alg)
		}
	})
}
