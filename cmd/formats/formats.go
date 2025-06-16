// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package formats

import (
	"github.com/open-policy-agent/opa/v1/util"
)

type option = string

const (
	Pretty   option = "pretty"
	JSON     option = "json"
	GoBench  option = "gobench"
	Values   option = "values"
	Bindings option = "bindings"
	Source   option = "source"
	Raw      option = "raw"
	Discard  option = "discard"
)

// Returns an enum flag for the given formats, where the first provided format
// will be used as the default format.
func Flag(formats ...option) *util.EnumFlag {
	return util.NewEnumFlag(formats[0], formats)
}
