package util

import (
	"net/http"

	v1 "github.com/open-policy-agent/opa/v1/util"
)

// Note(philipc): Originally taken from server/server.go
// The DecodingLimitHandler handles validating that the gzip payload is within the
// allowed max size limit. Thus, in the event of a forged payload size trailer,
// the worst that can happen is that we waste memory up to the allowed max gzip
// payload size, but not an unbounded amount of memory, as was potentially
// possible before.
func ReadMaybeCompressedBody(r *http.Request) ([]byte, error) {
	return v1.ReadMaybeCompressedBody(r)
}
