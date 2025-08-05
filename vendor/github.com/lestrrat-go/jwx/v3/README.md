# github.com/lestrrat-go/jwx/v3 [![CI](https://github.com/lestrrat-go/jwx/actions/workflows/ci.yml/badge.svg)](https://github.com/lestrrat-go/jwx/actions/workflows/ci.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/lestrrat-go/jwx/v3.svg)](https://pkg.go.dev/github.com/lestrrat-go/jwx/v3) [![codecov.io](https://codecov.io/github/lestrrat-go/jwx/coverage.svg?branch=v3)](https://codecov.io/github/lestrrat-go/jwx?branch=v3)

Go module implementing various JWx (JWA/JWE/JWK/JWS/JWT, otherwise known as JOSE) technologies.

If you are using this module in your product or your company, please add  your product and/or company name in the [Wiki](https://github.com/lestrrat-go/jwx/wiki/Users)! It really helps keeping up our motivation.

# Features

* Complete coverage of JWA/JWE/JWK/JWS/JWT, not just JWT+minimum tool set.
  * Supports JWS messages with multiple signatures, both compact and JSON serialization
  * Supports JWS with detached payload
  * Supports JWS with unencoded payload (RFC7797)
  * Supports JWE messages with multiple recipients, both compact and JSON serialization
  * Most operations work with either JWK or raw keys e.g. *rsa.PrivateKey, *ecdsa.PrivateKey, etc).
* Opinionated, but very uniform API. Everything is symmetric, and follows a standard convention
  * jws.Parse/Verify/Sign
  * jwe.Parse/Encrypt/Decrypt
  * Arguments are organized as explicit required parameters and optional WithXXXX() style options.
