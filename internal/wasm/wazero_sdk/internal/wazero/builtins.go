// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package wazero

import (
	"fmt"
	"strconv"

	"github.com/open-policy-agent/opa/topdown"
)

// generates the map of id to Builtin from the policy module
func newBuiltinTable(mod Module) map[int32]topdown.BuiltinFunc {
	builtinStrAddr := mod.builtins(mod.ctx)
	builtinsJSON, err := mod.jsonDump(mod.ctx, (builtinStrAddr))
	if err != nil {
		panic(err)
	}
	builtinStr := mod.readStr(uint32(builtinsJSON))
	builtinNameMap := parseJSONString(builtinStr)
	builtinIDMap, err := getFuncs(builtinNameMap)
	if err != nil {
		panic(err)
	}
	return builtinIDMap
}

// json string parser
func parseJSONString(str string) map[string]int32 {
	currKey := ""
	inKey := false
	inVal := false
	currVal := ""
	out := map[string]int32{}
	for _, char := range str {
		switch char {
		case '"':
			inKey = !inKey
		case '{':
		case '}':
			val, _ := strconv.ParseInt(currVal, 10, 32)
			out[currKey] = int32(val)
		case ':':
			inVal = true
		case ',':
			val, _ := strconv.ParseInt(currVal, 10, 32)
			out[currKey] = int32(val)
			inVal = false
			currVal = ""
			currKey = ""
		default:
			if inKey {
				currKey += string(char)
			} else if inVal {
				currVal += string(char)
			}
		}

	}
	return out
}

// returns the id->function map from the name->id map
func getFuncs(ids map[string]int32) (map[int32]topdown.BuiltinFunc, error) {
	out := map[int32]topdown.BuiltinFunc{}
	for name, id := range ids {
		out[id] = topdown.GetBuiltin(name)
		if out[id] == nil && name != "" {
			return out, fmt.Errorf("no function named %s", name)
		}
	}
	return out, nil
}
