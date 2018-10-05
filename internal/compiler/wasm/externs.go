// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package wasm

import "github.com/open-policy-agent/opa/internal/wasm/module"

type extern struct {
	Name      string
	Module    string
	Index     uint32
	TypeIndex uint32
}

const opaModuleName = "opa"

const (
	opaParseJSON uint32 = iota
	opaBoolean
	opaStringTerminated
	opaNumberInt
	opaValueNotEqual
	opaValueGet
)

var externs = [...]module.Import{
	{
		Name: "opa_json_parse",
		Descriptor: module.FunctionImport{
			Func: funcInt32Int32retInt32,
		},
	},
	{
		Name: "opa_boolean",
		Descriptor: module.FunctionImport{
			Func: funcInt32retInt32,
		},
	},
	{
		Name: "opa_string_terminated",
		Descriptor: module.FunctionImport{
			Func: funcInt32retInt32,
		},
	},
	{
		Name: "opa_number_int",
		Descriptor: module.FunctionImport{
			Func: funcInt64retInt32,
		},
	},
	{
		Name: "opa_value_not_equal",
		Descriptor: module.FunctionImport{
			Func: funcInt32Int32retInt32,
		},
	},
	{
		Name: "opa_value_get",
		Descriptor: module.FunctionImport{
			Func: funcInt32Int32retInt32,
		},
	},
}
