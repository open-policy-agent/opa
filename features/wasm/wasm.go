// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Import this package to enable evaluation of rego code using the
// built-in wasm engine.
package wasm

import (
	v1 "github.com/open-policy-agent/opa/v1/features/wasm"
)

// OPA is an implementation of the OPA SDK.
type OPA = v1.OPA
