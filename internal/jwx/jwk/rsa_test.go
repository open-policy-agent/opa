package jwk_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
	"github.com/open-policy-agent/opa/internal/jwx/jwk"
)

func TestRSA(t *testing.T) {
	verify := func(t *testing.T, key jwk.Key) {
		t.Helper()

		rsaKey, err := key.Materialize()
		if err != nil {
			t.Fatalf("Materialize() failed: %s", err.Error())
		}

		newKey, err := jwk.New(rsaKey)
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

		if bytes.Compare(jsonBuf1, jsonBuf2) != 0 {
			t.Fatal("JSON marshal buffers do not match")
		}
	}
	t.Run("Public Key", func(t *testing.T) {
		const jwkSrc = `{
  "e": "AQAB",
  "kty": "RSA",
  "n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw"
}`

		var jwkKey jwk.Key

		// It might be a single key
		rawKeyJSON := &jwk.RawKeyJSON{}
		err := json.Unmarshal([]byte(jwkSrc), rawKeyJSON)
		if err != nil {
			t.Fatalf("Failed to unmarshal JWK: %s", err.Error())
		}
		jwkKey, err = rawKeyJSON.GenerateKey()
		if _, ok := jwkKey.(*jwk.RSAPublicKey); !ok {
			t.Fatalf("Key type should be of type: %s", fmt.Sprintf("%T", jwkKey))
		}
		rsaKey, err := jwkKey.Materialize()
		if err != nil {
			t.Fatal("Failed to materialize symmetric key")
		}
		if jwk.GetKeyTypeFromKey(rsaKey) != jwa.RSA {
			t.Fatal("Wrong Key Type")
		}

		verify(t, jwkKey)
	})
	t.Run("No Key Type Error", func(t *testing.T) {
		const jwkSrc = `{
  "e": "AQAB",
  "n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw"
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
	t.Run("Single Private Key Error", func(t *testing.T) {
		const jwkSrc = `{
  "e": "AQAB",
  "kty": "RSA",
  "n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
  "d": "X4cTteJY_gn4FYPsXB8rdXix5vwsg1FLN5E3EaG6RJoVH-HLLKD9M7dx5oo7GURknchnrRweUkC7hT5fJLM0WbFAKNLWY2vv7B6NqXSzUvxT0_YSfqijwp3RTzlBaCxWp4doFk5N2o8Gy_nHNKroADIkJ46pRUohsXywbReAdYaMwFs9tv8d_cPVY3i07a3t8MN6TNwm0dSawm9v47UiCl3Sk5ZiG7xojPLu4sbg1U2jx4IBTNBznbJSzFHK66jT8bgkuqsk0GjskDJk19Z4qwjwbsnn4j2WBii3RL-Us2lGVkY8fkFzme1z0HbIkfz0Y6mqnOYtqc0X4jfcKoAC8Q",
  "p": "83i-7IvMGXoMXCskv73TKr8637FiO7Z27zv8oj6pbWUQyLPQBQxtPVnwD20R-60eTDmD2ujnMt5PoqMrm8RfmNhVWDtjjMmCMjOpSXicFHj7XOuVIYQyqVWlWEh6dN36GVZYk93N8Bc9vY41xy8B9RzzOGVQzXvNEvn7O0nVbfs",
  "dp": "G4sPXkc6Ya9y8oJW9_ILj4xuppu0lzi_H7VTkS8xj5SdX3coE0oimYwxIi2emTAue0UOa5dpgFGyBJ4c8tQ2VF402XRugKDTP8akYhFo5tAA77Qe_NmtuYZc3C3m3I24G2GvR5sSDxUyAN2zq8Lfn9EUms6rY3Ob8YeiKkTiBj0",
  "dq": "s9lAH9fggBsoFR8Oac2R_E2gw282rT2kGOAhvIllETE1efrA6huUUvMfBcMpn8lqeW6vzznYY5SSQF7pMdC_agI3nG8Ibp1BUb0JUiraRNqUfLhcQb_d9GF4Dh7e74WbRsobRonujTYN1xCaP6TO61jvWrX-L18txXw494Q_cgk",
  "qi": "GyM_p6JrXySiz1toFgKbWV-JdI3jQ4ypu9rbMWx3rQJBfmt0FoYzgUIZEVFEcOqwemRN81zoDAaa-Bk0KWNGDjJHZDdDmFhW3AN7lI-puxk_mHZGJ11rxyR8O55XLSe3SPmRfKwZI6yU24ZxvQKFYItdldUKGzO6Ia6zTKhAVRU",
  "alg": "RS256",
  "kid": "2011-04-29"
}`
		// It might be a single key
		rawKeyJSON := &jwk.RawKeyJSON{}
		err := json.Unmarshal([]byte(jwkSrc), rawKeyJSON)
		if err != nil {
			t.Fatalf("Failed to unmarshal JWK: %s", err.Error())
		}
		_, err = rawKeyJSON.GenerateKey()
		if err == nil {
			t.Fatalf("Key generation should fail")
		}
	})

	t.Run("Single Private Key", func(t *testing.T) {
		const jwkSrc = `{
  "e": "AQAB",
  "kty": "RSA",
  "n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
  "d": "X4cTteJY_gn4FYPsXB8rdXix5vwsg1FLN5E3EaG6RJoVH-HLLKD9M7dx5oo7GURknchnrRweUkC7hT5fJLM0WbFAKNLWY2vv7B6NqXSzUvxT0_YSfqijwp3RTzlBaCxWp4doFk5N2o8Gy_nHNKroADIkJ46pRUohsXywbReAdYaMwFs9tv8d_cPVY3i07a3t8MN6TNwm0dSawm9v47UiCl3Sk5ZiG7xojPLu4sbg1U2jx4IBTNBznbJSzFHK66jT8bgkuqsk0GjskDJk19Z4qwjwbsnn4j2WBii3RL-Us2lGVkY8fkFzme1z0HbIkfz0Y6mqnOYtqc0X4jfcKoAC8Q",
  "p": "83i-7IvMGXoMXCskv73TKr8637FiO7Z27zv8oj6pbWUQyLPQBQxtPVnwD20R-60eTDmD2ujnMt5PoqMrm8RfmNhVWDtjjMmCMjOpSXicFHj7XOuVIYQyqVWlWEh6dN36GVZYk93N8Bc9vY41xy8B9RzzOGVQzXvNEvn7O0nVbfs",
  "q": "3dfOR9cuYq-0S-mkFLzgItgMEfFzB2q3hWehMuG0oCuqnb3vobLyumqjVZQO1dIrdwgTnCdpYzBcOfW5r370AFXjiWft_NGEiovonizhKpo9VVS78TzFgxkIdrecRezsZ-1kYd_s1qDbxtkDEgfAITAG9LUnADun4vIcb6yelxk",
  "dp": "G4sPXkc6Ya9y8oJW9_ILj4xuppu0lzi_H7VTkS8xj5SdX3coE0oimYwxIi2emTAue0UOa5dpgFGyBJ4c8tQ2VF402XRugKDTP8akYhFo5tAA77Qe_NmtuYZc3C3m3I24G2GvR5sSDxUyAN2zq8Lfn9EUms6rY3Ob8YeiKkTiBj0",
  "dq": "s9lAH9fggBsoFR8Oac2R_E2gw282rT2kGOAhvIllETE1efrA6huUUvMfBcMpn8lqeW6vzznYY5SSQF7pMdC_agI3nG8Ibp1BUb0JUiraRNqUfLhcQb_d9GF4Dh7e74WbRsobRonujTYN1xCaP6TO61jvWrX-L18txXw494Q_cgk",
  "qi": "GyM_p6JrXySiz1toFgKbWV-JdI3jQ4ypu9rbMWx3rQJBfmt0FoYzgUIZEVFEcOqwemRN81zoDAaa-Bk0KWNGDjJHZDdDmFhW3AN7lI-puxk_mHZGJ11rxyR8O55XLSe3SPmRfKwZI6yU24ZxvQKFYItdldUKGzO6Ia6zTKhAVRU",
  "alg": "RS256",
  "kid": "2011-04-29"
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
			if _, ok := jwkKey.(*jwk.RSAPrivateKey); !ok {
				t.Fatalf("Key type should be of type: %s", fmt.Sprintf("%T", jwkKey))
			}
			rsaKey, err := jwkKey.Materialize()
			if err != nil {
				t.Fatal("Failed to materialize symmetric key")
			}
			if jwk.GetKeyTypeFromKey(rsaKey) != jwa.RSA {
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
      "kty": "RSA",
      "alg": "RS256",
      "n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
      "e": "AQAB",
      "d": "X4cTteJY_gn4FYPsXB8rdXix5vwsg1FLN5E3EaG6RJoVH-HLLKD9M7dx5oo7GURknchnrRweUkC7hT5fJLM0WbFAKNLWY2vv7B6NqXSzUvxT0_YSfqijwp3RTzlBaCxWp4doFk5N2o8Gy_nHNKroADIkJ46pRUohsXywbReAdYaMwFs9tv8d_cPVY3i07a3t8MN6TNwm0dSawm9v47UiCl3Sk5ZiG7xojPLu4sbg1U2jx4IBTNBznbJSzFHK66jT8bgkuqsk0GjskDJk19Z4qwjwbsnn4j2WBii3RL-Us2lGVkY8fkFzme1z0HbIkfz0Y6mqnOYtqc0X4jfcKoAC8Q",
      "p": "83i-7IvMGXoMXCskv73TKr8637FiO7Z27zv8oj6pbWUQyLPQBQxtPVnwD20R-60eTDmD2ujnMt5PoqMrm8RfmNhVWDtjjMmCMjOpSXicFHj7XOuVIYQyqVWlWEh6dN36GVZYk93N8Bc9vY41xy8B9RzzOGVQzXvNEvn7O0nVbfs",
      "q": "3dfOR9cuYq-0S-mkFLzgItgMEfFzB2q3hWehMuG0oCuqnb3vobLyumqjVZQO1dIrdwgTnCdpYzBcOfW5r370AFXjiWft_NGEiovonizhKpo9VVS78TzFgxkIdrecRezsZ-1kYd_s1qDbxtkDEgfAITAG9LUnADun4vIcb6yelxk",
      "dp": "G4sPXkc6Ya9y8oJW9_ILj4xuppu0lzi_H7VTkS8xj5SdX3coE0oimYwxIi2emTAue0UOa5dpgFGyBJ4c8tQ2VF402XRugKDTP8akYhFo5tAA77Qe_NmtuYZc3C3m3I24G2GvR5sSDxUyAN2zq8Lfn9EUms6rY3Ob8YeiKkTiBj0",
      "dq": "s9lAH9fggBsoFR8Oac2R_E2gw282rT2kGOAhvIllETE1efrA6huUUvMfBcMpn8lqeW6vzznYY5SSQF7pMdC_agI3nG8Ibp1BUb0JUiraRNqUfLhcQb_d9GF4Dh7e74WbRsobRonujTYN1xCaP6TO61jvWrX-L18txXw494Q_cgk",
      "qi": "GyM_p6JrXySiz1toFgKbWV-JdI3jQ4ypu9rbMWx3rQJBfmt0FoYzgUIZEVFEcOqwemRN81zoDAaa-Bk0KWNGDjJHZDdDmFhW3AN7lI-puxk_mHZGJ11rxyR8O55XLSe3SPmRfKwZI6yU24ZxvQKFYItdldUKGzO6Ia6zTKhAVRU",
      "kid": "2011-04-29"
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
				t.Fatalf("Failed to generate key: %s", err.Error())
			}
			if _, ok := jwkKey.(*jwk.RSAPrivateKey); !ok {
				t.Fatalf("Key type should be of type: %s", fmt.Sprintf("%T", jwkKey))
			}
		}
		verify(t, jwkKey)
	})
}

