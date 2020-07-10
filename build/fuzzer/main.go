package fuzz

import (
	"bytes"

	"github.com/open-policy-agent/opa/ast"
)

var blacklist = []string{}

func Fuzz(data []byte) int {
	for i := range blacklist {
		if bytes.Contains(data, []byte(blacklist[i])) {
			return -1
		}
	}
	str := string(data)
	_, _, err := ast.ParseStatements("", str)
	if err == nil {
		ast.CompileModules(map[string]string{"": str})
		return 1
	}
	return 0
}
