// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package debug

import (
	v1 "github.com/open-policy-agent/opa/v1/debug"
)

type ThreadID = v1.ThreadID

// Thread represents a single thread of execution.
type Thread = v1.Thread

// Scope represents the variable state of a StackFrame.
type Scope = v1.Scope
