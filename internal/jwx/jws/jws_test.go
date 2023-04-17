package jws_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
	"github.com/open-policy-agent/opa/internal/jwx/jwk"
	"github.com/open-policy-agent/opa/internal/jwx/jws"
	"github.com/open-policy-agent/opa/internal/jwx/jws/sign"
	"github.com/open-policy-agent/opa/internal/jwx/jws/verify"
)

const examplePayload = `{"iss":"joe",` + "\r\n" + ` "exp":1300819380,` + "\r\n" + ` "http://example.com/is_root":true}`
const exampleCompactSerialization = `eyJ0eXAiOiJKV1QiLA0KICJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ.dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk`

func TestParseErrors(t *testing.T) {

	t.Run("Empty bytes.Buffer", func(t *testing.T) {
		_, err := jws.ParseString("")
		if err == nil {
			t.Fatal("Parsing an empty buffer should result in an error")
		}
	})
	t.Run("Compact missing parts", func(t *testing.T) {
		incoming := strings.Join(
			(strings.Split(
				exampleCompactSerialization,
				".",
			))[:2],
			".",
		)
		_, err := jws.ParseString(incoming)
		if err == nil {
			t.Fatalf("Parsing compact serialization with less than 3 parts should be an error")
		}
	})
	const badValue = "%badvalue%"
	t.Run("Compact bad header", func(t *testing.T) {
		parts := strings.Split(exampleCompactSerialization, ".")
		parts[0] = "badValue"
		incoming := strings.Join(parts, ".")

		_, err := jws.ParseString(incoming)
		if err == nil {
			t.Fatal("Parsing compact serialization with bad header should be an error")
		}
	})
	t.Run("Compact bad Payload", func(t *testing.T) {
		parts := strings.Split(exampleCompactSerialization, ".")
		parts[1] = badValue
		incoming := strings.Join(parts, ".")

		_, err := jws.ParseString(incoming)
		if err == nil {
			t.Fatal("Parsing compact serialization with bad Payload should be an error")
		}
	})
	t.Run("Compact bad Signature", func(t *testing.T) {
		parts := strings.Split(exampleCompactSerialization, ".")
		parts[2] = badValue
		incoming := strings.Join(parts, ".")

		_, err := jws.ParseString(incoming)
		if err == nil {
			t.Fatal("Parsing compact serialization with bad Signature should be an error")
		}
	})
}

func TestAlgError(t *testing.T) {

	t.Run("Unknown Algorithm", func(t *testing.T) {
		const hdr = `{"typ":"JWT",` + "\r\n" + ` "alg":"unknown"}`
		var standardHeaders jws.StandardHeaders
		err := json.Unmarshal([]byte(hdr), &standardHeaders)
		if err != nil {
			t.Fatal(err)
		}
		if standardHeaders.Algorithm != jwa.Unsupported {
			t.Errorf("expected unsupported algorithm")
		}
	})
}

func TestRoundTrip(t *testing.T) {
	payload := []byte("Lorem ipsum")
	sharedKey := []byte("Avracadabra")

	hmacAlgorithms := []jwa.SignatureAlgorithm{jwa.HS256, jwa.HS384, jwa.HS512}
	for _, alg := range hmacAlgorithms {
		t.Run("HMAC "+alg.String(), func(t *testing.T) {
			signed, err := jws.SignWithOption(payload, alg, sharedKey)
			if err != nil {
				t.Fatalf("Failed to sign input: %s", err.Error())
			}
			verified, err := jws.Verify(signed, alg, sharedKey)
			if err != nil {
				t.Fatalf("Message verification failed: %s", err.Error())
			}
			if !bytes.Equal(payload, verified) {
				t.Fatalf("Mismatched payload (%s):(%s)", payload, verified)
			}
		})
	}
}

