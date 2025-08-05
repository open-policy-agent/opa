// Package jwk implements JWK as described in https://tools.ietf.org/html/rfc7517
//
// This package implements jwk.Key to represent a single JWK, and jwk.Set to represent
// a set of JWKs.
//
// The `jwk.Key` type is an interface, which hides the underlying implementation for
// each key type. Each key type can further be converted to interfaces for known
// types, such as `jwk.ECDSAPrivateKey`, `jwk.RSAPublicKey`, etc. This may not necessarily
// work for third party key types (see section on "Registering a key type" below).
//
// Users can create a JWK in two ways. One is to unmarshal a JSON representation of a
// key. The second one is to use `jwk.Import()` to import a raw key and convert it to
// a jwk.Key.
//
// # Simple Usage
//
// You can parse a JWK from a JSON payload:
//
//	jwk.ParseKey([]byte(`{"kty":"EC",...}`))
//
// You can go back and forth between raw key types and JWKs:
//
//	jwkKey, _ := jwk.Import(rsaPrivateKey)
//	var rawKey *rsa.PRrivateKey
//	jwkKey.Raw(&rawKey)
//
// You can use them to sign/verify/encrypt/decrypt:
//
//	jws.Sign([]byte(`...`), jws.WithKey(jwa.RS256, jwkKey))
//	jwe.Encrypt([]byte(`...`), jwe.WithKey(jwa.RSA_OAEP, jwkKey))
//
// See examples/jwk_parse_example_test.go and other files in the exmaples/ directory for more.
//
// # Advanced Usage: Registering a custom key type and conversion routines
//
// Caveat Emptor: Functionality around registering keys
// (KeyProbe/KeyParser/KeyImporter/KeyExporter) should be considered experimental.
// While we expect that the functionality itself will remain, the API may
// change in backward incompatible ways, even during minor version
// releases.
//
// ## tl;dr
//
// * KeyProbe: Used for parsing JWKs in JSON format. Probes hint fields to be used for later parsing by KeyParser
// * KeyParser: Used for parsing JWKs in JSON format. Parses the JSON payload into a jwk.Key using the KeyProbe as hint
// * KeyImporter: Used for converting raw key into jwk.Key.
// * KeyExporter: Used for converting jwk.Key into raw key.
//
// ## Overview
//
// You can add the ability to use a JWK type that this library does not
// implement out of the box. You can do this by registering your own
// KeyParser, KeyImporter, and KeyExporter instances.
//
//	func init() {
//	  jwk.RegiserProbeField(reflect.StructField{Name: "SomeHint", Type: reflect.TypeOf(""), Tag: `json:"some_hint"`})
//	  jwk.RegisterKeyParser(&MyKeyParser{})
//	  jwk.RegisterKeyImporter(&MyKeyImporter{})
//	  jwk.RegisterKeyExporter(&MyKeyExporter{})
//	}
//
// The KeyParser is used to parse JSON payloads and conver them into a jwk.Key.
// The KeyImporter is used to convert a raw key (e.g. *rsa.PrivateKey, *ecdsa.PrivateKey, etc) into a jwk.Key.
// The KeyExporter is used to convert a jwk.Key into a raw key.
//
// Although we believe the mechanism has been streamline quite a lot, it is also true
// that the entire process of parsing and converting keys are much more convoluted than you might
// think. Please know before hand that if you intend to add support for a new key type,
// it _WILL_ require you to learn this module pretty much in-and-out.
//
// Read on for more explanation.
//
// ## Registering a KeyParser
//
// In order to understand how parsing works, we need to explain how the `jwk.ParseKey()` works.
//
// The first thing that occurs when parsing a key is a partial
// unmarshaling of the payload into a hint / probe object.
//
// Because the `json.Unmarshal` works by calling the `UnmarshalJSON`
// method on a concrete object, we need to create a concrete object first.
// In order/ to create the appropriate Go object, we need to know which concrete
// object to create from the JSON payload, meaning we need to peek into the
// payload and figure out what type of key it is.
//
// In order to do this, we effectively need to parse the JSON payload twice.
// First, we "probe" the payload to figure out what kind of key it is, then
// we parse it again to create the actual key object.
//
// For probing, we create a new "probe" object (KeyProbe, which is not
// directly available to end users) to populate the object with hints from the payload.
// For example, a JWK representing an RSA key would look like:
//
//	{ "kty": "RSA", "n": ..., "e": ..., ... }
//
// The default KeyProbe is constructed to unmarshal "kty" and "d" fields,
// because that is enough information to determine what kind of key to
// construct.
//
// For example, if the payload contains "kty" field with the value "RSA",
// we know that it's an RSA key. If it contains "EC", we know that it's
// an EC key. Furthermore, if the payload contains some value in the "d" field, we can
// also tell that this is a private key, as only private keys need
// this field.
//
// For most cases, the default KeyProbe implementation should be sufficient.
// However, there may be cases in the future where there are new key types
// that require further information. Perhaps you are embedding another hint
// in your JWK to further specify what kind of key it is. In that case, you
// would need to probe more.
//
// Normally you can only change how an object is unmarshaled by specifying
// JSON tags when defining a struct, but we use `reflect` package capabilities
// to create an object dynamically, which is shared among all parsing operations.
//
// To add a new field to be probed, you need to register a new `reflect.StructField`
// object that has all of the information. For example, the code below would
// register a field named "MyHint" that is of type string, and has a JSON tag
// of "my_hint".
//
//	jwk.RegisterProbeField(reflect.StructField{Name: "MyHint", Type: reflect.TypeOf(""), Tag: `json:"my_hint"`})
//
// The value of this field can be retrieved by calling `Get()` method on the
// KeyProbe object (from the `KeyParser`'s `ParseKey()` method discussed later)
//
//	var myhint string
//	_ = probe.Get("MyHint", &myhint)
//
//	var kty string
//	_ = probe.Get("Kty", &kty)
//
// This mechanism allows you to be flexible when trying to determine the key type
// to instantiate.
//
// ## Parse via the KeyParser
//
// When `jwk.Parse` / `jwk.ParseKey` is called, the library will first probe
// the payload as discussed above.
//
// Once the probe is done, the library will iterate over the registered parsers
// and attempt to parse the key by calling their `ParseKey()` methods.
//
// The parsers will be called in reverse order that they were registered.
// This means that it will try all parsers that were registered by third
// parties, and once those are exhausted, the default parser will be used.
//
// Each parser's `ParseKey()“ method will receive three arguments: the probe object, a
// KeyUnmarshaler, and the raw payload. The probe object can be used
// as a hint to determine what kind of key to instantiate. An example
// pseudocode may look like this:
//
//	var kty string
//	_ = probe.Get("Kty", &kty)
//	switch kty {
//	case "RSA":
//	  // create an RSA key
//	case "EC":
//	  // create an EC key
//	...
//	}
//
// The `KeyUnmarshaler` is a thin wrapper around `json.Unmarshal`. It works almost
// identical to `json.Unmarshal`, but it allows us to add extra magic that is
// specific to this library (which users do not need to be aware of) before calling
// the actual `json.Unmarshal`. Please use the `KeyUnmarshaler` to unmarshal JWKs instead of `json.Unmarshal`.
//
// Putting it all together, the boiler plate for registering a new parser may look like this:
//
//	   func init() {
//	     jwk.RegisterFieldProbe(reflect.StructField{Name: "MyHint", Type: reflect.TypeOf(""), Tag: `json:"my_hint"`})
//	     jwk.RegisterParser(&MyKeyParser{})
//	   }
//
//	   type MyKeyParser struct { ... }
//	   func(*MyKeyParser) ParseKey(rawProbe *KeyProbe, unmarshaler KeyUnmarshaler, data []byte) (jwk.Key, error) {
//	     // Create concrete type
//	     var hint string
//	     if err := probe.Get("MyHint", &hint); err != nil {
//	        // if it doesn't have the `my_hint` field, it probably means
//	        // it's not for us, so we return ContinueParseError so that
//	        // the next parser can pick it up
//	        return nil, jwk.ContinueParseError()
//	     }
//
//	     // Use hint to determine concrete key type
//	     var key jwk.Key
//	     switch hint {
//	     case ...:
//	      key = = myNewAwesomeJWK()
//			  ...
//	     }
//
//	     return unmarshaler.Unmarshal(data, key)
//	   }
//
// ## Registering KeyImporter/KeyExporter
//
// If you are going to do anything with the key that was parsed by your KeyParser,
// you will need to tell the library how to convert back and forth between
// raw keys and JWKs. Conversion from raw keys to jwk.Keys are done by KeyImporters,
// and conversion from jwk.Keys to raw keys are done by KeyExporters.
//
// ## Using jwk.Import() using KeyImporter
//
// Each KeyImporter is hooked to run against a specific raw key type.
//
// When `jwk.Import()` is called, the library will iterate over all registered
// KeyImporters for the specified raw key type, and attempt to convert the raw
// key to a JWK by calling the `Import()` method on each KeyImporter.
//
// The KeyImporter's `Import()` method will receive the raw key to be converted,
// and should return a JWK or an error if the conversion fails, or the return
// `jwk.ContinueError()` if the specified raw key cannot be handled by ths/ KeyImporter.
//
// Once a KeyImporter is available, you will be able to pass the raw key to `jwk.Import()`.
// The following example shows how you might register a KeyImporter for a hypotheical
// mypkg.SuperSecretKey:
//
//	 	jwk.RegisterKeyImporter(&mypkg.SuperSecretKey{}, jwk.KeyImportFunc(imnportSuperSecretKey))
//
//	 	func importSuperSecretKey(key any) (jwk.Key, error) {
//	 	  mykey, ok := key.(*mypkg.SuperSecretKey)
//	 	  if !ok {
//	         // You must return jwk.ContinueError here, or otherwise
//	         // processing will stop with an error
//	 	    return nil, fmt.Errorf("invalid key type %T for importer: %w", key, jwk.ContinueError())
//	 	  }
//
//	 	  return mypkg.SuperSecretJWK{ .... }, nil // You could reuse existing JWK types if you can
//			}
//
// ## Registering a KeyExporter
//
// KeyExporters are the opposite of KeyImporters: they convert a JWK to a raw key when `key.Raw(...)` is
// called. If you intend to use `key.Raw(...)` for a JWK created using one of your KeyImporters,
// you will also
//
// KeyExporters are registered by key type. For example, if you want to register a KeyExporter for
// RSA keys, you would do:
//
//	jwk.RegisterKeyExporter(jwa.RSA, jwk.KeyExportFunc(exportRSAKey))
//
// For a given JWK, it will be passed a "destination" object to store the exported raw key. For example,
// an RSA-based private JWK can be exported to a `*rsa.PrivateKey` or to a `*any`, but not
// to a `*ecdsa.PrivateKey`:
//
//	var dst *rsa.PrivateKey
//	key.Raw(&dst) // OK
//
//	var dst any
//	key.Raw(&dst) // OK
//
//	var dst *ecdsa.PrivateKey
//	key.Raw(&dst) // Error, if key is an RSA key
//
// You will need to handle this distinction yourself in your KeyImporter. For example, certain
// elliptic curve keys can be expressed in JWK in the same format, minus the "kty". In that case
// you will need to check for the type of the destination object and return an error if it is
// not compatible with your key.
//
//	var raw mypkg.PrivateKey // assume a hypothetical private key type using a different curve than standard ones lie P-256
//	key, _ := jwk.Import(raw)
//	// key could be jwk.ECDSAPrivateKey, with different curve than P-256
//
//	var dst *ecdsa.PrivateKey
//	key.Raw(&dst) // your KeyImporter will be called with *ecdsa.PrivateKey, which is not compatible with your key
//
// To implement this your code should look like the following:
//
//	jwk.RegisterKeyExporter(jwk.EC, jwk.KeyExportFunc(exportMyKey))
//
//	func exportMyKey(key jwk.Key, hint any) (any, error) {
//	   // check if the type of object in hint is compatible with your key
//	   switch hint.(type) {
//	   case *mypkg.PrivateKey, *any:
//	     // OK, we can proceed
//	   default:
//	     // Not compatible, return jwk.ContinueError
//	     return nil, jwk.ContinueError()
//	   }
//
//	   // key is a jwk.ECDSAPrivateKey or jwk.ECDSAPublicKey
//	   switch key := key.(type) {
//	   case jwk.ECDSAPrivateKey:
//	      // convert key to mypkg.PrivateKey
//	   case jwk.ECDSAPublicKey:
//	      // convert key to mypkg.PublicKey
//	   default:
//	     // Not compatible, return jwk.ContinueError
//	     return nil, jwk.ContinueError()
//	   }
//	   return ..., nil
//	}
package jwk
