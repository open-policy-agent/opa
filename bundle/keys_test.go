// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"crypto/rsa"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/util/test"
)

func TestValidateAndInjectDefaultsVerificationConfig(t *testing.T) {

	tests := map[string]struct {
		publicKeys map[string]*KeyConfig
		vc         *VerificationConfig
		wantErr    bool
		err        error
	}{
		"valid_config_no_key": {
			map[string]*KeyConfig{},
			NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "", nil),
			false, nil,
		},
		"valid_config_with_key": {
			map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}},
			NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "foo", "", nil),
			false, nil,
		},
		"valid_config_with_key_not_found": {
			map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}},
			NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "bar", "", nil),
			true, fmt.Errorf("key id bar not found"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			err := tc.vc.ValidateAndInjectDefaults(tc.publicKeys)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}

			if !reflect.DeepEqual(tc.vc.PublicKeys, tc.publicKeys) {
				t.Fatalf("Expected public keys %v but got %v", tc.publicKeys, tc.vc.PublicKeys)
			}
		})
	}
}

func TestGetPublicKey(t *testing.T) {
	tests := map[string]struct {
		input   string
		vc      *VerificationConfig
		kc      *KeyConfig
		wantErr bool
		err     error
	}{
		"key_found": {
			"foo",
			NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "", nil),
			&KeyConfig{Key: "secret", Algorithm: "HS256"},
			false, nil,
		},
		"key_not_found": {
			"foo",
			NewVerificationConfig(map[string]*KeyConfig{}, "", "", nil),
			nil,
			true, fmt.Errorf("verification key corresponding to ID foo not found"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			kc, err := tc.vc.GetPublicKey(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}

			if !reflect.DeepEqual(kc, tc.kc) {
				t.Fatalf("Expected key config %v but got %v", tc.kc, kc)
			}
		})
	}
}

