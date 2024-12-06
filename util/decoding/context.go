package decoding

import (
	"context"

	v1 "github.com/open-policy-agent/opa/v1/util/decoding"
)

func AddServerDecodingMaxLen(ctx context.Context, maxLen int64) context.Context {
	return v1.AddServerDecodingMaxLen(ctx, maxLen)
}

func AddServerDecodingGzipMaxLen(ctx context.Context, maxLen int64) context.Context {
	return v1.AddServerDecodingGzipMaxLen(ctx, maxLen)
}

// Used for enforcing max body content limits when dealing with chunked requests.
func GetServerDecodingMaxLen(ctx context.Context) (int64, bool) {
	return v1.GetServerDecodingMaxLen(ctx)
}

func GetServerDecodingGzipMaxLen(ctx context.Context) (int64, bool) {
	return v1.GetServerDecodingGzipMaxLen(ctx)
}
