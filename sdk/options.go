// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package sdk

import (
	v1 "github.com/open-policy-agent/opa/v1/sdk"
)

// Options contains parameters to setup and configure OPA.
// deprecated: use v1.sdk.Options instead
type Options = v1.Options

// ConfigOptions contains parameters to (re-)configure OPA.
// deprecated: use v1.sdk.ConfigOptions instead
type ConfigOptions = v1.ConfigOptions
