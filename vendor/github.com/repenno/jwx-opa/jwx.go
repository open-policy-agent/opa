// Package jwx contains tools that deal with the various JWx (JOSE)
// technologies such as JWT, JWS, JWE, etc in Go.
//
//    JWS (https://tools.ietf.org/html/rfc7515)
//    JWE (https://tools.ietf.org/html/rfc7516)
//    JWK (https://tools.ietf.org/html/rfc7517)
//    JWA (https://tools.ietf.org/html/rfc7518)
//    JWT (https://tools.ietf.org/html/rfc7519)
//
// The primary focus of this library tool set is to implement the extremely
// flexible OAuth2 / OpenID Connect protocols. There are many other libraries
// out there that deal with all or parts of these JWx technologies:
//
//    https://github.com/dgrijalva/jwt-go
//    https://github.com/square/go-jose
//    https://github.com/coreos/oidc
//    https://golang.org/x/oauth2
//
// This library exists because there was a need for a toolset that encompasses
// the whole set of JWx technologies in a highly customizable manner, in one package.
//
// You can find more high level documentation at Github (https://github.com/lestrrat-go/jwx)
package jwx

// Version describes the version of this library.
const Version = "0.0.1"
