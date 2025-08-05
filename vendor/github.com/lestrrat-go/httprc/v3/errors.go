package httprc

import "errors"

var errResourceAlreadyExists = errors.New(`resource already exists`)

func ErrResourceAlreadyExists() error {
	return errResourceAlreadyExists
}

var errAlreadyRunning = errors.New(`client is already running`)

func ErrAlreadyRunning() error {
	return errAlreadyRunning
}

var errResourceNotFound = errors.New(`resource not found`)

func ErrResourceNotFound() error {
	return errResourceNotFound
}

var errTransformerRequired = errors.New(`transformer is required`)

func ErrTransformerRequired() error {
	return errTransformerRequired
}

var errURLCannotBeEmpty = errors.New(`URL cannot be empty`)

func ErrURLCannotBeEmpty() error {
	return errURLCannotBeEmpty
}

var errUnexpectedStatusCode = errors.New(`unexpected status code`)

func ErrUnexpectedStatusCode() error {
	return errUnexpectedStatusCode
}

var errTransformerFailed = errors.New(`failed to transform response body`)

func ErrTransformerFailed() error {
	return errTransformerFailed
}

var errRecoveredFromPanic = errors.New(`recovered from panic`)

func ErrRecoveredFromPanic() error {
	return errRecoveredFromPanic
}

var errBlockedByWhitelist = errors.New(`blocked by whitelist`)

func ErrBlockedByWhitelist() error {
	return errBlockedByWhitelist
}
