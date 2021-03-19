// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"crypto/ecdsa"
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

func TestGetPrivateKey(t *testing.T) {
	privateKey := `-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`

	pkcs8privateKey := `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDa6aUuhBZnXWnm
Cfy8ZIMchsnabGGatVW5LjEQnwHjREArZ49Y/THMcZdnhZ1cn5WuyRyCoUqXd9uv
IOyMH2M2fFPwR1np4ylDpS+AIJbpGIHuqmcvBfMrD1ADn6HoNCwdARyziot+9js+
A6brtDSbuw+kvTqsndF97rLvzBCxiM1vMGlQ91u6Qcwxrw6eydisRpzXRkSTeJB2
rR54sVNbV8PY5/LVtjqF5VJ1ZzIgByU1gKuVz7ngX/CLSqM2O4C1mKAet3Ib1+Ws
r9wc0b5EGx1kdKumtfqBSFc78ShuhV1URlbYxs8GpNOcD8szI5hRlLYhb4zq3Viz
OxKjOKmBAgMBAAECggEBAK+DJCxnOo8lFgKZf0iMTZJRfwTgYGDpghE2N6Bb2+ea
kNg773IpjgOcDwew2LmqORgppfIV3vgR4NBIVV8Cy0ij5ah/jFc5CZxyk+LmPhgk
zgfMF25cFtovLLe7BNRm//dBLQHF0pG4WUcfJnVTxdoV4DT0glZjMdMFzfD0a23q
BS+Eu68mItCy6Mll7ys+Z9F1+iTwMCVAxKi7/bavqa5tifhk+i9/9ioGBKo+gbHz
fbOrWGrlfSn1eJX539ekVTA3TnWe+IMHjBSIYJcUtHFa5ngdnG2p0vFSVGwcoQPP
1Sbd1A1qbe6lz25V+Km+QFvYMXC3WBSMykOeFFCweQECgYEA7iQzaU81VUruw3ZQ
3tfbyu79hQZV9WNTJhMi8dlPU7BnAxtZeAXzYf0eTokquphq1KVdtie5IjVg7djO
AaKIXCOB9GdL+l+wwbmgn0iHXbpVrWZCYXd+SC99xdUQs+m28pgt/IF1Dp78bv4G
KFZSDUSmOj0i//AFm72r8aOYe5sCgYEA61RLXewT8eZU784BTVDXgpe4+l/bANFt
jWv0oK7AofoHPZfo+K9cYxZcJazsHc2Cg4KHTeCI1VUAFujO//i1G22Lm5hUKyy3
v7hWsjruBhUT5hQA7EXzGCaUSMGwD1UsJs3umC56nzr42cL2HRhrX+SWfMipHepy
3y3NED6WxxMCgYAz3vi/0Hv6dxboxmW5FGWQn1vjVMz2ZUsgOPzclwv7W6okeBmV
1h38Uwj97Ey9ViO268osuhxOQjg5toawvnlbMHTHCpT3FU7H86nz5/VsSgENgv+k
gUWlbYrEw7MerSKnVtR1crFPnPu5JWWr9ZlrwG9Asj5kZyChmr/QI2U8TwKBgQDN
BA/w0EYD/T1L+bXKrL5D6Hhfr/i0ur9tcHqbLgNmWdPLBjgRx3x+WrGGpSLDSBIH
DkVgRFgROs8sJkCIYh0tuv7gXBIf1wJyBV+KQKqzI9PFIvI25S3GgX238P24LeSc
HdZaQEvVwuOfmykc6fRJg3TTW2FyTZkr89Pt7gkffwKBgHGeJkFc6LFeHIwa3SbS
qAVebnCAfNo9hHxz3xYA0PaCF3Kr1X9z4X2tF2Za7nWfVbfWViAncLrJgjnHRdrs
f10hbJEuLFhD1c2dNjwqflANV5OanG1syqYqil5TgWm1AaRFj+PbRPk0FRfF9y+e
tKaHBn4eyNlKjQaEn16ZxKJm
-----END PRIVATE KEY-----`

	pkcs1ecPrivateKey := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEINMW3Ro+oSlbPebDGzeu9w4Eug5ZS/TdjnfnqBP0tMVaoAoGCCqGSM49
AwEHoUQDQgAEkla2v5uQDXr/WoXdCyD3OfAn21K+suzymtp9qAWqRTXWK0a09/cW
Go/Uf1QsCMvmJJ5n9QZb15mhdReiCy4bNw==
-----END EC PRIVATE KEY-----`

	pkcs8ecPrivateKey := `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgZAUy0S0Dow25efPX
SXNNy1EFGSxFEjEQMWSo5/PoL16hRANCAAS1MkJ0tCo++7BktJcmXusp55WyB6n1
qnby6ICFV1o3cV2WFc5PVToBVoPEyUZQ7KFz/3znYQ44fbclemgU/5mf
-----END PRIVATE KEY-----`

	files := map[string]string{
		"private.pem": privateKey,
		"pkcs8.pem":   pkcs8privateKey,
		"pkcs1ec.pem": pkcs1ecPrivateKey,
		"pkcs8ec.pem": pkcs8ecPrivateKey,
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

		sc = NewSigningConfig(filepath.Join(rootDir, "pkcs8.pem"), "", "")

		result, err = sc.GetPrivateKey()
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		_, ok = result.(*rsa.PrivateKey)
		if !ok {
			t.Fatalf("Expected key type *rsa.PrivateKey but got %T", result)
		}

		sc = NewSigningConfig(filepath.Join(rootDir, "pkcs1ec.pem"), "ES256", "")
		result, err = sc.GetPrivateKey()
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		_, ok = result.(*ecdsa.PrivateKey)
		if !ok {
			t.Fatalf("Expected key type *ecdsa.PrivateKey but got %T", result)
		}

		sc = NewSigningConfig(filepath.Join(rootDir, "pkcs8ec.pem"), "ES256", "")
		result, err = sc.GetPrivateKey()
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		_, ok = result.(*ecdsa.PrivateKey)
		if !ok {
			t.Fatalf("Expected key type *ecdsa.PrivateKey but got %T", result)
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
