// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bodylimit

import (
	"context"
	"errors"
	"net/http"

	"github.com/open-policy-agent/opa/v1/util"
)

type v2CompatibilityKey struct{}

// WithV2Compatibility enables v2 request-size error responses.
func WithV2Compatibility(ctx context.Context) context.Context {
	return context.WithValue(ctx, v2CompatibilityKey{}, struct{}{})
}

// ErrorStatus returns the status for a request-body error.
func ErrorStatus(ctx context.Context, err error) int {
	if ctx.Value(v2CompatibilityKey{}) != nil {
		_, bodyTooLarge := errors.AsType[*util.BodyTooLargeError](err)
		_, maxBytes := errors.AsType[*http.MaxBytesError](err)
		if bodyTooLarge || maxBytes {
			return http.StatusRequestEntityTooLarge
		}
	}
	return http.StatusBadRequest
}
