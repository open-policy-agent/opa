// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	v1 "github.com/open-policy-agent/opa/v1/ast"
)

// InternedBooleanTerm returns an interned term with the given boolean value.
func InternedBooleanTerm(b bool) *Term {
	return v1.InternedBooleanTerm(b)
}

// InternedIntNumberTerm returns a term with the given integer value. The term is
// cached between -1 to 512, and for values outside of that range, this function
// is equivalent to ast.IntNumberTerm.
func InternedIntNumberTerm(i int) *Term {
	return v1.InternedIntNumberTerm(i)
}

// InternedIntFromString returns a term with the given integer value if the string
// maps to an interned term. If the string does not map to an interned term, nil is
// returned.
func InternedIntNumberTermFromString(s string) *Term {
	return v1.InternedIntNumberTermFromString(s)
}

// HasInternedIntNumberTerm returns true if the given integer value maps to an interned
// term, otherwise false.
func HasInternedIntNumberTerm(i int) bool {
	return v1.HasInternedIntNumberTerm(i)
}
