// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
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
