// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
	"github.com/open-policy-agent/opa/internal/jwx/jwk"
	"github.com/open-policy-agent/opa/internal/jwx/jws/sign"
	"github.com/open-policy-agent/opa/v1/ast"
)

const privateKey = `{
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

const publicKey = `{
    "kty":"RSA",
    "n":"ofgWCuLjybRlzo0tZWJjNiuSfb4p4fAkd_wWJcyQoTbji9k0l8W26mPddxHmfHQp-Vaw-4qPCJrcS2mJPMEzP1Pt0Bm4d4QlL-yRT-SFd2lZS-pCgNMsD1W_YpRPEwOWvG6b32690r2jZ47soMZo9wGzjb_7OMg0LOL-bSf63kpaSHSXndS5z5rexMdbBYUsLA9e-KXBdQOS-UTo7WTBEMa2R2CapHg665xsmtdVMTBQY4uDZlxvb3qCo5ZwKh9kG4LT6_I5IhlJH7aGhyxXFvUK-DWNmoudF8NAco9_h9iaGNj8q2ethFkMLs91kzk2PAcDTW9gb54h4FRWyuXpoQ",
    "e":"AQAB"
  }`

const keys = `{"keys": [` + publicKey + `]}`

func BenchmarkTokens(b *testing.B) {
	resetJwtCache()

	bctx := BuiltinContext{
		Time: ast.NumberTerm(int64ToJSONNumber(time.Now().UnixNano())),
	}
	iter := func(*ast.Term) error { return nil }

	keysTerm := ast.ObjectTerm(ast.Item(ast.StringTerm("cert"), ast.StringTerm(keys)))

	worker := func(id int, jobs <-chan string, results chan<- bool) {
		for jwt := range jobs {
			err := builtinJWTDecodeVerify(bctx, []*ast.Term{ast.NewTerm(ast.String(jwt)), keysTerm}, iter)
			if err != nil {
				results <- false
				b.Fatal(err)
			}
			results <- true
		}
	}

	jwtCounts := []int{1, 5, 10, 100}
	concurrencyLevels := []int{1, 10, 100, 1000}
	for _, jwtCount := range jwtCounts {
		jwts := make([]string, jwtCount)

		for i := 0; i < jwtCount; i++ {
			jwts[i] = createJwt(b, fmt.Sprintf(`{"i": %d}`, i))
		}

		for _, concurrencyLevel := range concurrencyLevels {
			b.Run(fmt.Sprintf("concurrency: %d, JWT count: %d", concurrencyLevel, jwtCount), func(b *testing.B) {
				count := b.N
				jobs := make(chan string, count)
				results := make(chan bool, count)

				for w := 0; w < concurrencyLevel; w++ {
					go worker(w, jobs, results)
				}

				b.ResetTimer()

				for i := 0; i < count; i++ {
					jobs <- jwts[i%jwtCount]
				}

				close(jobs)

				for i := 0; i < count; i++ {
					<-results
				}
			})
		}
	}
}

func createJwt(b *testing.B, payload string) string {
	const hdr = `{"alg":"RS256"}`

	var jwkKeySet *jwk.Set
	jwkKeySet, err := jwk.ParseString(privateKey)
	if err != nil {
		b.Fatalf("Failed to parse JWK: %s", err.Error())
	}
	signer, err := sign.New(jwa.RS256)
	if err != nil {
		b.Fatalf("Failed to create signer: %s", err.Error())
	}

	hdrStr := base64.RawURLEncoding.EncodeToString([]byte(hdr))
	payloadStr := base64.RawURLEncoding.EncodeToString([]byte(payload))

	signingInput := strings.Join(
		[]string{
			hdrStr,
			payloadStr,
		}, ".",
	)
	privateKey, err := jwkKeySet.Keys[0].Materialize()
	if err != nil {
		b.Fatalf("Failed to materialize key: %s", err.Error())
	}
	signature, err := signer.Sign([]byte(signingInput), privateKey)
	if err != nil {
		b.Fatalf("Failed to sign message: %s", err.Error())
	}
	encSignature := base64.RawURLEncoding.EncodeToString(signature)

	encoded := strings.Join(
		[]string{
			signingInput,
			encSignature,
		}, ".",
	)

	return encoded
}
