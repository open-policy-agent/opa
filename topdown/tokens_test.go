package topdown

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/jwx/jwk"
	"github.com/open-policy-agent/opa/internal/jwx/jws"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
)

func TestParseTokenConstraints(t *testing.T) {
	wallclock := ast.NumberTerm(int64ToJSONNumber(time.Now().UnixNano()))
	t.Run("Empty", func(t *testing.T) {
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
		pubKey := constraints.keys[0].(*ecdsa.PublicKey)
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
		elPubKey := constraints.keys[0].(*ecdsa.PublicKey)
		if elPubKey.Curve != elliptic.P256() {
			t.Errorf("curve: %v", elPubKey.Curve)
		}

		rsaPubKey := constraints.keys[1].(*rsa.PublicKey)
		if rsaPubKey.Size() != 256 {
			t.Errorf("expected size 256 found %d", rsaPubKey.Size())
		}
	})
	t.Run("Time", func(t *testing.T) {
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
		c := ast.NewObject()
		c.Insert(ast.StringTerm("whatever"), ast.StringTerm("junk"))
		_, err := parseTokenConstraints(c, wallclock)
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
		err    string
	}{

		"https://tools.ietf.org/html/rfc7515#appendix-A.3",
		"`" + es256Hdr + "`",
		"`" + examplePayload + "`",
		"`" + ecKey + "`",

		"",
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

	ctx := context.Background()
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

	standardHeaders := &jws.StandardHeaders{}
	err = json.Unmarshal([]byte(es256Hdr), standardHeaders)
	if err != nil {
		t.Fatal("Failed to parse header")
	}
	alg := standardHeaders.GetAlgorithm()

	keys, err := jwk.ParseString(ecKey)
	if err != nil {
		t.Fatal("Failed to parse JWK")
	}
	key, err := keys.Keys[0].Materialize()
	if err != nil {
		t.Fatal("Failed to create private key")
	}
	publicKey, err := jwk.GetPublicKey(key)
	if err != nil {
		t.Fatalf("failed to get public key: %v", err)
	}

	// Verify with vendor library

	verifiedPayload, err := jws.Verify([]byte(result.(string)), alg, publicKey)
	if err != nil || string(verifiedPayload) != examplePayload {
		t.Fatal("Failed to verify message")
	}
}

// TestTopDownJWTEncodeSignEC needs to perform all tests inline because we do not know the
// expected values before hand
func TestTopDownJWTEncodeSignES512(t *testing.T) {

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
		err    string
	}{

		"https://tools.ietf.org/html/rfc7515#appendix-A.4",
		"`" + es512Hdr + "`",
		"`" + examplePayload + "`",
		"`" + ecKey + "`",

		"",
	}
	type test struct {
		note  string
		rules []string
	}
	var tests []test

	tests = append(tests, test{
		params.note,
		[]string{fmt.Sprintf(`p = x { io.jwt.encode_sign_raw(%s, %s, %s, x) }`, params.input1, params.input2, params.input3)},
	})

	tc := tests[0]

	compiler, err := compileRules(nil, tc.rules, nil)

	if err != nil {
		t.Errorf("%v: Compiler error: %v", tc.note, err)
		return
	}
	store := inmem.New()
	path := []string{"generated", "p"}
	var inputTerm *ast.Term

	ctx := context.Background()
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

	standardHeaders := &jws.StandardHeaders{}
	err = json.Unmarshal([]byte(es512Hdr), standardHeaders)
	if err != nil {
		t.Fatal("Failed to parse header")
	}
	alg := standardHeaders.GetAlgorithm()

	keys, err := jwk.ParseString(ecKey)
	if err != nil {
		t.Fatalf("Failed to parse JWK: %v", err)
	}
	key, err := keys.Keys[0].Materialize()
	if err != nil {
		t.Fatalf("Failed to create private key: %v", err)
	}
	publicKey, err := jwk.GetPublicKey(key)
	if err != nil {
		t.Fatalf("Failed to get public key: %v", err)
	}

	// Verify with vendor library

	verifiedPayload, err := jws.Verify([]byte(result.(string)), alg, publicKey)
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
	query := `io.jwt.encode_sign({"alg": "ES256"},{"pay": "load"},
	  {"kty":"EC",
	   "crv":"P-256",
	   "x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU",
	   "y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0",
	   "d":"jpsQnnGQmL-YBIffH1136cspYG6-0iY7X1fCE9-E9LI"
	  }, x)`
	encodedSigned := "eyJhbGciOiAiRVMyNTYifQ.eyJwYXkiOiAibG9hZCJ9.-LoHxtbT8t_TnqlLyONI4BtjvfkySO8TcoCFENqTTH2AKxvn29nAjxOdlbY-0EKVM2nJ4ukCx4IGtZtuwXr0VQ"

	for i := 0; i < 10; i++ {
		q := NewQuery(ast.MustParseBody(query)).
			WithSeed(&cng{}).
			WithStrictBuiltinErrors(true).
			WithCompiler(ast.NewCompiler())

		qrs, err := q.Run(context.Background())
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
