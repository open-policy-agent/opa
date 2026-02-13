package topdown

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
)

func TestParseTokenConstraints(t *testing.T) {
	t.Parallel()

	wallclock := ast.NumberTerm(int64ToJSONNumber(time.Now().UnixNano()))
	t.Run("Empty", func(t *testing.T) {
		t.Parallel()

		c := ast.NewObject()
		constraints, err := parseTokenConstraints(c, wallclock)
		if err != nil {
			t.Fatalf("parseTokenConstraints: %v", err)
		}
		if constraints.alg != "" {
			t.Errorf("alg: %v", constraints.alg)
		}
		if constraints.keys != nil {
			t.Errorf("key: %v", constraints.keys)
		}
	})
	t.Run("Alg", func(t *testing.T) {
		t.Parallel()

		c := ast.NewObject()
		c.Insert(ast.StringTerm("alg"), ast.StringTerm("RS256"))
		constraints, err := parseTokenConstraints(c, wallclock)
		if err != nil {
			t.Fatalf("parseTokenConstraints: %v", err)
		}
		if constraints.alg != "RS256" {
			t.Errorf("alg: %v", constraints.alg)
		}
	})
	t.Run("Cert", func(t *testing.T) {
		t.Parallel()

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
		constraints, err := parseTokenConstraints(c, wallclock)
		if err != nil {
			t.Fatalf("parseTokenConstraints: %v", err)
		}
		pubKey := constraints.keys[0].key.(*ecdsa.PublicKey)
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
	t.Run("Cert Multi Key", func(t *testing.T) {
		t.Parallel()

		c := ast.NewObject()
		c.Insert(ast.StringTerm("cert"), ast.StringTerm(`{
    "keys": [
        {
          "kty": "EC",
          "use": "sig",
          "crv": "P-256",
          "kid": "k1",
          "x": "9Qq5S5VqMQoH-FOI4atcH6V3bua03C-5ZMZMG1rszwA",
          "y": "LLbFxWkGBEBrTm1GMYZJy1OXCH1KLweJMCgIEPIsibU",
          "alg": "ES256"
        },
        {
          "kty": "RSA",
          "e": "AQAB",
          "use": "enc",
          "kid": "k2",
          "alg": "RS256",
          "n": "sGu-fYVE2nq2dPxJlqAMI0Z8G3FD0XcWDnD8mkfO1ddKRGuUQZmfj4gWeZGyIk3cnuoy7KJCEqa3daXc08QHuFZyfn0rH33t8_AFsvb0q0i7R2FK-Gdqs_E0-sGpYMsRJdZWfCioLkYjIHEuVnRbi3DEsWqe484rEGbKF60jNRgGC4b-8pz-E538ZkssWxcqHrYIj5bjGEU36onjS3M_yrTuNvzv_8wRioK4fbcwmGne9bDxu8LcoSReWpPn0CnUkWnfqroRcMJnC87ZuJagDW1ZWCmU3psdsVanmFFh0DP6z0fsA4h8G2n9-qp-LEKFaWwo3IWlOsIzU3MHdcEiGw"
        }
	]
}
`))
		constraints, err := parseTokenConstraints(c, wallclock)
		if err != nil {
			t.Fatalf("parseTokenConstraints: %v", err)
		}
		elPubKey := constraints.keys[0].key.(*ecdsa.PublicKey)
		if elPubKey.Curve != elliptic.P256() {
			t.Errorf("curve: %v", elPubKey.Curve)
		}

		rsaPubKey := constraints.keys[1].key.(*rsa.PublicKey)
		if rsaPubKey.Size() != 256 {
			t.Errorf("expected size 256 found %d", rsaPubKey.Size())
		}
	})
	t.Run("Time", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		wallclock := ast.NumberTerm(int64ToJSONNumber(now.UnixNano()))

		t.Run("if provided, is parsed properly", func(t *testing.T) {
			c := ast.NewObject()
			c.Insert(ast.StringTerm("time"), wallclock)
			constraints, err := parseTokenConstraints(c, ast.NumberTerm("12134"))
			if err != nil {
				t.Fatalf("parseTokenConstraints: %v", err)
			}
			if exp, act := float64(now.UnixNano()), constraints.time; exp != act {
				t.Errorf("expected time constraint to be %f, got %f", exp, act)
			}
		})

		t.Run("unset, defaults to wallclock", func(t *testing.T) {
			t.Parallel()

			c := ast.NewObject() // 'time' constraint is unset
			constraints, err := parseTokenConstraints(c, wallclock)
			if err != nil {
				t.Fatalf("parseTokenConstraints: %v", err)
			}
			if exp, act := float64(now.UnixNano()), constraints.time; exp != act {
				t.Errorf("expected time constraint to be %f, got %f", exp, act)
			}
		})
	})

	t.Run("Unrecognized", func(t *testing.T) {
		t.Parallel()

		c := ast.NewObject()
		c.Insert(ast.StringTerm("whatever"), ast.StringTerm("junk"))
		_, err := parseTokenConstraints(c, wallclock)
		if err == nil {
			t.Fatalf("parseTokenConstraints: %v", err)
		}
	})
}

func TestParseTokenHeader(t *testing.T) {
	t.Parallel()

	t.Run("Errors", func(t *testing.T) {
		t.Parallel()

		token := &JSONWebToken{
			header: "",
		}
		if err := token.decodeHeader(); err == nil {
			t.Fatalf("token.decodeHeader: %v", err)
		}
		token.header = "###"
		if err := token.decodeHeader(); err == nil {
			t.Fatalf("token.decodeHeader: %v", err)
		}
		token.header = base64.RawURLEncoding.EncodeToString([]byte(`{`))
		if err := token.decodeHeader(); err == nil {
			t.Fatalf("token.decodeHeader: %v", err)
		}
		token.header = base64.RawURLEncoding.EncodeToString([]byte(`{}`))
		if err := token.decodeHeader(); err != nil {
			t.Fatalf("token.decodeHeader: %v", err)
		}
		header, err := parseTokenHeader(token)
		if err != nil {
			t.Fatalf("parseTokenHeader: %v", err)
		}
		if header.valid() {
			t.Fatalf("tokenHeader valid")
		}
	})
	t.Run("Alg", func(t *testing.T) {
		t.Parallel()

		token := &JSONWebToken{
			header: base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`)),
		}
		if err := token.decodeHeader(); err != nil {
			t.Fatalf("token.decodeHeader: %v", err)
		}
		header, err := parseTokenHeader(token)
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

func TestTopDownJWTEncodeSignES256(t *testing.T) {
	t.Parallel()

	const examplePayload = `{"iss":"joe",` + "\r\n" + ` "exp":1300819380,` + "\r\n" + ` "http://example.com/is_root":true}`
	const es256Hdr = `{"alg":"ES256"}`
	const ecKey = `{
    "kty":"EC",
    "crv":"P-256",
    "x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU",
    "y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0",
    "d":"jpsQnnGQmL-YBIffH1136cspYG6-0iY7X1fCE9-E9LI"
  }`

	params := struct {
		note   string
		input1 string
		input2 string
		input3 string
	}{
		"https://tools.ietf.org/html/rfc7515#appendix-A.3",
		"`" + es256Hdr + "`",
		"`" + examplePayload + "`",
		"`" + ecKey + "`",
	}
	type test struct {
		note  string
		rules []string
	}

	tc := test{
		params.note,
		[]string{fmt.Sprintf(`p = x { io.jwt.encode_sign_raw(%s, %s, %s, x) }`, params.input1, params.input2, params.input3)},
	}

	compiler, err := compileRules(nil, tc.rules, nil)
	if err != nil {
		t.Errorf("%v: Compiler error: %v", tc.note, err)
		return
	}
	store := inmem.New()
	path := []string{"generated", "p"}
	var inputTerm *ast.Term

	ctx := t.Context()
	txn := storage.NewTransactionOrDie(ctx, store)

	defer store.Abort(ctx, txn)

	var lhs *ast.Term
	if len(path) == 0 {
		lhs = ast.NewTerm(ast.DefaultRootRef)
	} else {
		lhs = ast.MustParseTerm("data." + strings.Join(path, "."))
	}

	rhs := ast.VarTerm(ast.WildcardPrefix + "result")
	body := ast.NewBody(ast.Equality.Expr(lhs, rhs))

	query := NewQuery(body).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(inputTerm)

	var tracer BufferTracer

	if os.Getenv("OPA_TRACE_TEST") != "" {
		query = query.WithTracer(&tracer)
	}

	qrs, err := query.Run(ctx)

	if tracer != nil {
		PrettyTrace(os.Stdout, tracer)
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(qrs) == 0 {
		t.Fatal("Undefined result")
	}

	result, err := ast.JSON(qrs[0][rhs.Value.(ast.Var)].Value)
	if err != nil {
		t.Fatal(err)
	}
	// Verification

	var headers map[string]any
	err = json.Unmarshal([]byte(es256Hdr), &headers)
	if err != nil {
		t.Fatal("Failed to parse header")
	}
	algStr, ok := headers["alg"].(string)
	if !ok {
		t.Fatal("Failed to get algorithm from header")
	}
	alg, ok := jwa.LookupSignatureAlgorithm(algStr)
	if !ok {
		t.Fatalf("Failed to get algorithm: %s", algStr)
	}

	keys, err := jwk.ParseString(ecKey)
	if err != nil {
		t.Fatal("Failed to parse JWK")
	}
	jwkKey, ok := keys.Key(0)
	if !ok {
		t.Fatal("Failed to get first key")
	}
	var key any
	err = jwk.Export(jwkKey, &key)
	if err != nil {
		t.Fatal("Failed to create private key")
	}
	publicKey, err := jwk.PublicKeyOf(key)
	if err != nil {
		t.Fatalf("failed to get public key: %v", err)
	}
	var rawPublicKey any
	err = jwk.Export(publicKey, &rawPublicKey)
	if err != nil {
		t.Fatalf("failed to export public key: %v", err)
	}

	// Verify with vendor library

	verifiedPayload, err := jws.Verify([]byte(result.(string)), jws.WithKey(alg, rawPublicKey))
	if err != nil || string(verifiedPayload) != examplePayload {
		t.Fatal("Failed to verify message")
	}
}

// TestTopDownJWTEncodeSignEC needs to perform all tests inline because we do not know the
// expected values before hand
func TestTopDownJWTEncodeSignES512(t *testing.T) {
	t.Parallel()

	const examplePayload = `{"iss":"joe",` + "\r\n" + ` "exp":1300819380,` + "\r\n" + ` "http://example.com/is_root":true}`
	const es512Hdr = `{"alg":"ES512"}`
	const ecKey = `{
"kty":"EC",
"crv":"P-521",
"x":"AekpBQ8ST8a8VcfVOTNl353vSrDCLLJXmPk06wTjxrrjcBpXp5EOnYG_NjFZ6OvLFV1jSfS9tsz4qUxcWceqwQGk",
"y":"ADSmRA43Z1DSNx_RvcLI87cdL07l6jQyyBXMoxVg_l2Th-x3S1WDhjDly79ajL4Kkd0AZMaZmh9ubmf63e3kyMj2",
"d":"AY5pb7A0UFiB3RELSD64fTLOSV_jazdF7fLYyuTw8lOfRhWg6Y6rUrPAxerEzgdRhajnu0ferB0d53vM9mE15j2C"
}`

	params := struct {
		note   string
		input1 string
		input2 string
		input3 string
	}{
		"https://tools.ietf.org/html/rfc7515#appendix-A.4",
		"`" + es512Hdr + "`",
		"`" + examplePayload + "`",
		"`" + ecKey + "`",
	}
	type test struct {
		note  string
		rules []string
	}
	tests := []test{{
		params.note,
		[]string{fmt.Sprintf(`p = x { io.jwt.encode_sign_raw(%s, %s, %s, x) }`, params.input1, params.input2, params.input3)},
	}}

	tc := tests[0]

	compiler, err := compileRules(nil, tc.rules, nil)
	if err != nil {
		t.Errorf("%v: Compiler error: %v", tc.note, err)
		return
	}
	store := inmem.New()
	path := []string{"generated", "p"}
	var inputTerm *ast.Term

	ctx := t.Context()
	txn := storage.NewTransactionOrDie(ctx, store)

	defer store.Abort(ctx, txn)

	var lhs *ast.Term
	if len(path) == 0 {
		lhs = ast.NewTerm(ast.DefaultRootRef)
	} else {
		lhs = ast.MustParseTerm("data." + strings.Join(path, "."))
	}

	rhs := ast.VarTerm(ast.WildcardPrefix + "result")
	body := ast.NewBody(ast.Equality.Expr(lhs, rhs))

	query := NewQuery(body).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(inputTerm)

	var tracer BufferTracer

	if os.Getenv("OPA_TRACE_TEST") != "" {
		query = query.WithTracer(&tracer)
	}

	qrs, err := query.Run(ctx)

	if tracer != nil {
		PrettyTrace(os.Stdout, tracer)
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(qrs) == 0 {
		t.Fatal("Undefined result")
	}

	result, err := ast.JSON(qrs[0][rhs.Value.(ast.Var)].Value)
	if err != nil {
		t.Fatal(err)
	}
	// Verification

	var headers map[string]any
	err = json.Unmarshal([]byte(es512Hdr), &headers)
	if err != nil {
		t.Fatal("Failed to parse header")
	}
	algStr, ok := headers["alg"].(string)
	if !ok {
		t.Fatal("Failed to get algorithm from header")
	}
	alg, ok := jwa.LookupSignatureAlgorithm(algStr)
	if !ok {
		t.Fatalf("Failed to get algorithm: %s", algStr)
	}

	keys, err := jwk.ParseString(ecKey)
	if err != nil {
		t.Fatalf("Failed to parse JWK: %v", err)
	}
	jwkKey, ok := keys.Key(0)
	if !ok {
		t.Fatal("Failed to get first key")
	}
	var key any
	err = jwk.Export(jwkKey, &key)
	if err != nil {
		t.Fatalf("Failed to create private key: %v", err)
	}
	publicKey, err := jwk.PublicKeyOf(key)
	if err != nil {
		t.Fatalf("Failed to get public key: %v", err)
	}
	var rawPublicKey any
	err = jwk.Export(publicKey, &rawPublicKey)
	if err != nil {
		t.Fatalf("failed to export public key: %v", err)
	}

	// Verify with vendor library

	verifiedPayload, err := jws.Verify([]byte(result.(string)), jws.WithKey(alg, rawPublicKey))
	if err != nil || string(verifiedPayload) != examplePayload {
		t.Fatal("Failed to verify message")
	}
}

// NOTE(sr): The stdlib ecdsa package will randomly read 1 byte from the source
// and discard it: so passing a fixed-seed `rand.New(rand.Source(seed))` via
// `rego.WithSeed` will not do the trick, the output would still randomly be
// one of two possible signatures. To fix that for testing, we're reaching
// deeper here, and use a "constant number generator". It doesn't matter if the
// first byte is discarded, the second one looks just the same.
type cng struct{}

func (*cng) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 4
	}
	return len(p), nil
}

func TestTopdownJWTEncodeSignECWithSeedReturnsSameSignature(t *testing.T) {
	t.Parallel()

	query := `io.jwt.encode_sign({"alg": "ES256"},{"pay": "load"},
	  {"kty":"EC",
	   "crv":"P-256",
	   "x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU",
	   "y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0",
	   "d":"jpsQnnGQmL-YBIffH1136cspYG6-0iY7X1fCE9-E9LI"
	  }, x)`

	// NOTE(ae): the signature differs between Go 1.23 and 1.24, as the latter uses the rand/v2 package (or that's my take)
	var encodedSigned string
	if runtime.Version() < "go1.24" {
		encodedSigned = "eyJhbGciOiJFUzI1NiJ9.eyJwYXkiOiJsb2FkIn0.wDU6G2XTYFP3QdVYhy-PBzkacEFNJwVT4HPQHOLtUmJu-OcVUaX9n-Ukv50AJwoF59L2wS5aOzoUwuru48Q4tw"
	} else {
		encodedSigned = "eyJhbGciOiJFUzI1NiJ9.eyJwYXkiOiJsb2FkIn0.WAh1ydGVRdVwXNQ9i71LqUJSrs3WVDZENdN58jCkecC2oCXEnqcviaADIwcZbYmns5IfHNV1Euo6vBm75o5l9A"
	}

	for range 10 {
		q := NewQuery(ast.MustParseBody(query)).
			WithSeed(&cng{}).
			WithStrictBuiltinErrors(true).
			WithCompiler(ast.NewCompiler())

		qrs, err := q.Run(t.Context())
		if err != nil {
			t.Fatal(err)
		} else if len(qrs) != 1 {
			t.Fatal("expected exactly one result but got:", qrs)
		}

		if exp, act := 1, len(qrs); exp != act {
			t.Fatalf("expected %d results, got %d", exp, act)
		}

		if exp, act := ast.String(encodedSigned), qrs[0][ast.Var("x")].Value; !exp.Equal(act) {
			t.Fatalf("unexpected result: want %v, got %v", exp, act)
		}
	}
}

func TestTopdownJWTUnknownAlgTypesDiscardedFromJWKS(t *testing.T) {
	t.Parallel()

	cert := `{
    "keys": [
	    {
          "kty": "RSA",
          "e": "AQAB",
          "use": "enc",
          "kid": "k3",
          "alg": "RS256",
          "n": "sGu-fYVE2nq2dPxJlqAMI0Z8G3FD0XcWDnD8mkfO1ddKRGuUQZmfj4gWeZGyIk3cnuoy7KJCEqa3daXc08QHuFZyfn0rH33t8_AFsvb0q0i7R2FK-Gdqs_E0-sGpYMsRJdZWfCioLkYjIHEuVnRbi3DEsWqe484rEGbKF60jNRgGC4b-8pz-E538ZkssWxcqHrYIj5bjGEU36onjS3M_yrTuNvzv_8wRioK4fbcwmGne9bDxu8LcoSReWpPn0CnUkWnfqroRcMJnC87ZuJagDW1ZWCmU3psdsVanmFFh0DP6z0fsA4h8G2n9-qp-LEKFaWwo3IWlOsIzU3MHdcEiGw"
        },
	    {
		  "kid": "encryption algorithm",
		  "kty": "RSA",
		  "alg": "RSA-OAEP",
		  "use": "enc",
		  "n": "onlqv4UZx5ZabJ3TCq-IO0s0xaOwo6fWl9o4SzLXPbGtvxonQhoYOeMlS0XkdEdLzB-eqh_hkQ",
		  "e": "AQAB",
		  "x5c": [
			"MIICnTCCAYUCBgGAmcG0xjANBgkqhkiG9w0BAQsFADASMRAwDgYDVQQ2YVaQn47Eew=="
		  ],
		  "x5t": "WKfdwdQkg",
		  "x5t#S256": "2_FidAwjlCQl20"
		}
	]
}
`
	keys, err := getKeysFromCertOrJWK(cert)
	if err != nil {
		t.Fatal(err)
	}

	if len(keys) != 1 {
		t.Errorf("expected only one key as inavlid one should have been discarded")
	}

	if keys[0].alg != "RS256" {
		t.Errorf("expected key with RS256 alg")
	}
}

func TestTopdownJWTVerifyOnlyVerifiesUsingApplicableKeys(t *testing.T) {
	t.Parallel()

	cert := ast.MustInterfaceToValue(`{
    "keys": [
        {
          "kty": "EC",
          "use": "sig",
          "crv": "P-256",
          "kid": "k1",
          "x": "9Qq5S5VqMQoH-FOI4atcH6V3bua03C-5ZMZMG1rszwA",
          "y": "LLbFxWkGBEBrTm1GMYZJy1OXCH1KLweJMCgIEPIsibU",
          "alg": "ES256"
        },
        {
          "kty": "RSA",
          "e": "AQAB",
          "use": "enc",
          "kid": "k2",
          "alg": "RS256",
          "n": "sGu-fYVE2nq2dPxJlqAMI0Z8G3FD0XcWDnD8mkfO1ddKRGuUQZmfj4gWeZGyIk3cnuoy7KJCEqa3daXc08QHuFZyfn0rH33t8_AFsvb0q0i7R2FK-Gdqs_E0-sGpYMsRJdZWfCioLkYjIHEuVnRbi3DEsWqe484rEGbKF60jNRgGC4b-8pz-E538ZkssWxcqHrYIj5bjGEU36onjS3M_yrTuNvzv_8wRioK4fbcwmGne9bDxu8LcoSReWpPn0CnUkWnfqroRcMJnC87ZuJagDW1ZWCmU3psdsVanmFFh0DP6z0fsA4h8G2n9-qp-LEKFaWwo3IWlOsIzU3MHdcEiGw"
        },
	    {
          "kty": "RSA",
          "e": "AQAB",
          "use": "enc",
          "kid": "k3",
          "alg": "RS256",
          "n": "sGu-fYVE2nq2dPxJlqAMI0Z8G3FD0XcWDnD8mkfO1ddKRGuUQZmfj4gWeZGyIk3cnuoy7KJCEqa3daXc08QHuFZyfn0rH33t8_AFsvb0q0i7R2FK-Gdqs_E0-sGpYMsRJdZWfCioLkYjIHEuVnRbi3DEsWqe484rEGbKF60jNRgGC4b-8pz-E538ZkssWxcqHrYIj5bjGEU36onjS3M_yrTuNvzv_8wRioK4fbcwmGne9bDxu8LcoSReWpPn0CnUkWnfqroRcMJnC87ZuJagDW1ZWCmU3psdsVanmFFh0DP6z0fsA4h8G2n9-qp-LEKFaWwo3IWlOsIzU3MHdcEiGw"
        },
	    {
		  "kid": "unknown algorithm",
		  "kty": "RSA",
		  "alg": "RSA-OAEP",
		  "use": "enc",
		  "n": "onlqv4UZx5ZabJ3TCq-IO0s0xaOwo6fWl9o4SzLXPbGtvxonQhoYOeMlS0XkdEdLzB-eqh_hkQ",
		  "e": "AQAB",
		  "x5c": [
			"MIICnTCCAYUCBgGAmcG0xjANBgkqhkiG9w0BAQsFADASMRAwDgYDVQQ2YVaQn47Eew=="
		  ],
		  "x5t": "WKfdwdQkg",
		  "x5t#S256": "2_FidAw.....jlCQl20"
		}
	]
}
`)

	cases := []struct {
		note              string
		header            string
		expectVerifyCalls int
	}{
		{
			note:              "verification considers only key with matching kid, if present",
			header:            `{"alg":"RS256", "kid": "k2"}`,
			expectVerifyCalls: 1,
		},
		{
			note:              "verification considers any key with matching alg, if no kid matches",
			header:            `{"alg":"RS256", "kid": "not-in-jwks"}`,
			expectVerifyCalls: 2,
		},
		{
			note:              "verification without kid considers only keys with alg matched from header",
			header:            `{"alg":"RS256"}`,
			expectVerifyCalls: 2,
		},
		{
			note:              "verification is is skipped if alg unknown",
			header:            `{"alg":"xyz"}`,
			expectVerifyCalls: 0,
		},
	}

	bctx := BuiltinContext{}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			header := base64.RawURLEncoding.EncodeToString([]byte(tc.header))
			payload := base64.RawURLEncoding.EncodeToString([]byte("{}"))
			signature := base64.RawURLEncoding.EncodeToString([]byte("ignored"))

			token := ast.MustInterfaceToValue(fmt.Sprintf("%s.%s.%s", header, payload, signature))

			verifyCalls := 0
			verifier := func(_ any, _ []byte, _ []byte) error {
				verifyCalls++
				return errors.New("fail")
			}

			_, err := builtinJWTVerify(bctx, token, cert, sha256.New, verifier)
			if err != nil {
				t.Fatal(err)
			}

			if verifyCalls != tc.expectVerifyCalls {
				t.Errorf("expected %d calls to verify token, got %d", tc.expectVerifyCalls, verifyCalls)
			}
		})
	}
}

func TestTopdownJWTDecodeVerifyIgnoresKeysOfUnknownAlgInJWKS(t *testing.T) {
	t.Parallel()

	c := ast.NewObject()
	c.Insert(ast.StringTerm("cert"), ast.StringTerm(`{
    "keys": [
        {
          "kty": "EC",
          "use": "sig",
          "crv": "P-256",
          "kid": "k1",
          "x": "9Qq5S5VqMQoH-FOI4atcH6V3bua03C-5ZMZMG1rszwA",
          "y": "LLbFxWkGBEBrTm1GMYZJy1OXCH1KLweJMCgIEPIsibU",
          "alg": "ES256"
        },
        {
          "kty": "RSA",
          "e": "AQAB",
          "use": "enc",
          "kid": "k2",
          "alg": "RS256",
          "n": "sGu-fYVE2nq2dPxJlqAMI0Z8G3FD0XcWDnD8mkfO1ddKRGuUQZmfj4gWeZGyIk3cnuoy7KJCEqa3daXc08QHuFZyfn0rH33t8_AFsvb0q0i7R2FK-Gdqs_E0-sGpYMsRJdZWfCioLkYjIHEuVnRbi3DEsWqe484rEGbKF60jNRgGC4b-8pz-E538ZkssWxcqHrYIj5bjGEU36onjS3M_yrTuNvzv_8wRioK4fbcwmGne9bDxu8LcoSReWpPn0CnUkWnfqroRcMJnC87ZuJagDW1ZWCmU3psdsVanmFFh0DP6z0fsA4h8G2n9-qp-LEKFaWwo3IWlOsIzU3MHdcEiGw"
        },
	    {
          "kty": "RSA",
          "e": "AQAB",
          "use": "enc",
          "kid": "k3",
          "alg": "RS256",
          "n": "sGu-fYVE2nq2dPxJlqAMI0Z8G3FD0XcWDnD8mkfO1ddKRGuUQZmfj4gWeZGyIk3cnuoy7KJCEqa3daXc08QHuFZyfn0rH33t8_AFsvb0q0i7R2FK-Gdqs_E0-sGpYMsRJdZWfCioLkYjIHEuVnRbi3DEsWqe484rEGbKF60jNRgGC4b-8pz-E538ZkssWxcqHrYIj5bjGEU36onjS3M_yrTuNvzv_8wRioK4fbcwmGne9bDxu8LcoSReWpPn0CnUkWnfqroRcMJnC87ZuJagDW1ZWCmU3psdsVanmFFh0DP6z0fsA4h8G2n9-qp-LEKFaWwo3IWlOsIzU3MHdcEiGw"
        },
	    {
		  "kid": "unknown algorithm",
		  "kty": "RSA",
		  "alg": "RSA-OAEP",
		  "use": "enc",
		  "n": "onlqv4UZx5ZabJ3TCq-IO0s0xaOwo6fWl9o4SzLXPbGtvxonQhoYOeMlS0XkdEdLzB-eqh_hkQ",
		  "e": "AQAB",
		  "x5c": [
			"MIICnTCCAYUCBgGAmcG0xjANBgkqhkiG9w0BAQsFADASMRAwDgYDVQQ2YVaQn47Eew=="
		  ],
		  "x5t": "WKfdwdQkg",
		  "x5t#S256": "2_FidAw.....jlCQl20"
		}
	]
}
`))

	wallclock := ast.NumberTerm(int64ToJSONNumber(time.Now().UnixNano()))
	constraints, err := parseTokenConstraints(c, wallclock)
	if err != nil {
		t.Fatal(err)
	}

	if len(constraints.keys) != 3 {
		t.Errorf("expected 3 keys in JWKS, got %d", len(constraints.keys))
	}

	for _, key := range constraints.keys {
		if key.alg == "RSA-OAEP" {
			t.Errorf("expected alg: RSA-OAEP to be removed from key set")
		}
	}
}

func TestBuiltinJWTDecodeVerify_TokenCache(t *testing.T) {
	ctx := t.Context()

	const privateKey = `{
		"kty":"RSA",
		"n":"ofgWCuLjybRlzo0tZWJjNiuSfb4p4fAkd_wWJcyQoTbji9k0l8W26mPddxHmfHQp-Vaw-4qPCJrcS2mJPMEzP1Pt0Bm4d4QlL-yRT-SFd2lZS-pCgNMsD1W_YpRPEwOWvG6b32690r2jZ47soMZo9wGzjb_7OMg0LOL-bSf63kpaSHSXndS5z5rexMdbBYUsLA9e-KXBdQOS-UTo7WTBEMa2R2CapHg665xsmtdVMTBQY4uDZlxvb3qCo5ZwKh9kG4LT6_I5IhlJH7aGhyxXFvUK-DWNmoudF8NAco9_h9iaGNj8q2ethFkMLs91kzk2PAcDTW9gb54h4FRWyuXpoQ",
		"e":"AQAB",
		"d":"Eq5xpGnNCivDflJsRQBXHx1hdR1k6Ulwe2JZD50LpXyWPEAeP88vLNO97IjlA7_GQ5sLKMgvfTeXZx9SE-7YwVol2NXOoAJe46sui395IW_GO-pWJ1O0BkTGoVEn2bKVRUCgu-GjBVaYLU6f3l9kJfFNS3E0QbVdxzubSu3Mkqzjkn439X0M_V51gfpRLI9JYanrC4D4qAdGcopV_0ZHHzQlBjudU2QvXt4ehNYTCBr6XCLQUShb1juUO1ZdiYoFaFQT5Tw8bGUl_x_jTj3ccPDVZFD9pIuhLhBOneufuBiB4cS98l2SR_RQyGWSeWjnczT0QU91p1DhOVRuOopznQ",
		"p":"4BzEEOtIpmVdVEZNCqS7baC4crd0pqnRH_5IB3jw3bcxGn6QLvnEtfdUdiYrqBdss1l58BQ3KhooKeQTa9AB0Hw_Py5PJdTJNPY8cQn7ouZ2KKDcmnPGBY5t7yLc1QlQ5xHdwW1VhvKn-nXqhJTBgIPgtldC-KDV5z-y2XDwGUc",
		"q":"uQPEfgmVtjL0Uyyx88GZFF1fOunH3-7cepKmtH4pxhtCoHqpWmT8YAmZxaewHgHAjLYsp1ZSe7zFYHj7C6ul7TjeLQeZD_YwD66t62wDmpe_HlB-TnBA-njbglfIsRLtXlnDzQkv5dTltRJ11BKBBypeeF6689rjcJIDEz9RWdc",
		"dp":"BwKfV3Akq5_MFZDFZCnW-wzl-CCo83WoZvnLQwCTeDv8uzluRSnm71I3QCLdhrqE2e9YkxvuxdBfpT_PI7Yz-FOKnu1R6HsJeDCjn12Sk3vmAktV2zb34MCdy7cpdTh_YVr7tss2u6vneTwrA86rZtu5Mbr1C1XsmvkxHQAdYo0",
		"dq":"h_96-mK1R_7glhsum81dZxjTnYynPbZpHziZjeeHcXYsXaaMwkOlODsWa7I9xXDoRwbKgB719rrmI2oKr6N3Do9U0ajaHF-NKJnwgjMd2w9cjz3_-kyNlxAr2v4IKhGNpmM5iIgOS1VZnOZ68m6_pbLBSp3nssTdlqvd0tIiTHU",
		"qi":"IYd7DHOhrWvxkwPQsRM2tOgrjbcrfvtQJipd-DlcxyVuuM9sQLdgjVk2oy26F0EmpScGLq2MowX7fhd_QJQ3ydy5cY7YIBi87w93IKLEdfnbJtoOPLUW0ITrJReOgo1cq9SbsxYawBgfp_gh6A5603k2-ZQwVK0JKSHuLFkuQ3U"
	}`

	const publicKey = `{
		"kty":"RSA",
		"n":"ofgWCuLjybRlzo0tZWJjNiuSfb4p4fAkd_wWJcyQoTbji9k0l8W26mPddxHmfHQp-Vaw-4qPCJrcS2mJPMEzP1Pt0Bm4d4QlL-yRT-SFd2lZS-pCgNMsD1W_YpRPEwOWvG6b32690r2jZ47soMZo9wGzjb_7OMg0LOL-bSf63kpaSHSXndS5z5rexMdbBYUsLA9e-KXBdQOS-UTo7WTBEMa2R2CapHg665xsmtdVMTBQY4uDZlxvb3qCo5ZwKh9kG4LT6_I5IhlJH7aGhyxXFvUK-DWNmoudF8NAco9_h9iaGNj8q2ethFkMLs91kzk2PAcDTW9gb54h4FRWyuXpoQ",
		"e":"AQAB"
	}`

	const keys = `{"keys": [` + publicKey + `]}`

	keysTerm := ast.ObjectTerm(ast.Item(ast.StringTerm("cert"), ast.StringTerm(keys)))

	jwt := createJwtT(t, `{"i": "foo"}`, privateKey)
	jwtTerm := ast.NewTerm(ast.String(jwt))

	t.Run("no cache", func(t *testing.T) {
		var verified bool
		iter := func(r *ast.Term) error {
			verified = bool(r.Value.(*ast.Array).Get(ast.NumberTerm("0")).Value.(ast.Boolean))
			return nil
		}

		bctx := BuiltinContext{
			Context: ctx,
			Time:    ast.NumberTerm(int64ToJSONNumber(time.Now().UnixNano())),
		}

		err := builtinJWTDecodeVerify(bctx, []*ast.Term{jwtTerm, keysTerm}, iter)
		if err != nil {
			t.Fatalf("unexpected error: %q", err)
		}

		if !verified {
			t.Fatal("expected token to be successfully verified")
		}
	})

	config := cache.Config{
		InterQueryBuiltinValueCache: cache.InterQueryBuiltinValueCacheConfig{
			NamedCacheConfigs: map[string]*cache.NamedValueCacheConfig{
				tokenCacheName: {
					MaxNumEntries: &[]int{5}[0],
				},
			},
		},
	}

	t.Run("cache", func(t *testing.T) {
		var verified bool
		iter := func(r *ast.Term) error {
			verified = bool(r.Value.(*ast.Array).Get(ast.NumberTerm("0")).Value.(ast.Boolean))
			return nil
		}

		bctx := BuiltinContext{
			Context:                     ctx,
			Time:                        ast.NumberTerm(int64ToJSONNumber(time.Now().UnixNano())),
			InterQueryBuiltinValueCache: cache.NewInterQueryValueCache(ctx, &config),
		}

		t.Run("successful verification", func(t *testing.T) {
			err := builtinJWTDecodeVerify(bctx, []*ast.Term{jwtTerm, keysTerm}, iter)
			if err != nil {
				t.Fatalf("unexpected error: %q", err)
			}

			if !verified {
				t.Fatal("expected token to be successfully verified")
			}

			k := createTokenCacheKey(ast.String(jwt), keysTerm.Value)
			if _, ok := bctx.InterQueryBuiltinValueCache.GetCache(tokenCacheName).Get(k); !ok {
				t.Fatal("expected token to be cached")
			}
		})

		t.Run("failed verification (bad signature)", func(t *testing.T) {
			badJwt := createBadJwt(t, `{"i": "foo"}`)
			badJwtTerm := ast.NewTerm(ast.String(badJwt))

			err := builtinJWTDecodeVerify(bctx, []*ast.Term{badJwtTerm, keysTerm}, iter)
			if err != nil {
				t.Fatalf("unexpected error: %q", err)
			}

			if verified {
				t.Fatal("expected token to fail verification")
			}

			k := createTokenCacheKey(ast.String(badJwt), keysTerm.Value)
			if _, ok := bctx.InterQueryBuiltinValueCache.GetCache(tokenCacheName).Get(k); !ok {
				t.Fatal("expected token to be cached")
			}
		})

		t.Run("iss constraint check", func(t *testing.T) {
			jwt := createJwtT(t, `{"i": "foo", "iss": "foo"}`, privateKey)
			jwtTerm := ast.NewTerm(ast.String(jwt))

			constraints := ast.ObjectTerm(
				ast.Item(ast.StringTerm("cert"), ast.StringTerm(keys)),
				ast.Item(ast.StringTerm("iss"), ast.StringTerm("bar")),
			)

			err := builtinJWTDecodeVerify(bctx, []*ast.Term{jwtTerm, constraints}, iter)
			if err != nil {
				t.Fatalf("unexpected error: %q", err)
			}

			if verified {
				t.Fatal("expected token to fail verification")
			}

			k := createTokenCacheKey(ast.String(jwt), keysTerm.Value)
			if _, ok := bctx.InterQueryBuiltinValueCache.GetCache(tokenCacheName).Get(k); !ok {
				t.Fatal("expected token to be cached")
			}
		})

		t.Run("nbf constraint check", func(t *testing.T) {
			now := time.Second * 0

			// Token's nbf is 1 sec in the future.
			jwt := createJwtT(t, fmt.Sprintf(`{"i": "foo", "nbf": %d}`, 1), privateKey)
			jwtTerm := ast.NewTerm(ast.String(jwt))

			bctx.Time = ast.NumberTerm(int64ToJSONNumber(int64(now)))

			err := builtinJWTDecodeVerify(bctx, []*ast.Term{jwtTerm, keysTerm}, iter)
			if err != nil {
				t.Fatalf("unexpected error: %q", err)
			}

			// Token's nbf is in the future, so it should not be verified.
			if verified {
				t.Fatal("expected token to fail verification")
			}

			k := createTokenCacheKey(ast.String(jwt), keysTerm.Value)
			if _, ok := bctx.InterQueryBuiltinValueCache.GetCache(tokenCacheName).Get(k); !ok {
				t.Fatal("expected token to be cached")
			}

			// Move time to the future, so the token is now valid.
			now = time.Second * 2
			bctx.Time = ast.NumberTerm(int64ToJSONNumber(int64(now)))

			err = builtinJWTDecodeVerify(bctx, []*ast.Term{jwtTerm, keysTerm}, iter)
			if err != nil {
				t.Fatalf("unexpected error: %q", err)
			}

			if !verified {
				t.Fatal("expected token to be successfully verified")
			}
		})

		t.Run("exp constraint check", func(t *testing.T) {
			now := time.Second * 0

			// Token's exp is 1 sec in the future.
			jwt := createJwtT(t, fmt.Sprintf(`{"i": "foo", "exp": %d}`, 1), privateKey)
			jwtTerm := ast.NewTerm(ast.String(jwt))

			bctx.Time = ast.NumberTerm(int64ToJSONNumber(int64(now)))

			err := builtinJWTDecodeVerify(bctx, []*ast.Term{jwtTerm, keysTerm}, iter)
			if err != nil {
				t.Fatalf("unexpected error: %q", err)
			}

			// Token's exp is in the future, so it should be verified.
			if !verified {
				t.Fatal("expected token to be successfully verified")
			}

			k := createTokenCacheKey(ast.String(jwt), keysTerm.Value)
			if _, ok := bctx.InterQueryBuiltinValueCache.GetCache(tokenCacheName).Get(k); !ok {
				t.Fatal("expected token to be cached")
			}

			// Move time to the future, so the token is now expired.
			now = time.Second * 2
			bctx.Time = ast.NumberTerm(int64ToJSONNumber(int64(now)))

			err = builtinJWTDecodeVerify(bctx, []*ast.Term{jwtTerm, keysTerm}, iter)
			if err != nil {
				t.Fatalf("unexpected error: %q", err)
			}

			if verified {
				t.Fatal("expected token to fail verification")
			}
		})
	})
}

func createJwtT(t *testing.T, payload string, privateKey string) string {
	t.Helper()

	jwt, err := createJwt(payload, privateKey)
	if err != nil {
		t.Fatal(err)
	}

	return jwt
}

func createBadJwt(t *testing.T, payload string) string {
	t.Helper()

	return strings.Join(
		[]string{
			base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`)),
			base64.RawURLEncoding.EncodeToString([]byte(payload)),
			base64.RawURLEncoding.EncodeToString([]byte(`bad_signature`)),
		}, ".",
	)
}

