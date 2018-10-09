// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package wasm

import (
	"github.com/open-policy-agent/opa/internal/wasm/module"
	"github.com/open-policy-agent/opa/internal/wasm/types"
)

const (
	funcInt32Int32retInt32 uint32 = iota
	funcInt32retInt32             = iota
	funcInt64retInt32             = iota
)

var functypes = [...]module.FunctionType{
	{
		Params:  []types.ValueType{types.I32, types.I32},
		Results: []types.ValueType{types.I32},
	},
	{
		Params:  []types.ValueType{types.I32},
		Results: []types.ValueType{types.I32},
	},
	{
		Params:  []types.ValueType{types.I64},
		Results: []types.ValueType{types.I32},
	},
}
