// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package wazero

import (
	"encoding/json"
	"fmt"

	"github.com/open-policy-agent/opa/topdown"
)

// generates the map of id to Builtin from the policy module
func newBuiltinTable(mod Module) map[int32]topdown.BuiltinFunc {
	builtinStrAddr := mod.builtins(mod.ctx)
	builtinsJSON, err := mod.jsonDump(mod.ctx, (builtinStrAddr))
	if err != nil {
		panic(err)
	}
	builtinStr := mod.readStr(builtinsJSON)
	var builtinNameMap map[string]int32
	err = json.Unmarshal([]byte(builtinStr), &builtinNameMap)
	if err != nil {
		panic(err)
	}
	builtinIDMap, err := getFuncs(builtinNameMap)
	if err != nil {
		panic(err)
	}
	return builtinIDMap
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