func TestRSAMaterializeErrors(t *testing.T) {
	t.Run("No Standard Public Key", func(t *testing.T) {
		rsaPublicKey := &jwk.RSAPublicKey{}
		_, err := rsaPublicKey.Materialize()
		if err == nil {
			t.Fatal("Materialize key should fail")
		}
	})
	t.Run("No Standard Private Key", func(t *testing.T) {
		rsaPrivateKey := &jwk.RSAPrivateKey{}
		_, err := rsaPrivateKey.Materialize()
		if err == nil {
			t.Fatal("Materialize key should fail")
		}
	})
}

func TestRSAGenerateErrors(t *testing.T) {
	t.Run("Raw JWK missing D or E", func(t *testing.T) {
		rawKeyJSON := &jwk.RawKeyJSON{}
		rsaPublicKey := &jwk.RSAPublicKey{}
		err := rsaPublicKey.GenerateKey(rawKeyJSON)
		if err == nil {
			t.Fatal("Materialize key should fail")
		}
	})
	t.Run("Raw JWK missing D or E", func(t *testing.T) {
		rawKeyJSON := &jwk.RawKeyJSON{}
		rsaPrivateKey := &jwk.RSAPrivateKey{}
		err := rsaPrivateKey.GenerateKey(rawKeyJSON)
		if err == nil {
			t.Fatal("Materialize key should fail")
		}
	})
}
