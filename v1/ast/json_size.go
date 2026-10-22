// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"unicode/utf8"
)

// JSONByteSize returns the length in bytes of the encoding/json serialization
// of v, as produced by json.Marshal on the result of [JSON], without building
// it. It reports false if v contains anything it can't size exactly, in which
// case the caller must fall back to marshaling.
func JSONByteSize(v Value) (int, bool) {
	switch v := v.(type) {
	case Null:
		return 4, true
	case Boolean:
		if v {
			return 4, true
		}
		return 5, true
	case Number:
		return len(v), true
	case String:
		return jsonStringSize(string(v)), true
	case *Array:
		total := jsonDelimiterSize(v.Len())
		for i := range v.Len() {
			n, ok := JSONByteSize(v.Elem(i).Value)
			if !ok {
				return 0, false
			}
			total += n
		}
		return total, true
	case *set:
		total := jsonDelimiterSize(v.Len())
		for _, t := range v.sortedKeys() {
			n, ok := JSONByteSize(t.Value)
			if !ok {
				return 0, false
			}
			total += n
		}
		return total, true
	case *lazyObj:
		// JSON marshals the native value as is, which can format numbers
		// differently than their AST counterparts.
		bs, err := json.Marshal(v.native)
		return len(bs), err == nil
	case *object:
		total := jsonDelimiterSize(v.Len())
		for _, elem := range v.sortedKeys() {
			k, isStr := elem.key.Value.(String)
			if !isStr {
				return 0, false
			}
			n, ok := JSONByteSize(elem.value.Value)
			if !ok {
				return 0, false
			}
			total += jsonStringSize(string(k)) + 1 + n // key, colon, value
		}
		return total, true
	}
	return 0, false
}

// jsonDelimiterSize returns the size of the brackets and separators of a collection
// with n elements.
func jsonDelimiterSize(n int) int {
	if n > 1 {
		return 2 + n - 1
	}
	return 2
}

// invalidUTF8Size is the encoded size of an invalid UTF-8 byte. encoding/json
// escapes it as \ufffd, but with GOEXPERIMENT=jsonv2 or Go 1.27 it's written as
// the raw replacement character, so probe the encoder in use.
var invalidUTF8Size = func() int {
	bs, err := json.Marshal("\xff")
	if err != nil {
		return 6
	}
	return len(bs) - 2
}()

// jsonStringSize mirrors the escaping of encoding/json with HTML escaping on,
// as used by json.Marshal.
func jsonStringSize(s string) int {
	n := 2 // quotes
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			switch {
			case b == '"' || b == '\\' || b == '\b' || b == '\f' || b == '\n' || b == '\r' || b == '\t':
				n += 2
			case b < 0x20 || b == '<' || b == '>' || b == '&':
				n += 6
			default:
				n++
			}
			i++
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			n += invalidUTF8Size
		case r == '\u2028', r == '\u2029':
			n += 6
		default:
			n += size
		}
		i += size
	}
	return n
}