func TestVerifyWithJWKSet(t *testing.T) {

	payload := []byte("Hello, World!")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate key: %s", err.Error())
	}
	jwkKey, err := jwk.New(&key.PublicKey)
	if err != nil {
		t.Fatalf("Failed to create JWX Private Key: %s", err.Error())
	}
	err = jwkKey.Set(jwk.AlgorithmKey, jwa.RS256)
	if err != nil {
		t.Fatalf("Failed to set alg: %s", err.Error())
	}
	signature, err := jws.SignWithOption(payload, jwa.RS256, key)
	if err != nil {
		t.Fatalf("Failed to sign message: %s", err.Error())
	}

	_, err = jws.VerifyWithJWKSet(signature, &jwk.Set{Keys: []jwk.Key{jwkKey}})
	if err != nil {
		t.Fatalf("Failed to verify with JWKSet: %s", err.Error())
	}

	verified, err := jws.VerifyWithJWK(signature, jwkKey)
	if err != nil {
		t.Fatalf("Failed to verify with JWK: %s", err.Error())
	}

	if !bytes.Equal(payload, verified) {
		t.Fatalf("Mismatched payload (%s):(%s)", payload, verified)
	}
}

func TestRoundtrip_RSACompact(t *testing.T) {
	payload := []byte("Hello, World!")
	for _, alg := range []jwa.SignatureAlgorithm{jwa.RS256, jwa.RS384, jwa.RS512, jwa.PS256, jwa.PS384, jwa.PS512} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("Failed to generate key: %s", err.Error())
		}

		buf, err := jws.SignWithOption(payload, alg, key)
		if err != nil {
			t.Fatalf("Failed to sign message: %s", err.Error())
		}

		verified, err := jws.Verify(buf, alg, &key.PublicKey)
		if err != nil {
			t.Fatalf("Failed to verify signature: %s", err.Error())
		}

		if !bytes.Equal(payload, verified) {
			t.Fatalf("Mismatched payloads (%s):(%s)", payload, verified)
		}
	}
}

