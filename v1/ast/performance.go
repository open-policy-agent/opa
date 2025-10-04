// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package ast

import (
	"strings"
	"sync"
)

var builtinNamesByNumParts = sync.OnceValue(func() map[int][]string {
	m := map[int][]string{}
	for name := range BuiltinMap {
		parts := strings.Count(name, ".") + 1
		if parts > 1 {
			m[parts] = append(m[parts], name)
		}
	}
	return m
})

// BuiltinNameFromRef attempts to extract a known built-in function name from a ref,
// in the most efficient way possible. I.e. without allocating memory for a new string.
// If no built-in function name can be extracted, the second return value is false.
func BuiltinNameFromRef(ref Ref) (string, bool) {
	reflen := len(ref)
	if reflen == 0 {
		return "", false
	}

	_var, ok := ref[0].Value.(Var)
	if !ok {
		return "", false
	}

	varName := string(_var)
	if reflen == 1 {
		if _, ok := BuiltinMap[varName]; ok {
			return varName, true
		}
		return "", false
	}

	totalLen := len(varName)
	for _, term := range ref[1:] {
		if _, ok = term.Value.(String); !ok {
			return "", false
		}
		totalLen += 1 + len(term.Value.(String)) // account for dot
	}

	matched, ok := builtinNamesByNumParts()[reflen]
	if !ok {
		return "", false
	}

	for _, name := range matched {
		if len(name) != totalLen {
			continue
		}

		dotPos := strings.IndexByte(name, '.')
		if name[:dotPos] != varName {
			continue
		}

		remaining := name[dotPos+1:]

		for _, term := range ref[1:] {
			ts := string(term.Value.(String))
			if remaining == ts {
				return name, true
			}
			if dotPos = strings.IndexByte(remaining, '.'); dotPos == -1 {
				break
			}
			if remaining[:dotPos] != ts {
				break
			}
			remaining = remaining[dotPos+1:]
		}
	}

	return "", false
}
