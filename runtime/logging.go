// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"net/http"

	"github.com/open-policy-agent/opa/logging"
	v1 "github.com/open-policy-agent/opa/v1/runtime"
)

// LoggingHandler returns an http.Handler that will print log messages
// containing the request information as well as response status and latency.
type LoggingHandler = v1.LoggingHandler

// NewLoggingHandler returns a new http.Handler.
func NewLoggingHandler(logger logging.Logger, inner http.Handler) http.Handler {
	return v1.NewLoggingHandler(logger, inner)
}