func createJwt(payload string, privateKey string) (string, error) {
	jwkKeySet, err := jwk.ParseString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to parse JWK: %s", err.Error())
	}

	jwkKey, ok := jwkKeySet.Key(0)
	if !ok {
		return "", errors.New("failed to get first key from JWK set")
	}

	var pk any
	if err := jwk.Export(jwkKey, &pk); err != nil {
		return "", fmt.Errorf("failed to materialize key: %s", err.Error())
	}

	alg := jwa.RS256()

	// Create protected headers
	protectedHeaders := jws.NewHeaders()
	if err := protectedHeaders.Set(jws.AlgorithmKey, alg); err != nil {
		return "", fmt.Errorf("failed to set algorithm header: %s", err.Error())
	}

	signed, err := jws.Sign([]byte(payload), jws.WithKey(alg, pk, jws.WithProtectedHeaders(protectedHeaders)))
	if err != nil {
		return "", fmt.Errorf("failed to sign message: %s", err.Error())
	}

	return string(signed), nil
}

func TestBuiltinJWTVerify_TokenCache(t *testing.T) {
	tests := []struct {
		note    string
		jwt     string
		key     string
		badKey  string
		builtin func(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error
	}{
		{
			note:    "HS256",
			jwt:     `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjoiYWxpY2UiLCJhenAiOiJhbGljZSIsInN1Ym9yZGluYXRlcyI6W10sImhyIjpmYWxzZX0.rz3jTY033z-NrKfwrK89_dcLF7TN4gwCMj-fVBDyLoM`,
			key:     `secret`,
			badKey:  `bad_secret`,
			builtin: builtinJWTVerifyHS256,
		},
		{
			note:    "HS384",
			jwt:     `eyJhbGciOiJIUzM4NCJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.g98lHYzuqINVppLMoEZT7jlpX0IBSo9zKGoN9DhQg7Ua3YjLXbJMjzESjIHXOGLB`,
			key:     `secret`,
			badKey:  `bad_secret`,
			builtin: builtinJWTVerifyHS384,
		},
		{
			note:    "HS512",
			jwt:     `eyJhbGciOiJIUzUxMiJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.F6-xviRhK2OLcJJHFivhQqMN_dgX5boDrwbVKkdo9flQQNk-AaKpH3uYycFvBEd_erVefcsri_PkL4fjLSZ7ZA`,
			key:     `secret`,
			badKey:  `bad_secret`,
			builtin: builtinJWTVerifyHS512,
		},
		{
			note:    "RS256",
			jwt:     `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJmb28iOiJiYXIiLCJuYmYiOjE0NDQ0Nzg0MDB9.N0-EVdv5pvUfZYFRzMGnsWpNLHgwMEgViPwpuLBEtt32682OgnOK-N4X-2gpQEjQIbUr0IFym8YsRQU9GZvqQP72Sd6yOQNGSNeE74DpUZCAjBa9SBIb1UlD2MxZB-e7YJiEyo7pZhimaqorXrgorlaXYGMvsCFWDYmBLzGaGYaGJyEpkZHzHb7ujsDrJJjdEtDV3kh13gTHzLPvqnoXuuxelXye_8LPIhvgDy52gT4shUEso71pJCMv_IqAR19ljVE17lJzoi6VhRn6ReNUE-yg4KfCO4Ypnuu-mcQr7XtmSYoWkX72L5UQ-EyWkoz-w0SYKoJTPzHkTL2thYStksVpeNkGuck25aUdtrQgmPbao0QOWBFlkg03e6mPCD2-aXOt1ofth9mZGjxWMHX-mUqHaNmaWM3WhRztJ73hWrmB1YOdYQtOEHejfvR_td5tqIw4W6ufRy2ScOypGQe7kNaUZxpgxZ1927ZGNiQgawIOAQwXOcFx1JNSEIeg55-cYJrHPxsXGOB9ZxW-qnswmFJp474iUVXjzGhLexJDXBwvKGs_O3JFjMsvyV9_hm7bnQU0vG_HgPYs5i9VOHRMujq1vFBcm52TFVOBGdWaGfb9RRdLLYvVkJLk0Poh19rsCWb7-Vc3mAaGGpvuk4Wv-PnGGNC-V-FQqIbijHDrn_g`,
			key:     "-----BEGIN CERTIFICATE-----\nMIIFiDCCA3ACCQCGV6XsfG/oRTANBgkqhkiG9w0BAQUFADCBhTELMAkGA1UEBhMC\nVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEO\nMAwGA1UECgwFU3R5cmExDDAKBgNVBAsMA0RldjESMBAGA1UEAwwJbG9jYWxob3N0\nMRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5cmEwHhcNMTgwMzA2MDAxNTU5WhcNMTkw\nMzA2MDAxNTU5WjCBhTELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWEx\nFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEOMAwGA1UECgwFU3R5cmExDDAKBgNVBAsM\nA0RldjESMBAGA1UEAwwJbG9jYWxob3N0MRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5\ncmEwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDucnAwTRA0zqDQ671L\nKWOVwhjhycFyzyhZUd7vhsnslOBiYM6TYIDXhETfAk2RQoRE/9xF16woMD8FOglc\nlSuhi+GNfFRif6LfArm84ZFj1ZS1MX2logikhXhRJQ7AOHe5+ED0re3KH5lWyqfz\nR6bQuPYwTQSBJy6Tq7T9RiOM29yadCX64OaCEbzEFmHtNlbb5px4zCVvgskg/fpV\nGGCMpAYjGDatbxE5eAloVs1EJuI5RSqWr1JRm6EejxM04BFdfGn1HgWrsKXtlvBa\n00/AC0zXL5n6LK7+L3WbRguVTZcE4Yu70gDwhmM+VsKeT9LKClX003BNj0NJDRB9\ndw9MaWxsXDNHNOWEfbnASXeP7ZRv3D81ftij6P8SL14ZnxyrRty8TAN4ij3wd41l\nastRQCtrJFi+HzO606XOp6HDzBoWT0DGl8Sn2hZ6RLPyBnD04vvvcSGeCVjHGOQ8\nc3OTroK58u5MR/q4T00sTkeeVAxuKoEWKsjIBYYrJTe/a2mEq9yiDGbPNYDnWnQZ\njSUZm+Us23Y2sm/agZ5zKXcEuoecGL6sYCixr/xeB9BPxEiTthH+0M8OY99qpIhz\nSmj41wdgQfzZi/6B8pIr77V/KywYKxJEmzw8Uy48aC/rZ8WsT8QdKwclo1aiNJhx\n79OvGbZFoeHD/w7igpx+ttpF/wIDAQABMA0GCSqGSIb3DQEBBQUAA4ICAQC3wWUs\nfXz+aSfFVz+O3mLFkr65NIgazbGAySgMgMNVuadheIkPL4k21atyflfpx4pg9FGv\n40vWCLMajpvynfz4oqah0BACnpqzQ8Dx6HYkmlXK8fLB+WtPrZBeUEsGPKuJYt4M\nd5TeY3VpNgWOPXmnE4lvxHZqh/8OwmOpjBfC9E3e2eqgwiwOkXnMaZEPgKP6JiWk\nEFaQ9jgMQqJZnNcv6NmiqqsZeI0/NNjBpkmEWQl+wLegVusHiQ0FMBMQ0taEo21r\nzUwHoNJR3h3wgGQiKxKOH1FUKHBV7hEqObLraD/hfG5xYucJfvvAAP1iH0ycPs+9\nhSccrn5/HY1c9AZnW8Kh7atp/wFP+sHjtECWK/lUmXfhASS293hprCpJk2n9pkmR\nziXKJhjwkxlC8NcHuiVfaxdfDa4+1Qta2gK7GEypbvLoEmIt/dsYUsxUg84lwJJ9\nnyC/pfZ5a8wFSf186JeVH4kHd3bnkzlQz460HndOMSJ/Xi1wSfuZlOVupFf8TVKl\np4j28MTLH2Wqx50NssKThdaX6hoCiMqreYa+EVaN1f/cIGQxZSCzdzMCKqdB8lKB\n3Eax+5zsIa/UyPwGxZcyXBRHAlz5ZnkjuRxInyiMkBWWz3IZXjTe6Fq8BNd2UWNc\nw35+2nO5n1LKXgR2+nzhZUOk8TPsi9WUywRluQ==\n-----END CERTIFICATE-----",
			badKey:  "-----BEGIN CERTIFICATE-----\nMIIDXDCCAkSgAwIBAgIBLjANBgkqhkiG9w0BAQsFADBWMQswCQYDVQQGEwJVUzEV\nMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMD\nRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3QwHhcNMjAwNTA3MDg1MTAzWhcNMjAwNTA3\nMTA1MTAzWjBWMQswCQYDVQQGEwJVUzEVMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4w\nDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3Qw\nggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDeRmygX/fOOUu5Wm91PFNo\nsHDG1CzG9a1iKBjUeMgi9bXXScUfatPmsNlxb56uSi0RXUsvJmY/yxkIIhRyapxW\n49j2idAM3SGGL1nOZf/XdpDHYsAFFZ237HGb8DOEk/p3xCFv0tH/iQ+kLP36EM1+\ntn6BfUXdJnVyvkSK2iMNeRY7A4DMX7sGX39LXsVJiCokIC8E0QUFrSjvrAm9ejKE\ntPojydo4c3VUxLfmFuyMXoD3bfk1Jv5i2J5RjtomjgK6zNCvgYzpspiodHChkzlU\nX8yk2YqlAHX3XdJA94LaDE2kNXiOQnFkUb8GsP7hmEbwGtMUEQie+jfgKplxJ49B\nAgMBAAGjNTAzMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAM\nBgNVHRMBAf8EAjAAMA0GCSqGSIb3DQEBCwUAA4IBAQC9f2/kxT7DnQ94ownhHvd6\nrzk1WirI90rFM2MxhfkaDrOHhSGZL9nDf6TIZ4qeFKZXthpKxpiZm2Oxmn+vUsik\nW6bYjq1nX0GCchQLaaFf9Jh1IOLwkfoBdX55tV8xUGHRWgDlCuGbqiixz+Bm0Kap\nkmbyJynVcoiKhdLyYm/YTn/pC32SJW666reQ+0qCAoxzLQowBetHjwDam9RsDEf4\n+JRDjYPutNXyJ5X8BaBA6PzHanzMG/7RFYcx/2YhXwVxdfPHku4ALJcddIGAGNx2\n5yte+HY0aEu+06J67eD9+4fU7NixRMKigk9KbjqpeWD+0be+VgX8Dot4jaISgI/3\n-----END CERTIFICATE-----",
			builtin: builtinJWTVerifyRS256,
		},
		{
			note:    "RS384",
			jwt:     `eyJhbGciOiJSUzM4NCJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.b__y2zjqMoD7iWbHeQ0lNpnche3ph5-AwrIQICLMQQGtEz9WMBteHydkC5g01bm3TBX1d04Z5IEOsuK6btAtWma04c5NYqaUyNEUJKYCFoY02uH0jGdGfL6R5Kkv0lkNvN0s3Nex9jMaVVgqx8bcrOU0uRBFT67sXcm11LHaB9BwKFslolzHClxgXy5RIZb4OFk_7Yk7xTC6PcvEWkkGR9uXBhfDEig5WqdwOWPeulimvARDw14U35rzeh9xpGAPjBKeE-y20fXAk0cSF1H69C-Qa1jDQheYIrAJ6XMYGNZWuay5-smmeefe67eweEt1q-AD1NFepqkmZX382DGuYQ`,
			key:     "-----BEGIN CERTIFICATE-----\nMIIDXDCCAkSgAwIBAgIBLjANBgkqhkiG9w0BAQsFADBWMQswCQYDVQQGEwJVUzEV\nMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMD\nRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3QwHhcNMjAwNTA3MDg1MTAzWhcNMjAwNTA3\nMTA1MTAzWjBWMQswCQYDVQQGEwJVUzEVMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4w\nDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3Qw\nggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDeRmygX/fOOUu5Wm91PFNo\nsHDG1CzG9a1iKBjUeMgi9bXXScUfatPmsNlxb56uSi0RXUsvJmY/yxkIIhRyapxW\n49j2idAM3SGGL1nOZf/XdpDHYsAFFZ237HGb8DOEk/p3xCFv0tH/iQ+kLP36EM1+\ntn6BfUXdJnVyvkSK2iMNeRY7A4DMX7sGX39LXsVJiCokIC8E0QUFrSjvrAm9ejKE\ntPojydo4c3VUxLfmFuyMXoD3bfk1Jv5i2J5RjtomjgK6zNCvgYzpspiodHChkzlU\nX8yk2YqlAHX3XdJA94LaDE2kNXiOQnFkUb8GsP7hmEbwGtMUEQie+jfgKplxJ49B\nAgMBAAGjNTAzMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAM\nBgNVHRMBAf8EAjAAMA0GCSqGSIb3DQEBCwUAA4IBAQC9f2/kxT7DnQ94ownhHvd6\nrzk1WirI90rFM2MxhfkaDrOHhSGZL9nDf6TIZ4qeFKZXthpKxpiZm2Oxmn+vUsik\nW6bYjq1nX0GCchQLaaFf9Jh1IOLwkfoBdX55tV8xUGHRWgDlCuGbqiixz+Bm0Kap\nkmbyJynVcoiKhdLyYm/YTn/pC32SJW666reQ+0qCAoxzLQowBetHjwDam9RsDEf4\n+JRDjYPutNXyJ5X8BaBA6PzHanzMG/7RFYcx/2YhXwVxdfPHku4ALJcddIGAGNx2\n5yte+HY0aEu+06J67eD9+4fU7NixRMKigk9KbjqpeWD+0be+VgX8Dot4jaISgI/3\n-----END CERTIFICATE-----",
			badKey:  "-----BEGIN CERTIFICATE-----\nMIIFiDCCA3ACCQCGV6XsfG/oRTANBgkqhkiG9w0BAQUFADCBhTELMAkGA1UEBhMC\nVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEO\nMAwGA1UECgwFU3R5cmExDDAKBgNVBAsMA0RldjESMBAGA1UEAwwJbG9jYWxob3N0\nMRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5cmEwHhcNMTgwMzA2MDAxNTU5WhcNMTkw\nMzA2MDAxNTU5WjCBhTELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWEx\nFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEOMAwGA1UECgwFU3R5cmExDDAKBgNVBAsM\nA0RldjESMBAGA1UEAwwJbG9jYWxob3N0MRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5\ncmEwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDucnAwTRA0zqDQ671L\nKWOVwhjhycFyzyhZUd7vhsnslOBiYM6TYIDXhETfAk2RQoRE/9xF16woMD8FOglc\nlSuhi+GNfFRif6LfArm84ZFj1ZS1MX2logikhXhRJQ7AOHe5+ED0re3KH5lWyqfz\nR6bQuPYwTQSBJy6Tq7T9RiOM29yadCX64OaCEbzEFmHtNlbb5px4zCVvgskg/fpV\nGGCMpAYjGDatbxE5eAloVs1EJuI5RSqWr1JRm6EejxM04BFdfGn1HgWrsKXtlvBa\n00/AC0zXL5n6LK7+L3WbRguVTZcE4Yu70gDwhmM+VsKeT9LKClX003BNj0NJDRB9\ndw9MaWxsXDNHNOWEfbnASXeP7ZRv3D81ftij6P8SL14ZnxyrRty8TAN4ij3wd41l\nastRQCtrJFi+HzO606XOp6HDzBoWT0DGl8Sn2hZ6RLPyBnD04vvvcSGeCVjHGOQ8\nc3OTroK58u5MR/q4T00sTkeeVAxuKoEWKsjIBYYrJTe/a2mEq9yiDGbPNYDnWnQZ\njSUZm+Us23Y2sm/agZ5zKXcEuoecGL6sYCixr/xeB9BPxEiTthH+0M8OY99qpIhz\nSmj41wdgQfzZi/6B8pIr77V/KywYKxJEmzw8Uy48aC/rZ8WsT8QdKwclo1aiNJhx\n79OvGbZFoeHD/w7igpx+ttpF/wIDAQABMA0GCSqGSIb3DQEBBQUAA4ICAQC3wWUs\nfXz+aSfFVz+O3mLFkr65NIgazbGAySgMgMNVuadheIkPL4k21atyflfpx4pg9FGv\n40vWCLMajpvynfz4oqah0BACnpqzQ8Dx6HYkmlXK8fLB+WtPrZBeUEsGPKuJYt4M\nd5TeY3VpNgWOPXmnE4lvxHZqh/8OwmOpjBfC9E3e2eqgwiwOkXnMaZEPgKP6JiWk\nEFaQ9jgMQqJZnNcv6NmiqqsZeI0/NNjBpkmEWQl+wLegVusHiQ0FMBMQ0taEo21r\nzUwHoNJR3h3wgGQiKxKOH1FUKHBV7hEqObLraD/hfG5xYucJfvvAAP1iH0ycPs+9\nhSccrn5/HY1c9AZnW8Kh7atp/wFP+sHjtECWK/lUmXfhASS293hprCpJk2n9pkmR\nziXKJhjwkxlC8NcHuiVfaxdfDa4+1Qta2gK7GEypbvLoEmIt/dsYUsxUg84lwJJ9\nnyC/pfZ5a8wFSf186JeVH4kHd3bnkzlQz460HndOMSJ/Xi1wSfuZlOVupFf8TVKl\np4j28MTLH2Wqx50NssKThdaX6hoCiMqreYa+EVaN1f/cIGQxZSCzdzMCKqdB8lKB\n3Eax+5zsIa/UyPwGxZcyXBRHAlz5ZnkjuRxInyiMkBWWz3IZXjTe6Fq8BNd2UWNc\nw35+2nO5n1LKXgR2+nzhZUOk8TPsi9WUywRluQ==\n-----END CERTIFICATE-----",
			builtin: builtinJWTVerifyRS384,
		},
		{
			note:    "RS512",
			jwt:     `eyJhbGciOiJSUzUxMiJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.VSe3qK5Gp0Q0_5nRgMFu25yw74FIgX-kXPOemSi62l-AxeVdUw8rOpEFrSTCaVjd3mPfKb-B056a-gtrbpXK9sUQnFdqdsyt8gHK-umz5lVyWfoAgj51Ontv-9K_pRORD9wqKqdTLZjCxJ5tyKoO0gY3SwwqSqGrp85vUjvEcK3jbMKINGRUNnOokeSm7byUEJsfKVUbPboSX1TGyvjDOZxxSITj8-bzZZ3F21DJ23N2IiJN7FW8Xj-SYyphXo-ML50o5bjW9YlQ5BDk-RW1I4eE-KpsxhApPv_xIgE8d89PVtXFuoJtv0yLRaZ1q04Fl9KNoMyZrmr349yppn0JlQ`,
			key:     "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3kZsoF/3zjlLuVpvdTxT\naLBwxtQsxvWtYigY1HjIIvW110nFH2rT5rDZcW+erkotEV1LLyZmP8sZCCIUcmqc\nVuPY9onQDN0hhi9ZzmX/13aQx2LABRWdt+xxm/AzhJP6d8Qhb9LR/4kPpCz9+hDN\nfrZ+gX1F3SZ1cr5EitojDXkWOwOAzF+7Bl9/S17FSYgqJCAvBNEFBa0o76wJvXoy\nhLT6I8naOHN1VMS35hbsjF6A9235NSb+YtieUY7aJo4CuszQr4GM6bKYqHRwoZM5\nVF/MpNmKpQB1913SQPeC2gxNpDV4jkJxZFG/BrD+4ZhG8BrTFBEInvo34CqZcSeP\nQQIDAQAB\n-----END PUBLIC KEY-----",
			badKey:  "-----BEGIN CERTIFICATE-----\nMIIFiDCCA3ACCQCGV6XsfG/oRTANBgkqhkiG9w0BAQUFADCBhTELMAkGA1UEBhMC\nVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEO\nMAwGA1UECgwFU3R5cmExDDAKBgNVBAsMA0RldjESMBAGA1UEAwwJbG9jYWxob3N0\nMRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5cmEwHhcNMTgwMzA2MDAxNTU5WhcNMTkw\nMzA2MDAxNTU5WjCBhTELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWEx\nFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEOMAwGA1UECgwFU3R5cmExDDAKBgNVBAsM\nA0RldjESMBAGA1UEAwwJbG9jYWxob3N0MRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5\ncmEwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDucnAwTRA0zqDQ671L\nKWOVwhjhycFyzyhZUd7vhsnslOBiYM6TYIDXhETfAk2RQoRE/9xF16woMD8FOglc\nlSuhi+GNfFRif6LfArm84ZFj1ZS1MX2logikhXhRJQ7AOHe5+ED0re3KH5lWyqfz\nR6bQuPYwTQSBJy6Tq7T9RiOM29yadCX64OaCEbzEFmHtNlbb5px4zCVvgskg/fpV\nGGCMpAYjGDatbxE5eAloVs1EJuI5RSqWr1JRm6EejxM04BFdfGn1HgWrsKXtlvBa\n00/AC0zXL5n6LK7+L3WbRguVTZcE4Yu70gDwhmM+VsKeT9LKClX003BNj0NJDRB9\ndw9MaWxsXDNHNOWEfbnASXeP7ZRv3D81ftij6P8SL14ZnxyrRty8TAN4ij3wd41l\nastRQCtrJFi+HzO606XOp6HDzBoWT0DGl8Sn2hZ6RLPyBnD04vvvcSGeCVjHGOQ8\nc3OTroK58u5MR/q4T00sTkeeVAxuKoEWKsjIBYYrJTe/a2mEq9yiDGbPNYDnWnQZ\njSUZm+Us23Y2sm/agZ5zKXcEuoecGL6sYCixr/xeB9BPxEiTthH+0M8OY99qpIhz\nSmj41wdgQfzZi/6B8pIr77V/KywYKxJEmzw8Uy48aC/rZ8WsT8QdKwclo1aiNJhx\n79OvGbZFoeHD/w7igpx+ttpF/wIDAQABMA0GCSqGSIb3DQEBBQUAA4ICAQC3wWUs\nfXz+aSfFVz+O3mLFkr65NIgazbGAySgMgMNVuadheIkPL4k21atyflfpx4pg9FGv\n40vWCLMajpvynfz4oqah0BACnpqzQ8Dx6HYkmlXK8fLB+WtPrZBeUEsGPKuJYt4M\nd5TeY3VpNgWOPXmnE4lvxHZqh/8OwmOpjBfC9E3e2eqgwiwOkXnMaZEPgKP6JiWk\nEFaQ9jgMQqJZnNcv6NmiqqsZeI0/NNjBpkmEWQl+wLegVusHiQ0FMBMQ0taEo21r\nzUwHoNJR3h3wgGQiKxKOH1FUKHBV7hEqObLraD/hfG5xYucJfvvAAP1iH0ycPs+9\nhSccrn5/HY1c9AZnW8Kh7atp/wFP+sHjtECWK/lUmXfhASS293hprCpJk2n9pkmR\nziXKJhjwkxlC8NcHuiVfaxdfDa4+1Qta2gK7GEypbvLoEmIt/dsYUsxUg84lwJJ9\nnyC/pfZ5a8wFSf186JeVH4kHd3bnkzlQz460HndOMSJ/Xi1wSfuZlOVupFf8TVKl\np4j28MTLH2Wqx50NssKThdaX6hoCiMqreYa+EVaN1f/cIGQxZSCzdzMCKqdB8lKB\n3Eax+5zsIa/UyPwGxZcyXBRHAlz5ZnkjuRxInyiMkBWWz3IZXjTe6Fq8BNd2UWNc\nw35+2nO5n1LKXgR2+nzhZUOk8TPsi9WUywRluQ==\n-----END CERTIFICATE-----",
			builtin: builtinJWTVerifyRS512,
		},
		{
			note:    "PS256",
			jwt:     `eyJ0eXAiOiAiSldUIiwgImFsZyI6ICJQUzI1NiJ9.eyJuYmYiOiAxNDQ0NDc4NDAwLCAiZm9vIjogImJhciJ9.i0F3MHWzOsBNLqjQzK1UVeQid9xPMowCoUsoM-C2BDxUY-FMKmCeJ1NJ4TGnS9HzFK1ftEvRnPT7EOxOkHPoCk1rz3feTFgtHtNzQqLM1IBTnz6aHHOrda_bKPHH9ZIYCRQUPXhpC90ivW_IJR-f7Z1WLrMXaJ71i1XteruENHrJJJDn0HedHG6N0VHugBHrak5k57cbE31utAdx83TEd8v2Y8wAkCJXKrdmTa-8419LNxW_yjkvoDD53n3X5CHhYkSymU77p0v6yWO38qDWeKJ-Fm_PrMAo72_rizDBj_yPa5LA3bT_EnsgZtC-sp8_SCDIH41bjiCGpRHhqgZmyw`,
			key:     `{"kty":"RSA","e":"AQAB","kid":"bf688c97-bf51-49ba-b9d3-115195bb0eb8","n":"uJApsyzFv-Y85M5JjezHvMDw_spgVCI7BqpYhnzK3xXw1dnkz1bWXGA9yF6AeADlE-1yc1ozrAURTnFSihIgj414i3MC2_0FkNcdAbnX7d9q9_jdCkHda4HER0zzXCaHlgnzoAz6edUU800-h0LleLnfgg4UST-0DFTCIGpfTbs7OPSy2WgT1vP6xbB45CUOJA7o0q6XE-hdhWWN0plrDiYD-0Y1SpOQYXmHhSmr-WVeKeoh5_0zeEVab6TQYec_16ByEyepaZB0g6WyGkFE6aG1NrpvDd24s_h7BAJg_S2mtu1lKWEqYjOgwzEl5XQQyXbpnq1USb12ArX16rZdew"}`,
			badKey:  "-----BEGIN CERTIFICATE-----\nMIIFiDCCA3ACCQCGV6XsfG/oRTANBgkqhkiG9w0BAQUFADCBhTELMAkGA1UEBhMC\nVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEO\nMAwGA1UECgwFU3R5cmExDDAKBgNVBAsMA0RldjESMBAGA1UEAwwJbG9jYWxob3N0\nMRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5cmEwHhcNMTgwMzA2MDAxNTU5WhcNMTkw\nMzA2MDAxNTU5WjCBhTELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWEx\nFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEOMAwGA1UECgwFU3R5cmExDDAKBgNVBAsM\nA0RldjESMBAGA1UEAwwJbG9jYWxob3N0MRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5\ncmEwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDucnAwTRA0zqDQ671L\nKWOVwhjhycFyzyhZUd7vhsnslOBiYM6TYIDXhETfAk2RQoRE/9xF16woMD8FOglc\nlSuhi+GNfFRif6LfArm84ZFj1ZS1MX2logikhXhRJQ7AOHe5+ED0re3KH5lWyqfz\nR6bQuPYwTQSBJy6Tq7T9RiOM29yadCX64OaCEbzEFmHtNlbb5px4zCVvgskg/fpV\nGGCMpAYjGDatbxE5eAloVs1EJuI5RSqWr1JRm6EejxM04BFdfGn1HgWrsKXtlvBa\n00/AC0zXL5n6LK7+L3WbRguVTZcE4Yu70gDwhmM+VsKeT9LKClX003BNj0NJDRB9\ndw9MaWxsXDNHNOWEfbnASXeP7ZRv3D81ftij6P8SL14ZnxyrRty8TAN4ij3wd41l\nastRQCtrJFi+HzO606XOp6HDzBoWT0DGl8Sn2hZ6RLPyBnD04vvvcSGeCVjHGOQ8\nc3OTroK58u5MR/q4T00sTkeeVAxuKoEWKsjIBYYrJTe/a2mEq9yiDGbPNYDnWnQZ\njSUZm+Us23Y2sm/agZ5zKXcEuoecGL6sYCixr/xeB9BPxEiTthH+0M8OY99qpIhz\nSmj41wdgQfzZi/6B8pIr77V/KywYKxJEmzw8Uy48aC/rZ8WsT8QdKwclo1aiNJhx\n79OvGbZFoeHD/w7igpx+ttpF/wIDAQABMA0GCSqGSIb3DQEBBQUAA4ICAQC3wWUs\nfXz+aSfFVz+O3mLFkr65NIgazbGAySgMgMNVuadheIkPL4k21atyflfpx4pg9FGv\n40vWCLMajpvynfz4oqah0BACnpqzQ8Dx6HYkmlXK8fLB+WtPrZBeUEsGPKuJYt4M\nd5TeY3VpNgWOPXmnE4lvxHZqh/8OwmOpjBfC9E3e2eqgwiwOkXnMaZEPgKP6JiWk\nEFaQ9jgMQqJZnNcv6NmiqqsZeI0/NNjBpkmEWQl+wLegVusHiQ0FMBMQ0taEo21r\nzUwHoNJR3h3wgGQiKxKOH1FUKHBV7hEqObLraD/hfG5xYucJfvvAAP1iH0ycPs+9\nhSccrn5/HY1c9AZnW8Kh7atp/wFP+sHjtECWK/lUmXfhASS293hprCpJk2n9pkmR\nziXKJhjwkxlC8NcHuiVfaxdfDa4+1Qta2gK7GEypbvLoEmIt/dsYUsxUg84lwJJ9\nnyC/pfZ5a8wFSf186JeVH4kHd3bnkzlQz460HndOMSJ/Xi1wSfuZlOVupFf8TVKl\np4j28MTLH2Wqx50NssKThdaX6hoCiMqreYa+EVaN1f/cIGQxZSCzdzMCKqdB8lKB\n3Eax+5zsIa/UyPwGxZcyXBRHAlz5ZnkjuRxInyiMkBWWz3IZXjTe6Fq8BNd2UWNc\nw35+2nO5n1LKXgR2+nzhZUOk8TPsi9WUywRluQ==\n-----END CERTIFICATE-----",
			builtin: builtinJWTVerifyPS256,
		},
		{
			note:    "PS384",
			jwt:     `eyJhbGciOiJQUzM4NCJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.EHPUvPr6uJOYqdza95WbM1SYD8atZHJEVRggpwOWnHGsjQBoEarJb8QgW7TY22OXwGw2HWluTiyT_MAz02NaHRzZv6AgrmxCLChMWkCHLwPxqjs0xSvVAMLzHHq2X2Bcujo9KORGudR7zKz8pOX5Mfnm7Z6OGtqPCPLaIdVJlddNsG6a571NOuVuDWbcg0omeRDANZpCZMJeAQN2M-4Q61ef6zcQHK1R-QqzBhw6HzMgqR1LRJ0xbrmD-L5o53JM3pV1e1juKNXVK3vWkDQRCQORFn1lyH5isfSsiiHW-x90sUC7TrU_cOji4MMmOCME6kkwxe57ZgpeXtdVTvldpw`,
			key:     "-----BEGIN CERTIFICATE-----\nMIIDXDCCAkSgAwIBAgIBKjANBgkqhkiG9w0BAQsFADBWMQswCQYDVQQGEwJVUzEV\nMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMD\nRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3QwHhcNMjAwNTA3MDkxMjU2WhcNMjAwNTA3\nMTExMjU2WjBWMQswCQYDVQQGEwJVUzEVMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4w\nDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3Qw\nggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDtyVWH2FE8cU8LRcArH4Tw\nDhBOFcmJF28LrvRObcbDYsae6rEwby0aRgSxjTEMgyGBjroBSl22wOSA93kwx4pu\npfXEqbwywn9FhyKBb/OXQSglPmwrpmzQtPJGzBHncL+PPjRhPfqimwf7ZIPKAAgI\nz9O6ppGhE/x4Ct444jthUIBZuG5cUXhgiPBQdIQ3K88QhgVwcufTZkNHj4iSDfhl\nVFDHVjXjd2B/yGODjyv0TyChV0YBNGjMv7YFLWmIFUFzK+6qNSxu4czPtRkyfwaV\n2GW/PBT5f8fc6fKgQZ6k6BLK6+pi0iPh5TUizyHtqtueWDbrJ+wfdJilQS8D+EGr\nAgMBAAGjNTAzMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAM\nBgNVHRMBAf8EAjAAMA0GCSqGSIb3DQEBCwUAA4IBAQBgVAM50/0aBTBxESYKIKN4\nE+qbV6aE0C+wYJLet0EWPTxwmFZamq5LNEO/D6xyoY5WHY60EyHRMs0agSB/ATBX\n5ULdEwh9G0NjxqivCcoddQ1fuVS2PrrqNL7VlRnYbTpd8/Dh4qnyl5FltlyZ/29L\ny7BWOwlcBlZdhsfH8svNX4PUxjRD+jmnczCDi7XSOKT8htKUV2ih1c9JrWpIhCi/\nHzEXkaAxNdhBNdIsLQMo3qq9fkSgNZQk9/ecJNPeuJ/UYyr5Xa4PxIWl4U+P7yuI\n+Q3FSPmUbiVsSGqMhh6V/DN8M+T5/KiSB47gFOfxc2/RR5aw4HkSp3WxwbT9njbE\n-----END CERTIFICATE-----",
			badKey:  `{"kty":"RSA","e":"AQAB","kid":"bf688c97-bf51-49ba-b9d3-115195bb0eb8","n":"uJApsyzFv-Y85M5JjezHvMDw_spgVCI7BqpYhnzK3xXw1dnkz1bWXGA9yF6AeADlE-1yc1ozrAURTnFSihIgj414i3MC2_0FkNcdAbnX7d9q9_jdCkHda4HER0zzXCaHlgnzoAz6edUU800-h0LleLnfgg4UST-0DFTCIGpfTbs7OPSy2WgT1vP6xbB45CUOJA7o0q6XE-hdhWWN0plrDiYD-0Y1SpOQYXmHhSmr-WVeKeoh5_0zeEVab6TQYec_16ByEyepaZB0g6WyGkFE6aG1NrpvDd24s_h7BAJg_S2mtu1lKWEqYjOgwzEl5XQQyXbpnq1USb12ArX16rZdew"}`,
			builtin: builtinJWTVerifyPS384,
		},
		{
			note:    "PS512",
			jwt:     `eyJhbGciOiJQUzUxMiJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.VRlkPtiUq5MmBNgyuBqxv2_aX40STrWrBB2sSmGbxI78jVG_3hVoh7Mk-wUmFL389qpf05xNdn-gpMe-MSDUux7U7EuFspFZdYTUBo9wRvEBe4e1rHUCG00lVdYCG7eEgbAxM3cUhrHRwExBte30qBrFFUY9FgG-kJdYhgyh7VquMGuKgiS8CP_H0Gp1mIvTw6eEnSFAoKiryw9edUZ78pHELNn4y18YZvEndeNZh7f19LCtrB0G2bJUHGM4vPcwo2D-UAhEFBpSlnnqXDLSWOhUgLNLu0kZACXhT808KT6fdF6eFihdThmWN7_HUz2znjrjs71CqqDJgLhkGs8UvQ`,
			key:     "-----BEGIN CERTIFICATE-----\nMIIDXDCCAkSgAwIBAgIBKjANBgkqhkiG9w0BAQsFADBWMQswCQYDVQQGEwJVUzEV\nMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMD\nRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3QwHhcNMjAwNTA3MDkxMjU2WhcNMjAwNTA3\nMTExMjU2WjBWMQswCQYDVQQGEwJVUzEVMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4w\nDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3Qw\nggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDtyVWH2FE8cU8LRcArH4Tw\nDhBOFcmJF28LrvRObcbDYsae6rEwby0aRgSxjTEMgyGBjroBSl22wOSA93kwx4pu\npfXEqbwywn9FhyKBb/OXQSglPmwrpmzQtPJGzBHncL+PPjRhPfqimwf7ZIPKAAgI\nz9O6ppGhE/x4Ct444jthUIBZuG5cUXhgiPBQdIQ3K88QhgVwcufTZkNHj4iSDfhl\nVFDHVjXjd2B/yGODjyv0TyChV0YBNGjMv7YFLWmIFUFzK+6qNSxu4czPtRkyfwaV\n2GW/PBT5f8fc6fKgQZ6k6BLK6+pi0iPh5TUizyHtqtueWDbrJ+wfdJilQS8D+EGr\nAgMBAAGjNTAzMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAM\nBgNVHRMBAf8EAjAAMA0GCSqGSIb3DQEBCwUAA4IBAQBgVAM50/0aBTBxESYKIKN4\nE+qbV6aE0C+wYJLet0EWPTxwmFZamq5LNEO/D6xyoY5WHY60EyHRMs0agSB/ATBX\n5ULdEwh9G0NjxqivCcoddQ1fuVS2PrrqNL7VlRnYbTpd8/Dh4qnyl5FltlyZ/29L\ny7BWOwlcBlZdhsfH8svNX4PUxjRD+jmnczCDi7XSOKT8htKUV2ih1c9JrWpIhCi/\nHzEXkaAxNdhBNdIsLQMo3qq9fkSgNZQk9/ecJNPeuJ/UYyr5Xa4PxIWl4U+P7yuI\n+Q3FSPmUbiVsSGqMhh6V/DN8M+T5/KiSB47gFOfxc2/RR5aw4HkSp3WxwbT9njbE\n-----END CERTIFICATE-----",
			badKey:  `{"kty":"RSA","e":"AQAB","kid":"bf688c97-bf51-49ba-b9d3-115195bb0eb8","n":"uJApsyzFv-Y85M5JjezHvMDw_spgVCI7BqpYhnzK3xXw1dnkz1bWXGA9yF6AeADlE-1yc1ozrAURTnFSihIgj414i3MC2_0FkNcdAbnX7d9q9_jdCkHda4HER0zzXCaHlgnzoAz6edUU800-h0LleLnfgg4UST-0DFTCIGpfTbs7OPSy2WgT1vP6xbB45CUOJA7o0q6XE-hdhWWN0plrDiYD-0Y1SpOQYXmHhSmr-WVeKeoh5_0zeEVab6TQYec_16ByEyepaZB0g6WyGkFE6aG1NrpvDd24s_h7BAJg_S2mtu1lKWEqYjOgwzEl5XQQyXbpnq1USb12ArX16rZdew"}`,
			builtin: builtinJWTVerifyPS512,
		},
		{
			note:    "ES256",
			jwt:     `eyJ0eXAiOiAiSldUIiwgImFsZyI6ICJFUzI1NiJ9.eyJuYmYiOiAxNDQ0NDc4NDAwLCAiaXNzIjogInh4eCJ9.lArczfN-pIL8oUU-7PU83u-zfXougXBZj6drFeKFsPEoVhy9WAyiZlRshYqjTSXdaw8yw2L-ovt4zTUZb2PWMg`,
			key:     `{"kty":"EC","crv":"P-256","x":"z8J91ghFy5o6f2xZ4g8LsLH7u2wEpT2ntj8loahnlsE","y":"7bdeXLH61KrGWRdh7ilnbcGQACxykaPKfmBccTHIOUo"}`,
			badKey:  "-----BEGIN CERTIFICATE-----\nMIICDDCCAZOgAwIBAgIBIzAKBggqhkjOPQQDAzBWMQswCQYDVQQGEwJVUzEVMBMG\nA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2\nMRIwEAYDVQQDEwlsb2NhbGhvc3QwHhcNMjAwNTA3MDk0MzU1WhcNMjAwNTA3MTE0\nMzU1WjBWMQswCQYDVQQGEwJVUzEVMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYD\nVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3QwdjAQ\nBgcqhkjOPQIBBgUrgQQAIgNiAARjcwW7g9wx4ePsuwcVzDJCVo4f8I1C1X5US4B1\nrWN+5zFSJoGCKaPTXMDhAdS08D1G20AIRmA0AlVVXRxrZYZ+Y282O6s+EGsB5T1W\nMCnUFk2Sa+xZiGPApYz4zSGbNEqjNTAzMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUE\nDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMAoGCCqGSM49BAMDA2cAMGQCMGSG\nVjx3DZP71ZGNDBw+AVdhNU3pgJW8kNpqjta3HFLb6pzqNOsfOn1ZeIWciEcyEgIw\nTGxli48W1AJ2s7Pw+3wOA6f9HAmczJPaiZ9CY038UiT8mk+pND5FEdqLhT/5lMEz\n-----END CERTIFICATE-----",
			builtin: builtinJWTVerifyES256,
		},
		{
			note:    "ES384",
			jwt:     `eyJhbGciOiJFUzM4NCJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.w85PzWrIQbJBOROnah0pa8or2LsXWnj88bwG1R-zf5Mm20CaYGPKPTQEsU_y-dzaWyDV1Na7nfaGaH3Khcvj8yS-bidZ0OZVVFDk9oabX7ZYvAHo2pTAOfxc11TeOYSF`,
			key:     "-----BEGIN CERTIFICATE-----\nMIICDDCCAZOgAwIBAgIBIzAKBggqhkjOPQQDAzBWMQswCQYDVQQGEwJVUzEVMBMG\nA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2\nMRIwEAYDVQQDEwlsb2NhbGhvc3QwHhcNMjAwNTA3MDk0MzU1WhcNMjAwNTA3MTE0\nMzU1WjBWMQswCQYDVQQGEwJVUzEVMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYD\nVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3QwdjAQ\nBgcqhkjOPQIBBgUrgQQAIgNiAARjcwW7g9wx4ePsuwcVzDJCVo4f8I1C1X5US4B1\nrWN+5zFSJoGCKaPTXMDhAdS08D1G20AIRmA0AlVVXRxrZYZ+Y282O6s+EGsB5T1W\nMCnUFk2Sa+xZiGPApYz4zSGbNEqjNTAzMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUE\nDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMAoGCCqGSM49BAMDA2cAMGQCMGSG\nVjx3DZP71ZGNDBw+AVdhNU3pgJW8kNpqjta3HFLb6pzqNOsfOn1ZeIWciEcyEgIw\nTGxli48W1AJ2s7Pw+3wOA6f9HAmczJPaiZ9CY038UiT8mk+pND5FEdqLhT/5lMEz\n-----END CERTIFICATE-----",
			badKey:  `{"kty":"EC","crv":"P-256","x":"z8J91ghFy5o6f2xZ4g8LsLH7u2wEpT2ntj8loahnlsE","y":"7bdeXLH61KrGWRdh7ilnbcGQACxykaPKfmBccTHIOUo"}`,
			builtin: builtinJWTVerifyES384,
		},
		{
			note:    "ES512",
			jwt:     `eyJhbGciOiJFUzUxMiJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.AYpssEoEqq9We9aKsnRykpECAVEOBRJJu8UgDzoL-F8fmB2LPxpS4Gl7D-9wAO5AJt4-9YSsgOb5FLc20MrZN30AAFYopZf75T1pEJQFrdDmOKT45abbrorcR7G_AHDbhBdDNM_R6GojYFg_HPxHndof745Yq5Tfw9PpJc-9kSyk6kqO`,
			key:     "-----BEGIN CERTIFICATE-----\nMIICWDCCAbmgAwIBAgIBAjAKBggqhkjOPQQDBDBWMQswCQYDVQQGEwJVUzEVMBMG\nA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2\nMRIwEAYDVQQDEwlsb2NhbGhvc3QwHhcNMjAwNTA3MTA1NDM3WhcNMjAwNTA3MTI1\nNDM3WjBWMQswCQYDVQQGEwJVUzEVMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYD\nVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3QwgZsw\nEAYHKoZIzj0CAQYFK4EEACMDgYYABAHLm3IMD/88vC/S1cCTyjrCjwHIGsjibFBw\nPBXt36YKCjUdS7jiJJR5YQVPypSv7gPaKKn1E8CqkfVdd3rrp1TocAEms4XvigtW\nZBZzffw9xyZCgmtQ2dTHsufi/5W/Yx8N3Uw+D2wl1LKcJraouo+qgamGfuou6WbA\noPEtdOg0+B4jF6M1MDMwDgYDVR0PAQH/BAQDAgWgMBMGA1UdJQQMMAoGCCsGAQUF\nBwMBMAwGA1UdEwEB/wQCMAAwCgYIKoZIzj0EAwQDgYwAMIGIAkIAzAAYDqMghX3S\n8UbS8s5TPAztJy9oNXFra5V8pPlUdNFc2ov2LN++scW46wCb/cJUyEc58sY7xFuK\nI5sCOkv95N8CQgFXmu354LZJ31zIovuUA8druOPe3TDnxMGwEEm2Lt43JNuhzNyP\nhJYh9/QKfe2AiwrLXEG4VVOIXdjq7vexl87evg==\n-----END CERTIFICATE-----",
			badKey:  `{"kty":"EC","crv":"P-256","x":"z8J91ghFy5o6f2xZ4g8LsLH7u2wEpT2ntj8loahnlsE","y":"7bdeXLH61KrGWRdh7ilnbcGQACxykaPKfmBccTHIOUo"}`,
			builtin: builtinJWTVerifyES512,
		},
		{
			note:    "EdDSA",
			jwt:     `eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.JkKWCY39IdWEQttmdqR7VdsvT-_QxheW_eb0S5wr_j83ltux_JDUIXs7a3Dtn3xuqzuhetiuJrWIvy5TzimeCg`,
			key:     "-----BEGIN PUBLIC KEY-----\nMCowBQYDK2VwAyEAwmK6SSAu2E9V7uynkCKEaj5nZJyTvNG4x0KohsRzLpg=\n-----END PUBLIC KEY-----",
			badKey:  `{"kty":"EC","crv":"P-256","x":"z8J91ghFy5o6f2xZ4g8LsLH7u2wEpT2ntj8loahnlsE","y":"7bdeXLH61KrGWRdh7ilnbcGQACxykaPKfmBccTHIOUo"}`,
			builtin: builtinJWTVerifyEdDSA,
		},
	}

	ctx := t.Context()

	cacheConfig := cache.Config{
		InterQueryBuiltinValueCache: cache.InterQueryBuiltinValueCacheConfig{
			NamedCacheConfigs: map[string]*cache.NamedValueCacheConfig{
				tokenCacheName: {
					MaxNumEntries: &[]int{5}[0],
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var verified bool
			iter := func(r *ast.Term) error {
				verified = bool(r.Value.(ast.Boolean))
				return nil
			}

			bctx := BuiltinContext{
				Context:                     ctx,
				Time:                        ast.NumberTerm(int64ToJSONNumber(time.Now().UnixNano())),
				InterQueryBuiltinValueCache: cache.NewInterQueryValueCache(ctx, &cacheConfig),
			}

			t.Run("successful verification", func(t *testing.T) {
				operands := []*ast.Term{ast.StringTerm(tc.jwt), ast.StringTerm(tc.key)}
				err := tc.builtin(bctx, operands, iter)
				if err != nil {
					t.Fatalf("unexpected error: %q", err)
				}

				if !verified {
					t.Fatal("expected token to be successfully verified")
				}

				k := createTokenCacheKey(ast.String(tc.jwt), ast.String(tc.key))
				if _, ok := bctx.InterQueryBuiltinValueCache.GetCache(tokenCacheName).Get(k); !ok {
					t.Fatal("expected token to be cached")
				}
			})

			t.Run("failed verification", func(t *testing.T) {
				operands := []*ast.Term{ast.StringTerm(tc.jwt), ast.StringTerm(tc.badKey)}
				err := tc.builtin(bctx, operands, iter)
				if err != nil {
					t.Fatalf("unexpected error: %q", err)
				}

				if verified {
					t.Fatal("expected token to fail verification")
				}

				k := createTokenCacheKey(ast.String(tc.jwt), ast.String(tc.badKey))
				if _, ok := bctx.InterQueryBuiltinValueCache.GetCache(tokenCacheName).Get(k); !ok {
					t.Fatal("expected token to be cached")
				}
			})
		})
	}
}

func TestBuiltinJWTDecodeVerify(t *testing.T) {
	tests := []struct {
		note    string
		jwt     string
		key     string
		keyType string
	}{
		{
			note:    "HS256",
			jwt:     `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjoiYWxpY2UiLCJhenAiOiJhbGljZSIsInN1Ym9yZGluYXRlcyI6W10sImhyIjpmYWxzZX0.rz3jTY033z-NrKfwrK89_dcLF7TN4gwCMj-fVBDyLoM`,
			key:     `secret`,
			keyType: `secret`,
		},
		{
			note:    "HS384",
			jwt:     `eyJhbGciOiJIUzM4NCJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.g98lHYzuqINVppLMoEZT7jlpX0IBSo9zKGoN9DhQg7Ua3YjLXbJMjzESjIHXOGLB`,
			key:     `secret`,
			keyType: `secret`,
		},
		{
			note:    "HS512",
			jwt:     `eyJhbGciOiJIUzUxMiJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.F6-xviRhK2OLcJJHFivhQqMN_dgX5boDrwbVKkdo9flQQNk-AaKpH3uYycFvBEd_erVefcsri_PkL4fjLSZ7ZA`,
			key:     `secret`,
			keyType: `secret`,
		},
		{
			note:    "RS256",
			jwt:     `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJmb28iOiJiYXIiLCJuYmYiOjE0NDQ0Nzg0MDB9.N0-EVdv5pvUfZYFRzMGnsWpNLHgwMEgViPwpuLBEtt32682OgnOK-N4X-2gpQEjQIbUr0IFym8YsRQU9GZvqQP72Sd6yOQNGSNeE74DpUZCAjBa9SBIb1UlD2MxZB-e7YJiEyo7pZhimaqorXrgorlaXYGMvsCFWDYmBLzGaGYaGJyEpkZHzHb7ujsDrJJjdEtDV3kh13gTHzLPvqnoXuuxelXye_8LPIhvgDy52gT4shUEso71pJCMv_IqAR19ljVE17lJzoi6VhRn6ReNUE-yg4KfCO4Ypnuu-mcQr7XtmSYoWkX72L5UQ-EyWkoz-w0SYKoJTPzHkTL2thYStksVpeNkGuck25aUdtrQgmPbao0QOWBFlkg03e6mPCD2-aXOt1ofth9mZGjxWMHX-mUqHaNmaWM3WhRztJ73hWrmB1YOdYQtOEHejfvR_td5tqIw4W6ufRy2ScOypGQe7kNaUZxpgxZ1927ZGNiQgawIOAQwXOcFx1JNSEIeg55-cYJrHPxsXGOB9ZxW-qnswmFJp474iUVXjzGhLexJDXBwvKGs_O3JFjMsvyV9_hm7bnQU0vG_HgPYs5i9VOHRMujq1vFBcm52TFVOBGdWaGfb9RRdLLYvVkJLk0Poh19rsCWb7-Vc3mAaGGpvuk4Wv-PnGGNC-V-FQqIbijHDrn_g`,
			key:     "-----BEGIN CERTIFICATE-----\nMIIFiDCCA3ACCQCGV6XsfG/oRTANBgkqhkiG9w0BAQUFADCBhTELMAkGA1UEBhMC\nVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEO\nMAwGA1UECgwFU3R5cmExDDAKBgNVBAsMA0RldjESMBAGA1UEAwwJbG9jYWxob3N0\nMRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5cmEwHhcNMTgwMzA2MDAxNTU5WhcNMTkw\nMzA2MDAxNTU5WjCBhTELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWEx\nFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEOMAwGA1UECgwFU3R5cmExDDAKBgNVBAsM\nA0RldjESMBAGA1UEAwwJbG9jYWxob3N0MRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5\ncmEwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDucnAwTRA0zqDQ671L\nKWOVwhjhycFyzyhZUd7vhsnslOBiYM6TYIDXhETfAk2RQoRE/9xF16woMD8FOglc\nlSuhi+GNfFRif6LfArm84ZFj1ZS1MX2logikhXhRJQ7AOHe5+ED0re3KH5lWyqfz\nR6bQuPYwTQSBJy6Tq7T9RiOM29yadCX64OaCEbzEFmHtNlbb5px4zCVvgskg/fpV\nGGCMpAYjGDatbxE5eAloVs1EJuI5RSqWr1JRm6EejxM04BFdfGn1HgWrsKXtlvBa\n00/AC0zXL5n6LK7+L3WbRguVTZcE4Yu70gDwhmM+VsKeT9LKClX003BNj0NJDRB9\ndw9MaWxsXDNHNOWEfbnASXeP7ZRv3D81ftij6P8SL14ZnxyrRty8TAN4ij3wd41l\nastRQCtrJFi+HzO606XOp6HDzBoWT0DGl8Sn2hZ6RLPyBnD04vvvcSGeCVjHGOQ8\nc3OTroK58u5MR/q4T00sTkeeVAxuKoEWKsjIBYYrJTe/a2mEq9yiDGbPNYDnWnQZ\njSUZm+Us23Y2sm/agZ5zKXcEuoecGL6sYCixr/xeB9BPxEiTthH+0M8OY99qpIhz\nSmj41wdgQfzZi/6B8pIr77V/KywYKxJEmzw8Uy48aC/rZ8WsT8QdKwclo1aiNJhx\n79OvGbZFoeHD/w7igpx+ttpF/wIDAQABMA0GCSqGSIb3DQEBBQUAA4ICAQC3wWUs\nfXz+aSfFVz+O3mLFkr65NIgazbGAySgMgMNVuadheIkPL4k21atyflfpx4pg9FGv\n40vWCLMajpvynfz4oqah0BACnpqzQ8Dx6HYkmlXK8fLB+WtPrZBeUEsGPKuJYt4M\nd5TeY3VpNgWOPXmnE4lvxHZqh/8OwmOpjBfC9E3e2eqgwiwOkXnMaZEPgKP6JiWk\nEFaQ9jgMQqJZnNcv6NmiqqsZeI0/NNjBpkmEWQl+wLegVusHiQ0FMBMQ0taEo21r\nzUwHoNJR3h3wgGQiKxKOH1FUKHBV7hEqObLraD/hfG5xYucJfvvAAP1iH0ycPs+9\nhSccrn5/HY1c9AZnW8Kh7atp/wFP+sHjtECWK/lUmXfhASS293hprCpJk2n9pkmR\nziXKJhjwkxlC8NcHuiVfaxdfDa4+1Qta2gK7GEypbvLoEmIt/dsYUsxUg84lwJJ9\nnyC/pfZ5a8wFSf186JeVH4kHd3bnkzlQz460HndOMSJ/Xi1wSfuZlOVupFf8TVKl\np4j28MTLH2Wqx50NssKThdaX6hoCiMqreYa+EVaN1f/cIGQxZSCzdzMCKqdB8lKB\n3Eax+5zsIa/UyPwGxZcyXBRHAlz5ZnkjuRxInyiMkBWWz3IZXjTe6Fq8BNd2UWNc\nw35+2nO5n1LKXgR2+nzhZUOk8TPsi9WUywRluQ==\n-----END CERTIFICATE-----",
			keyType: `cert`,
		},
		{
			note:    "RS384",
			jwt:     `eyJhbGciOiJSUzM4NCJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.b__y2zjqMoD7iWbHeQ0lNpnche3ph5-AwrIQICLMQQGtEz9WMBteHydkC5g01bm3TBX1d04Z5IEOsuK6btAtWma04c5NYqaUyNEUJKYCFoY02uH0jGdGfL6R5Kkv0lkNvN0s3Nex9jMaVVgqx8bcrOU0uRBFT67sXcm11LHaB9BwKFslolzHClxgXy5RIZb4OFk_7Yk7xTC6PcvEWkkGR9uXBhfDEig5WqdwOWPeulimvARDw14U35rzeh9xpGAPjBKeE-y20fXAk0cSF1H69C-Qa1jDQheYIrAJ6XMYGNZWuay5-smmeefe67eweEt1q-AD1NFepqkmZX382DGuYQ`,
			key:     "-----BEGIN CERTIFICATE-----\nMIIDXDCCAkSgAwIBAgIBLjANBgkqhkiG9w0BAQsFADBWMQswCQYDVQQGEwJVUzEV\nMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMD\nRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3QwHhcNMjAwNTA3MDg1MTAzWhcNMjAwNTA3\nMTA1MTAzWjBWMQswCQYDVQQGEwJVUzEVMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4w\nDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3Qw\nggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDeRmygX/fOOUu5Wm91PFNo\nsHDG1CzG9a1iKBjUeMgi9bXXScUfatPmsNlxb56uSi0RXUsvJmY/yxkIIhRyapxW\n49j2idAM3SGGL1nOZf/XdpDHYsAFFZ237HGb8DOEk/p3xCFv0tH/iQ+kLP36EM1+\ntn6BfUXdJnVyvkSK2iMNeRY7A4DMX7sGX39LXsVJiCokIC8E0QUFrSjvrAm9ejKE\ntPojydo4c3VUxLfmFuyMXoD3bfk1Jv5i2J5RjtomjgK6zNCvgYzpspiodHChkzlU\nX8yk2YqlAHX3XdJA94LaDE2kNXiOQnFkUb8GsP7hmEbwGtMUEQie+jfgKplxJ49B\nAgMBAAGjNTAzMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAM\nBgNVHRMBAf8EAjAAMA0GCSqGSIb3DQEBCwUAA4IBAQC9f2/kxT7DnQ94ownhHvd6\nrzk1WirI90rFM2MxhfkaDrOHhSGZL9nDf6TIZ4qeFKZXthpKxpiZm2Oxmn+vUsik\nW6bYjq1nX0GCchQLaaFf9Jh1IOLwkfoBdX55tV8xUGHRWgDlCuGbqiixz+Bm0Kap\nkmbyJynVcoiKhdLyYm/YTn/pC32SJW666reQ+0qCAoxzLQowBetHjwDam9RsDEf4\n+JRDjYPutNXyJ5X8BaBA6PzHanzMG/7RFYcx/2YhXwVxdfPHku4ALJcddIGAGNx2\n5yte+HY0aEu+06J67eD9+4fU7NixRMKigk9KbjqpeWD+0be+VgX8Dot4jaISgI/3\n-----END CERTIFICATE-----",
			keyType: `cert`,
		},
		{
			note:    "RS512",
			jwt:     `eyJhbGciOiJSUzUxMiJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.VSe3qK5Gp0Q0_5nRgMFu25yw74FIgX-kXPOemSi62l-AxeVdUw8rOpEFrSTCaVjd3mPfKb-B056a-gtrbpXK9sUQnFdqdsyt8gHK-umz5lVyWfoAgj51Ontv-9K_pRORD9wqKqdTLZjCxJ5tyKoO0gY3SwwqSqGrp85vUjvEcK3jbMKINGRUNnOokeSm7byUEJsfKVUbPboSX1TGyvjDOZxxSITj8-bzZZ3F21DJ23N2IiJN7FW8Xj-SYyphXo-ML50o5bjW9YlQ5BDk-RW1I4eE-KpsxhApPv_xIgE8d89PVtXFuoJtv0yLRaZ1q04Fl9KNoMyZrmr349yppn0JlQ`,
			key:     "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3kZsoF/3zjlLuVpvdTxT\naLBwxtQsxvWtYigY1HjIIvW110nFH2rT5rDZcW+erkotEV1LLyZmP8sZCCIUcmqc\nVuPY9onQDN0hhi9ZzmX/13aQx2LABRWdt+xxm/AzhJP6d8Qhb9LR/4kPpCz9+hDN\nfrZ+gX1F3SZ1cr5EitojDXkWOwOAzF+7Bl9/S17FSYgqJCAvBNEFBa0o76wJvXoy\nhLT6I8naOHN1VMS35hbsjF6A9235NSb+YtieUY7aJo4CuszQr4GM6bKYqHRwoZM5\nVF/MpNmKpQB1913SQPeC2gxNpDV4jkJxZFG/BrD+4ZhG8BrTFBEInvo34CqZcSeP\nQQIDAQAB\n-----END PUBLIC KEY-----",
			keyType: `cert`,
		},
		{
			note:    "PS256",
			jwt:     `eyJ0eXAiOiAiSldUIiwgImFsZyI6ICJQUzI1NiJ9.eyJuYmYiOiAxNDQ0NDc4NDAwLCAiZm9vIjogImJhciJ9.i0F3MHWzOsBNLqjQzK1UVeQid9xPMowCoUsoM-C2BDxUY-FMKmCeJ1NJ4TGnS9HzFK1ftEvRnPT7EOxOkHPoCk1rz3feTFgtHtNzQqLM1IBTnz6aHHOrda_bKPHH9ZIYCRQUPXhpC90ivW_IJR-f7Z1WLrMXaJ71i1XteruENHrJJJDn0HedHG6N0VHugBHrak5k57cbE31utAdx83TEd8v2Y8wAkCJXKrdmTa-8419LNxW_yjkvoDD53n3X5CHhYkSymU77p0v6yWO38qDWeKJ-Fm_PrMAo72_rizDBj_yPa5LA3bT_EnsgZtC-sp8_SCDIH41bjiCGpRHhqgZmyw`,
			key:     `{"kty":"RSA","e":"AQAB","kid":"bf688c97-bf51-49ba-b9d3-115195bb0eb8","n":"uJApsyzFv-Y85M5JjezHvMDw_spgVCI7BqpYhnzK3xXw1dnkz1bWXGA9yF6AeADlE-1yc1ozrAURTnFSihIgj414i3MC2_0FkNcdAbnX7d9q9_jdCkHda4HER0zzXCaHlgnzoAz6edUU800-h0LleLnfgg4UST-0DFTCIGpfTbs7OPSy2WgT1vP6xbB45CUOJA7o0q6XE-hdhWWN0plrDiYD-0Y1SpOQYXmHhSmr-WVeKeoh5_0zeEVab6TQYec_16ByEyepaZB0g6WyGkFE6aG1NrpvDd24s_h7BAJg_S2mtu1lKWEqYjOgwzEl5XQQyXbpnq1USb12ArX16rZdew"}`,
			keyType: `cert`,
		},
		{
			note:    "PS384",
			jwt:     `eyJhbGciOiJQUzM4NCJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.EHPUvPr6uJOYqdza95WbM1SYD8atZHJEVRggpwOWnHGsjQBoEarJb8QgW7TY22OXwGw2HWluTiyT_MAz02NaHRzZv6AgrmxCLChMWkCHLwPxqjs0xSvVAMLzHHq2X2Bcujo9KORGudR7zKz8pOX5Mfnm7Z6OGtqPCPLaIdVJlddNsG6a571NOuVuDWbcg0omeRDANZpCZMJeAQN2M-4Q61ef6zcQHK1R-QqzBhw6HzMgqR1LRJ0xbrmD-L5o53JM3pV1e1juKNXVK3vWkDQRCQORFn1lyH5isfSsiiHW-x90sUC7TrU_cOji4MMmOCME6kkwxe57ZgpeXtdVTvldpw`,
			key:     "-----BEGIN CERTIFICATE-----\nMIIDXDCCAkSgAwIBAgIBKjANBgkqhkiG9w0BAQsFADBWMQswCQYDVQQGEwJVUzEV\nMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMD\nRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3QwHhcNMjAwNTA3MDkxMjU2WhcNMjAwNTA3\nMTExMjU2WjBWMQswCQYDVQQGEwJVUzEVMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4w\nDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3Qw\nggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDtyVWH2FE8cU8LRcArH4Tw\nDhBOFcmJF28LrvRObcbDYsae6rEwby0aRgSxjTEMgyGBjroBSl22wOSA93kwx4pu\npfXEqbwywn9FhyKBb/OXQSglPmwrpmzQtPJGzBHncL+PPjRhPfqimwf7ZIPKAAgI\nz9O6ppGhE/x4Ct444jthUIBZuG5cUXhgiPBQdIQ3K88QhgVwcufTZkNHj4iSDfhl\nVFDHVjXjd2B/yGODjyv0TyChV0YBNGjMv7YFLWmIFUFzK+6qNSxu4czPtRkyfwaV\n2GW/PBT5f8fc6fKgQZ6k6BLK6+pi0iPh5TUizyHtqtueWDbrJ+wfdJilQS8D+EGr\nAgMBAAGjNTAzMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAM\nBgNVHRMBAf8EAjAAMA0GCSqGSIb3DQEBCwUAA4IBAQBgVAM50/0aBTBxESYKIKN4\nE+qbV6aE0C+wYJLet0EWPTxwmFZamq5LNEO/D6xyoY5WHY60EyHRMs0agSB/ATBX\n5ULdEwh9G0NjxqivCcoddQ1fuVS2PrrqNL7VlRnYbTpd8/Dh4qnyl5FltlyZ/29L\ny7BWOwlcBlZdhsfH8svNX4PUxjRD+jmnczCDi7XSOKT8htKUV2ih1c9JrWpIhCi/\nHzEXkaAxNdhBNdIsLQMo3qq9fkSgNZQk9/ecJNPeuJ/UYyr5Xa4PxIWl4U+P7yuI\n+Q3FSPmUbiVsSGqMhh6V/DN8M+T5/KiSB47gFOfxc2/RR5aw4HkSp3WxwbT9njbE\n-----END CERTIFICATE-----",
			keyType: `cert`,
		},
		{
			note:    "PS512",
			jwt:     `eyJhbGciOiJQUzUxMiJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.VRlkPtiUq5MmBNgyuBqxv2_aX40STrWrBB2sSmGbxI78jVG_3hVoh7Mk-wUmFL389qpf05xNdn-gpMe-MSDUux7U7EuFspFZdYTUBo9wRvEBe4e1rHUCG00lVdYCG7eEgbAxM3cUhrHRwExBte30qBrFFUY9FgG-kJdYhgyh7VquMGuKgiS8CP_H0Gp1mIvTw6eEnSFAoKiryw9edUZ78pHELNn4y18YZvEndeNZh7f19LCtrB0G2bJUHGM4vPcwo2D-UAhEFBpSlnnqXDLSWOhUgLNLu0kZACXhT808KT6fdF6eFihdThmWN7_HUz2znjrjs71CqqDJgLhkGs8UvQ`,
			key:     "-----BEGIN CERTIFICATE-----\nMIIDXDCCAkSgAwIBAgIBKjANBgkqhkiG9w0BAQsFADBWMQswCQYDVQQGEwJVUzEV\nMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMD\nRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3QwHhcNMjAwNTA3MDkxMjU2WhcNMjAwNTA3\nMTExMjU2WjBWMQswCQYDVQQGEwJVUzEVMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4w\nDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3Qw\nggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDtyVWH2FE8cU8LRcArH4Tw\nDhBOFcmJF28LrvRObcbDYsae6rEwby0aRgSxjTEMgyGBjroBSl22wOSA93kwx4pu\npfXEqbwywn9FhyKBb/OXQSglPmwrpmzQtPJGzBHncL+PPjRhPfqimwf7ZIPKAAgI\nz9O6ppGhE/x4Ct444jthUIBZuG5cUXhgiPBQdIQ3K88QhgVwcufTZkNHj4iSDfhl\nVFDHVjXjd2B/yGODjyv0TyChV0YBNGjMv7YFLWmIFUFzK+6qNSxu4czPtRkyfwaV\n2GW/PBT5f8fc6fKgQZ6k6BLK6+pi0iPh5TUizyHtqtueWDbrJ+wfdJilQS8D+EGr\nAgMBAAGjNTAzMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAM\nBgNVHRMBAf8EAjAAMA0GCSqGSIb3DQEBCwUAA4IBAQBgVAM50/0aBTBxESYKIKN4\nE+qbV6aE0C+wYJLet0EWPTxwmFZamq5LNEO/D6xyoY5WHY60EyHRMs0agSB/ATBX\n5ULdEwh9G0NjxqivCcoddQ1fuVS2PrrqNL7VlRnYbTpd8/Dh4qnyl5FltlyZ/29L\ny7BWOwlcBlZdhsfH8svNX4PUxjRD+jmnczCDi7XSOKT8htKUV2ih1c9JrWpIhCi/\nHzEXkaAxNdhBNdIsLQMo3qq9fkSgNZQk9/ecJNPeuJ/UYyr5Xa4PxIWl4U+P7yuI\n+Q3FSPmUbiVsSGqMhh6V/DN8M+T5/KiSB47gFOfxc2/RR5aw4HkSp3WxwbT9njbE\n-----END CERTIFICATE-----",
			keyType: `cert`,
		},
		{
			note:    "ES256",
			jwt:     `eyJ0eXAiOiAiSldUIiwgImFsZyI6ICJFUzI1NiJ9.eyJuYmYiOiAxNDQ0NDc4NDAwLCAiaXNzIjogInh4eCJ9.lArczfN-pIL8oUU-7PU83u-zfXougXBZj6drFeKFsPEoVhy9WAyiZlRshYqjTSXdaw8yw2L-ovt4zTUZb2PWMg`,
			key:     `{"kty":"EC","crv":"P-256","x":"z8J91ghFy5o6f2xZ4g8LsLH7u2wEpT2ntj8loahnlsE","y":"7bdeXLH61KrGWRdh7ilnbcGQACxykaPKfmBccTHIOUo"}`,
			keyType: `cert`,
		},
		{
			note:    "ES384",
			jwt:     `eyJhbGciOiJFUzM4NCJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.w85PzWrIQbJBOROnah0pa8or2LsXWnj88bwG1R-zf5Mm20CaYGPKPTQEsU_y-dzaWyDV1Na7nfaGaH3Khcvj8yS-bidZ0OZVVFDk9oabX7ZYvAHo2pTAOfxc11TeOYSF`,
			key:     "-----BEGIN CERTIFICATE-----\nMIICDDCCAZOgAwIBAgIBIzAKBggqhkjOPQQDAzBWMQswCQYDVQQGEwJVUzEVMBMG\nA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2\nMRIwEAYDVQQDEwlsb2NhbGhvc3QwHhcNMjAwNTA3MDk0MzU1WhcNMjAwNTA3MTE0\nMzU1WjBWMQswCQYDVQQGEwJVUzEVMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYD\nVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3QwdjAQ\nBgcqhkjOPQIBBgUrgQQAIgNiAARjcwW7g9wx4ePsuwcVzDJCVo4f8I1C1X5US4B1\nrWN+5zFSJoGCKaPTXMDhAdS08D1G20AIRmA0AlVVXRxrZYZ+Y282O6s+EGsB5T1W\nMCnUFk2Sa+xZiGPApYz4zSGbNEqjNTAzMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUE\nDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMAoGCCqGSM49BAMDA2cAMGQCMGSG\nVjx3DZP71ZGNDBw+AVdhNU3pgJW8kNpqjta3HFLb6pzqNOsfOn1ZeIWciEcyEgIw\nTGxli48W1AJ2s7Pw+3wOA6f9HAmczJPaiZ9CY038UiT8mk+pND5FEdqLhT/5lMEz\n-----END CERTIFICATE-----",
			keyType: `cert`,
		},
		{
			note:    "ES512",
			jwt:     `eyJhbGciOiJFUzUxMiJ9.eyJTY29wZXMiOlsiZm9vIiwiYmFyIl0sIm5iZiI6MTQ1MTYwNjQwMH0.AYpssEoEqq9We9aKsnRykpECAVEOBRJJu8UgDzoL-F8fmB2LPxpS4Gl7D-9wAO5AJt4-9YSsgOb5FLc20MrZN30AAFYopZf75T1pEJQFrdDmOKT45abbrorcR7G_AHDbhBdDNM_R6GojYFg_HPxHndof745Yq5Tfw9PpJc-9kSyk6kqO`,
			key:     "-----BEGIN CERTIFICATE-----\nMIICWDCCAbmgAwIBAgIBAjAKBggqhkjOPQQDBDBWMQswCQYDVQQGEwJVUzEVMBMG\nA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYDVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2\nMRIwEAYDVQQDEwlsb2NhbGhvc3QwHhcNMjAwNTA3MTA1NDM3WhcNMjAwNTA3MTI1\nNDM3WjBWMQswCQYDVQQGEwJVUzEVMBMGA1UEBxMMUmVkd29vZCBDaXR5MQ4wDAYD\nVQQKEwVTdHlyYTEMMAoGA1UECxMDRGV2MRIwEAYDVQQDEwlsb2NhbGhvc3QwgZsw\nEAYHKoZIzj0CAQYFK4EEACMDgYYABAHLm3IMD/88vC/S1cCTyjrCjwHIGsjibFBw\nPBXt36YKCjUdS7jiJJR5YQVPypSv7gPaKKn1E8CqkfVdd3rrp1TocAEms4XvigtW\nZBZzffw9xyZCgmtQ2dTHsufi/5W/Yx8N3Uw+D2wl1LKcJraouo+qgamGfuou6WbA\noPEtdOg0+B4jF6M1MDMwDgYDVR0PAQH/BAQDAgWgMBMGA1UdJQQMMAoGCCsGAQUF\nBwMBMAwGA1UdEwEB/wQCMAAwCgYIKoZIzj0EAwQDgYwAMIGIAkIAzAAYDqMghX3S\n8UbS8s5TPAztJy9oNXFra5V8pPlUdNFc2ov2LN++scW46wCb/cJUyEc58sY7xFuK\nI5sCOkv95N8CQgFXmu354LZJ31zIovuUA8druOPe3TDnxMGwEEm2Lt43JNuhzNyP\nhJYh9/QKfe2AiwrLXEG4VVOIXdjq7vexl87evg==\n-----END CERTIFICATE-----",
			keyType: `cert`,
		},
		{
			note:    "EdDSA",
			jwt:     `eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.JkKWCY39IdWEQttmdqR7VdsvT-_QxheW_eb0S5wr_j83ltux_JDUIXs7a3Dtn3xuqzuhetiuJrWIvy5TzimeCg`,
			key:     "-----BEGIN PUBLIC KEY-----\nMCowBQYDK2VwAyEAwmK6SSAu2E9V7uynkCKEaj5nZJyTvNG4x0KohsRzLpg=\n-----END PUBLIC KEY-----",
			keyType: `cert`,
		},
	}

	ctx := t.Context()

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var verified bool
			iter := func(r *ast.Term) error {
				verified = bool(r.Value.(*ast.Array).Get(ast.NumberTerm("0")).Value.(ast.Boolean))
				return nil
			}

			bctx := BuiltinContext{
				Context: ctx,
				Time:    ast.NumberTerm(int64ToJSONNumber(time.Now().UnixNano())),
			}

			keysTerm := ast.ObjectTerm(ast.Item(ast.StringTerm(tc.keyType), ast.StringTerm(tc.key)))
			operands := []*ast.Term{ast.StringTerm(tc.jwt), keysTerm}
			err := builtinJWTDecodeVerify(bctx, operands, iter)
			if err != nil {
				t.Fatalf("unexpected error: %q", err)
			}

			if !verified {
				t.Fatal("expected token to be successfully verified")
			}
		})
	}
}
