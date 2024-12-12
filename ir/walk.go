// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ir

import v1 "github.com/open-policy-agent/opa/v1/ir"

// Visitor defines the interface for visiting IR nodes.
type Visitor = v1.Visitor

// Walk invokes the visitor for nodes under x.
func Walk(vis Visitor, x interface{}) error {
	return v1.Walk(vis, x)
}
