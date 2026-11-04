// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bodylimit

import (
	"errors"
	"net/http"

	"github.com/open-policy-agent/opa/v1/util"
)

// ErrorStatus returns the status for a request-body error.
func ErrorStatus(v2Compatible bool, err error) int {
	if v2Compatible {
		_, bodyTooLarge := errors.AsType[*util.BodyTooLargeError](err)
		_, maxBytes := errors.AsType[*http.MaxBytesError](err)
		if bodyTooLarge || maxBytes {
			return http.StatusRequestEntityTooLarge
		}
	}
	return http.StatusBadRequest
}
