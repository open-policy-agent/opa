// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
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

// concurrency:_1,_JWT_count:_1-16                              27595       43449 ns/op     18555 B/op    217 allocs/op // parsing keys on every call
// concurrency:_1,_JWT_count:_100-16                            27784       43247 ns/op     18548 B/op    217 allocs/op // parsing keys on every call
// concurrency:_1,_JWT_count:_1,_JWKS:_8_keys_with_x5c-16        4021      301697 ns/op    263249 B/op   1633 allocs/op // parsing keys on every call
// concurrency:_1,_JWT_count:_100,_JWKS:_8_keys_with_x5c-16      3932      303247 ns/op    263244 B/op   1633 allocs/op // parsing keys on every call
// concurrency:_1,_JWT_count:_1-16                              38950       29902 ns/op      7656 B/op     97 allocs/op // caching parsed keys
// concurrency:_1,_JWT_count:_100-16                            39981       29833 ns/op      7656 B/op     97 allocs/op // caching parsed keys
// concurrency:_1,_JWT_count:_1,_JWKS:_8_keys_with_x5c-16       39172       30709 ns/op      7656 B/op     97 allocs/op // caching parsed keys
// concurrency:_1,_JWT_count:_100,_JWKS:_8_keys_with_x5c-16     38886       30736 ns/op      7656 B/op     97 allocs/op // caching parsed keys
func BenchmarkTokens(b *testing.B) {
	ctx := b.Context()
	iter := func(*ast.Term) error { return nil }

	// The default configuration, as used by the server: the io_jwt cache is
	// disabled and the io_jwt_keys cache is enabled.
	bctx := BuiltinContext{
		Context:                     ctx,
		Time:                        ast.NumberTerm(int64ToJSONNumber(time.Now().UnixNano())),
		InterQueryBuiltinValueCache: cache.NewInterQueryValueCache(ctx, &cache.Config{}),
	}

	for _, ks := range benchmarkKeySets(b) {
		keysTerm := ast.ObjectTerm(ast.Item(ast.StringTerm("cert"), ast.StringTerm(ks.jwks)))

		worker := func(jobs <-chan string, results chan<- bool) {
			for jwt := range jobs {
				err := builtinJWTDecodeVerify(bctx, []*ast.Term{ast.NewTerm(ast.String(jwt)), keysTerm}, iter)
				if err != nil {
					results <- false
				}
				results <- true
			}
		}

		jwtCounts := []int{1, 5, 6, 10, 100}
		concurrencyLevels := []int{1, 1000}
		for _, jwtCount := range jwtCounts {
			jwts := make([]string, jwtCount)

			for i := range jwtCount {
				jwts[i] = createJwtB(b, fmt.Sprintf(`{"i": %d}`, i))
			}

			for _, concurrencyLevel := range concurrencyLevels {
				b.Run(fmt.Sprintf("concurrency: %d, JWT count: %d%s", concurrencyLevel, jwtCount, ks.suffix), func(b *testing.B) {
					count := b.N
					jobs := make(chan string, count)
					results := make(chan bool, count)

					for range concurrencyLevel {
						go worker(jobs, results)
					}

					b.ResetTimer()

					for i := range count {
						jobs <- jwts[i%jwtCount]
					}

					close(jobs)

					for range count {
						r := <-results
						if !r {
							b.Fatal("failed to verify JWT")
						}
					}
				})
			}
		}
	}
}

