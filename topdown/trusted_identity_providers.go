package topdown

import (
	"errors"
	"fmt"
	"github.com/coreos/go-oidc"
	jwt "github.com/square/go-jose/v3/jwt"
	"golang.org/x/net/context"
	"sync"
	"time"
)


var globalTrustedIdProviderManager TrustedIdProviderManagerImpl
var globalTrustedIssuerContextTimeout = 30 * time.Second

// May be used to manage an token verification against an entire collection
// of trusted IdP's.
type TrustedIdProviderManager interface {

	// On nil error, IDToken contains the parsed jwt token iff there exist
	// at least one trusted identity provider in this managed set of IdP's
	// that successfully validated this token. Else, (if not valid or not
	// trusted or an error is encountered), a non-nil error returned and
	// IDToken is nil.
	VerifyToken(token *string) (*oidc.IDToken, error)
}

// Private struct to implement interface, TrustedIdProviderManager.
// We use a sync map to safely manage a collection of trusted
// issuers and their verifiers.
type TrustedIdProviderManagerImpl struct {

	// Thread safe Map of string of trustedIssuer to *oidc.IDTokenVerifier trustedIssuerVerifier.
	trustedVerifiers sync.Map

}

func CreateOrGetVerifier(idp *string) (*oidc.IDTokenVerifier, error) {

	// If we already have a verifier for this issuer, use it.
	if loadedVerifier, ok := globalTrustedIdProviderManager.trustedVerifiers.Load(*idp); ok {
		return loadedVerifier.(*oidc.IDTokenVerifier), nil
	}

	// Create a new issuer verifier, save it, and use it.
	ctx, _ := context.WithTimeout(context.Background(), globalTrustedIssuerContextTimeout)
	provider, err := oidc.NewProvider(ctx, *idp)
	if err != nil {
		return nil, err
	}

	var verifier = provider.Verifier(
		&oidc.Config{
			SkipClientIDCheck: true, // Today, just trust all clients from the idp
			SkipExpiryCheck: false,  // Enforce issued at, and, expires at. Your local clock time matters.
			SkipIssuerCheck: false,  // Enforce iss claim in tokens, too. ONLY FOR TESTING.
		})

	// Save this verifier for latter requests.
	globalTrustedIdProviderManager.trustedVerifiers.Store(*idp, verifier)

	return verifier, nil
}

func GetTrustedIdentityProviderManager(trustedIdentityProviders []*string) (*TrustedIdProviderManagerImpl, error) {

	// Our subset of trustedIdProviders that we'll return.
	var ret TrustedIdProviderManagerImpl

	for _, trustedIssuer := range trustedIdentityProviders {
		// Skip nil issuers for safety.
		if trustedIssuer == nil {
			continue
		}

		verifier, err := CreateOrGetVerifier(trustedIssuer)
		if err != nil {
			return nil, err
		}

		ret.trustedVerifiers.Store(*trustedIssuer, verifier)
	}

	return &ret, nil
}

func (idpm *TrustedIdProviderManagerImpl) VerifyToken(token *string) (*oidc.IDToken, error) {

	// Parse the token.
	parsed, err := jwt.ParseSigned(*token)
	if err != nil {
		return nil, err
	}

	// Extract the claims.
	extractedClaims := &jwt.Claims{}
	err = parsed.UnsafeClaimsWithoutVerification(extractedClaims)
	if err != nil {
		return nil, err
	}

	// Find the verifier for the token issuer.
	verifier, exists := idpm.trustedVerifiers.Load(extractedClaims.Issuer)
	if !exists {
		return nil, errors.New(fmt.Sprintf("%s is not a trusted issuer", extractedClaims.Issuer))
	}

	// Verify the token (signature, expiry, and issuer)
	typedVerifier := verifier.(*oidc.IDTokenVerifier)
	ctx, cancel := context.WithTimeout(context.TODO(), globalTrustedIssuerContextTimeout)
	defer cancel()  // releases resources if slowOperation completes before timeout elapses
	verifiedToken, err := typedVerifier.Verify(ctx, *token)
	if err != nil {
		return nil, err
	}

	// Return the claims
	return verifiedToken, nil
}


