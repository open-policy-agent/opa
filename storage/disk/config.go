// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	v1 "github.com/open-policy-agent/opa/v1/storage/disk"
)

var ErrInvalidPartitionPath = v1.ErrInvalidPartitionPath

// OptionsFromConfig parses the passed config, extracts the disk storage
// settings, validates it, and returns a *Options struct pointer on success.
func OptionsFromConfig(raw []byte, id string) (*Options, error) {
	return v1.OptionsFromConfig(raw, id)
}