func TestEncode(t *testing.T) {
	// HS256Compact tests that https://tools.ietf.org/html/rfc7515#appendix-A.1 works
	t.Run("HS256Compact", func(t *testing.T) {
		const hdr = `{"typ":"JWT",` + "\r\n" + ` "alg":"HS256"}`
		const hmacKey = `AyM1SysPpbyDfgZld3umj1qzKObwVMkoqQ-EstJQLr_T-1qS0gZH75aKtMN3Yj0iPS4hcgUuTwjAzZr1Z9CAow`
		const expectedCompact = `eyJ0eXAiOiJKV1QiLA0KICJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ.dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk`

		hmacKeyDecoded, err := base64.RawURLEncoding.DecodeString(hmacKey)
		if err != nil {
			t.Fatalf("Failed to decode HMAC Key: %s", err.Error())
		}

		hdrBuf := base64.RawURLEncoding.EncodeToString([]byte(hdr))
		payload := base64.RawURLEncoding.EncodeToString([]byte(examplePayload))
		signingInput := strings.Join(
			[]string{
				hdrBuf,
				payload,
			}, ".",
		)

		signer, err := sign.New(jwa.HS256)
		if err != nil {
			t.Fatalf("Failed to create HMAC signer: %s", err.Error())
		}

		signature, err := signer.Sign([]byte(signingInput), hmacKeyDecoded)
		if err != nil {
			t.Fatalf("Failed to sign input: %s", err.Error())
		}
		encSignature := base64.RawURLEncoding.EncodeToString(signature)
		realizedCompact := strings.Join(
			[]string{
				signingInput,
				encSignature,
			}, ".",
		)

		if expectedCompact != realizedCompact {
			t.Fatal("Mismatched compact serializations")
		}

		msg, err := jws.ParseString(realizedCompact)
		if err != nil {
			t.Fatalf("Failed to parse realized serialization: %s", err.Error())
		}

		signatures := msg.GetSignatures()
		if len(signatures) != 1 {
			t.Fatalf("Invalid number of signatures: %d", len(signatures))
		}

		algorithm := signatures[0].ProtectedHeaders().GetAlgorithm()
		if algorithm != jwa.HS256 {
			t.Fatal("Algorithm in header does not match")
		}

		v, err := verify.New(jwa.HS256)
		if err != nil {
			t.Fatalf("Failed to create verifier: %s", err.Error())
		}

		err = v.Verify([]byte(signingInput), signature, hmacKeyDecoded)
		if err != nil {
			t.Fatalf("Message verification failed: %s", err.Error())
		}
	})
	t.Run("HS256CompactLiteral", func(t *testing.T) {
		const hdr = `{"typ":"JWT",` + "\r\n" + ` "alg":"HS256"}`
		const jwkSrc = `{
"kty":"oct",
"k":"AyM1SysPpbyDfgZld3umj1qzKObwVMkoqQ-EstJQLr_T-1qS0gZH75aKtMN3Yj0iPS4hcgUuTwjAzZr1Z9CAow"
}`

		hdrBytes := []byte(hdr)

		standardHeaders := &jws.StandardHeaders{}
		err := json.Unmarshal(hdrBytes, standardHeaders)
		if err != nil {
			t.Fatal("Failed to parse Protected header")
		}
		alg := standardHeaders.GetAlgorithm()

		keys, err := jwk.ParseString(jwkSrc)
		if err != nil {
			t.Fatalf("Failed to parse JWK: %s", err.Error())
		}
		key, err := keys.Keys[0].Materialize()
		if err != nil {
			t.Fatal("Failed to parse key")
		}
		var jwsCompact []byte
		jwsCompact, err = jws.SignLiteral([]byte(examplePayload), alg, key, hdrBytes, rand.Reader)
		if err != nil {
			t.Fatal("Failed to sign message")
		}

		msg, err := jws.ParseByte(jwsCompact)
		if err != nil {
			t.Fatalf("Failed to parse compact serialization: %s", err.Error())
		}
		signatures := msg.GetSignatures()

		algorithm := signatures[0].ProtectedHeaders().GetAlgorithm()
		if algorithm != alg {
			t.Fatal("Algorithm in header does not match")
		}

		v, err := verify.New(alg)
		if err != nil {
			t.Fatalf("Failed to create verifier: %s", err.Error())
		}
		hdrBuf := base64.RawURLEncoding.EncodeToString([]byte(hdr))
		payload := base64.RawURLEncoding.EncodeToString([]byte(examplePayload))

		signingInput := strings.Join(
			[]string{
				hdrBuf,
				payload,
			},
			".",
		)

		err = v.Verify([]byte(signingInput), signatures[0].GetSignature(), key)
		if err != nil {
			return
		}
	})
	t.Run("ES512Compact", func(t *testing.T) {
		// ES256Compact tests that https://tools.ietf.org/html/rfc7515#appendix-A.3 works
		hdr := []byte{123, 34, 97, 108, 103, 34, 58, 34, 69, 83, 53, 49, 50, 34, 125}
		const jwkSrc = `{
"kty":"EC",
"crv":"P-521",
"x":"AekpBQ8ST8a8VcfVOTNl353vSrDCLLJXmPk06wTjxrrjcBpXp5EOnYG_NjFZ6OvLFV1jSfS9tsz4qUxcWceqwQGk",
"y":"ADSmRA43Z1DSNx_RvcLI87cdL07l6jQyyBXMoxVg_l2Th-x3S1WDhjDly79ajL4Kkd0AZMaZmh9ubmf63e3kyMj2",
"d":"AY5pb7A0UFiB3RELSD64fTLOSV_jazdF7fLYyuTw8lOfRhWg6Y6rUrPAxerEzgdRhajnu0ferB0d53vM9mE15j2C"
}`

		// "GetPayload"
		jwsPayload := []byte{80, 97, 121, 108, 111, 97, 100}

		standardHeaders := &jws.StandardHeaders{}
		err := json.Unmarshal(hdr, standardHeaders)
		if err != nil {
			t.Fatal("Failed to parse header")
		}
		alg := standardHeaders.GetAlgorithm()

		keys, err := jwk.ParseString(jwkSrc)
		if err != nil {
			t.Fatalf("Failed to parse JWK: %s", err.Error())
		}
		key, err := keys.Keys[0].Materialize()
		if err != nil {
			t.Fatal("Failed to create private key")
		}
		var jwsCompact []byte
		jwsCompact, err = jws.SignWithOption(jwsPayload, alg, key)
		if err != nil {
			t.Fatal("Failed to sign message")
		}

		// Verify with standard ecdsa library
		parts, err := jws.SplitCompact(string(jwsCompact[:]))
		if err != nil {
			t.Fatal("Failed to split compact JWT")
		}
		decodedJwsSignature, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil {
			t.Fatal("Failed to sign message")
		}
		r, s := &big.Int{}, &big.Int{}
		n := len(decodedJwsSignature) / 2
		r.SetBytes(decodedJwsSignature[:n])
		s.SetBytes(decodedJwsSignature[n:])
		signingHdr := base64.RawURLEncoding.EncodeToString(hdr)
		signingPayload := base64.RawURLEncoding.EncodeToString(jwsPayload)
		jwsSigningInput := strings.Join(
			[]string{
				signingHdr,
				signingPayload,
			}, ".",
		)
		hashed512 := sha512.Sum512([]byte(jwsSigningInput))
		ecdsaPrivateKey := key.(*ecdsa.PrivateKey)
		verified := ecdsa.Verify(&ecdsaPrivateKey.PublicKey, hashed512[:], r, s)
		if !verified {
			t.Fatal("Failed to verify message")
		}

		// Verify with API library

		publicKey, err := jwk.GetPublicKey(key)
		if err != nil {
			t.Fatal("Failed to get public from private key")
		}
		verifiedPayload, err := jws.Verify(jwsCompact, alg, publicKey)
		if err != nil || string(verifiedPayload) != string(jwsPayload) {
			t.Fatal("Failed to verify message")
		}
	})
	t.Run("RS256Compact", func(t *testing.T) {
		// RS256Compact tests that https://tools.ietf.org/html/rfc7515#appendix-A.2 works
		const hdr = `{"alg":"RS256"}`
		const expected = `eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ.cC4hiUPoj9Eetdgtv3hF80EGrhuB__dzERat0XF9g2VtQgr9PJbu3XOiZj5RZmh7AAuHIm4Bh-0Qc_lF5YKt_O8W2Fp5jujGbds9uJdbF9CUAr7t1dnZcAcQjbKBYNX4BAynRFdiuB--f_nZLgrnbyTyWzO75vRK5h6xBArLIARNPvkSjtQBMHlb1L07Qe7K0GarZRmB_eSN9383LcOLn6_dO--xi12jzDwusC-eOkHWEsqtFZESc6BfI7noOPqvhJ1phCnvWh6IeYI2w9QOYEUipUTI8np6LbgGY9Fs98rqVt5AXLIhWkWywlVmtVrBp0igcN_IoypGlUPQGe77Rw`
		const jwkSrc = `{
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

		var jwkKeySet *jwk.Set
		jwkKeySet, err := jwk.ParseString(jwkSrc)
		if err != nil {
			t.Fatalf("Failed to parse JWK: %s", err.Error())
		}
		signer, err := sign.New(jwa.RS256)
		if err != nil {
			t.Fatalf("Failed to create signer: %s", err.Error())
		}

		hdrStr := base64.RawURLEncoding.EncodeToString([]byte(hdr))
		payload := base64.RawURLEncoding.EncodeToString([]byte(examplePayload))

		signingInput := strings.Join(
			[]string{
				hdrStr,
				payload,
			}, ".",
		)
		privateKey, err := jwkKeySet.Keys[0].Materialize()
		if err != nil {
			t.Fatalf("Failed to materialize key: %s", err.Error())
		}
		signature, err := signer.Sign([]byte(signingInput), privateKey)
		if err != nil {
			t.Fatalf("Failed to sign message: %s", err.Error())
		}
		encSignature := base64.RawURLEncoding.EncodeToString(signature)

		encoded := strings.Join(
			[]string{
				signingInput,
				encSignature,
			}, ".",
		)

		if expected != encoded {
			t.Fatal("Mismatched compact serialization")
		}

		msg, err := jws.ParseString(encoded)
		if err != nil {
			t.Fatalf("Failed to parse JWS: %s", err.Error())
		}

		signatures := msg.GetSignatures()

		algorithm := signatures[0].ProtectedHeaders().GetAlgorithm()
		if algorithm != jwa.RS256 {
			t.Fatal("Algorithm in header does not match")
		}

		v, err := verify.New(jwa.RS256)
		if err != nil {
			t.Fatalf("Failed to create verifier: %s", err.Error())
		}
		publicKey, err := jwk.GetPublicKey(privateKey)
		if err != nil {
			t.Fatalf("Failed to get public key: %s", err.Error())
		}

		err = v.Verify([]byte(signingInput), signature, publicKey)
		if err != nil {
			t.Fatalf("Message verification failed: %s", err.Error())
		}
	})
	t.Run("ES256Compact", func(t *testing.T) {
		// ES256Compact tests that https://tools.ietf.org/html/rfc7515#appendix-A.3 works
		const hdr = `{"alg":"ES256"}`
		const jwkSrc = `{
    "kty":"EC",
    "crv":"P-256",
    "x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU",
    "y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0",
    "d":"jpsQnnGQmL-YBIffH1136cspYG6-0iY7X1fCE9-E9LI"
  }`

		var jwkKeySet *jwk.Set
		jwkKeySet, err := jwk.ParseString(jwkSrc)
		if err != nil {
			t.Fatalf("Failed to parse JWK: %s", err.Error())
		}

		signer, err := sign.New(jwa.ES256)
		if err != nil {
			t.Fatalf("Failed to create signer: %s", err.Error())
		}

		hdrStr := base64.RawURLEncoding.EncodeToString([]byte(hdr))
		payload := base64.RawURLEncoding.EncodeToString([]byte(examplePayload))

		signingInput := strings.Join(
			[]string{
				hdrStr,
				payload,
			}, ".",
		)

		privateKey, err := jwkKeySet.Keys[0].Materialize()
		if err != nil {
			t.Fatalf("Failed to materialize key: %s", err.Error())
		}
		signature, err := signer.Sign([]byte(signingInput), privateKey)
		if err != nil {
			t.Fatalf("Failed to sign message: %s", err.Error())
		}
		encSignature := base64.RawURLEncoding.EncodeToString(signature)

		encoded := strings.Join(
			[]string{
				signingInput,
				encSignature,
			}, ".",
		)

		// The Signature contains random factor, so unfortunately we can't match
		// the output against a fixed expected outcome. We'll wave doing an
		// exact match, and just try to verify using the Signature

		msg, err := jws.ParseString(encoded)
		if err != nil {
			t.Fatalf("Failed to parse JWS: %s", err.Error())
		}

		signatures := msg.GetSignatures()

		algorithm := signatures[0].ProtectedHeaders().GetAlgorithm()
		if algorithm != jwa.ES256 {
			t.Fatal("Algorithm in header does not match")
		}

		v, err := verify.New(jwa.ES256)
		if err != nil {
			t.Fatalf("Failed to create verifier: %s", err.Error())
		}
		publicKey, err := jwk.GetPublicKey(privateKey)
		if err != nil {
			t.Fatalf("Failed to get public key: %s", err.Error())
		}
		err = v.Verify([]byte(signingInput), signature, publicKey)
		if err != nil {
			t.Fatalf("Message verification failed: %s", err.Error())
		}
	})
}

func TestDecode_ES384Compact_NoSigTrim(t *testing.T) {
	incoming := "eyJhbGciOiJFUzM4NCIsInR5cCI6IkpXVCIsImtpZCI6IjE5MzFmZTQ0YmFhMWNhZTkyZWUzNzYzOTQ0MDU1OGMwODdlMTRlNjk5ZWU5NjVhM2Q1OGU1MmU2NGY4MDE0NWIifQ.eyJpc3MiOiJicmt0LWNsaS0xLjAuN3ByZTEiLCJpYXQiOjE0ODQ2OTU1MjAsImp0aSI6IjgxYjczY2Y3In0.DdFi0KmPHSv4PfIMGcWGMSRLmZsfRPQ3muLFW6Ly2HpiLFFQWZ0VEanyrFV263wjlp3udfedgw_vrBLz3XC8CkbvCo_xeHMzaTr_yfhjoheSj8gWRLwB-22rOnUX_M0A"
	const jwkSrc = `{
    "kty":"EC",
    "crv":"P-384",
    "x":"YHVZ4gc1RDoqxKm4NzaN_Y1r7R7h3RM3JMteC478apSKUiLVb4UNytqWaLoE6ygH",
    "y":"CRKSqP-aYTIsqJfg_wZEEYUayUR5JhZaS2m4NLk2t1DfXZgfApAJ2lBO0vWKnUMp"
  }`

	var jwkKeySet *jwk.Set
	jwkKeySet, err := jwk.ParseString(jwkSrc)
	if err != nil {
		t.Fatalf("Failed to parse JWK: %s", err.Error())
	}
	v, err := verify.New(jwa.ES384)
	if err != nil {
		t.Fatalf("Failed to create verifier: %s", err.Error())
	}

	parts, err := jws.SplitCompact(incoming)
	if err != nil {
		t.Fatalf("Failed to spli compact serialization: %s", err.Error())
	}

	decodedSignature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("Failed to decode signature: %s", err.Error())
	}
	publicKey, err := jwkKeySet.Keys[0].Materialize()
	if err != nil {
		t.Fatalf("Failed to materialize keys: %v", err)
	}
	signingInput := strings.Join(
		[]string{
			parts[0],
			parts[1],
		}, ".",
	)

	err = v.Verify([]byte(signingInput), decodedSignature, publicKey)
	if err != nil {
		t.Fatalf("Message verification failed: %s", err.Error())
	}
}

func TestSignErrors(t *testing.T) {

	t.Run("Bad algorithm", func(t *testing.T) {
		_, err := jws.SignWithOption([]byte(nil), jwa.SignatureAlgorithm("FooBar"), nil)
		if err == nil {
			t.Fatal("Unknown algorithm should return error")
		}
	})
	t.Run("No private key", func(t *testing.T) {
		_, err := jws.SignWithOption([]byte{'a', 'b', 'c'}, jwa.RS256, nil)
		if err == nil {
			t.Fatal("SignWithOption with no private key should return error")
		}
	})
	t.Run("RSA verify with no public key", func(t *testing.T) {
		_, err := jws.Verify([]byte(nil), jwa.RS256, nil)
		if err == nil {
			t.Fatal("Verify with no private key should return error")
		}
	})
	t.Run("Invalid signature algorithm", func(t *testing.T) {
		_, err := jws.SignLiteral([]byte("payload"), jwa.SignatureAlgorithm("dummy"), nil, []byte("header"), rand.Reader)
		if err == nil {
			t.Fatal("JWS signing should have failed")
		}
	})
	t.Run("Invalid signature algorithm", func(t *testing.T) {
		_, err := jws.SignLiteral([]byte("payload"), jwa.SignatureAlgorithm("dummy"), nil, []byte("header"), rand.Reader)
		if err == nil {
			t.Fatal("JWS signing should have failed")
		}
	})
}

func TestVerifyErrors(t *testing.T) {

	t.Run("Invalid compact serialization", func(t *testing.T) {
		message := []byte("some.message")
		_, err := jws.Verify(message, jwa.ES256, nil)
		if err == nil {
			t.Fatal("JWS verification should have failed")
		}
	})
	t.Run("Invalid signature encoding", func(t *testing.T) {
		message := []byte("some.message.c29tZSBtZXNz?Wdl")
		_, err := jws.Verify(message, jwa.ES256, nil)
		if err == nil {
			t.Fatal("JWS verification should have failed")
		}
	})
	t.Run("Invalid Key", func(t *testing.T) {
		rsaPublicKey := &jwk.RSAPublicKey{}
		_, err := jws.VerifyWithJWK([]byte("some.message.signed"), rsaPublicKey)
		if err == nil {
			t.Fatal("JWS verification should have failed")
		}
	})
}
