// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package status

import (
	v1 "github.com/open-policy-agent/opa/v1/plugins/logs/status"
)

// Status represents the status of processing a decision log.
type Status = v1.Status

type HTTPError = v1.HTTPError
