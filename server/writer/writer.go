// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package writer contains utilities for writing responses in the server.
package writer

import (
	"net/http"

	"github.com/open-policy-agent/opa/v1/server/types"
	v1 "github.com/open-policy-agent/opa/v1/server/writer"
)

// HTTPStatus is used to set a specific status code
// Adapted from https://stackoverflow.com/questions/27711154/what-response-code-to-return-on-a-non-supported-http-method-on-rest
func HTTPStatus(code int) http.HandlerFunc {
	return v1.HTTPStatus(code)
}

// ErrorAuto writes a response with status and code set automatically based on
// the type of err.
func ErrorAuto(w http.ResponseWriter, err error) {
	v1.ErrorAuto(w, err)
}

// ErrorString writes a response with specified status, code, and message set to
// the err's string representation.
func ErrorString(w http.ResponseWriter, status int, code string, err error) {
	v1.ErrorString(w, status, code, err)
}

// Error writes a response with specified status and error response.
func Error(w http.ResponseWriter, status int, err *types.ErrorV1) {
	v1.Error(w, status, err)
}

// JSON writes a response with the specified status code and object. The object
// will be JSON serialized.
// Deprecated: This method is problematic when using a non-200 status `code`: if
// encoding the payload fails, it'll print "superfluous call to WriteHeader()"
// logs.
func JSON(w http.ResponseWriter, code int, v interface{}, pretty bool) {
	v1.JSON(w, code, v, pretty)
}

// JSONOK is a helper for status "200 OK" responses
func JSONOK(w http.ResponseWriter, v interface{}, pretty bool) {
	v1.JSONOK(w, v, pretty)
}

// Bytes writes a response with the specified status code and bytes.
// Deprecated: Unused in OPA, will be removed in the future.
func Bytes(w http.ResponseWriter, code int, bs []byte) {
	v1.Bytes(w, code, bs)
}