* Extra utilities
  * `jwk.Cache` to always keep a JWKS up-to-date
  * [bazel](https://bazel.build)-ready

Some more in-depth discussion on why you might want to use this library over others
can be found in the [Description section](#description)

If you are using v0 or v1, you are strongly encouraged to migrate to using v3
(the version that comes with the README you are reading).

# SYNOPSIS

<!-- INCLUDE(examples/jwx_readme_example_test.go) -->
```go
package examples_test

import (
  "bytes"
  "fmt"
  "net/http"
  "time"

  "github.com/lestrrat-go/jwx/v3/jwa"
  "github.com/lestrrat-go/jwx/v3/jwe"
  "github.com/lestrrat-go/jwx/v3/jwk"
  "github.com/lestrrat-go/jwx/v3/jws"
  "github.com/lestrrat-go/jwx/v3/jwt"
)

func Example() {
  // Parse, serialize, slice and dice JWKs!
  privkey, err := jwk.ParseKey(jsonRSAPrivateKey)
  if err != nil {
    fmt.Printf("failed to parse JWK: %s\n", err)
    return
  }

  pubkey, err := jwk.PublicKeyOf(privkey)
  if err != nil {
    fmt.Printf("failed to get public key: %s\n", err)
    return
  }

  // Work with JWTs!
  {
    // Build a JWT!
    tok, err := jwt.NewBuilder().
      Issuer(`github.com/lestrrat-go/jwx`).
      IssuedAt(time.Now()).
      Build()
    if err != nil {
      fmt.Printf("failed to build token: %s\n", err)
      return
    }

    // Sign a JWT!
    signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), privkey))
    if err != nil {
      fmt.Printf("failed to sign token: %s\n", err)
      return
    }

    // Verify a JWT!
    {
      verifiedToken, err := jwt.Parse(signed, jwt.WithKey(jwa.RS256(), pubkey))
      if err != nil {
        fmt.Printf("failed to verify JWS: %s\n", err)
        return
      }
      _ = verifiedToken
    }

    // Work with *http.Request!
    {
      req, err := http.NewRequest(http.MethodGet, `https://github.com/lestrrat-go/jwx`, nil)
      req.Header.Set(`Authorization`, fmt.Sprintf(`Bearer %s`, signed))

      verifiedToken, err := jwt.ParseRequest(req, jwt.WithKey(jwa.RS256(), pubkey))
      if err != nil {
        fmt.Printf("failed to verify token from HTTP request: %s\n", err)
        return
      }
      _ = verifiedToken
    }
  }

  // Encrypt and Decrypt arbitrary payload with JWE!
  {
    encrypted, err := jwe.Encrypt(payloadLoremIpsum, jwe.WithKey(jwa.RSA_OAEP(), jwkRSAPublicKey))
    if err != nil {
      fmt.Printf("failed to encrypt payload: %s\n", err)
      return
    }

    decrypted, err := jwe.Decrypt(encrypted, jwe.WithKey(jwa.RSA_OAEP(), jwkRSAPrivateKey))
    if err != nil {
      fmt.Printf("failed to decrypt payload: %s\n", err)
      return
    }

    if !bytes.Equal(decrypted, payloadLoremIpsum) {
      fmt.Printf("verified payload did not match\n")
      return
    }
  }

  // Sign and Verify arbitrary payload with JWS!
  {
    signed, err := jws.Sign(payloadLoremIpsum, jws.WithKey(jwa.RS256(), jwkRSAPrivateKey))
    if err != nil {
      fmt.Printf("failed to sign payload: %s\n", err)
      return
    }

    verified, err := jws.Verify(signed, jws.WithKey(jwa.RS256(), jwkRSAPublicKey))
    if err != nil {
      fmt.Printf("failed to verify payload: %s\n", err)
      return
    }

    if !bytes.Equal(verified, payloadLoremIpsum) {
      fmt.Printf("verified payload did not match\n")
      return
    }
  }
  // OUTPUT:
}
```
source: [examples/jwx_readme_example_test.go](https://github.com/lestrrat-go/jwx/blob/v3/examples/jwx_readme_example_test.go)
<!-- END INCLUDE -->

# How-to Documentation

* [API documentation](https://pkg.go.dev/github.com/lestrrat-go/jwx/v3)
* [How-to style documentation](./docs)
* [Runnable Examples](./examples)

# Description

This Go module implements JWA, JWE, JWK, JWS, and JWT. Please see the following table for the list of
available packages:

| Package name                                              | Notes                                           |
|-----------------------------------------------------------|-------------------------------------------------|
| [jwt](https://github.com/lestrrat-go/jwx/tree/v3/jwt) | [RFC 7519](https://tools.ietf.org/html/rfc7519) |
| [jwk](https://github.com/lestrrat-go/jwx/tree/v3/jwk) | [RFC 7517](https://tools.ietf.org/html/rfc7517) + [RFC 7638](https://tools.ietf.org/html/rfc7638) |
| [jwa](https://github.com/lestrrat-go/jwx/tree/v3/jwa) | [RFC 7518](https://tools.ietf.org/html/rfc7518) |
| [jws](https://github.com/lestrrat-go/jwx/tree/v3/jws) | [RFC 7515](https://tools.ietf.org/html/rfc7515) + [RFC 7797](https://tools.ietf.org/html/rfc7797) |
| [jwe](https://github.com/lestrrat-go/jwx/tree/v3/jwe) | [RFC 7516](https://tools.ietf.org/html/rfc7516) |
## History

My goal was to write a server that heavily uses JWK and JWT. At first glance
the libraries that already exist seemed sufficient, but soon I realized that

1. To completely implement the protocols, I needed the entire JWT, JWK, JWS, JWE (and JWA, by necessity).
2. Most of the libraries that existed only deal with a subset of the various JWx specifications that were necessary to implement their specific needs

For example, a certain library looks like it had most of JWS, JWE, JWK covered, but then it lacked the ability to include private claims in its JWT responses. Another library had support of all the private claims, but completely lacked in its flexibility to generate various different response formats.

Because I was writing the server side (and the client side for testing), I needed the *entire* JOSE toolset to properly implement my server, **and** they needed to be *flexible* enough to fulfill the entire spec that I was writing.

So here's `github.com/lestrrat-go/jwx/v3`. This library is extensible, customizable, and hopefully well organized to the point that it is easy for you to slice and dice it.

## Why would I use this library?

There are several other major Go modules that handle JWT and related data formats,
so why should you use this library?

From a purely functional perspective, the only major difference is this:
Whereas most other projects only deal with what they seem necessary to handle
JWTs, this module handles the **_entire_** spectrum of JWS, JWE, JWK, and JWT.

That is, if you need to not only parse JWTs, but also to control JWKs, or
if you need to handle payloads that are NOT JWTs, you should probably consider
using this module. You should also note that JWT is built _on top_ of those
other technologies. You simply cannot have a complete JWT package without
implementing the entirety of JWS/JWE/JWK, which this library does.

Next, from an implementation perspective, this module differs significantly
from others in that it tries very hard to expose only the APIs, and not the
internal data. For example, individual JWT claims are not accessible through
struct field lookups. You need to use one of the getter methods.

This is because this library takes the stance that the end user is fully capable
and even willing to shoot themselves on the foot when presented with a lax
API. By making sure that users do not have access to open structs, we can protect
users from doing silly things like creating _incomplete_ structs, or access the
structs concurrently without any protection. This structure also allows
us to put extra smarts in the structs, such as doing the right thing when
you want to parse / write custom fields (this module does not require the user
to specify alternate structs to parse objects with custom fields)

In the end I think it comes down to your usage pattern, and priorities.
Some general guidelines that come to mind are:

* If you want a single library to handle everything JWx, such as using JWE, JWK, JWS, handling [auto-refreshing JWKs](https://github.com/lestrrat-go/jwx/blob/v3/docs/04-jwk.md#auto-refreshing-remote-keys), use this module.
* If you want to honor all possible custom fields transparently, use this module.
* If you want a standardized clean API, use this module.

Otherwise, feel free to choose something else.

# Contributions

## Issues

For bug reports and feature requests, please try to follow the issue templates as much as possible.
For either bug reports or feature requests, failing tests are even better.

## Pull Requests

Please make sure to include tests that exercise the changes you made.

If you are editing auto-generated files (those files with the `_gen.go` suffix, please make sure that you do the following:

1. Edit the generator, not the generated files (e.g. internal/cmd/genreadfile/main.go)
2. Run `make generate` (or `go generate`) to generate the new code
3. Commit _both_ the generator _and_ the generated files

## Discussions / Usage

Please try [discussions](https://github.com/lestrrat-go/jwx/tree/v3/discussions) first.

# Related Modules

* [github.com/lestrrat-go/echo-middleware-jwx](https://github.com/lestrrat-go/echo-middleware-jwx) - Sample Echo middleware
* [github.com/jwx-go/crypto-signer/gcp](https://github.com/jwx-go/crypto-signer/tree/main/gcp) - GCP KMS wrapper that implements [`crypto.Signer`](https://pkg.go.dev/crypto#Signer)
* [github.com/jwx-go/crypto-signer/aws](https://github.com/jwx-go/crypto-signer/tree/main/aws) - AWS KMS wrapper that implements [`crypto.Signer`](https://pkg.go.dev/crypto#Signer)

# Credits

* Initial work on this library was generously sponsored by HDE Inc (https://www.hde.co.jp)
* Lots of code, especially JWE was initially taken from go-jose library (https://github.com/square/go-jose)
* Lots of individual contributors have helped this project over the years. Thank each and everyone of you very much.

# Quid pro quo

If you use this software to build products in a for-profit organization, we ask you to _consider_
contributing back to FOSS in the following manner:

* For every 100 employees (direct hires) of your organization, please consider contributing minimum of $1 every year to either this project, **or** another FOSS projects that this project uses. For example, for 100 employees, we ask you contribute $100 yearly; for 10,000 employees, we ask you contribute $10,000 yearly.
* If possible, please make this information public. You do not need to disclose the amount you are contributing, but please make the information that you are contributing to particular FOSS projects public. For this project, please consider writing your name on the [Wiki](https://github.com/lestrrat-go/jwx/wiki/Users)

This is _NOT_ a licensing term: you are still free to use this software according to the license it
comes with. This clause is only a plea for people to acknowledge the work from FOSS developers whose
work you rely on each and everyday.
