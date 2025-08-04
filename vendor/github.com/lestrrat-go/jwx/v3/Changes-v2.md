# Incompatible Changes from v1 to v2

These are changes that are incompatible with the v1.x.x version.

* [tl;dr](#tldr) - If you don't feel like reading the details -- but you will read the details, right?
* [Detailed List of Changes](#detailed-list-of-changes) - A comprehensive list of changes from v1 to v2

# tl;dr

## JWT 

```go
// most basic
jwt.Parse(serialized, jwt.WithKey(alg, key)) // NOTE: verification and validation are ENABLED by default!
jwt.Sign(token, jwt.WithKey(alg,key))

// with a jwk.Set
jwt.Parse(serialized, jwt.WithKeySet(set))

// UseDefault/InferAlgorithm with JWKS
jwt.Parse(serialized, jwt.WithKeySet(set,
  jws.WithUseDefault(true), jws.WithInferAlgorithm(true))

// Use `jku`
jwt.Parse(serialized, jwt.WithVerifyAuto(...))

// Any other custom key provisioning (using functions in this
// example, but can be anything that fulfills jws.KeyProvider)
jwt.Parse(serialized, jwt.WithKeyProvider(jws.KeyProviderFunc(...)))
```

## JWK

```go
// jwk.New() was confusing. Renamed to fit the actual implementation
key, err := jwk.FromRaw(rawKey)

// Algorithm() now returns jwa.KeyAlgorithm type. `jws.Sign()`
// and other function that receive JWK algorithm names accept
// this new type, so you can use the same key and do the following
// (previously you needed to type assert)
jws.Sign(payload, jws.WithKey(key.Algorithm(), key))

// If you need the specific type, type assert
key.Algorithm().(jwa.SignatureAlgorithm)

// jwk.AutoRefresh is no more. Use jwk.Cache
cache := jwk.NewCache(ctx, options...)

// Certificate chains are no longer jwk.CertificateChain type, but
// *(github.com/lestrrat-go/jwx/cert).Chain
cc := key.X509CertChain() // this is *cert.Chain now
```

## JWS

```go
// basic
jws.Sign(payload, jws.WithKey(alg, key))
jws.Sign(payload, jws.WithKey(alg, key), jws.WithKey(alg, key), jws.WithJSON(true))
jws.Verify(signed, jws.WithKey(alg, key))

// other ways to pass the key
jws.Sign(payload, jws.WithKeySet(jwks))
jws.Sign(payload, jws.WithKeyProvider(kp))

// retrieve the key that succeeded in verifying
var keyUsed interface{}
jws.Verify(signed, jws.WithKeySet(jwks), jws.WithKeyUsed(&keyUsed))
```

## JWE 

```go
// basic
jwe.Encrypt(payload, jwe.WithKey(alg, key)) // other defaults are inferred
jwe.Encrypt(payload, jwe.WithKey(alg, key), jwe.WithKey(alg, key), jwe.WithJSON(true))
jwe.Decrypt(encrypted, jwe.WithKey(alg, key))

// other ways to pass the key
jwe.Encrypt(payload, jwe.WithKeySet(jwks))
jwe.Encrypt(payload, jwe.WithKeyProvider(kp))

// retrieve the key that succeeded in decrypting
var keyUsed interface{}
jwe.Verify(signed, jwe.WithKeySet(jwks), jwe.WithKeyUsed(&keyUsed))
```

# Detailed List of Changes

## Module

* Module now requires go 1.16

* Use of github.com/pkg/errors is no more. If you were relying on behavior
  that depends on the errors being an instance of github.com/pkg/errors
  then you need to change your code

* File-generation tools have been moved out of internal/ directories.
  These files pre-dates Go modules, and they were in internal/ in order
  to avoid being listed in the `go doc` -- however, now that we can
  make them separate modules this is no longer necessary.

* New package `cert` has been added to handle `x5c` certificate
  chains, and to work with certificates
    * cert.Chain to store base64 encoded ASN.1 DER format certificates
    * cert.EncodeBase64 to encode ASN.1 DER format certificate using base64
    * cert.Create to create a base64 encoded ASN.1 DER format certificates
    * cert.Parse to parse base64 encoded ASN.1 DER format certificates

## JWE

* `jwe.Compact()`'s signature has changed to 
  `jwe.Compact(*jwe.Message, ...jwe.CompactOption)`

* `jwe.JSON()` has been removed. You can generate JSON serialization
  using `jwe.Encrypt(jwe.WitJSON())` or `json.Marshal(jwe.Message)`

* `(jwe.Message).Decrypt()` has been removed. Since formatting of the
  original serialized message matters (including whitespace), using a parsed
  object was inherently confusing.

* `jwe.Encrypt()` can now generate JWE messages in either compact or JSON
  forms. By default, the compact form is used. JSON format can be
  enabled by using the `jwe.WithJSON` option.

* `jwe.Encrypt()` can now accept multiple keys by passing multiple
  `jwe.WithKey()` options. This can be used with `jwe.WithJSON` to
  create JWE messages with multiple recipients.

* `jwe.DecryptEncryptOption()` has been renamed to `jwe.EncryptDecryptOption()`.
  This is so that it is more uniform with `jws` equivalent of `jws.SignVerifyOption()`
  where the producer (`Sign`) comes before the consumer (`Verify`) in the naming

* `jwe.WithCompact` and `jwe.WithJSON` options have been added
  to control the serialization format.

* jwe.Decrypt()'s method signature has been changed to `jwt.Decrypt([]byte, ...jwe.DecryptOption) ([]byte, error)`.
  These options can be stacked. Therefore, you could configure the
  verification process to attempt a static key pair, a JWKS, and only
  try other forms if the first two fails, for example.

  - For static key pair, use `jwe.WithKey()`
  - For static JWKS, use `jwe.WithKeySet()` (NOTE: InferAlgorithmFromKey like in `jws` package is NOT supported)
  - For custom, possibly dynamic key provisioning, use `jwe.WithKeyProvider()`

* jwe.Decrypter has been unexported. Users did not need this.
 
* jwe.WithKeyProvider() has been added to specify arbitrary
  code to specify which keys to try.

* jwe.KeyProvider interface has been added

* jwe.KeyProviderFunc has been added
 
* `WithPostParser()` has been removed. You can achieve the same effect
  by using `jwe.WithKeyProvider()`. Because this was the only consumer for
  `jwe.DecryptCtx`, this type has been removed as well.

* `x5c` field type has been changed to `*cert.Chain` instead of `[]string`

* Method signature for `jwe.Parse()` has been changed to include options,
  but options are currently not used

* `jwe.ReadFile` now supports the option `jwe.WithFS` which allows you to
  read data from arbitrary `fs.FS` objects

* jwe.WithKeyUsed has been added to allow users to retrieve
  the key used for decryption. This is useful in cases you provided
  multiple keys and you want to know which one was successful

## JWK

* `jwk.New()` has been renamed to `jwk.FromRaw()`, which hopefully will
  make it easier for the users what the input should be.

* `jwk.Set` has many interface changes:
  * Changed methods to match jwk.Key and its semantics:
    * Field is now Get() (returns values for arbitrary fields other than keys). Fetching a key is done via Key()
    * Remove() now removes arbitrary fields, not keys. to remove keys, use RemoveKey()
    * Iterate has been added to iterate through all non-key fields.
  * Add is now AddKey(Key) string, and returns an error when the same key is added
  * Get is now Key(int) (Key, bool)
  * Remove is now RemoveKey(Key) error
  * Iterate is now Keys(context.Context) KeyIterator
  * Clear is now Clear() error

* `jwk.CachedSet` has been added. You can create a `jwk.Set` that is backed by
  `jwk.Cache` so you can do this:

```go
cache := jkw.NewCache(ctx)
cachedSet := jwk.NewCachedSet(cache, jwksURI)

// cachedSet is always the refreshed, cached version from jwk.Cache
jws.Verify(signed, jws.WithKeySet(cachedSet))
```

* `jwk.NewRSAPRivateKey()`, `jwk.NewECDSAPrivateKey()`, etc have been removed.
  There is no longer any way to create concrete types of `jwk.Key` 

* `jwk.Key` type no longer supports direct unmarshaling via `json.Unmarshal()`,
  because you can no longer instantiate concrete `jwk.Key` types. You will need to
  use `jwk.ParseKey()`. See the documentation for ways to parse JWKs.

* `(jwk.Key).Algorithm()` is now of `jwk.KeyAlgorithm` type. This field used
  to be `string` and therefore could not be passed directly to `jwt.Sign()`
  `jws.Sign()`, `jwe.Encrypt()`, et al. This is no longer the case, and
  now you can pass it directly. See 
  https://github.com/lestrrat-go/jwx/blob/v2/docs/99-faq.md#why-is-jwkkeyalgorithm-and-jwakeyalgorithm-so-confusing
  for more details

* `jwk.Fetcher` and `jwk.FetchFunc` has been added.
  They represent something that can fetch a `jwk.Set`

* `jwk.CertificateChain` has been removed, use `*cert.Chain`
* `x5c` field type has been changed to `*cert.Chain` instead of `[]*x509.Certificate`

* `jwk.ReadFile` now supports the option `jwk.WithFS` which allows you to
  read data from arbitrary `fs.FS` objects

* Added `jwk.PostFetcher`, `jwk.PostFetchFunc`, and `jwk.WithPostFetch` to
  allow users to get at the `jwk.Set` that was fetched in `jwk.Cache`.
  This will make it possible for users to supply extra information and edit
  `jwk.Set` after it has been fetched and parsed, but before it is cached.
  You could, for example, modify the `alg` field so that it's easier to
  work with when you use it in `jws.Verify` later.

* Reworked `jwk.AutoRefresh` in terms of `github.com/lestrrat-go/httprc`
  and renamed it `jwk.Cache`.

  Major difference between `jwk.AutoRefresh` and `jwk.Cache` is that while
  former used one `time.Timer` per resource, the latter uses a static timer
  (based on `jwk.WithRefreshWindow()` value, default 15 minutes) that periodically
  refreshes all resources that were due to be refreshed within that time frame.

  This method may cause your updates to happen slightly later, but uses significantly
  less resources and is less prone to clogging.

* Reimplemented `jwk.Fetch` in terms of `github.com/lestrrat-go/httprc`.

* Previously `jwk.Fetch` and `jwk.AutoRefresh` respected backoff options,
  but this has been removed. This is to avoid unwanted clogging of the fetch workers
  which is the default processing mode in `github.com/lestrrat-go/httprc`.

  If you are using backoffs, you need to control your inputs more carefully so as
  not to clog your fetch queue, and therefore you should be writing custom code that
  suits your needs

## JWS

* `jws.Sign()` can now generate JWS messages in either compact or JSON
  forms. By default, the compact form is used. JSON format can be
  enabled by using the `jws.WithJSON` option.

* `jws.Sign()` can now accept multiple keys by passing multiple
  `jws.WithKey()` options. This can be used with `jws.WithJSON` to
  create JWS messages with multiple signatures.

* `jws.WithCompact` and `jws.WithJSON` options have been added
  to control the serialization format.

* jws.Verify()'s method signature has been changed to `jwt.Verify([]byte, ...jws.VerifyOption) ([]byte, error)`.
  These options can be stacked. Therefore, you could configure the
  verification process to attempt a static key pair, a JWKS, and only
  try other forms if the first two fails, for example.
 
  - For static key pair, use `jws.WithKey()`
  - For static JWKS, use `jws.WithKeySet()`
  - For enabling verification using `jku`, use `jws.WithVerifyAuto()`
  - For custom, possibly dynamic key provisioning, use `jws.WithKeyProvider()`

* jws.WithVerify() has been removed.

* jws.WithKey() has been added to specify an algorithm + key to
  verify the payload with.

* jws.WithKeySet() has been added to specify a JWKS to be used for
  verification. By default `kid` AND `alg` must match between the signature
  and the key.

  The option can take further suboptions:

```go
jws.Parse(serialized,
  jws.WithKeySet(set,
    // by default `kid` is required. set false to disable.
    jws.WithRequireKid(false),
    // optionally skip matching kid if there's exactly one key in set
    jws.WithUseDefault(true),
    // infer algorithm name from key type
    jws.WithInferAlgorithm(true),
  ),
)
```

* `jws.VerifuAuto` has been removed in favor of using
  `jws.WithVerifyAuto` option with `jws.Verify()`

* `jws.WithVerifyAuto` has been added to enable verification
  using `jku`.

  The first argument must be a jwk.Fetcher object, but can be
  set to `nil` to use the default implementation which is `jwk.Fetch`

  The rest of the arguments are treated as options passed to the
  `(jwk.Fetcher).Fetch()` function.

* Remove `jws.WithPayloadSigner()`. This should be completely replaceable
  using `jws.WithKey()`

* jws.WithKeyProvider() has been added to specify arbitrary
  code to specify which keys to try.

* jws.KeyProvider interface has been added

* jws.KeyProviderFunc has been added
 
* jws.WithKeyUsed has been added to allow users to retrieve
  the key used for verification. This is useful in cases you provided
  multiple keys and you want to know which one was successful

* `x5c` field type has been changed to `*cert.Chain` instead of `[]string`

* `jws.ReadFile` now supports the option `jws.WithFS` which allows you to
  read data from arbitrary `fs.FS` objects

## JWT

* `jwt.Parse` now verifies the signature and validates the token
  by default. You must disable it explicitly using `jwt.WithValidate(false)`
  and/or `jwt.WithVerify(false)` if you only want to parse the JWT message.

  If you don't want either, a convenience function `jwt.ParseInsecure`
  has been added.

* `jwt.Parse` can only parse raw JWT (JSON) or JWS (JSON or Compact).
  It no longer accepts JWE messages.

* `jwt.WithDecrypt` has been removed

* `jwt.WithJweHeaders` has been removed

* `jwt.WithVerify()` has been renamed to `jwt.WithKey()`. The option can
  be used for signing, encryption, and parsing.

* `jwt.Validator` has been changed to return `jwt.ValidationError`.
  If you provide a custom validator, you should wrap the error with
  `jwt.NewValidationError()`

* `jwt.UseDefault()` has been removed. You should use `jws.WithUseDefault()`
  as a suboption in the `jwt.WithKeySet()` option.

```go
jwt.Parse(serialized, jwt.WithKeySet(set, jws.WithUseDefault(true)))
```

* `jwt.InferAlgorithmFromKey()` has been removed. You should use
  `jws.WithInferAlgorithmFromKey()` as a suboption in the `jwt.WithKeySet()` option.

```go
jwt.Parse(serialized, jwt.WithKeySet(set, jws.WithInferAlgorithmFromKey(true)))
```

* jwt.WithKeySetProvider has been removed. Use `jwt.WithKeyProvider()`
  instead. If jwt.WithKeyProvider seems a bit complicated,  use a combination of
  JWS parse, no-verify/validate JWT parse, and an extra JWS verify:

```go
msg, _ := jws.Parse(signed)
token, _ := jwt.Parse(msg.Payload(), jwt.WithVerify(false), jwt.WithValidate(false))
// Get information out of token, for example, `iss`
switch token.Issuer() {
case ...:
  jws.Verify(signed, jwt.WithKey(...))
}
```

* `jwt.WithHeaders` and `jwt.WithJwsHeaders` have been removed.
  You should be able to use the new `jwt.WithKey` option to pass headers

* `jwt.WithSignOption` and `jwt.WithEncryptOption` have been added as
  escape hatches for options that are declared in `jws` and `jwe` packages
  but not in `jwt`

* `jwt.ReadFile` now supports the option `jwt.WithFS` which allows you to
  read data from arbitrary `fs.FS` objects

* `jwt.Sign()` has been changed so that it works more like the new `jws.Sign()`

