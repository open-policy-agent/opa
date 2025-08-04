package jwe

import "errors"

type encryptError struct {
	error
}

func (e encryptError) Unwrap() error {
	return e.error
}

func (encryptError) Is(err error) bool {
	_, ok := err.(encryptError)
	return ok
}

var errDefaultEncryptError = encryptError{errors.New(`encrypt error`)}

// EncryptError returns an error that can be passed to `errors.Is` to check if the error is an error returned by `jwe.Encrypt`.
func EncryptError() error {
	return errDefaultEncryptError
}

type decryptError struct {
	error
}

func (e decryptError) Unwrap() error {
	return e.error
}

func (decryptError) Is(err error) bool {
	_, ok := err.(decryptError)
	return ok
}

var errDefaultDecryptError = decryptError{errors.New(`decrypt error`)}

// DecryptError returns an error that can be passed to `errors.Is` to check if the error is an error returned by `jwe.Decrypt`.
func DecryptError() error {
	return errDefaultDecryptError
}

type recipientError struct {
	error
}

func (e recipientError) Unwrap() error {
	return e.error
}

func (recipientError) Is(err error) bool {
	_, ok := err.(recipientError)
	return ok
}

var errDefaultRecipientError = recipientError{errors.New(`recipient error`)}

// RecipientError returns an error that can be passed to `errors.Is` to check if the error is
// an error that occurred while attempting to decrypt a JWE message for a particular recipient.
//
// For example, if the JWE message failed to parse during `jwe.Decrypt`, it will be a
// `jwe.DecryptError`, but NOT `jwe.RecipientError`. However, if the JWE message could not
// be decrypted for any of the recipients, then it will be a `jwe.RecipientError`
// (actually, it will be _multiple_ `jwe.RecipientError` errors, one for each recipient)
func RecipientError() error {
	return errDefaultRecipientError
}

type parseError struct {
	error
}

func (e parseError) Unwrap() error {
	return e.error
}

func (parseError) Is(err error) bool {
	_, ok := err.(parseError)
	return ok
}

var errDefaultParseError = parseError{errors.New(`parse error`)}

// ParseError returns an error that can be passed to `errors.Is` to check if the error
// is an error returned by `jwe.Parse` and related functions.
func ParseError() error {
	return errDefaultParseError
}
