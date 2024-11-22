// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import v1 "github.com/open-policy-agent/opa/v1/ast"

// FIXME: Do we need to duplicate all built-in declarations too?

type Builtin = v1.Builtin

var Builtins = v1.Builtins

func RegisterBuiltin(b *Builtin) {
	v1.RegisterBuiltin(b)
}

var DefaultBuiltins = v1.DefaultBuiltins

var BuiltinMap = v1.BuiltinMap
