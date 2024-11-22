// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package extension

import (
	v1 "github.com/open-policy-agent/opa/v1/loader/extension"
)

// Handler is used to unmarshal a byte slice of a registered extension
// EXPERIMENTAL: Please don't rely on this functionality, it may go
// away or change in the future.
type Handler = v1.Handler

// RegisterExtension registers a Handler for a certain file extension, including
// the dot: ".json", not "json".
// EXPERIMENTAL: Please don't rely on this functionality, it may go
// away or change in the future.
func RegisterExtension(name string, handler Handler) {
	v1.RegisterExtension(name, handler)
}

// FindExtension ios used to look up a registered extension Handler
// EXPERIMENTAL: Please don't rely on this functionality, it may go
// away or change in the future.
func FindExtension(ext string) Handler {
	return v1.FindExtension(ext)
}
