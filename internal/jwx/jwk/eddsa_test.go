package jwk_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
	"github.com/open-policy-agent/opa/internal/jwx/jwk"
)

func TestEdDSA(t *testing.T) {
	verify := func(t *testing.T, key jwk.Key) {
		t.Helper()

		eddsaKey, err := key.Materialize()
		if err != nil {
			t.Fatalf("Materialize() failed: %s", err.Error())
		}

		newKey, err := jwk.New(eddsaKey)
		if err != nil {
			t.Fatalf("jwk.New failed: %s", err.Error())
		}

		err = key.Walk(func(k string, v interface{}) error {
			return newKey.Set(k, v)
		})
		if err != nil {
			t.Fatalf("Failed to walk key: %s", err.Error())
		}

		jsonBuf1, err := json.Marshal(key)
		if err != nil {
			t.Fatalf("JSON marshal failed: %s", err.Error())
		}

		jsonBuf2, err := json.Marshal(newKey)
		if err != nil {
			t.Fatalf("JSON marshal failed: %s", err.Error())
		}

		if !bytes.Equal(jsonBuf1, jsonBuf2) {
			t.Fatal("JSON marshal buffers do not match")
		}
	}
	t.Run("Public Key", func(t *testing.T) {
		const jwkSrc = `{
            "kty": "OKP",
            "alg": "EdDSA",
            "crv": "Ed25519",
            "x": "MCowBQYDK2VwAyEAsD8QauV-Cgr7kPoZ3MVDDYzov7d8p8LjKOLXI3ni2ew",
            "use": "sig"
        }`

		var jwkKey jwk.Key

		// It might be a single key
		rawKeyJSON := &jwk.RawKeyJSON{}
		err := json.Unmarshal([]byte(jwkSrc), rawKeyJSON)
		if err != nil {
			t.Fatalf("Failed to unmarshal JWK: %s", err.Error())
		}
		jwkKey, err = rawKeyJSON.GenerateKey()
		if err != nil {
			t.Fatalf("Failed to generate key: %v", err)
		}
		if _, ok := jwkKey.(*jwk.EdDSAPublicKey); !ok {
			t.Fatalf("Key type should be of type: %s", fmt.Sprintf("%T", jwkKey))
		}
		eddsaKey, err := jwkKey.Materialize()
		if err != nil {
			t.Fatalf("Failed to materialize symmetric key: %v", err)
		}
		if jwk.GetKeyTypeFromKey(eddsaKey) != jwa.OctetKeyPair {
			t.Fatal("Wrong Key Type")
		}

		verify(t, jwkKey)
	})
	t.Run("No Key Type Error", func(t *testing.T) {
		const jwkSrc = `{
            "alg": "EdDSA",
            "crv": "Ed25519",
            "x": "MCowBQYDK2VwAyEAsD8QauV-Cgr7kPoZ3MVDDYzov7d8p8LjKOLXI3ni2ew",
            "use": "sig"
        }`

		// It might be a single key
		rawKeyJSON := &jwk.RawKeyJSON{}
		err := json.Unmarshal([]byte(jwkSrc), rawKeyJSON)
		if err != nil {
			t.Fatalf("Failed to unmarshal JWK: %s", err.Error())
		}
		_, err = rawKeyJSON.GenerateKey()
		if err == nil {
			t.Fatal("Key generation should have failed")
		}

	})
	t.Run("Single Private Key", func(t *testing.T) {
		const jwkSrc = `{
            "kty": "OKP",
            "alg": "EdDSA",
            "crv": "Ed25519",
            "d": "MC4CAQAwBQYDK2VwBCIEIEFrKpchjkEL8RoDUfE40sCNvFrnaYvODrLa0eUI0V9-",
            "use": "sig"
        }`
		var jwkKey jwk.Key
		rawKeySetJSON := &jwk.RawKeySetJSON{}
		err := json.Unmarshal([]byte(jwkSrc), rawKeySetJSON)
		if err != nil {
			t.Fatalf("Failed to unmarshal JWK Set: %s", err.Error())
		}
		if len(rawKeySetJSON.Keys) == 0 {
			// It might be a single key
			rawKeyJSON := &jwk.RawKeyJSON{}
			err := json.Unmarshal([]byte(jwkSrc), rawKeyJSON)
			if err != nil {
				t.Fatalf("Failed to unmarshal JWK: %s", err.Error())
			}
			jwkKey, err = rawKeyJSON.GenerateKey()
			if err != nil {
				t.Fatalf("Failed to generate key: %v", err)
			}
			if _, ok := jwkKey.(*jwk.EdDSAPrivateKey); !ok {
				t.Fatalf("Key type should be of type: %s", fmt.Sprintf("%T", jwkKey))
			}
			eddsaKey, err := jwkKey.Materialize()
			if err != nil {
				t.Fatal("Failed to materialize symmetric key")
			}
			if jwk.GetKeyTypeFromKey(eddsaKey) != jwa.OctetKeyPair {
				t.Fatal("Wrong Key Type")
			}
		} else {
			t.Fatal("Incorrect number of keys")
		}
		verify(t, jwkKey)
	})
	t.Run("JWK Set of one Private Key", func(t *testing.T) {
		jwkSrc := `{
  "keys": [
    {
            "kty": "OKP",
            "alg": "EdDSA",
            "crv": "Ed25519",
            "d": "MC4CAQAwBQYDK2VwBCIEIEFrKpchjkEL8RoDUfE40sCNvFrnaYvODrLa0eUI0V9-",
            "use": "sig"
        }
  ]
}`
		var jwkKey jwk.Key
		rawKeySetJSON := &jwk.RawKeySetJSON{}
		err := json.Unmarshal([]byte(jwkSrc), rawKeySetJSON)
		if err != nil {
			t.Fatalf("Failed to unmarshal JWK Set: %s", err.Error())
		}
		if len(rawKeySetJSON.Keys) == 0 {
			t.Fatal("Incorrect number of keys")
		} else {
			rawKeyJSON := rawKeySetJSON.Keys[0]
			jwkKey, err = rawKeyJSON.GenerateKey()
			if err != nil {
				t.Fatalf("Failed to generate key: %v", err)
			}
			if _, ok := jwkKey.(*jwk.EdDSAPrivateKey); !ok {
				t.Fatalf("Key type should be of type: %s", fmt.Sprintf("%T", jwkKey))
			}
		}
		verify(t, jwkKey)
	})
}

