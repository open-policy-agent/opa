// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import "testing"

func TestBuiltinTrim(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"trims '!¡' from string", []string{`p[x] { x := trim("¡¡¡foo, bar!!!", "!¡") }`}, `["foo, bar"]`},
		{"trims nothing from string", []string{`p[x] { x := trim("¡¡¡foo, bar!!!", "i") }`}, `["¡¡¡foo, bar!!!"]`},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}

func TestBuiltinTrimLeft(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"trims leading '!¡' from string", []string{`p[x] { x := trim_left("¡¡¡foo, bar!!!", "!¡") }`}, `["foo, bar!!!"]`},
		{"trims nothing from string", []string{`p[x] { x := trim_left("!!!foo, bar¡¡¡", "¡") }`}, `["!!!foo, bar¡¡¡"]`},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}

func TestBuiltinTrimPrefix(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"trims prefix '!¡' from string", []string{`p[x] { x := trim_prefix("¡¡¡foo, bar!!!", "¡¡¡foo") }`}, `[", bar!!!"]`},
		{"trims nothing from string", []string{`p[x] { x := trim_prefix("¡¡¡foo, bar!!!", "¡¡¡bar") }`}, `["¡¡¡foo, bar!!!"]`},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}

func TestBuiltinTrimRight(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"trims trailing '!¡' from string", []string{`p[x] { x := trim_right("¡¡¡foo, bar!!!", "!¡") }`}, `["¡¡¡foo, bar"]`},
		{"trims nothing from string", []string{`p[x] { x := trim_right("!!!foo, bar¡¡¡", "!") }`}, `["!!!foo, bar¡¡¡"]`},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}

func TestBuiltinTrimSuffix(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"trims suffix '!¡' from string", []string{`p[x] { x := trim_suffix("¡¡¡foo, bar!!!", ", bar!!!") }`}, `["¡¡¡foo"]`},
		{"trims nothing from string", []string{`p[x] { x := trim_suffix("¡¡¡foo, bar!!!", ", foo!!!") }`}, `["¡¡¡foo, bar!!!"]`},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}

func TestBuiltinTrimSpace(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"trims all leading and trailing white space from string", []string{`p[x] { x := trim_space(" \t\n foo, bar \n\t\r\n") }`}, `["foo, bar"]`},
		{"trims nothing from string", []string{`p[x] { x := trim_space("foo, bar") }`}, `["foo, bar"]`},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}

func TestReplaceN(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"replace multiple patterns", []string{`p[x] { x = strings.replace_n({"<": "&lt;", ">": "&gt;"}, "This is <b>HTML</b>!") }`}, `["This is &lt;b&gt;HTML&lt;/b&gt;!"]`},
		{"find no patterns", []string{`p[x] { x = strings.replace_n({"old1": "new1", "old2": "new2"}, "Everything is new1, new2") }`}, `["Everything is new1, new2"]`},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}
