# JWK [![Go Reference](https://pkg.go.dev/badge/github.com/lestrrat-go/jwx/v3/jwk.svg)](https://pkg.go.dev/github.com/lestrrat-go/jwx/v3/jwk)

Package jwk implements JWK as described in [RFC7517](https://tools.ietf.org/html/rfc7517).
If you are looking to use JWT wit JWKs, look no further than [github.com/lestrrat-go/jwx](../jwt).

* Parse and work with RSA/EC/Symmetric/OKP JWK types
  * Convert to and from JSON
  * Convert to and from raw key types (e.g. *rsa.PrivateKey)
* Ability to keep a JWKS fresh using *jwk.AutoRefresh

## Supported key types:

| kty | Curve                   | Go Key Type                                   |
|:----|:------------------------|:----------------------------------------------|
| RSA | N/A                     | rsa.PrivateKey / rsa.PublicKey (2)            |
| EC  | P-256<br>P-384<br>P-521<br>secp256k1 (1) | ecdsa.PrivateKey / ecdsa.PublicKey (2)        |
| oct | N/A                     | []byte                                        |
| OKP | Ed25519 (1)             | ed25519.PrivateKey / ed25519.PublicKey (2)    |
|     | X25519 (1)              | (jwx/)x25519.PrivateKey / x25519.PublicKey (2)|

* Note 1: Experimental
* Note 2: Either value or pointers accepted (e.g. rsa.PrivateKey or *rsa.PrivateKey)

# Documentation

Please read the [API reference](https://pkg.go.dev/github.com/lestrrat-go/jwx/v3/jwk), or
the how-to style documentation on how to use JWK can be found in the [docs directory](../docs/04-jwk.md).

# Auto-Refresh a key during a long-running process

<!-- INCLUDE(examples/jwk_cache_example_test.go) -->
```go
package examples_test

import (
  "context"
  "fmt"
  "time"

  "github.com/lestrrat-go/httprc/v3"
  "github.com/lestrrat-go/jwx/v3/jwk"
)

func Example_jwk_cache() {
  ctx, cancel := context.WithCancel(context.Background())
  defer cancel()

  const googleCerts = `https://www.googleapis.com/oauth2/v3/certs`

  // First, set up the `jwk.Cache` object. You need to pass it a
  // `context.Context` object to control the lifecycle of the background fetching goroutine.
  c, err := jwk.NewCache(ctx, httprc.NewClient())
  if err != nil {
    fmt.Printf("failed to create cache: %s\n", err)
    return
  }

  // Tell *jwk.Cache that we only want to refresh this JWKS periodically.
  if err := c.Register(ctx, googleCerts); err != nil {
    fmt.Printf("failed to register google JWKS: %s\n", err)
    return
  }

  // Pretend that this is your program's main loop
MAIN:
  for {
    select {
    case <-ctx.Done():
      break MAIN
    default:
    }
    keyset, err := c.Lookup(ctx, googleCerts)
    if err != nil {
      fmt.Printf("failed to fetch google JWKS: %s\n", err)
      return
    }
    _ = keyset
    // The returned `keyset` will always be "reasonably" new.
    //
    // By "reasonably" we mean that we cannot guarantee that the keys will be refreshed
    // immediately after it has been rotated in the remote source. But it should be close\
    // enough, and should you need to forcefully refresh the token using the `(jwk.Cache).Refresh()` method.
    //
    // If refetching the keyset fails, a cached version will be returned from the previous
    // successful sync

    // Do interesting stuff with the keyset... but here, we just
    // sleep for a bit
    time.Sleep(time.Second)

    // Because we're a dummy program, we just cancel the loop now.
    // If this were a real program, you presumably loop forever
    cancel()
  }
  // OUTPUT:
}
```
source: [examples/jwk_cache_example_test.go](https://github.com/lestrrat-go/jwx/blob/v3/examples/jwk_cache_example_test.go)
<!-- END INCLUDE -->

Parse and use a JWK key:

<!-- INCLUDE(examples/jwk_example_test.go) -->
```go
package examples_test

import (
  "context"
  "fmt"
  "log"

  "github.com/lestrrat-go/jwx/v3/internal/json"
  "github.com/lestrrat-go/jwx/v3/jwk"
)

func Example_jwk_usage() {
  // Use jwk.Cache if you intend to keep reuse the JWKS over and over
  set, err := jwk.Fetch(context.Background(), "https://www.googleapis.com/oauth2/v3/certs")
  if err != nil {
    log.Printf("failed to parse JWK: %s", err)
    return
  }

  // Key sets can be serialized back to JSON
  {
    jsonbuf, err := json.Marshal(set)
    if err != nil {
      log.Printf("failed to marshal key set into JSON: %s", err)
      return
    }
    log.Printf("%s", jsonbuf)
  }

  for i := 0; i < set.Len(); i++ {
    var rawkey any // This is where we would like to store the raw key, like *rsa.PrivateKey or *ecdsa.PrivateKey
    key, ok := set.Key(i)  // This retrieves the corresponding jwk.Key
    if !ok {
      log.Printf("failed to get key at index %d", i)
      return
    }

    // jws and jwe operations can be performed using jwk.Key, but you could also
    // covert it to their "raw" forms, such as *rsa.PrivateKey or *ecdsa.PrivateKey
    if err := jwk.Export(key, &rawkey); err != nil {
      log.Printf("failed to create public key: %s", err)
      return
    }
    _ = rawkey

    // You can create jwk.Key from a raw key, too
    fromRawKey, err := jwk.Import(rawkey)
    if err != nil {
      log.Printf("failed to acquire raw key from jwk.Key: %s", err)
      return
    }

    // Keys can be serialized back to JSON
    jsonbuf, err := json.Marshal(key)
    if err != nil {
      log.Printf("failed to marshal key into JSON: %s", err)
      return
    }

    fromJSONKey, err := jwk.Parse(jsonbuf)
    if err != nil {
      log.Printf("failed to parse json: %s", err)
      return
    }
    _ = fromJSONKey
    _ = fromRawKey
  }
  // OUTPUT:
}

//nolint:govet
func Example_jwk_marshal_json() {
  // JWKs that inherently involve randomness such as RSA and EC keys are
  // not used in this example, because they may produce different results
  // depending on the environment.
  //
  // (In fact, even if you use a static source of randomness, tests may fail
  // because of internal changes in the Go runtime).

  raw := []byte("01234567890123456789012345678901234567890123456789ABCDEF")

  // This would create a symmetric key
  key, err := jwk.Import(raw)
  if err != nil {
    fmt.Printf("failed to create symmetric key: %s\n", err)
    return
  }
  if _, ok := key.(jwk.SymmetricKey); !ok {
    fmt.Printf("expected jwk.SymmetricKey, got %T\n", key)
    return
  }

  key.Set(jwk.KeyIDKey, "mykey")

  buf, err := json.MarshalIndent(key, "", "  ")
  if err != nil {
    fmt.Printf("failed to marshal key into JSON: %s\n", err)
    return
  }
  fmt.Printf("%s\n", buf)

  // OUTPUT:
  // {
  //   "k": "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODlBQkNERUY",
  //   "kid": "mykey",
  //   "kty": "oct"
  // }
}
```
source: [examples/jwk_example_test.go](https://github.com/lestrrat-go/jwx/blob/v3/examples/jwk_example_test.go)
<!-- END INCLUDE -->
