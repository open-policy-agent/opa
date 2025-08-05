package jwt

import (
	jwterrs "github.com/lestrrat-go/jwx/v3/jwt/internal/errors"
)

// ClaimNotFoundError returns the opaque error value that is returned when
// `jwt.Get` fails to find the requested claim.
//
// This value should only be used for comparison using `errors.Is()`.
func ClaimNotFoundError() error {
	return jwterrs.ErrClaimNotFound
}

// ClaimAssignmentFailedError returns the opaque error value that is returned
// when `jwt.Get` fails to assign the value to the destination. For example,
// this can happen when the value is a string, but you passed a &int as the
// destination.
//
// This value should only be used for comparison using `errors.Is()`.
func ClaimAssignmentFailedError() error {
	return jwterrs.ErrClaimAssignmentFailed
}

// UnknownPayloadTypeError returns the opaque error value that is returned when
// `jwt.Parse` fails due to not being able to deduce the format of
// the incoming buffer.
//
// This value should only be used for comparison using `errors.Is()`.
func UnknownPayloadTypeError() error {
	return jwterrs.ErrUnknownPayloadType
}

// ParseError returns the opaque error that is returned from jwt.Parse when
// the input is not a valid JWT.
//
// This value should only be used for comparison using `errors.Is()`.
func ParseError() error {
	return jwterrs.ErrParse
}

// ValidateError returns the immutable error used for validation errors
//
// This value should only be used for comparison using `errors.Is()`.
func ValidateError() error {
	return jwterrs.ErrValidateDefault
}

// InvalidIssuerError returns the immutable error used when `iss` claim
// is not satisfied
//
// This value should only be used for comparison using `errors.Is()`.
func InvalidIssuerError() error {
	return jwterrs.ErrInvalidIssuerDefault
}

// TokenExpiredError returns the immutable error used when `exp` claim
// is not satisfied.
//
// This value should only be used for comparison using `errors.Is()`.
func TokenExpiredError() error {
	return jwterrs.ErrTokenExpiredDefault
}

// InvalidIssuedAtError returns the immutable error used when `iat` claim
// is not satisfied
//
// This value should only be used for comparison using `errors.Is()`.
func InvalidIssuedAtError() error {
	return jwterrs.ErrInvalidIssuedAtDefault
}

// TokenNotYetValidError returns the immutable error used when `nbf` claim
// is not satisfied
//
// This value should only be used for comparison using `errors.Is()`.
func TokenNotYetValidError() error {
	return jwterrs.ErrTokenNotYetValidDefault
}

// InvalidAudienceError returns the immutable error used when `aud` claim
// is not satisfied
//
// This value should only be used for comparison using `errors.Is()`.
func InvalidAudienceError() error {
	return jwterrs.ErrInvalidAudienceDefault
}

// MissingRequiredClaimError returns the immutable error used when the claim
// specified by `jwt.IsRequired()` is not present.
//
// This value should only be used for comparison using `errors.Is()`.
func MissingRequiredClaimError() error {
	return jwterrs.ErrMissingRequiredClaimDefault
}
