package jws

import (
	"fmt"
)

type signError struct {
	error
}

var errDefaultSignError = signerr(`unknown error`)

// SignError returns an error that can be passed to `errors.Is` to check if the error is a sign error.
func SignError() error {
	return errDefaultSignError
}

func (e signError) Unwrap() error {
	return e.error
}

func (signError) Is(err error) bool {
	_, ok := err.(signError)
	return ok
}

func signerr(f string, args ...any) error {
	return signError{fmt.Errorf(`jws.Sign: `+f, args...)}
}

// This error is returned when jws.Verify fails, but note that there's another type of
// message that can be returned by jws.Verify, which is `errVerification`.
type verifyError struct {
	error
}

var errDefaultVerifyError = verifyerr(`unknown error`)

// VerifyError returns an error that can be passed to `errors.Is` to check if the error is a verify error.
func VerifyError() error {
	return errDefaultVerifyError
}

func (e verifyError) Unwrap() error {
	return e.error
}

func (verifyError) Is(err error) bool {
	_, ok := err.(verifyError)
	return ok
}

func verifyerr(f string, args ...any) error {
	return verifyError{fmt.Errorf(`jws.Verify: `+f, args...)}
}

// verificationError is returned when the actual _verification_ of the key/payload fails.
type verificationError struct {
	error
}

var errDefaultVerificationError = verificationError{fmt.Errorf(`unknown verification error`)}

// VerificationError returns an error that can be passed to `errors.Is` to check if the error is a verification error.
func VerificationError() error {
	return errDefaultVerificationError
}

func (e verificationError) Unwrap() error {
	return e.error
}

func (verificationError) Is(err error) bool {
	_, ok := err.(verificationError)
	return ok
}

type parseError struct {
	error
}

var errDefaultParseError = parseerr(`unknown error`)

// ParseError returns an error that can be passed to `errors.Is` to check if the error is a parse error.
func ParseError() error {
	return errDefaultParseError
}

func (e parseError) Unwrap() error {
	return e.error
}

func (parseError) Is(err error) bool {
	_, ok := err.(parseError)
	return ok
}

func bparseerr(prefix string, f string, args ...any) error {
	return parseError{fmt.Errorf(prefix+": "+f, args...)}
}

func parseerr(f string, args ...any) error {
	return bparseerr(`jws.Parse`, f, args...)
}

func sparseerr(f string, args ...any) error {
	return bparseerr(`jws.ParseString`, f, args...)
}

func rparseerr(f string, args ...any) error {
	return bparseerr(`jws.ParseReader`, f, args...)
}
