// Package errors exist to store errors for jwt and openid packages.
//
// It's internal because we don't want to expose _anything_ about these errors
// so users absolutely cannot do anything other than use them as opaque errors.
package errors

import (
	"errors"
	"fmt"
)

var (
	ErrClaimNotFound               = ClaimNotFoundError{}
	ErrClaimAssignmentFailed       = ClaimAssignmentFailedError{Err: errors.New(`claim assignment failed`)}
	ErrUnknownPayloadType          = errors.New(`unknown payload type (payload is not JWT?)`)
	ErrParse                       = ParseError{error: errors.New(`jwt.Parse: unknown error`)}
	ErrValidateDefault             = ValidationError{errors.New(`unknown error`)}
	ErrInvalidIssuerDefault        = InvalidIssuerError{errors.New(`"iss" not satisfied`)}
	ErrTokenExpiredDefault         = TokenExpiredError{errors.New(`"exp" not satisfied: token is expired`)}
	ErrInvalidIssuedAtDefault      = InvalidIssuedAtError{errors.New(`"iat" not satisfied`)}
	ErrTokenNotYetValidDefault     = TokenNotYetValidError{errors.New(`"nbf" not satisfied: token is not yet valid`)}
	ErrInvalidAudienceDefault      = InvalidAudienceError{errors.New(`"aud" not satisfied`)}
	ErrMissingRequiredClaimDefault = &MissingRequiredClaimError{error: errors.New(`required claim is missing`)}
)

type ClaimNotFoundError struct {
	Name string
}

func (e ClaimNotFoundError) Error() string {
	// This error message uses "field" instead of "claim" for backwards compatibility,
	// but it shuold really be "claim" since it refers to a JWT claim.
	return fmt.Sprintf(`field "%s" not found`, e.Name)
}

func (e ClaimNotFoundError) Is(target error) bool {
	_, ok := target.(ClaimNotFoundError)
	return ok
}

type ClaimAssignmentFailedError struct {
	Err error
}

func (e ClaimAssignmentFailedError) Error() string {
	// This error message probably should be tweaked, but it is this way
	// for backwards compatibility.
	return fmt.Sprintf(`failed to assign value to dst: %s`, e.Err.Error())
}

func (e ClaimAssignmentFailedError) Unwrap() error {
	return e.Err
}

func (e ClaimAssignmentFailedError) Is(target error) bool {
	_, ok := target.(ClaimAssignmentFailedError)
	return ok
}

type ParseError struct {
	error
}

func (e ParseError) Unwrap() error {
	return e.error
}

func (ParseError) Is(err error) bool {
	_, ok := err.(ParseError)
	return ok
}

func ParseErrorf(prefix, f string, args ...any) error {
	return ParseError{fmt.Errorf(prefix+": "+f, args...)}
}

type ValidationError struct {
	error
}

func (ValidationError) Is(err error) bool {
	_, ok := err.(ValidationError)
	return ok
}

func (err ValidationError) Unwrap() error {
	return err.error
}

func ValidateErrorf(f string, args ...any) error {
	return ValidationError{fmt.Errorf(`jwt.Validate: `+f, args...)}
}

type InvalidIssuerError struct {
	error
}

func (err InvalidIssuerError) Is(target error) bool {
	_, ok := target.(InvalidIssuerError)
	return ok
}

func (err InvalidIssuerError) Unwrap() error {
	return err.error
}

func IssuerErrorf(f string, args ...any) error {
	return InvalidIssuerError{fmt.Errorf(`"iss" not satisfied: `+f, args...)}
}

type TokenExpiredError struct {
	error
}

func (err TokenExpiredError) Is(target error) bool {
	_, ok := target.(TokenExpiredError)
	return ok
}

func (err TokenExpiredError) Unwrap() error {
	return err.error
}

type InvalidIssuedAtError struct {
	error
}

func (err InvalidIssuedAtError) Is(target error) bool {
	_, ok := target.(InvalidIssuedAtError)
	return ok
}

func (err InvalidIssuedAtError) Unwrap() error {
	return err.error
}

type TokenNotYetValidError struct {
	error
}

func (err TokenNotYetValidError) Is(target error) bool {
	_, ok := target.(TokenNotYetValidError)
	return ok
}

func (err TokenNotYetValidError) Unwrap() error {
	return err.error
}

type InvalidAudienceError struct {
	error
}

func (err InvalidAudienceError) Is(target error) bool {
	_, ok := target.(InvalidAudienceError)
	return ok
}

func (err InvalidAudienceError) Unwrap() error {
	return err.error
}

func AudienceErrorf(f string, args ...any) error {
	return InvalidAudienceError{fmt.Errorf(`"aud" not satisfied: `+f, args...)}
}

type MissingRequiredClaimError struct {
	error

	claim string
}

func (err *MissingRequiredClaimError) Is(target error) bool {
	err1, ok := target.(*MissingRequiredClaimError)
	if !ok {
		return false
	}
	return err1 == ErrMissingRequiredClaimDefault || err1.claim == err.claim
}

func MissingRequiredClaimErrorf(name string) error {
	return &MissingRequiredClaimError{claim: name, error: fmt.Errorf(`required claim "%s" is missing`, name)}
}
