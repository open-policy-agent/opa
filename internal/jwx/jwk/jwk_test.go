package jwk_test

import (
	"testing"

	"github.com/open-policy-agent/opa/internal/jwx/jwk"
)

func TestNew(t *testing.T) {
	k, err := jwk.New(nil)
	if k != nil {
		t.Fatalf("key should be nil: %s", err.Error())
	}
	if err == nil {
		t.Fatal("nil key should cause an error")
	}
}

func TestGetPublicKeyErrors(t *testing.T) {

	t.Run("Key is nil", func(t *testing.T) {
		_, err := jwk.GetPublicKey(nil)
		if err == nil {
			t.Fatal("GetPublicKey should have failed")
		}
	})
	t.Run("Key type is invalid", func(t *testing.T) {
		_, err := jwk.GetPublicKey("dummy")
		if err == nil {
			t.Fatal("GetPublicKey should have failed")
		}
	})
}

func TestGetPublicKey(t *testing.T) {

	t.Run("Symmetric Key", func(t *testing.T) {
		_, err := jwk.GetPublicKey([]byte("GawgguFyGrWKav7AX4VKUg"))
		if err != nil {
			t.Fatalf("GetPublicKey failed: %s", err.Error())
		}
	})
}

func TestJwkNewErrors(t *testing.T) {

	t.Run("Key type is invalid", func(t *testing.T) {
		_, err := jwk.New("dummy")
		if err == nil {
			t.Fatal("JwkNew should have failed")
		}
	})
}

func TestParseErrors(t *testing.T) {

	t.Run("Invalid JSON", func(t *testing.T) {
		var jwkSrc = []byte(`{
  "keys" [
    {
      "kty": "EC",
      "crv": "P-256",
      "x": "MKBCTNIcKUSDii11ySs3526iDZ8AiTo7Tu6KPAqv7D4",
      "y": "4Etl6SRW2YiLUrN5vfvVHuhp7x8PxltmWWlbbM4IFyM",
      "use": "enc",
      "kid": "1"
    },
    {
      "kty": "RSA",
      "n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
      "e": "AQAB",
      "alg": "RS256",
      "kid": "2011-04-29"
    }
  ]
}`)

		_, err := jwk.ParseBytes(jwkSrc)
		if err == nil {
			t.Fatalf("JWK Parsing should have failed")
		}
	})
	t.Run("Invalid JSON Key Set", func(t *testing.T) {
		var jwkSrc = []byte(`{
  "keys" :[
    {
      "kty": "EC",
      "crv": "P-256",
      "y": "4Etl6SRW2YiLUrN5vfvVHuhp7x8PxltmWWlbbM4IFyM",
      "use": "enc",
      "kid": "1"
    },
    {
      "kty": "RSA",
      "n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
      "e": "AQAB",
      "alg": "RS256",
      "kid": "2011-04-29"
    }
  ]
}`)

		_, err := jwk.ParseBytes(jwkSrc)
		if err == nil {
			t.Fatalf("JWK Parsing should have failed")
		}
	})

	t.Run("Invalid JWK JSON", func(t *testing.T) {
		var jwkSrc = []byte(`{
      "kty": "EC",
      "crv": "P-256",
      "y": "4Etl6SRW2YiLUrN5vfvVHuhp7x8PxltmWWlbbM4IFyM",
      "use": "enc",
      "kid": "1"
}`)

		_, err := jwk.ParseBytes(jwkSrc)
		if err == nil {
			t.Fatalf("JWK Parsing should have failed")
		}
	})
	t.Run("Invalid Key Type", func(t *testing.T) {
		const jwkSrc = `{
  "e": "AQAB",
  "kty": "invalid",
  "n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw"
}`

		_, err := jwk.ParseString(jwkSrc)
		if err == nil {
			t.Fatal("JWK parse should have failed")
		}
	})
	t.Run("Invalid Key Ops", func(t *testing.T) {
		const jwkSrc = `{
  "keys": [
    {
      "kty": "EC",
      "crv": "P-256",
      "key_ops": [
        "invalid",
        "sign"
      ],
      "x": "MKBCTNIcKUSDii11ySs3526iDZ8AiTo7Tu6KPAqv7D4",
      "y": "4Etl6SRW2YiLUrN5vfvVHuhp7x8PxltmWWlbbM4IFyM",
      "d": "870MB6gfuTJ4HtUnUvYMyJpr5eUZNP4Bk43bVdj3eAE"
    }
  ]
}`

		_, err := jwk.ParseString(jwkSrc)
		if err == nil {
			t.Fatal("JWK parse should have failed")
		}
	})
}

func TestAppendix(t *testing.T) {

	t.Run("A1", func(t *testing.T) {
		var jwkSrc = []byte(`{
  "keys": [
    {
      "kty": "EC",
      "crv": "P-256",
      "x": "MKBCTNIcKUSDii11ySs3526iDZ8AiTo7Tu6KPAqv7D4",
      "y": "4Etl6SRW2YiLUrN5vfvVHuhp7x8PxltmWWlbbM4IFyM",
      "use": "enc",
      "kid": "1"
    },
    {
      "kty": "RSA",
      "n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
      "e": "AQAB",
      "alg": "RS256",
      "kid": "2011-04-29"
    }
  ]
}`)

		var jwkKeySet *jwk.Set
		jwkKeySet, err := jwk.ParseBytes(jwkSrc)
		if err != nil {
			t.Fatalf("Failed to parse JWK Set: %s", err.Error())
		}
		if len(jwkKeySet.Keys) != 2 {
			t.Fatalf("Failed to parse JWK Set: %s", err.Error())
		}
	})
}
