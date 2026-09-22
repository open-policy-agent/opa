// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package corpusgen

import (
	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
)

// SetMarshalOptions puts opts in effect and returns a function that restores whatever was
// there before.
//
// The AST marshalling options are global state, so a generator that marshals one node has to
// put them back afterwards, and one that marshals a corpus sets them once for the pass.
// Passing astJSON.GetOptions() is how a caller that sets them per case saves them first.
func SetMarshalOptions(opts astJSON.Options) func() {
	restore := astJSON.GetOptions()
	astJSON.SetOptions(opts)
	return func() { astJSON.SetOptions(restore) }
}
