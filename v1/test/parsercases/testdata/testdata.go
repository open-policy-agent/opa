// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package testdata embeds the parser conformance test cases so that they can be
// consumed by tools outside of this repository, as v1/test/compilecases/testdata
// is for the compiler diagnostic cases.
package testdata

import "embed"

//go:embed v0 v1
var FS embed.FS
