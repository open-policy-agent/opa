// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"bytes"
	"errors"
	"testing"
)

func TestVerifyBundleSignature(t *testing.T) {
	badToken := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.4nrInalqppAc9EjsNUD9Y35amVpDGoRk4bkxzdY8fhs`
	signedTokenHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImZvbyJ9.eyJmaWxlcyI6W3sibmFtZSI6ImRiL3VhbTIvZW50aXRsZW1lbnRzL2RhdGEuanNvbiIsImhhc2giOiJjMjEzMTU0NGM3MTZhMjVhNWUzMWY1MDQzMDBmNTI0MGU4MjM1Y2FkYjlhNTdmMGJkMWI2ZjRiZDc0YjI2NjEyIiwiYWxnb3JpdGhtIjoic2hhMjU2In0seyJuYW1lIjoiZGIvdWFtMi9wb2xpY3kvb3BhLXBvbGljeS5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifV0sImlhdCI6MTU5MjI0ODAyNywiaXNzIjoiSldUU2VydmljZSIsInNjb3BlIjoid3JpdGUifQ.u7b_h88osJpU6VQLhIUmZjblsPNCO_kTqsVtpEAHavs`
	otherSignedTokenHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmaWxlcyI6W3sibmFtZSI6ImRiL3VhbTMvcm9sZXMvZGF0YS5qc29uIiwiaGFzaCI6ImMyMTMxNTQ0YzcxNmEyNWE1ZTMxZjUwNDMwMGY1MjQwZTgyMzVjYWRiOWE1N2YwYmQxYjZmNGJkNzRiMjY2MTIiLCJhbGdvcml0aG0iOiJtZDUifSx7Im5hbWUiOiJkYi91YW0zL3BvbGljeS9wb2xpY3kucmVnbyIsImhhc2giOiI0MmNmZTY3NjhiNTdiYjVmNzUwM2MxNjVjMjhkZDA3YWM1YjgxMzU1NGViYzg1MGYyY2MzNTg0M2U3MTM3YjFkIn0seyJuYW1lIjoiZGIvdWFtNC9wb2xpY3kvcmVnby5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQiLCJhbGdvcml0aG0iOiJzaGEzODQifV0sImlhdCI6MTU5MjI0ODAyNywiaXNzIjoiSldUU2VydmljZSIsImtleWlkIjoiYmFyIiwic2NvcGUiOiJ3cml0ZSJ9.d_NiBXF3zqNPZCEubQC1FC1IYwmwkYwjv00B5UyJ9Dk`
	badTokenHeaderBase64 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImZvby 9.eyJmaWxlcyI6W3sibmFtZSI6ImRiL3VhbTIvZW50aXRsZW1lbnRzL2RhdGEuanNvbiIsImhhc2giOiJjMjEzMTU0NGM3MTZhMjVhNWUzMWY1MDQzMDBmNTI0MGU4MjM1Y2FkYjlhNTdmMGJkMWI2ZjRiZDc0YjI2NjEyIiwiYWxnb3JpdGhtIjoic2hhMjU2In0seyJuYW1lIjoiZGIvdWFtMi9wb2xpY3kvb3BhLXBvbGljeS5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifV0sImlhdCI6MTU5MjI0ODAyNywiaXNzIjoiSldUU2VydmljZSIsInNjb3BlIjoid3JpdGUifQ.u7b_h88osJpU6VQLhIUmZjblsPNCO_kTqsVtpEAHavs`
	badTokenHeaderJSON := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImZvbyX9.eyJmaWxlcyI6W3sibmFtZSI6ImRiL3VhbTIvZW50aXRsZW1lbnRzL2RhdGEuanNvbiIsImhhc2giOiJjMjEzMTU0NGM3MTZhMjVhNWUzMWY1MDQzMDBmNTI0MGU4MjM1Y2FkYjlhNTdmMGJkMWI2ZjRiZDc0YjI2NjEyIiwiYWxnb3JpdGhtIjoic2hhMjU2In0seyJuYW1lIjoiZGIvdWFtMi9wb2xpY3kvb3BhLXBvbGljeS5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifV0sImlhdCI6MTU5MjI0ODAyNywiaXNzIjoiSldUU2VydmljZSIsInNjb3BlIjoid3JpdGUifQ.u7b_h88osJpU6VQLhIUmZjblsPNCO_kTqsVtpEAHavs`
	badTokenPayload := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmaWxlcyI6eyJuYW1lIjoiZGIvdWFtMi9wb2xpY3kvb3BhLXBvbGljeS5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifSwiaWF0IjoxNTkyMjQ4MDI3LCJpc3MiOiJKV1RTZXJ2aWNlIiwia2V5aWQiOiJmb28iLCJzY29wZSI6IndyaXRlIn0.J3KJFOycHPy4Wkw_LzzIKvTMqCsV8L8DdQW5Q-vieKg`

	tests := map[string]struct {
		input              SignaturesConfig
		readerVerifyConfig *VerificationConfig
		wantErr            bool
		err                error
	}{
		"no_signatures":       {SignaturesConfig{}, nil, true, errors.New(".signatures.json: missing JWT (expected exactly one)")},
		"multiple_signatures": {SignaturesConfig{Signatures: []string{signedTokenHS256, otherSignedTokenHS256}}, nil, true, errors.New(".signatures.json: multiple JWTs not supported (expected exactly one)")},
		"invalid_token":       {SignaturesConfig{Signatures: []string{badToken}}, nil, true, errors.New("failed to split compact serialization")},
		"invalid_token_header_base64": {
			SignaturesConfig{Signatures: []string{badTokenHeaderBase64}},
			NewVerificationConfig(nil, "", "", nil),
			true, errors.New("failed to base64 decode JWT headers: illegal base64 data at input byte 50"),
		},
		"invalid_token_header_json": {
			SignaturesConfig{Signatures: []string{badTokenHeaderJSON}},
			NewVerificationConfig(nil, "", "", nil),
			true, errors.New("failed to parse JWT headers: unexpected end of JSON input"),
		},
		"bad_token_payload": {SignaturesConfig{Signatures: []string{badTokenPayload}}, nil, true, errors.New("json: cannot unmarshal object into Go struct field DecodedSignature.files of type []bundle.FileInfo")},
		"valid_token_and_scope": {
			SignaturesConfig{Signatures: []string{signedTokenHS256}},
			NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "write", nil),
			false, nil,
		},
		"valid_token_and_scope_mismatch": {
			SignaturesConfig{Signatures: []string{signedTokenHS256}},
			NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "bad_scope", nil),
			true, errors.New("scope mismatch"),
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
	signedNoKeyIDTokenHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmaWxlcyI6W3sibmFtZSI6ImRiL3VhbTIvZW50aXRsZW1lbnRzL2RhdGEuanNvbiIsImhhc2giOiJjMjEzMTU0NGM3MTZhMjVhNWUzMWY1MDQzMDBmNTI0MGU4MjM1Y2FkYjlhNTdmMGJkMWI2ZjRiZDc0YjI2NjEyIiwiYWxnb3JpdGhtIjoic2hhMjU2In0seyJuYW1lIjoiZGIvdWFtMi9wb2xpY3kvb3BhLXBvbGljeS5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifV0sImlhdCI6MTU5MjI0ODAyNywiaXNzIjoiSldUU2VydmljZSIsInNjb3BlIjoid3JpdGUifQ.N2DaEyhfACMAij2sRC0ZDwNz4wI7BX_flH3IkKqMvE4`

	signedTokenHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImZvbyJ9.eyJmaWxlcyI6W3sibmFtZSI6ImRiL3VhbTIvZW50aXRsZW1lbnRzL2RhdGEuanNvbiIsImhhc2giOiJjMjEzMTU0NGM3MTZhMjVhNWUzMWY1MDQzMDBmNTI0MGU4MjM1Y2FkYjlhNTdmMGJkMWI2ZjRiZDc0YjI2NjEyIiwiYWxnb3JpdGhtIjoic2hhMjU2In0seyJuYW1lIjoiZGIvdWFtMi9wb2xpY3kvb3BhLXBvbGljeS5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifV0sImlhdCI6MTU5MjI0ODAyNywiaXNzIjoiSldUU2VydmljZSIsInNjb3BlIjoid3JpdGUifQ.u7b_h88osJpU6VQLhIUmZjblsPNCO_kTqsVtpEAHavs`

	signedTokenWithDeprecatedKidClaimHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmaWxlcyI6W3sibmFtZSI6ImRiL3VhbTIvZW50aXRsZW1lbnRzL2RhdGEuanNvbiIsImhhc2giOiJjMjEzMTU0NGM3MTZhMjVhNWUzMWY1MDQzMDBmNTI0MGU4MjM1Y2FkYjlhNTdmMGJkMWI2ZjRiZDc0YjI2NjEyIiwiYWxnb3JpdGhtIjoic2hhMjU2In0seyJuYW1lIjoiZGIvdWFtMi9wb2xpY3kvb3BhLXBvbGljeS5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifV0sImlhdCI6MTU5MjI0ODAyNywiaXNzIjoiSldUU2VydmljZSIsImtleWlkIjoiZm9vIiwic2NvcGUiOiJ3cml0ZSJ9.4nrInalqppAc9EjsNUD9Y35amVpDGoRk4bkxzdY8fhs`

	signedTokenWithBarKidHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImJhciJ9.eyJmaWxlcyI6W3sibmFtZSI6ImRiL3VhbTIvZW50aXRsZW1lbnRzL2RhdGEuanNvbiIsImhhc2giOiJjMjEzMTU0NGM3MTZhMjVhNWUzMWY1MDQzMDBmNTI0MGU4MjM1Y2FkYjlhNTdmMGJkMWI2ZjRiZDc0YjI2NjEyIiwiYWxnb3JpdGhtIjoic2hhMjU2In0seyJuYW1lIjoiZGIvdWFtMi9wb2xpY3kvb3BhLXBvbGljeS5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifV0sImlhdCI6MTU5MjI0ODAyNywiaXNzIjoiSldUU2VydmljZSIsInNjb3BlIjoid3JpdGUifQ.xW04tph2-dSy47uZ-CgiObvFWFLUsMym8N7ermgPZ0s`

	signedTokenRS256 := `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImZvbyJ9.eyJmaWxlcyI6W3sibmFtZSI6ImRiL3VhbTIvZW50aXRsZW1lbnRzL2RhdGEuanNvbiIsImhhc2giOiJjMjEzMTU0NGM3MTZhMjVhNWUzMWY1MDQzMDBmNTI0MGU4MjM1Y2FkYjlhNTdmMGJkMWI2ZjRiZDc0YjI2NjEyIiwiYWxnb3JpdGhtIjoic2hhMjU2In0seyJuYW1lIjoiZGIvdWFtMi9wb2xpY3kvb3BhLXBvbGljeS5yZWdvIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifV0sImlhdCI6MTU5MjI1MTQwNiwiaXNzIjoiSldUU2VydmljZSIsInNjb3BlIjoid3JpdGUifQ.UZcX_Qj29o213nwZ5mysiyNcgAp1AOqCkDLkj1KGZ7Cw3l17TbkF8t3FJd654sf762NGcABA0WQ2zxq-labXl_d5TKQmq23lurO3qLHI1cKQPyoxPbddGo78sJpMwWsiMyEYcyyKC9FpNUpa8cT81GPo4q9Zui8TAD0wtHh2YxBL7E8QYLJxcLMJTJxVBgrQGLXFT7vYRc6hJz9Vu-i0SWuLBGxOCmYoHr4DWFfy0IhAbhiUFyLRpukfBPjCJNYY5LgaZdN6LaYivtIEUms5jUuXubBimLJm7KOT5-bgJMXTcUsbrbl2Ma7PwG5IOKjjgyEvH8KVfGFZSzNrpuxKmA`

	publicKeyValid := `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA9KaakMv1XKKDaSch3PFR
3a27oaHp1GNTTNqvb1ZaHZXp+wuhYDwc/MTE67x9GCifvQBWzEGorgTq7aisiOyl
vKifwz6/wQ+62WHKG/sqKn2Xikp3P63aBIPlZcHbkyyRmL62yeyuzYoGvLEYel+m
z5SiKGBwviSY0Th2L4e5sGJuk2HOut6emxDi+E2Fuuj5zokFJvIT6Urlq8f3h6+l
GeR6HUOXqoYVf7ff126GP7dticTVBgibxkkuJFmpvQSW6xmxruT4k6iwjzbZHY7P
ypZ/TdlnuGC1cOpAVyU7k32IJ9CRbt3nwEf5U54LRXLLQjFixWZHwKdDiMTF4ws0
+wIDAQAB
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

	// Private key corresponding to the valid public key, for
	// re-generating the test signatures.
	/* privateKey := `-----BEGIN RSA PRIVATE KEY-----
	MIIEpQIBAAKCAQEA9KaakMv1XKKDaSch3PFR3a27oaHp1GNTTNqvb1ZaHZXp+wuh
	YDwc/MTE67x9GCifvQBWzEGorgTq7aisiOylvKifwz6/wQ+62WHKG/sqKn2Xikp3
	P63aBIPlZcHbkyyRmL62yeyuzYoGvLEYel+mz5SiKGBwviSY0Th2L4e5sGJuk2HO
	ut6emxDi+E2Fuuj5zokFJvIT6Urlq8f3h6+lGeR6HUOXqoYVf7ff126GP7dticTV
	BgibxkkuJFmpvQSW6xmxruT4k6iwjzbZHY7PypZ/TdlnuGC1cOpAVyU7k32IJ9CR
	bt3nwEf5U54LRXLLQjFixWZHwKdDiMTF4ws0+wIDAQABAoIBAQDDL0BVkUNZ+pYZ
	CI1ttmH4GCmAFKt3NR86S6Z3j08qF3arQWYoXw1JZLsu0ByFb7OxmFmncCLhYy8D
	GPU98H9x+p4rqR5XKvOJhwk2NbY4XCbQwARPm6Y6v/f+rSE/U+l9EXrHsrrrZNln
	JWtABpwRNKYCzJ5mNNBu6zrvRLuSygXG/U5wMnO7bLAJh7u3SUPd6Mkg+RTEu4jG
	wHuhcnbsbBdpLRHZKeGlfVL62sUXE1mrf09406U2i7us9BhvYC6pEgbVi7PZ0FBs
	Lx0W1PB0GdKS0hq6yKQhskBFI/CsGHoIjIsG7AiUyGwxLgeVHyOfUTZGaVRVZ99j
	+nz6rOhRAoGBAP6nW/oDaaVZM0JiO09KdbEmtUeNqKPGM1rNtakbiBz5JZ6M636L
	2Q///7jvyIrI6WNRTQh2ByqKpxwHjd0+mJa1CiOrTVE44mcxaXoSdQWtXJhZt24O
	kOwVWgy5P/nyIVq+l+vYFJvMq387c9h/l2cFaSa5fIiTbTFq3ue8Mmp1AoGBAPXx
	tPCHeGa8WwKvMwVB4hZ5XogTRLqtTIftqhrJvSp/2md60Xz7A35HuOTV22PFuoUt
	pIaJggPVHOd7tsSYEx2wjJpTNf+EWhIdp7kziHIqGgjj/d+NftRDvx3e/mJip4TI
	XEN0BzxCplU/eFPL8bvi03fjnCOh2iG599d8vtOvAoGBAMlB1ZRDLDSMydE2N2+U
	Bm3qjKyvTU+aLi4ek+rBopJbahrjfp61weg+R4mOoGznGmTu9TWxqjo5+JZTdhAc
	D5ZUIF5OXT3K+kvaJmVevvOsrpiNl0W451peCZwysFhGv4urRAAV9zumxwc4IndB
	Z5P5F8COKdj6wvqiXubAuwudAoGAD6gPaLB3DbM35/fXO6JyDhQz3F29plSZ5p1O
	kt382NPCx4ueAmLIWiWes5KZoMRZl1jMfHQMfsn2SRYrEGDN9rnieYCKk3WNdlHE
	95k8OmhLt/0rkCulw0V8yR4E+6ZkG6PVm8WrID7t78dWlZ8KCHfsFlm6+tm21SbN
	jD44t6kCgYEAnTHH8SOMoW/kMckESubV2y4rvdLol7nVX3ia7IGP8TA6NHsGT0Qb
	EOuKwnuQ3ZToMdNVrS2nFyLe/HohSbT0SrNu2j4YkZDxHGQImZvw/KtMFPyT+Ff8
	rtvSvuiJNjLUr6OcqxARXrmispBO/GxvWucF0tcpOcSGUDYHAD2yIAQ=
	-----END RSA PRIVATE KEY-----`*/

	tests := map[string]struct {
		token   string
		keys    map[string]*KeyConfig
		keyID   string
		scope   string
		wantErr bool
		err     error
	}{
		"no_public_key_id":          {signedNoKeyIDTokenHS256, map[string]*KeyConfig{}, "", "", true, errors.New("verification key ID is empty")},
		"actual_public_key_missing": {signedTokenHS256, map[string]*KeyConfig{}, "", "", true, errors.New("verification key corresponding to ID foo not found")},
		"deprecated_key_id_claim": {
			signedTokenWithDeprecatedKidClaimHS256,
			map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "write", // check valid keyId in deprecated claim is used
			false, nil,
		},
		"bad_public_key_algorithm": {
			signedTokenHS256,
			map[string]*KeyConfig{"foo": {Key: "somekey", Algorithm: "RS007"}}, "", "",
			true, errors.New("unsupported signature algorithm: RS007"),
		},
		"public_key_with_valid_HS256_sign": {
			signedTokenWithBarKidHS256,
			map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "foo", "write", // check keyId in OPA config takes precedence
			false, nil,
		},
		"public_key_with_invalid_HS256_sign": {
			signedTokenHS256,
			map[string]*KeyConfig{"foo": {Key: "bad_secret", Algorithm: "HS256"}}, "", "",
			true, errors.New("failed to verify message: failed to match hmac signature"),
		},
		"public_key_with_valid_RS256_sign": {
			signedTokenRS256,
			map[string]*KeyConfig{"foo": {Key: publicKeyValid, Algorithm: "RS256"}}, "", "write",
			false, nil,
		},
		"public_key_with_invalid_RS256_sign": {
			signedTokenRS256,
			map[string]*KeyConfig{"foo": {Key: publicKeyInvalid, Algorithm: "RS256"}}, "", "",
			true, errors.New("failed to verify message: crypto/rsa: verification error"),
		},
		"public_key_with_bad_cert_RS256": {
			signedTokenRS256,
			map[string]*KeyConfig{"foo": {Key: publicKeyBad, Algorithm: "RS256"}}, "foo", "",
			true, errors.New("failed to parse PEM block containing the key"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			_, err := verifyJWTSignature(tc.token, NewVerificationConfig(tc.keys, tc.keyID, tc.scope, nil))

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

	_, err := verifyJWTSignature(signedTokenRS256, NewVerificationConfig(keys, "foo", "write", nil))
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
			true, errors.New("file /.manifest not included in bundle signature"),
		},
		"bad_hashing_algorithm": {
			[][2]string{{"/.manifest", `{"revision": "quickbrownfaux"}`}},
			map[string]FileInfo{"/.manifest": {
				Name:      "/.manifest",
				Hash:      "e7dc95e14ad6cd75d044c13d52ee3ab1",
				Algorithm: "MD6",
			}},
			true, errors.New("unsupported hashing algorithm: MD6"),
		},
		"bad_digest": {
			[][2]string{{"/.manifest", `{"revision": "quickbrownfaux"}`}},
			map[string]FileInfo{"/.manifest": {
				Name:      "/.manifest",
				Hash:      "874984d68515ba2439c04dddf5b21574",
				Algorithm: MD5.String(),
			}},
			true, errors.New("/.manifest: digest mismatch (want: 874984d68515ba2439c04dddf5b21574, got: a005c38a509dc2d5a7407b9494efb2ad)"),
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

type CustomVerifier struct{}

func (*CustomVerifier) VerifyBundleSignature(sc SignaturesConfig, bvc *VerificationConfig) (map[string]FileInfo, error) {
	return map[string]FileInfo{}, nil
}

func TestCustomVerifier(t *testing.T) {
	custom := &CustomVerifier{}
	err := RegisterVerifier(defaultVerifierID, custom)
	if err == nil {
		t.Fatalf("Expected error when registering with default ID")
	}
	if err := RegisterVerifier("_test", custom); err != nil {
		t.Fatal(err)
	}
	defaultVerifier, err := GetVerifier(defaultVerifierID)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if _, isDefault := defaultVerifier.(*DefaultVerifier); !isDefault {
		t.Fatalf("Expected DefaultVerifier to be registered at key %s", defaultVerifierID)
	}
	customVerifier, err := GetVerifier("_test")
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if _, isCustom := customVerifier.(*CustomVerifier); !isCustom {
		t.Fatalf("Expected CustomVerifier to be registered at key _test")
	}
	if _, err = GetVerifier("_unregistered"); err == nil {
		t.Fatalf("Expected error when no Verifier exists at provided key")
	}
}
