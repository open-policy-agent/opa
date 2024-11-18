// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package rego_default_v1 sets the default [ast.RegoVersion] to [ast.RegoV1].
//
// Usage (import side effects only):
//
//	import _ "github.com/open-policy-agent/opa/internal/rego_default_v1"
package rego_default_v1

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/rego_default"
)

func init() {
	rego_default.DefaultRegoVersion = int(ast.RegoV1)
}
