// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package writer contains utilities for writing responses in the server.
package writer

import (
	"encoding/json"
	"net/http"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/server/types"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/pkg/errors"
)

// ErrorAuto writes a response with status and code set automatically based on
// the type of err.
func ErrorAuto(w http.ResponseWriter, err error) {
	var prev error
	for curr := err; curr != prev; {
		if types.IsBadRequest(curr) {
			ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
			return
		}
		if types.IsWriteConflict(curr) {
			ErrorString(w, http.StatusNotFound, types.CodeResourceConflict, err)
			return
		}
		if topdown.IsError(curr) {
			Error(w, http.StatusInternalServerError, types.NewErrorV1(types.CodeInternal, types.MsgEvaluationError).WithError(curr))
		}
		if ast.IsError(ast.InputErr, curr) {
			Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgInputDocError).WithError(curr))
			return
		}
		if storage.IsInvalidPatch(curr) {
			ErrorString(w, http.StatusBadRequest, types.CodeInvalidParameter, err)
			return
		}
		if storage.IsNotFound(curr) {
			ErrorString(w, http.StatusNotFound, types.CodeResourceNotFound, err)
			return
		}
		prev = curr
		curr = errors.Cause(prev)
	}
	ErrorString(w, http.StatusInternalServerError, types.CodeInternal, err)
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
}

// JSON writes a response with the specified status code and object. The object
// will be JSON serialized.
func JSON(w http.ResponseWriter, code int, v interface{}, pretty bool) {

	var bs []byte
	var err error

	if pretty {
		bs, err = json.MarshalIndent(v, "", "  ")
	} else {
		bs, err = json.Marshal(v)
	}

	if err != nil {
		ErrorAuto(w, err)
		return
	}
	headers := w.Header()
	headers.Add("Content-Type", "application/json")
	Bytes(w, code, bs)
}

// Bytes writes a response with the specified status code and bytes.
func Bytes(w http.ResponseWriter, code int, bs []byte) {
	w.WriteHeader(code)
	if code == 204 {
		return
	}
	w.Write(bs)
}
