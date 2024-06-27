package decoding

import "context"

type requestContextKey string

// Note(philipc): We can add functions later to add the max request body length
// to contexts, if we ever need to.
const (
	reqCtxKeyMaxLen     = requestContextKey("server-decoding-plugin-context-max-length")
	reqCtxKeyGzipMaxLen = requestContextKey("server-decoding-plugin-context-gzip-max-length")
)

func AddServerDecodingGzipMaxLen(ctx context.Context, maxLen int64) context.Context {
	return context.WithValue(ctx, reqCtxKeyGzipMaxLen, maxLen)
}

func GetServerDecodingGzipMaxLen(ctx context.Context) (int64, bool) {
	gzipMaxLength, ok := ctx.Value(reqCtxKeyGzipMaxLen).(int64)
	return gzipMaxLength, ok
}
