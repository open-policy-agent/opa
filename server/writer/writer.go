// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package writer contains utilities for writing responses in the server.
package writer

import (
	"encoding/json"
	"net/http"

	"github.com/open-policy-agent/opa/server/types"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
)

// HTTPStatus is used to set a specific status code
// Adapted from https://stackoverflow.com/questions/27711154/what-response-code-to-return-on-a-non-supported-http-method-on-rest
func HTTPStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(code)
	}
}

// ErrorAuto writes a response with status and code set automatically based on
// the type of err.
func ErrorAuto(w http.ResponseWriter, err error) {
	switch {
	case types.IsBadRequest(err):
		ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
	case storage.IsWriteConflictError(err):
		ErrorString(w, http.StatusNotFound, types.CodeResourceConflict, err)
	case topdown.IsError(err):
		Error(w, http.StatusInternalServerError, types.NewErrorV1(types.CodeInternal, types.MsgEvaluationError).WithError(err))
	case storage.IsInvalidPatch(err):
		ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
	case storage.IsNotFound(err):
		ErrorString(w, http.StatusNotFound, types.CodeResourceNotFound, err)
	default:
		ErrorString(w, http.StatusInternalServerError, types.CodeInternal, err)
	}
}

// ErrorString writes a response with specified status, code, and message set to
// the the err's string representation.
func ErrorString(w http.ResponseWriter, status int, code string, err error) {
	Error(w, status, types.NewErrorV1(code, err.Error()))
}

// Error writes a response with specified status and error response.
func Error(w http.ResponseWriter, status int, err *types.ErrorV1) {
	headers := w.Header()
	headers.Add("Content-Type", "application/json")
	Bytes(w, status, err.Bytes())
	_, _ = w.Write([]byte("\n"))
}

// JSON writes a response with the specified status code and object. The object
// will be JSON serialized.
func JSON(w http.ResponseWriter, code int, v interface{}, pretty bool) {
	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)

	if err := enc.Encode(v); err != nil {
		ErrorAuto(w, err)
		return
	}
}

// Bytes writes a response with the specified status code and bytes.
func Bytes(w http.ResponseWriter, code int, bs []byte) {
	w.WriteHeader(code)
	if code == 204 {
		return
	}
	_, _ = w.Write(bs)
}
