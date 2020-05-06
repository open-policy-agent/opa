// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package ref implements internal helpers for references
package ref

import (
	"strings"

	"github.com/open-policy-agent/opa/ast"
)

// ParseDataPath returns a ref from the slash separated path s rooted at data.
// All path segments are treated as identifier strings.
func ParseDataPath(s string) (ast.Ref, error) {
	s = strings.Replace(strings.Trim(s, "/"), "/", ".", -1)
	return ast.ParseRef("data." + s)
}
