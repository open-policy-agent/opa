package jwk_test

import (
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
	"github.com/open-policy-agent/opa/internal/jwx/jwk"
)

func TestHeader(t *testing.T) {

	privateHeaderParams := map[string]interface{}{"one": "1", "two": "11"}
	t.Run("RoundTrip", func(t *testing.T) {
		values := map[string]interface{}{
			jwk.KeyIDKey:         "helloworld01",
			jwk.KeyTypeKey:       jwa.RSA,
			jwk.KeyOpsKey:        jwk.KeyOperationList{jwk.KeyOpSign},
			jwk.KeyUsageKey:      "sig",
			jwk.PrivateParamsKey: privateHeaderParams,
		}

		var h jwk.StandardHeaders
		for k, v := range values {
			err := h.Set(k, v)
			if err != nil {
				t.Fatalf("failed to set value for: %s", k)
			}

			got, ok := h.Get(k)
			if !ok {
				t.Fatalf("failed to get value for: %s", k)
			}

			if !reflect.DeepEqual(v, got) {
				t.Fatalf("mismtached values for: %s", k)
			}

			err = h.Set(k, v)
			if err != nil {
				t.Fatalf("failed to set value for: %s", k)
			}
		}
	})
	t.Run("RoundTripError 1", func(t *testing.T) {

		type dummyStruct struct {
			dummy1 int
			dummy2 float64
		}
		dummy := &dummyStruct{1, 3.4}
		values := map[string]interface{}{
			jwk.AlgorithmKey:     dummy,
			jwk.KeyIDKey:         dummy,
			jwk.KeyTypeKey:       dummy,
			jwk.KeyUsageKey:      dummy,
			jwk.KeyOpsKey:        dummy,
			jwk.PrivateParamsKey: dummy,
			"invalid key":        "",
		}

		var h jwk.StandardHeaders
		for k, v := range values {
			err := h.Set(k, v)
			if err == nil {
				t.Fatalf("Setting %s value should have failed", k)
			}
		}
		if h.GetAlgorithm() != jwa.NoValue {
			t.Fatalf("Algorithm should be empty string")
		}
		if h.GetKeyID() != "" {
			t.Fatalf("KeyID should be empty string")
		}
		if h.GetKeyType() != "" {
			t.Fatalf("KeyType should be empty string")
		}
		if h.GetKeyUsage() != "" {
			t.Fatalf("KeyUsage should be empty string")
		}
		if h.GetKeyOps() != nil {
			t.Fatalf("KeyOps should be empty string")
		}
		if h.GetPrivateParams() != nil {
			t.Fatalf("Private params should be empty string")
		}
	})
	t.Run("RoundTripError 2", func(t *testing.T) {

		type dummyStruct struct {
			dummy1 int
			dummy2 float64
		}
		dummy := &dummyStruct{1, 3.4}
		values := map[string]interface{}{
			jwk.AlgorithmKey:     jwa.SignatureAlgorithm("dummy"),
			jwk.KeyIDKey:         1,
			jwk.KeyTypeKey:       jwa.KeyType("dummy"),
			jwk.KeyUsageKey:      dummy,
			jwk.KeyOpsKey:        []string{"unknown", "usage"},
			jwk.PrivateParamsKey: dummy,
			"invalid key":        "",
		}

		var h jwk.StandardHeaders
		for k, v := range values {
			err := h.Set(k, v)
			if err == nil {
				t.Fatalf("Setting %s value should have failed", k)
			}
		}
		if h.GetAlgorithm() != jwa.NoValue {
			t.Fatalf("Algorithm should be empty string")
		}
		if h.GetKeyID() != "" {
			t.Fatalf("KeyID should be empty string")
		}
		if h.GetKeyType() != "" {
			t.Fatalf("KeyType should be empty string")
		}
		if h.GetKeyUsage() != "" {
			t.Fatalf("KeyUsage should be empty string")
		}
		if h.GetKeyOps() != nil {
			t.Fatalf("KeyOps should be empty string")
		}
		if h.GetPrivateParams() != nil {
			t.Fatalf("Private params should be empty string")
		}
	})

	t.Run("Algorithm", func(t *testing.T) {
		var h jwk.StandardHeaders
		for _, value := range []interface{}{jwa.RS256, jwa.ES256} {
			err := h.Set("alg", value)
			if err != nil {
				t.Fatalf("Failed to set algorithm value: %s", err.Error())
			}
			got, ok := h.Get("alg")
			if !ok {
				t.Fatal("Failed to get algorithm")
			}
			if value != got {
				t.Fatalf("Algorithm values do not match %s:%s", value, got)
			}
		}
	})
	t.Run("KeyType", func(t *testing.T) {
		var h jwk.StandardHeaders
		for _, value := range []interface{}{jwa.RSA, "RSA"} {
			err := h.Set(jwk.KeyTypeKey, value)
			if err != nil {
				t.Fatalf("failed to set key type: %s", err.Error())
			}

			got, ok := h.Get(jwk.KeyTypeKey)
			if !ok {
				t.Fatal("failed to get key type")
			}

			var s string
			switch v := value.(type) {
			case jwa.KeyType:
				s = v.String()
			case string:
				s = v
			}

			if got != jwa.KeyType(s) {
				t.Fatal("expected and realized key types do not match")
			}
		}
	})
}
