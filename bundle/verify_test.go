// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"bytes"
	"fmt"
	"testing"
)

func TestVerifyBundleSignature(t *testing.T) {
	badToken := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.4nrInalqppAc9EjsNUD9Y35amVpDGoRk4bkxzdY8fhs`
	signedTokenHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmaWxlcyI6W3sibmFtZSI6ImRiL3VhbTIvZW50aXRsZW1lbnRzL2RhdGEuanNvbiIsImhhc2giOiJjMjEzMTU0NGM3MTZhMjVhNWUzMWY1MDQzMDBmNTI0MGU4MjM1Y2FkYjlhNTdmMGJkMWI2ZjRiZDc0YjI2NjEyIiwiYWxnb3JpdGhtIjoic2hhMjU2In0seyJuYW1lIjoiZGIvdWFtMi9wb2xpY3kvb3BhLXBvbGljeS5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifV0sImlhdCI6MTU5MjI0ODAyNywiaXNzIjoiSldUU2VydmljZSIsImtleWlkIjoiZm9vIiwic2NvcGUiOiJ3cml0ZSJ9.4nrInalqppAc9EjsNUD9Y35amVpDGoRk4bkxzdY8fhs`
	otherSignedTokenHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmaWxlcyI6W3sibmFtZSI6ImRiL3VhbTMvcm9sZXMvZGF0YS5qc29uIiwiaGFzaCI6ImMyMTMxNTQ0YzcxNmEyNWE1ZTMxZjUwNDMwMGY1MjQwZTgyMzVjYWRiOWE1N2YwYmQxYjZmNGJkNzRiMjY2MTIiLCJhbGdvcml0aG0iOiJtZDUifSx7Im5hbWUiOiJkYi91YW0zL3BvbGljeS9wb2xpY3kucmVnbyIsImhhc2giOiI0MmNmZTY3NjhiNTdiYjVmNzUwM2MxNjVjMjhkZDA3YWM1YjgxMzU1NGViYzg1MGYyY2MzNTg0M2U3MTM3YjFkIn0seyJuYW1lIjoiZGIvdWFtNC9wb2xpY3kvcmVnby5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQiLCJhbGdvcml0aG0iOiJzaGEzODQifV0sImlhdCI6MTU5MjI0ODAyNywiaXNzIjoiSldUU2VydmljZSIsImtleWlkIjoiYmFyIiwic2NvcGUiOiJ3cml0ZSJ9.d_NiBXF3zqNPZCEubQC1FC1IYwmwkYwjv00B5UyJ9Dk`
	badTokenPayload := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmaWxlcyI6eyJuYW1lIjoiZGIvdWFtMi9wb2xpY3kvb3BhLXBvbGljeS5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifSwiaWF0IjoxNTkyMjQ4MDI3LCJpc3MiOiJKV1RTZXJ2aWNlIiwia2V5aWQiOiJmb28iLCJzY29wZSI6IndyaXRlIn0.J3KJFOycHPy4Wkw_LzzIKvTMqCsV8L8DdQW5Q-vieKg`

	tests := map[string]struct {
		input              SignaturesConfig
		readerVerifyConfig *VerificationConfig
		wantErr            bool
		err                error
	}{
		"no_signatures":       {SignaturesConfig{}, nil, true, fmt.Errorf(".signatures.json: missing JWT (expected exactly one)")},
		"multiple_signatures": {SignaturesConfig{Signatures: []string{signedTokenHS256, otherSignedTokenHS256}}, nil, true, fmt.Errorf(".signatures.json: multiple JWTs not supported (expected exactly one)")},
		"invalid_token":       {SignaturesConfig{Signatures: []string{badToken}}, nil, true, fmt.Errorf("Failed to split compact serialization")},
		"bad_token_payload":   {SignaturesConfig{Signatures: []string{badTokenPayload}}, nil, true, fmt.Errorf("json: cannot unmarshal object into Go struct field DecodedSignature.files of type []bundle.FileInfo")},
		"valid_token_and_scope": {
			SignaturesConfig{Signatures: []string{signedTokenHS256}},
			NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "write", nil),
			false, nil,
		},
		"valid_token_and_scope_mismatch": {
			SignaturesConfig{Signatures: []string{signedTokenHS256}},
			NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "bad_scope", nil),
			true, fmt.Errorf("scope mismatch"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			_, err := VerifyBundleSignature(tc.input, tc.readerVerifyConfig)

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
		})
	}

	// verify the number files on the reader collected from the JWT
	sc := SignaturesConfig{Signatures: []string{signedTokenHS256}}
	verificationConfig := NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "write", nil)
	files, err := VerifyBundleSignature(sc, verificationConfig)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	expectedNumFiles := 2
	if len(files) != expectedNumFiles {
		t.Fatalf("Expected %v files in the JWT payloads but got %v", expectedNumFiles, len(files))
	}
}

