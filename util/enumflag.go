// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	v1 "github.com/open-policy-agent/opa/v1/util"
)

// EnumFlag implements the pflag.Value interface to provide enumerated command
// line parameter values.
type EnumFlag = v1.EnumFlag

// NewEnumFlag returns a new EnumFlag that has a defaultValue and vs enumerated
// values.
func NewEnumFlag(defaultValue string, vs []string) *EnumFlag {
	return v1.NewEnumFlag(defaultValue, vs)
}
