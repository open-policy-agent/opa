package handlers

import (
	"net/http"

	v1 "github.com/open-policy-agent/opa/v1/server/handlers"
)

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
