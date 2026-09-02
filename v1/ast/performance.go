// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package ast

import (
	"encoding"
	"slices"
	"strings"
	"sync"
)

// builtinNameShape packs the two properties of a dotted built-in name that are cheap
// to derive from a ref without allocating — its number of parts and its total length —
// into a single key, which keeps map lookups on the runtime's fast path for 64-bit
// keys. uint64 rather than int so that the shift is well defined on 32-bit platforms.
func builtinNameShape(parts, totalLen int) uint64 {
	return uint64(parts)<<32 | uint64(totalLen)
}

// builtinNamesByShape groups multi-part built-in names by shape, so that
// BuiltinNameFromRef only has to compare against names that could possibly match.
// Bucketing on length as well as part count narrows the candidates from ~100 names
// to a handful, and sorting keeps the scan order deterministic: ranging over
// BuiltinMap yields a fresh random order in every process, which would otherwise
// make the cost of a lookup vary several-fold from one run to the next.
var builtinNamesByShape = sync.OnceValue(func() map[uint64][]string {
	m := map[uint64][]string{}
	for name := range BuiltinMap {
		if parts := strings.Count(name, ".") + 1; parts > 1 {
			shape := builtinNameShape(parts, len(name))
			m[shape] = append(m[shape], name)
		}
	}
	for _, names := range m {
		slices.Sort(names)
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

	matched, ok := builtinNamesByShape()[builtinNameShape(reflen, totalLen)]
	if !ok {
		return "", false
	}

	for _, name := range matched {
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

func AppendDelimeted[T encoding.TextAppender](buf []byte, appenders []T, delim string) ([]byte, error) {
	for i, item := range appenders {
		if i > 0 {
			buf = append(buf, delim...)
		}
		var err error
		if buf, err = item.AppendText(buf); err != nil {
			return nil, err
		}
	}
	return buf, nil
}
