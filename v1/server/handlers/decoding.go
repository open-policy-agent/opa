package handlers

import (
	"net/http"

	"github.com/open-policy-agent/opa/internal/server/bodylimit"
)

// This handler provides hard limits on the size of the request body, for both
// the raw body content, and also for the decompressed size when gzip
// compression is used.
//
// The Content-Length restriction happens here in the handler, but the
// decompressed size limit is enforced later, in `util.ReadMaybeCompressedBody`.
// The handler passes the gzip size limits down to that function through the
// request context whenever gzip encoding is present.
//
// Deprecated: Configure request-body limits through the server decoding configuration instead.
func DecodingLimitsHandler(handler http.Handler, maxLength, gzipMaxLength int64) http.Handler {
	return bodylimit.Handler(handler, maxLength, gzipMaxLength, false)
}
