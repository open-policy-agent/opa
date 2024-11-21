package bundle

import (
	"errors"
	"fmt"

	"github.com/open-policy-agent/opa/v1/download"
)

// Errors represents a list of errors that occurred during a bundle load enriched by the bundle name.
type Errors []Error

func (e Errors) Unwrap() []error {
	output := make([]error, len(e))
	for i := range e {
		output[i] = e[i]
	}
	return output
}
func (e Errors) Error() string {
	err := errors.Join(e.Unwrap()...)
	return err.Error()
}

type Error struct {
	BundleName string
	Code       string
	HTTPCode   int
	Message    string
	Err        error
}

func NewBundleError(bundleName string, cause error) Error {
	var (
		httpError download.HTTPError
	)
	switch {
	case cause == nil:
		return Error{BundleName: bundleName, Code: "", HTTPCode: -1, Message: "", Err: nil}
	case errors.As(cause, &httpError):
		return Error{BundleName: bundleName, Code: errCode, HTTPCode: httpError.StatusCode, Message: httpError.Error(), Err: cause}
	default:
		return Error{BundleName: bundleName, Code: errCode, HTTPCode: -1, Message: cause.Error(), Err: cause}
	}
}

func (e Error) Error() string {
	return fmt.Sprintf("Bundle name: %s, Code: %s, HTTPCode: %d, Message: %s", e.BundleName, errCode, e.HTTPCode, e.Message)
}

func (e Error) Unwrap() error {
	return e.Err
}
