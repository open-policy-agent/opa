package handlers

import (
	"net/http"
	"strings"

	"github.com/klauspost/compress/gzhttp"
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
	wrap, err := gzhttp.NewWrapper(
		gzhttp.MinSize(gzipMinLength),
		gzhttp.CompressionLevel(gzipCompressionLevel),
		gzhttp.ContentTypeFilter(gzhttp.CompressAllContentTypeFilter),
		gzhttp.EnableZstd(false),
	)
	if err != nil {
		// Only returned for an invalid compression level; the encoding config
		// policy already restricts this to values gzhttp accepts.
		panic(err)
	}
	gzipHandler := wrap(handler)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isDataEndpoint(r) || isCompileEndpoint(r) {
			gzipHandler(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

func isDataEndpoint(req *http.Request) bool {
	isPostOrGetMethod := isPostMethod(req) || isGetMethod(req)
	isV1rV0 := strings.HasPrefix(req.URL.Path, "/v1/data") || strings.HasPrefix(req.URL.Path, "/v0/data")
	return isPostOrGetMethod && isV1rV0
}

func isCompileEndpoint(req *http.Request) bool {
	return isPostMethod(req) && strings.HasPrefix(req.URL.Path, "/v1/compile")
}

func isPostMethod(req *http.Request) bool {
	return req.Method == "POST"
}

func isGetMethod(req *http.Request) bool {
	return req.Method == "GET"
}
