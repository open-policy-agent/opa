// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"github.com/open-policy-agent/opa/logging"
)

// checkUserPrivileges is a no-op in Windows to avoid lookups with
// Active Directory and the like, when we don't care for the output
// anyways.
func checkUserPrivileges(logging.Logger) {}
