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
