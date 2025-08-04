# Incompatible Changes from v2 to v3

These are changes that are incompatible with the v2.x.x version.

# Detailed list of changes

## Module

* This module now requires Go 1.23

* All `xxx.Get()` methods have been changed from `Get(string) (interface{}, error)` to
  `Get(string, interface{}) error`, where the second argument should be a pointer
  to the storage destination of the field.

* All convenience accessors (e.g. `(jwt.Token).Subject`) now return `(T, bool)` instead of
  `T`. If you want an accessor that returns a single value, consider using `Get()`

* Most major errors can now be differentiated using `errors.Is`

## JWA

* All string constants have been renamed to equivalent functions that return a struct.
  You should rewrite `jwa.RS256` as `jwa.RS256()` and so forth.

* By default, only known algorithm names are accepted. For example, in our JWK tests,
  there are tests that deal with "ECMR" algorithm, but this will now fail by default.
  If you want this algorithm to succeed parsing, you need to call `jwa.RegisterXXXX`
  functions before using them.

* Previously, unmarshaling unquoted strings used to work (e.g. `var s = "RS256"`),
  but now they must conform to the JSON standard and be quoted (e.g. `var s = strconv.Quote("RS256")`)

## JWT

* All convenience accessors (e.g. `Subject`) now return `(T, bool)` instead of
  just `T`. If you want a single return value accessor, use `Get(dst) error` instead.

* Validation used to work for `iat`, `nbf`, `exp` fields where these fields were
  set to the explicit time.Time{} zero value, but now the _presence_ of these fields matter.

* Validation of fields related to time used to be truncated to one second accuracy,
  but no longer does so. To restore old behavior, you can either change the global settings by
  calling `jwt.Settings(jwt.WithTruncation(time.Second))`, or you can
  change it by each invocation by using `jwt.Validate(..., jwt.WithTruncation(time.Second))`

* Error names have been renamed. For example `jwt.ErrInvalidJWT` has been renamed to
  `jwt.UnknownPayloadTypeError` to better reflect what the error means. For other errors,
  `func ErrXXXX()` have generally been renamed to `func XXXError()`

* Validation errors are now wrapped. While `Validate()` returns a `ValidateError()` type,
  it can also be matched against more specific error types such as `TokenExpierdError()`
  using `errors.Is`

* `jwt.ErrMissingRequiredClaim` has been removed

## JWS

* Iterators have been completely removed.
* As a side effect of removing iterators, some methods such as `Copy()` lost the
  `context.Context` argument

* All convenience accessors (e.g. `Algorithm`) now return `(T, bool)` instead of
  just `T`. If you want a single return value accessor, use `Get(dst) error` instead.

* Errors from `jws.Sign` and `jws.Verify`, as well as `jws.Parse` (and friends)
  can now be differentiated by using `errors.Is`. All `jws.IsXXXXError` functions
  have been removed.

## JWE

* Iterators have been completely removed.
* As a side effect of removing iterators, some methods such as `Copy()` lost the
  `context.Context` argument

* All convenience accessors (e.g. `Algorithm`) now return `(T, bool)` instead of
  just `T`. If you want a single return value accessor, use `Get(dst) error` instead.

* Errors from `jwe.Decrypt` and `jwe.Encrypt`, as well as `jwe.Parse` (and friends)
  can now be differentiated by using `errors.Is`. All `jwe.IsXXXXrror` functions
  have been removed.

## JWK

* All convenience accessors (e.g. `Algorithm`, `Crv`) now return `(T, bool)` instead
  of just `T`, except `KeyType`, which _always_ returns a valid value. If you want a
  single return value accessor, use `Get(dst) error` instead.

* `jwk.KeyUsageType` can now be configured so that it's possible to assign values
  other than "sig" and "enc" via `jwk.RegisterKeyUsage()`. Furthermore, strict
  checks can be turned on/off against these registered values

* `jwk.Cache` has been completely re-worked based on github.com/lestrrat-go/httprc/v3.
  In particular, the default whitelist mode has changed from "block everything" to
  "allow everything".

* Experimental secp256k1 encoding/decoding for PEM encoded ASN.1 DER Format 
  has been removed. Instead, `jwk.PEMDecoder` and `jwk.PEMEncoder` have been
  added to support those who want to perform non-standard PEM encoding/decoding

* Iterators have been completely removed.

* `jwk/x25519` has been removed. To use X25519 keys, use `(crypto/ecdh).PrivateKey` and
  `(crypto/ecdh).PublicKey`. Similarly, internals have been reworked to use `crypto/ecdh`

* Parsing has completely been reworked. It is now possible to add your own `jwk.KeyParser`
  to generate a custom `jwk.Key` that this library may not natively support. Also see
  `jwk.RegisterKeyParser()`

* `jwk.KeyProbe` has been added to aid probing the JSON message. This is used to
  guess the type of key described in the JSON message before deciding which concrete
  type to instantiate, and aids implementing your own `jwk.KeyParser`. Also see
  `jwk.RegisterKeyProbe()`

* Conversion between raw keys and `jwk.Key` can be customized using `jwk.KeyImporter` and `jwk.KeyExporter`.
  Also see `jwk.RegisterKeyImporter()` and `jwk.RegisterKeyExporter()`

* Added `jwk/ecdsa` to keep track of which curves are available for ECDSA keys.

* `(jwk.Key).Raw()` has been deprecated. Use `jwk.Export()` instead to convert `jwk.Key`
  objects into their "raw" versions (e.g. `*rsa.PrivateKey`, `*ecdsa.PrivateKey`, etc).
  This is to allow third parties to register custom key types that this library does not
  natively support: Whereas a method must be bound to an object, and thus does not necessarily
  have a way to hook into a global settings (i.e. custom exporter/importer) for arbitrary
  key types, if the entrypoint is a function it's much easier and cleaner to for third-parties
  to take advantage and hook into the mechanisms.

* `jwk.FromRaw()` has been derepcated. Use `jwk.Import()` instead to convert "raw"
  keys (e.g. `*rsa.PrivateKEy`, `*Ecdsa.PrivateKey`, etc) int `jwk.Key`s.

* `(jwk.Key).FromRaw()` has been deprecated. The method `(jwk.Key).Import()` still exist for
  built-in types, but it is no longer part of any public API (`interface{}`).

* `jwk.Fetch` is marked as a simple wrapper around `net/http` and `jwk.Parse`.

* `jwk.SetGlobalFetcher` has been deprecated.

* `jwk.Fetcher` has been clearly marked as something that has limited
  usage for `jws.WithVerifyAuto`

* `jwk.Key` with P256/P386/P521 curves can be exporrted to `ecdh.PrivateKey`/`ecdh.PublicKey`