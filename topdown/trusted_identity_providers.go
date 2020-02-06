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


var globalTrustedIdProviderManager trustedIdProviderManager
var globalTrustedIssuerContextTimeout = time.Second * time.Duration(5)

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
type trustedIdProviderManager struct {

	// Thread safe Map of string of trustedIssuer to *oidc.IDTokenVerifier trustedIssuerVerifier.
	trustedVerifiers sync.Map

}

func GetTrustedIdentityProviderManager(trustedIdentityProviders []*string) (TrustedIdProviderManager, error) {

	// Our subset of trustedIdProviders that we'll return.
	var ret trustedIdProviderManager

	for _, trustedIssuer := range trustedIdentityProviders {
		// Skip nil issuers for safety.
		if trustedIssuer == nil {
			continue
		}

		// If we already have a verifier for this issuer, use it.
		if loadedVerifier, ok := globalTrustedIdProviderManager.trustedVerifiers.Load(*trustedIssuer); ok {
			ret.trustedVerifiers.Store(trustedIssuer, loadedVerifier)
			continue
		}

		// Create a new issuer verifier, save it, and use it.
		ctx, _ := context.WithTimeout(context.Background(), globalTrustedIssuerContextTimeout)
		provider, err := oidc.NewProvider(ctx, *trustedIssuer)
		if err != nil {
			return nil, err
		}

		var verifier = provider.Verifier(
			&oidc.Config{
				SkipClientIDCheck: true, // Today, we trust all clients from the idp
				SkipExpiryCheck: false,  // Enforce issued at, and, expires at. Your local clock time matters.
				SkipIssuerCheck: false,  // Enforce iss claim in tokens, too.
			})

		// Save this verifier for latter requests.
		globalTrustedIdProviderManager.trustedVerifiers.Store(*trustedIssuer, verifier)

		// Add this to the returned set of verifiers.
		ret.trustedVerifiers.Store(trustedIssuer, verifier)
	}

	return &ret, nil
}

func (idpm *trustedIdProviderManager) VerifyToken(token *string) (*oidc.IDToken, error) {

	// Parse the token.
	parsed, err := jwt.ParseSigned(*token)
	if err != nil {
		return nil, err
	}

	// Extract the claims.
	var extractedClaims jwt.Claims
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
	ctx, _ := context.WithTimeout(context.Background(), globalTrustedIssuerContextTimeout)
	verifiedToken, err := typedVerifier.Verify(ctx, *token)
	if err != nil {
		return nil, err
	}

	// Return the claims
	return verifiedToken, nil
}


