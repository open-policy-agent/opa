//go:build ignore

// This code is shared in the go.dev playground here for readers:
// https://go.dev/play/p/HMAPqDJM3jk
// The source is retained here for the reference of those working on the site
// and with the JWT examples.
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/open-policy-agent/opa/rego"
)

func main() {
	// Generate an ECDSA private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		fmt.Printf("Failed to generate private key: %v\n", err)
		return
	}

	// Create a certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Example"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	// Create a self-signed certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		fmt.Printf("Failed to create certificate: %v\n", err)
		return
	}

	// Encode the private key to PEM format
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		fmt.Printf("Failed to marshal private key: %v\n", err)
		return
	}
	privPem := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	fmt.Printf("Private Key:\n%s\n", privPem)

	// Encode the certificate to PEM format
	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	fmt.Printf("Certificate:\n%s\n", certPem)

	// Generate JWT token
	token := jwt.New()
	token.Set("email", "name@example.com")
	token.Set("groups", []string{"developers"})
	token.Set(jwt.IssuedAtKey, time.Now().Unix())
	token.Set(jwt.AudienceKey, "foo.example.com")
	token.Set(jwt.IssuerKey, "pki.example.com")
	token.Set(jwt.NotBeforeKey, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	token.Set(jwt.ExpirationKey, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	signed, err := jwt.Sign(token, jwa.ES256, priv)
	if err != nil {
		fmt.Printf("Failed to sign token: %v\n", err)
		return
	}

	fmt.Printf("JWT Token:\n%s\n", signed)
	bs, _ := json.MarshalIndent(token, "", "  ")
	fmt.Println("\nJWT Token (decoded):", string(bs))

	// Create JWK from the public key
	pubKey, err := jwk.New(&priv.PublicKey)
	if err != nil {
		fmt.Printf("Failed to create JWK from public key: %v\n", err)
		return
	}

	// Set key ID and other attributes for the public key
	pubKey.Set(jwk.KeyIDKey, "my-key-id")
	pubKey.Set(jwk.AlgorithmKey, "ES256")
	pubKey.Set(jwk.KeyUsageKey, "sig")

	// Create JWKS for the public key
	pubJwks := jwk.NewSet()
	pubJwks.Add(pubKey)

	// Marshal public JWKS to JSON
	pubJwksJson, err := json.MarshalIndent(pubJwks, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal public JWKS: %v\n", err)
		return
	}

	fmt.Printf("\nPublic JWKS:\n%s\n", pubJwksJson)

	ctx := context.Background()
	r := rego.New(
		rego.Query("io.jwt.verify_es256(input.token, input.jwks)"),
		rego.StrictBuiltinErrors(true),
		rego.Input(map[string]interface{}{
			"token": string(signed),
			"jwks":  string(pubJwksJson),
		}),
	)

	rs, err := r.Eval(ctx)
	if err != nil {
		fmt.Printf("Failed to evaluate rego: %v\n", err)
		return
	}

	if len(rs) == 0 {
		fmt.Println("Token verification failed")
	} else {
		fmt.Println("Token verification succeeded")
	}

	bs, _ = json.MarshalIndent(rs, "", "  ")
	fmt.Println("Rego Result:", string(bs))

	constraints := map[string]interface{}{
		"iss":  "pki.example.com",
		"cert": string(pubJwksJson),
	}
	r = rego.New(
		rego.Query(`io.jwt.decode_verify(input.token, input.constraints)`),
		rego.StrictBuiltinErrors(true),
		rego.Input(map[string]interface{}{
			"token":       string(signed),
			"constraints": constraints,
		}),
	)

	rs, err = r.Eval(ctx)
	if err != nil {
		fmt.Printf("Failed to evaluate rego: %v\n", err)
		return
	}

	if len(rs) == 0 {
		fmt.Println("Token decode failed")
	} else {
		fmt.Println("Token decode succeeded")
	}

	// generate copyable code for use in examples
	bs, _ = json.MarshalIndent(rs, "", "  ")
	fmt.Println("Rego Result:", string(bs))

	fmt.Println(fmt.Sprintf(`
package play

import rego.v1

token := %q

jwks := %s


o := io.jwt.decode_verify(token, {"cert": jwks })
`, signed, "`"+string(pubJwksJson)+"`"))
}