// concurrency:_1,_JWT_count:_1-16                              79759       15198 ns/op     12428 B/op    151 allocs/op // parsing keys on every call
// concurrency:_1,_JWT_count:_100-16                            26868       44637 ns/op     18933 B/op    229 allocs/op // parsing keys on every call
// concurrency:_1,_JWT_count:_1,_JWKS:_8_keys_with_x5c-16        4292      282278 ns/op    257131 B/op   1567 allocs/op // parsing keys on every call
// concurrency:_1,_JWT_count:_100,_JWKS:_8_keys_with_x5c-16      3916      310075 ns/op    263635 B/op   1645 allocs/op // parsing keys on every call
// concurrency:_1,_JWT_count:_1-16                            1000000        1059 ns/op      1536 B/op     31 allocs/op // caching parsed keys
// concurrency:_1,_JWT_count:_100-16                            38684       31180 ns/op      8040 B/op    109 allocs/op // caching parsed keys
// concurrency:_1,_JWT_count:_1,_JWKS:_8_keys_with_x5c-16      550896        2094 ns/op      1536 B/op     31 allocs/op // caching parsed keys
// concurrency:_1,_JWT_count:_100,_JWKS:_8_keys_with_x5c-16     37188       32376 ns/op      8040 B/op    109 allocs/op // caching parsed keys
func BenchmarkTokens_Cache(b *testing.B) {
	ctx := b.Context()
	iter := func(*ast.Term) error { return nil }

	bctx := BuiltinContext{
		Context: ctx,
		Time:    ast.NumberTerm(int64ToJSONNumber(time.Now().UnixNano())),
		InterQueryBuiltinValueCache: cache.NewInterQueryValueCache(ctx, &cache.Config{
			InterQueryBuiltinValueCache: cache.InterQueryBuiltinValueCacheConfig{
				NamedCacheConfigs: map[string]*cache.NamedValueCacheConfig{
					tokenCacheName: {
						MaxNumEntries: &[]int{5}[0],
					},
				},
			},
		}),
	}

	for _, ks := range benchmarkKeySets(b) {
		keysTerm := ast.ObjectTerm(ast.Item(ast.StringTerm("cert"), ast.StringTerm(ks.jwks)))

		worker := func(jobs <-chan string, results chan<- bool) {
			for jwt := range jobs {
				err := builtinJWTDecodeVerify(bctx, []*ast.Term{ast.NewTerm(ast.String(jwt)), keysTerm}, iter)
				if err != nil {
					results <- false
				}
				results <- true
			}
		}

		jwtCounts := []int{1, 5, 6, 10, 100}
		concurrencyLevels := []int{1, 1000}
		for _, jwtCount := range jwtCounts {
			jwts := make([]string, jwtCount)

			for i := range jwtCount {
				jwts[i] = createJwtB(b, fmt.Sprintf(`{"i": %d}`, i))
			}

			for _, concurrencyLevel := range concurrencyLevels {
				b.Run(fmt.Sprintf("concurrency: %d, JWT count: %d%s", concurrencyLevel, jwtCount, ks.suffix), func(b *testing.B) {
					count := b.N
					jobs := make(chan string, count)
					results := make(chan bool, count)

					for range concurrencyLevel {
						go worker(jobs, results)
					}

					b.ResetTimer()

					for i := range count {
						jobs <- jwts[i%jwtCount]
					}

					close(jobs)

					for range count {
						<-results
					}
				})
			}
		}
	}
}

func createJwtB(b *testing.B, payload string) string {
	b.Helper()

	jwt, err := createJwt(payload, privateKey)
	if err != nil {
		b.Fatal(err)
	}

	return jwt
}

type benchmarkKeySet struct {
	jwks   string
	suffix string
}

// benchmarkKeySets returns the key sets the token benchmarks verify against:
// the single key the benchmarks have always used, keeping their names
// unchanged, and a JWKS shaped like an identity provider's, with several keys
// each carrying an x5c certificate chain.
func benchmarkKeySets(b *testing.B) []benchmarkKeySet {
	b.Helper()

	return []benchmarkKeySet{
		{jwks: keys},
		{jwks: largeJWKS(b, 8), suffix: ", JWKS: 8 keys with x5c"},
	}
}

// largeJWKS returns a JWKS of n copies of publicKey, each with its own kid and
// a self-signed certificate in x5c. The copies share a key so that whichever
// one verifies the token, only parsing grows with n.
func largeJWKS(b *testing.B, n int) string {
	b.Helper()

	key, err := jwk.ParseKey([]byte(privateKey))
	if err != nil {
		b.Fatal(err)
	}
	var raw any
	if err := jwk.Export(key, &raw); err != nil {
		b.Fatal(err)
	}
	pk := raw.(*rsa.PrivateKey)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "opa-benchmark"},
		NotBefore:    time.Unix(0, 0),
		NotAfter:     time.Unix(0, 0).Add(100 * 365 * 24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &pk.PublicKey, pk)
	if err != nil {
		b.Fatal(err)
	}

	var base map[string]any
	if err := json.Unmarshal([]byte(publicKey), &base); err != nil {
		b.Fatal(err)
	}

	set := make([]map[string]any, n)
	for i := range n {
		k := map[string]any{
			"kid": fmt.Sprintf("key-%d", i),
			"use": "sig",
			"alg": "RS256",
			"x5c": []string{base64.StdEncoding.EncodeToString(der)},
		}
		maps.Copy(k, base)
		set[i] = k
	}

	bs, err := json.Marshal(map[string]any{"keys": set})
	if err != nil {
		b.Fatal(err)
	}
	return string(bs)
}
