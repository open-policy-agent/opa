package wazer

import (
	"fmt"

	"strconv"

	"github.com/open-policy-agent/opa/topdown"
)

func newBuiltinTable(mod Module) map[int32]topdown.BuiltinFunc {
	builtinStrAddr := mod.builtins(mod.ctx)
	builtinsJSON, err := mod.json_dump(mod.ctx, (builtinStrAddr))
	if err != nil {
		panic(err)
	}
	builtinStr := mod.readStr(uint32(builtinsJSON))
	builtinNameMap := parseJsonString(builtinStr)
	builtinIdMap, err := getFuncs(builtinNameMap)
	if err != nil {
		panic(err)
	}
	return builtinIdMap
}
func parseJsonString(str string) map[string]int32 {
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
