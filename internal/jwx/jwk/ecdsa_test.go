package jwk_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/internal/jwx/buffer"
	"github.com/open-policy-agent/opa/internal/jwx/jwa"
	"github.com/open-policy-agent/opa/internal/jwx/jwk"
)

func TestECDSA(t *testing.T) {

	t.Run("Key Generation Errors", func(t *testing.T) {
		jwkSrc := `{
  "keys": [
    {
      "kty": "EC",
      "crv": "P-256",
      "key_ops": [
        "verify",
        "sign"
      ],
      "x": "MKBCTNIcKUSDii11ySs3526iDZ8AiTo7Tu6KPAqv7D4",
      "y": "4Etl6SRW2YiLUrN5vfvVHuhp7x8PxltmWWlbbM4IFyM",
      "d": "870MB6gfuTJ4HtUnUvYMyJpr5eUZNP4Bk43bVdj3eAE"
    }
  ]
}`

		rawKeySetJSON := &jwk.RawKeySetJSON{}
		err := json.Unmarshal([]byte(jwkSrc), rawKeySetJSON)
		if err != nil {
			t.Fatalf("Failed to unmarshal JWK Set: %s", err.Error())
		}
		if len(rawKeySetJSON.Keys) != 1 {
			t.Fatalf("Failed to parse JWK Set: %v", err)
		}
		rawKeyJSON := rawKeySetJSON.Keys[0]
		curveName := rawKeyJSON.Crv
		if curveName != "P-256" {
			t.Fatalf("Curve name should be P-256, not: %s ", curveName)
		}
		rawKeyJSON.Crv = jwa.EllipticCurveAlgorithm("dummy")
		_, err = rawKeyJSON.GenerateKey()
		if err == nil {
			t.Fatal("Key generation should fail")
		}
		rawKeyJSON.Crv = jwa.P256
		rawKeyJSON.D = buffer.Buffer("1234")
		_, err = rawKeyJSON.GenerateKey()
		if err == nil {
			t.Fatal("Key generation should fail")
		}
	})
	t.Run("Parse Private Key", func(t *testing.T) {
		jwkSrc := `{
  "keys": [
    {
      "kty": "EC",
      "crv": "P-256",
      "key_ops": [
        "verify"
      ],
      "x": "MKBCTNIcKUSDii11ySs3526iDZ8AiTo7Tu6KPAqv7D4",
      "y": "4Etl6SRW2YiLUrN5vfvVHuhp7x8PxltmWWlbbM4IFyM",
      "d": "870MB6gfuTJ4HtUnUvYMyJpr5eUZNP4Bk43bVdj3eAE"
    }
  ]
}`

		var jwkSet *jwk.Set
		jwkSet, err := jwk.ParseString(jwkSrc)
		if err != nil {
			t.Fatalf("Failed to parse key: %s", err.Error())
		}
		jwkKey := jwkSet.Keys[0]
		privateKey, err := jwkKey.Materialize()
		if err != nil {
			t.Fatalf("Failed to expose private key: %s", err.Error())
		}
		if jwk.GetKeyTypeFromKey(privateKey) != jwa.EC {
			t.Fatal("Wrong Key Type")
		}
		if _, ok := privateKey.(*ecdsa.PrivateKey); !ok {
			t.Fatalf("Key type should be of type: %s", fmt.Sprintf("%T", privateKey))
		}
		publicKey, err := jwk.GetPublicKey(privateKey)
		if err != nil {
			t.Fatalf("Failed to expose public key: %s", err.Error())
		}
		if _, ok := publicKey.(*ecdsa.PublicKey); !ok {
			t.Fatalf("Key type should be of type: %s", fmt.Sprintf("%T", privateKey))
		}
	})
	t.Run("Initialization", func(t *testing.T) {
		// Generate an ECDSA P-256 test key.
		ecPrk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal("failed to generate EC P-256 key")
		}
		// Test initialization of a private EC JWK.
		prk, err := jwk.New(ecPrk)
		if err != nil {
			t.Fatal("failed to create new private key")
		}
		err = prk.Set(jwk.KeyIDKey, "MyKey")
		if err != nil {
			t.Fatalf("Faild to set KeyID: %s", err.Error())
		}
		if prk.GetKeyID() != "MyKey" {
			t.Fatalf("KeyID should be MyKey, not: %s", prk.GetKeyID())
		}

		if prk.GetKeyType() != jwa.EC {
			t.Fatalf("Key type should be %s, not: %s", jwa.EC, prk.GetKeyType())
		}

		// Test initialization of a public EC JWK.
		puk, err := jwk.New(&ecPrk.PublicKey)
		if err != nil {
			t.Fatal("failed to create new public key")
		}

		err = puk.Set(jwk.KeyIDKey, "MyKey")
		if err != nil {
			t.Fatalf("Faild to set KeyID: %s", err.Error())
		}
		if puk.GetKeyID() != "MyKey" {
			t.Fatalf("KeyID should be MyKey, not: %s", puk.GetKeyID())
		}

		if puk.GetKeyType() != jwa.EC {
			t.Fatalf("Key type should be %s, not: %s", jwa.EC, puk.GetKeyType())
		}
	})
	t.Run("Marshall Unmarshal Public Key", func(t *testing.T) {
		jwkSrc := `{
  "keys": [
    {
      "kty": "EC",
      "crv": "P-256",
      "key_ops": [
        "verify"
      ],
      "x": "MKBCTNIcKUSDii11ySs3526iDZ8AiTo7Tu6KPAqv7D4",
      "y": "4Etl6SRW2YiLUrN5vfvVHuhp7x8PxltmWWlbbM4IFyM",
      "d": "870MB6gfuTJ4HtUnUvYMyJpr5eUZNP4Bk43bVdj3eAE"
    }
  ]
}`

		var jwkSet *jwk.Set
		var jwkKey jwk.Key
		jwkSet, err := jwk.ParseString(jwkSrc)
		if err != nil {
			t.Fatalf("Failed to parse key: %s", err.Error())
		}
		jwkKey = jwkSet.Keys[0]
		privateKey, err := jwkKey.Materialize()
		if err != nil {
			t.Fatalf("Failed to expose private key: %s", err.Error())
		}
		if _, ok := privateKey.(*ecdsa.PrivateKey); !ok {
			t.Fatalf("Key type should be of type: %s", fmt.Sprintf("%T", privateKey))
		}
		publicKey, err := jwk.GetPublicKey(privateKey)
		if err != nil {
			t.Fatalf("Failed to expose public key: %s", err.Error())
		}
		if jwk.GetKeyTypeFromKey(publicKey) != jwa.EC {
			t.Fatal("Wrong Key Type")
		}
		if jwk.GetKeyTypeFromKey(nil) != jwa.InvalidKeyType {
			t.Fatal("Key should be invalid")
		}
		if _, ok := publicKey.(*ecdsa.PublicKey); !ok {
			t.Fatalf("Key type should be of type: %s", fmt.Sprintf("%T", privateKey))
		}
		eCDSAPublicKey, err := jwk.New(publicKey)
		if err != nil {
			t.Fatal("Failed to create ECDSAPublicKey")
		}
		newPublicKey, err := eCDSAPublicKey.Materialize()
		if err != nil {
			t.Fatalf("Failed to materialize key: %s", err.Error())
		}
		if !reflect.DeepEqual(publicKey, newPublicKey) {
			t.Fatal("ECDSA Public Keys do not match")
		}
	})
	t.Run("Unmarshal Private Key", func(t *testing.T) {
		jwkSrc := `{
  "keys": [
    {
      "kty": "EC",
      "d": "870MB6gfuTJ4HtUnUvYMyJpr5eUZNP4Bk43bVdj3eAE",
      "crv": "P-256",
      "key_ops": [
        "verify"
      ],
      "x": "MKBCTNIcKUSDii11ySs3526iDZ8AiTo7Tu6KPAqv7D4",
      "y": "4Etl6SRW2YiLUrN5vfvVHuhp7x8PxltmWWlbbM4IFyM"
    }
  ]
}`

		var jwkSet *jwk.Set
		var jwkKey jwk.Key
		jwkSet, err := jwk.ParseString(jwkSrc)
		if err != nil {
			t.Fatalf("Failed to parse key: %s", err.Error())
		}
		jwkKey = jwkSet.Keys[0]
		privateKey, err := jwkKey.Materialize()
		if err != nil {
			t.Fatalf("Failed to expose private key: %s", err.Error())
		}
		if _, ok := privateKey.(*ecdsa.PrivateKey); !ok {
			t.Fatalf("Key type should be of type: %s", fmt.Sprintf("%T", privateKey))
		}
		if _, ok := jwkKey.(*jwk.ECDSAPrivateKey); !ok {
			t.Fatalf("Key type should be of type: %s", fmt.Sprintf("%T", jwkKey))
		}
	})
	t.Run("Invalid ECDSA Private Key", func(t *testing.T) {
		const jwkSrc = `{
  "kty": "EC",
  "crv": "P-256",
  "y": "lf0u0pMj4lGAzZix5u4Cm5CMQIgMNpkwy163wtKYVKI",
  "d": "0g5vAEKzugrXaRbgKG0Tj2qJ5lMP4Bezds1_sTybkfk"
}`
		rawKeyJSON := &jwk.RawKeyJSON{}
		err := json.Unmarshal([]byte(jwkSrc), rawKeyJSON)
		if err != nil {
			t.Fatalf("Failed to unmarshal JWK Set: %s", err.Error())
		}
		_, err = rawKeyJSON.GenerateKey()
		if err == nil {
			t.Fatalf("Key Generation should fail")
		}
	})
}
