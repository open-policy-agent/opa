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
		// This check saves us a huge amount of work, as only very few built-in
		// names will have the exact same length as the ref we are checking.
		if len(name) != totalLen {
			continue
		}
		// Example: `name` is "io.jwt.decode" (and so is ref)
		// The first part is varName, which have already been established to be 'io':
		// io,   jwt.decode                              io   == io
		if curr, remaining, _ := strings.Cut(name, "."); curr == varName {
			// Loop over the remaining (now known to be string) terms in the ref, e.g. "jwt" and "decode"
			for _, term := range ref[1:] {
				ts := string(term.Value.(String))
				// First iteration: jwt.decode != jwt, so we continue cutting
				// Second iteration: remaining is "decode", and so is term
				if remaining == ts {
					return name, true
				}
				// Cutting remaining (e.g. jwt.decode), and we now get:
				// jwt,  decode,                                              false  || jwt  != jwt
				if curr, remaining, _ = strings.Cut(remaining, "."); remaining == "" || curr != ts {
					break
				}
			}
		}
	}

	return "", false
}