func TestEdDSAMaterializeErrors(t *testing.T) {
	t.Run("No Standard Public Key", func(t *testing.T) {
		eddsaPublicKey := &jwk.EdDSAPublicKey{}
		_, err := eddsaPublicKey.Materialize()
		if err == nil {
			t.Fatal("Materialize key should fail")
		}
	})
	t.Run("No Standard Private Key", func(t *testing.T) {
		eddsaPublicKey := &jwk.EdDSAPublicKey{}
		_, err := eddsaPublicKey.Materialize()
		if err == nil {
			t.Fatal("Materialize key should fail")
		}
	})
}

func TestEdDSAGenerateErrors(t *testing.T) {
	t.Run("Raw JWK missing X", func(t *testing.T) {
		rawKeyJSON := &jwk.RawKeyJSON{}
		eddsaPublicKey := &jwk.EdDSAPublicKey{}
		err := eddsaPublicKey.GenerateKey(rawKeyJSON)
		if err == nil {
			t.Fatal("Materialize key should fail")
		}
	})
	t.Run("Raw JWK missing D", func(t *testing.T) {
		rawKeyJSON := &jwk.RawKeyJSON{}
		eddsaPublicKey := &jwk.EdDSAPrivateKey{}
		err := eddsaPublicKey.GenerateKey(rawKeyJSON)
		if err == nil {
			t.Fatal("Materialize key should fail")
		}
	})
}