func TestParseKeysConfig(t *testing.T) {

	key := `-----BEGIN PUBLIC KEY----- MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA7nJwME0QNM6g0Ou9Sylj lcIY4cnBcs8oWVHe74bJ7JTgYmDOk2CA14RE3wJNkUKERP/cRdesKDA/BToJXJUr oYvhjXxUYn+i3wK5vOGRY9WUtTF9paIIpIV4USUOwDh3ufhA9K3tyh+ZVsqn80em 0Lj2ME0EgScuk6u0/UYjjNvcmnQl+uDmghG8xBZh7TZW2+aceMwlb4LJIP36VRhg jKQGIxg2rW8ROXgJaFbNRCbiOUUqlq9SUZuhHo8TNOARXXxp9R4Fq7Cl7ZbwWtNP wAtM1y+Z+iyu/i91m0YLlU2XBOGLu9IA8IZjPlbCnk/SygpV9NNwTY9DSQ0QfXcP TGlsbFwzRzTlhH25wEl3j+2Ub9w/NX7Yo+j/Ei9eGZ8cq0bcvEwDeIo98HeNZWrL UUArayRYvh8zutOlzqehw8waFk9AxpfEp9oWekSz8gZw9OL773EhnglYxxjkPHNz k66CufLuTEf6uE9NLE5HnlQMbiqBFirIyAWGKyU3v2tphKvcogxmzzWA51p0GY0l GZvlLNt2NrJv2oGecyl3BLqHnBi+rGAosa/8XgfQT8RIk7YR/tDPDmPfaqSIc0po +NcHYEH82Yv+gfKSK++1fyssGCsSRJs8PFMuPGgv62fFrE/EHSsHJaNWojSYce/T rxm2RaHhw/8O4oKcfrbaRf8CAwEAAQ== -----END PUBLIC KEY-----`

	config := fmt.Sprintf(`{"foo": {"algorithm": "HS256", "key": "FdFYFzERwC2uCBB46pZQi4GG85LujR8obt-KWRBICVQ"},
			  				"bar": {"key": %v}
				}`, key)

	tests := map[string]struct {
		input   string
		result  map[string]*KeyConfig
		wantErr bool
		err     error
	}{
		"valid_config_one_key": {
			`{"foo": {"algorithm": "HS256", "key": "FdFYFzERwC2uCBB46pZQi4GG85LujR8obt-KWRBICVQ"}}`,
			map[string]*KeyConfig{"foo": {Key: "FdFYFzERwC2uCBB46pZQi4GG85LujR8obt-KWRBICVQ", Algorithm: "HS256"}},
			false, nil,
		},
		"valid_config_two_key": {
			config,
			map[string]*KeyConfig{
				"foo": {Key: "FdFYFzERwC2uCBB46pZQi4GG85LujR8obt-KWRBICVQ", Algorithm: "HS256"},
				"bar": {Key: key, Algorithm: "RS256"},
			},
			false, nil,
		},
		"invalid_config_no_key": {
			`{"foo": {"algorithm": "HS256"}}`,
			nil,
			true, fmt.Errorf("invalid keys configuration: verification key empty for key ID foo"),
		},
		"valid_config_default_alg": {
			`{"foo": {"key": "FdFYFzERwC2uCBB46pZQi4GG85LujR8obt-KWRBICVQ"}}`,
			map[string]*KeyConfig{"foo": {Key: "FdFYFzERwC2uCBB46pZQi4GG85LujR8obt-KWRBICVQ", Algorithm: "RS256"}},
			false, nil,
		},
		"invalid_raw_key_config": {
			`{"bar": [1,2,3]}`,
			nil,
			true, fmt.Errorf("json: cannot unmarshal array into Go value of type bundle.KeyConfig"),
		},
		"invalid_raw_config": {
			`[1,2,3]`,
			nil,
			true, fmt.Errorf("json: cannot unmarshal array into Go value of type map[string]json.RawMessage"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			kc, err := ParseKeysConfig([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}

			if !reflect.DeepEqual(kc, tc.result) {
				t.Fatalf("Expected key config %v but got %v", tc.result, kc)
			}
		})
	}
}

func TestGetPrivateKey(t *testing.T) {
	privateKey := `-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`

	files := map[string]string{
		"private.pem": privateKey,
	}

	test.WithTempFS(files, func(rootDir string) {

		sc := NewSigningConfig(filepath.Join(rootDir, "private.pem"), "", "")

		result, err := sc.GetPrivateKey()
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		_, ok := result.(*rsa.PrivateKey)
		if !ok {
			t.Fatalf("Expected key type *rsa.PrivateKey but got %T", result)
		}

		// key file does not exist, check that error generated with RS56 as the signing algorithm
		sc = NewSigningConfig("private.pem", "", "")
		_, err = sc.GetPrivateKey()
		if err == nil {
			t.Fatal("Expected error but got nil")
		}

		errMsg := "failed to parse PEM block containing the key"
		if err.Error() != errMsg {
			t.Fatalf("Expected error message %v but got %v", errMsg, err.Error())
		}

		// secret provided on command-line
		sc = NewSigningConfig("mysecret", "HS256", "")
		result, err = sc.GetPrivateKey()
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		hmackey, ok := result.([]byte)
		if !ok {
			t.Fatalf("Expected key type []byte but got %T", result)
		}

		if string(hmackey) != "mysecret" {
			t.Fatalf("Expected HMAC key %v but got %v", "mysecret", string(hmackey))
		}
	})
}

func TestGetClaimsErrors(t *testing.T) {
	files := map[string]string{
		"claims.json": `["foo", "read"]`,
	}

	test.WithTempFS(files, func(rootDir string) {
		//json unmarshal error
		sc := NewSigningConfig("secret", "HS256", filepath.Join(rootDir, "claims.json"))
		_, err := sc.GetClaims()
		if err == nil {
			t.Fatal("Expected error but got nil")
		}

		// claims.json does not exist
		sc = NewSigningConfig("secret", "HS256", "claims.json")
		_, err = sc.GetClaims()
		if err == nil {
			t.Fatal("Expected error but got nil")
		}

		errMsg := "open claims.json: no such file or directory"
		if err.Error() != errMsg {
			t.Fatalf("Expected error message %v but got %v", errMsg, err.Error())
		}
	})
}

func TestKeyConfigEqual(t *testing.T) {
	tests := map[string]struct {
		a   *KeyConfig
		b   *KeyConfig
		exp bool
	}{
		"equal": {
			NewKeyConfig("foo", "RS256", "read"),
			NewKeyConfig("foo", "RS256", "read"),
			true,
		},
		"not_equal": {
			NewKeyConfig("foo", "RS256", "read"),
			NewKeyConfig("foo", "RS256", "write"),
			false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual := tc.a.Equal(tc.b)

			if actual != tc.exp {
				t.Fatalf("Expected config equal result %v but got %v", tc.exp, actual)
			}
		})
	}
}