func TestVerifyJWTSignature(t *testing.T) {
	signedTokenHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmaWxlcyI6W3sibmFtZSI6ImRiL3VhbTIvZW50aXRsZW1lbnRzL2RhdGEuanNvbiIsImhhc2giOiJjMjEzMTU0NGM3MTZhMjVhNWUzMWY1MDQzMDBmNTI0MGU4MjM1Y2FkYjlhNTdmMGJkMWI2ZjRiZDc0YjI2NjEyIiwiYWxnb3JpdGhtIjoic2hhMjU2In0seyJuYW1lIjoiZGIvdWFtMi9wb2xpY3kvb3BhLXBvbGljeS5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifV0sImlhdCI6MTU5MjI0ODAyNywiaXNzIjoiSldUU2VydmljZSIsImtleWlkIjoiZm9vIiwic2NvcGUiOiJ3cml0ZSJ9.4nrInalqppAc9EjsNUD9Y35amVpDGoRk4bkxzdY8fhs`

	signedTokenRS256 := `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJmaWxlcyI6W3sibmFtZSI6ImRiL3VhbTIvZW50aXRsZW1lbnRzL2RhdGEuanNvbiIsImhhc2giOiJjMjEzMTU0NGM3MTZhMjVhNWUzMWY1MDQzMDBmNTI0MGU4MjM1Y2FkYjlhNTdmMGJkMWI2ZjRiZDc0YjI2NjEyIiwiYWxnb3JpdGhtIjoic2hhMjU2In0seyJuYW1lIjoiZGIvdWFtMi9wb2xpY3kvb3BhLXBvbGljeS5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifV0sImlhdCI6MTU5MjI1MTQwNiwiaXNzIjoiSldUU2VydmljZSIsImtleWlkIjoiZm9vIiwic2NvcGUiOiJ3cml0ZSJ9.Xp1RTaiy9FXeELDWfdMYSsnRnmeBzRtoi4ewT5zr__4IXR0fgjpOOJgUoeFalTqumZQr3UO9PgSFM1Xfp1ivP8OQhMzCZEg8gI3yXQVfmu2Pb0Bo70t04AsqMjPG0XWIwYBgj20HwCrvjrzV3O9PfMWSLS03kL8iXzZDNu8BnN5BEuG6X2gpd-KcqIy_OMJLJSUaXvD8a7nvhKm0WbRFGImS2ioUK_9zmz2C6T5oedxPrlardsr6TjXDU4QzMSEYXzv0UnXCPxE0oxAktY47AylOpvg0E1AfYtFFiTpMIhtEU00rVOeFKiDicdG-ZxxicXZTYayd3O5kcu5LusUm7naeWGXc0mTNyFLUehqn3rQxgHUOgFmS_IruRVLLflAxHOoa-KWjkHeZYx5mQVAQJZqkR2kf1o31tcmXo8zqEYSywUy40e4xU9ZEJepQ21oS0NkJLq1hSSD-0lSo9rGqsLboxJ_ZHmC109YrGNyxj4-AoIB_6_9UPOu43o2ylDmyxtiti10FjaO5LhLgr9noI9yTiF-0N5nlAQqiIU6v5ImGb0kAaHrk8Jhin52WHMn3gbyC1Ss9bQgEBl71ZrSqG5mxlws86iHCJ6dLTk_7A9KecH24S_Pt8hufWQW9GpzoslditpQFH2fEKnGZqUipUE3qQI063xBAyeoX2YkiVIc`

	publicKeyValid := `-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA7nJwME0QNM6g0Ou9Sylj
lcIY4cnBcs8oWVHe74bJ7JTgYmDOk2CA14RE3wJNkUKERP/cRdesKDA/BToJXJUr
oYvhjXxUYn+i3wK5vOGRY9WUtTF9paIIpIV4USUOwDh3ufhA9K3tyh+ZVsqn80em
0Lj2ME0EgScuk6u0/UYjjNvcmnQl+uDmghG8xBZh7TZW2+aceMwlb4LJIP36VRhg
jKQGIxg2rW8ROXgJaFbNRCbiOUUqlq9SUZuhHo8TNOARXXxp9R4Fq7Cl7ZbwWtNP
wAtM1y+Z+iyu/i91m0YLlU2XBOGLu9IA8IZjPlbCnk/SygpV9NNwTY9DSQ0QfXcP
TGlsbFwzRzTlhH25wEl3j+2Ub9w/NX7Yo+j/Ei9eGZ8cq0bcvEwDeIo98HeNZWrL
UUArayRYvh8zutOlzqehw8waFk9AxpfEp9oWekSz8gZw9OL773EhnglYxxjkPHNz
k66CufLuTEf6uE9NLE5HnlQMbiqBFirIyAWGKyU3v2tphKvcogxmzzWA51p0GY0l
GZvlLNt2NrJv2oGecyl3BLqHnBi+rGAosa/8XgfQT8RIk7YR/tDPDmPfaqSIc0po
+NcHYEH82Yv+gfKSK++1fyssGCsSRJs8PFMuPGgv62fFrE/EHSsHJaNWojSYce/T
rxm2RaHhw/8O4oKcfrbaRf8CAwEAAQ==
-----END PUBLIC KEY-----`

	publicKeyInvalid := `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDMYfnvWtC8Id5bPKae5yXSxQTt
+Zpul6AnnZWfI2TtIarvjHBFUtXRo96y7hoL4VWOPKGCsRqMFDkrbeUjRrx8iL91
4/srnyf6sh9c8Zk04xEOpK1ypvBz+Ks4uZObtjnnitf0NBGdjMKxveTq+VE7BWUI
yQjtQ8mbDOsiLLvh7wIDAQAB
-----END PUBLIC KEY-----`

	publicKeyBad := `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDMYfnvWtC8Id5bPKae5yXSxQTt
+Zpul6AnnZWfI2TtIarvjHBFUtXRo96y7hoL4VWOPKGCsRqMFDkrbeUjRrx8iL91
4/srnyf6sh9c8Zk04xEOpK1ypvBz+Ks4uZObtjnnitf0NBGdjMKxveTq+VE7BWUI
yQjtQ8mbDOsiLLvh7wIDAQAB==
-----END PUBLIC KEY-----`

	tests := map[string]struct {
		token   string
		payload DecodedSignature
		keys    map[string]*KeyConfig
		keyID   string
		wantErr bool
		err     error
	}{
		"no_public_key_id":          {"", DecodedSignature{}, map[string]*KeyConfig{}, "", true, fmt.Errorf("verification key ID is empty")},
		"actual_public_key_missing": {"", DecodedSignature{KeyID: "foo"}, map[string]*KeyConfig{}, "", true, fmt.Errorf("verification key corresponding to ID foo not found")},
		"bad_public_key_algorithm": {
			"",
			DecodedSignature{KeyID: "foo"},
			map[string]*KeyConfig{"foo": {Key: "somekey", Algorithm: "RS007"}}, "",
			true, fmt.Errorf("unsupported signature algorithm: RS007"),
		},
		"public_key_with_valid_HS256_sign": {
			signedTokenHS256,
			DecodedSignature{KeyID: "bar"},
			map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "foo", // check keyId in OPA config takes precedence
			false, nil,
		},
		"public_key_with_invalid_HS256_sign": {
			signedTokenHS256,
			DecodedSignature{KeyID: "foo"},
			map[string]*KeyConfig{"foo": {Key: "bad_secret", Algorithm: "HS256"}}, "",
			true, fmt.Errorf("Failed to verify message: failed to match hmac signature"),
		},
		"public_key_with_valid_RS256_sign": {
			signedTokenRS256,
			DecodedSignature{KeyID: "foo"},
			map[string]*KeyConfig{"foo": {Key: publicKeyValid, Algorithm: "RS256"}}, "",
			false, nil,
		},
		"public_key_with_invalid_RS256_sign": {
			signedTokenRS256,
			DecodedSignature{KeyID: "foo"},
			map[string]*KeyConfig{"foo": {Key: publicKeyInvalid, Algorithm: "RS256"}}, "",
			true, fmt.Errorf("Failed to verify message: crypto/rsa: verification error"),
		},
		"public_key_with_bad_cert_RS256": {
			signedTokenRS256,
			DecodedSignature{},
			map[string]*KeyConfig{"foo": {Key: publicKeyBad, Algorithm: "RS256"}}, "foo",
			true, fmt.Errorf("failed to parse PEM block containing the key"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			err := verifyJWTSignature(tc.token, tc.payload, NewVerificationConfig(tc.keys, tc.keyID, "", nil))

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
		})
	}

	// public key id provided in OPA config, actual public key signed using RS256. Valid signature
	keys := map[string]*KeyConfig{}
	keys["foo"] = &KeyConfig{
		Key:       publicKeyValid,
		Algorithm: "RS256",
	}

	err := verifyJWTSignature(signedTokenRS256, DecodedSignature{}, NewVerificationConfig(keys, "foo", "", nil))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
}

func TestVerifyBundleFile(t *testing.T) {

	tests := map[string]struct {
		files       [][2]string
		readerFiles map[string]FileInfo
		wantErr     bool
		err         error
	}{
		"file_not_found": {
			[][2]string{{"/.manifest", `{"revision": "quickbrownfaux"}`}},
			map[string]FileInfo{},
			true, fmt.Errorf("file /.manifest not included in bundle signature"),
		},
		"bad_hashing_algorithm": {
			[][2]string{{"/.manifest", `{"revision": "quickbrownfaux"}`}},
			map[string]FileInfo{"/.manifest": {
				Name:      "/.manifest",
				Hash:      "e7dc95e14ad6cd75d044c13d52ee3ab1",
				Algorithm: "MD6",
			}},
			true, fmt.Errorf("unsupported hashing algorithm: MD6"),
		},
		"bad_digest": {
			[][2]string{{"/.manifest", `{"revision": "quickbrownfaux"}`}},
			map[string]FileInfo{"/.manifest": {
				Name:      "/.manifest",
				Hash:      "874984d68515ba2439c04dddf5b21574",
				Algorithm: MD5.String(),
			}},
			true, fmt.Errorf("/.manifest: digest mismatch (want: 874984d68515ba2439c04dddf5b21574, got: a005c38a509dc2d5a7407b9494efb2ad)"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			data := bytes.NewBufferString(tc.files[0][1])
			err := VerifyBundleFile(tc.files[0][0], *data, tc.readerFiles)

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
		})
	}
}
