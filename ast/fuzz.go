// +build gofuzz

package ast

import (
	"bytes"
)

var blacklist = []string{
	"{{{{{", // nested { and [ cause the parse time to explode
	"[[[[[",
	"[{{[{{{{[{{",
}


func Fuzz(data []byte) int {
	for i := range blacklist {
		if bytes.Contains(data, []byte(blacklist[i])) {
			return -1
		}
	}
	str := string(data)
	_, _, err := ParseStatements("", str)
	if err == nil {
		CompileModules(map[string]string{"": str})
		return 1
	}
	return 0
}
