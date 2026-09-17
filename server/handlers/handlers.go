// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package handlers

import (
	"net/http"

	v1 "github.com/open-policy-agent/opa/v1/server/handlers"
)

// This handler applies only for data and compile endpoints, for selected HTTP methods
//
// If the client asked for a gzip response, this handler will buffer the response and
// wait until it reached a certain threshold. If the threshold is not hit, the uncompressed response is sent
//
// If a gzip response is not asked by the client, it'll send the uncompressed response
//
// The threshold and the gzip compression level can be modified from server's configuration

func CompressHandler(handler http.Handler, gzipMinLength int, gzipCompressionLevel int) http.Handler {
	return v1.CompressHandler(handler, gzipMinLength, gzipCompressionLevel)
}

// This handler provides hard limits on the size of the request body, for both
// the raw body content, and also for the decompressed size when gzip
// compression is used.
//
// The Content-Length restriction happens here in the handler, but the
// decompressed size limit is enforced later, in `util.ReadMaybeCompressedBody`.
// The handler passes the gzip size limits down to that function through the
// request context whenever gzip encoding is present.
func DecodingLimitsHandler(handler http.Handler, maxLength, gzipMaxLength int64) http.Handler {
	return v1.DecodingLimitsHandler(handler, maxLength, gzipMaxLength)
}
