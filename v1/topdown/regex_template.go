package topdown

// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license as follows:

// Copyright (c) 2012 Rodrigo Moraes. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
// * Redistributions of source code must retain the above copyright
//  notice, this list of conditions and the following disclaimer.
// * Redistributions in binary form must reproduce the above
//  copyright notice, this list of conditions and the following disclaimer
//  in the documentation and/or other materials provided with the
//  distribution.
// * Neither the name of Google Inc. nor the names of its
//  contributors may be used to endorse or promote products derived from
//  this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// This file was forked from https://github.com/gorilla/mux/commit/eac83ba2c004bb75

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"strings"
)

// delimiterIndices returns the first level delimiter indices from a string.
// It returns an error in case of unbalanced delimiters.
func delimiterIndices(s string, delimiterStart, delimiterEnd byte) ([]int, error) {
	var level, idx int
	idxs := make([]int, 0)
	for i := range len(s) {
		switch s[i] {
		case delimiterStart:
			if level++; level == 1 {
				idx = i
			}
		case delimiterEnd:
			if level--; level == 0 {
				idxs = append(idxs, idx, i+1)
			} else if level < 0 {
				return nil, fmt.Errorf(`unbalanced braces in %q`, s)
			}
		}
	}

	if level != 0 {
		return nil, fmt.Errorf(`unbalanced braces in %q`, s)
	}

	return idxs, nil
}

// generateRegexTemplate creates and returns the pattern string compiled by
// compileRegexTemplate().
func generateRegexTemplate(tpl string, delimiterStart, delimiterEnd byte) (string, error) {
	// Check if it is well-formed.
	idxs, errBraces := delimiterIndices(tpl, delimiterStart, delimiterEnd)
	if errBraces != nil {
		return "", errBraces
	}
	var pattern strings.Builder
	pattern.WriteByte('^')

	var end int
	for i := 0; i < len(idxs); i += 2 {
		// Set all values we are interested in.
		raw := tpl[end:idxs[i]]
		end = idxs[i+1]
		patt := tpl[idxs[i]+1 : end-1]

		// Each variable pattern must be a valid regular expression on its own.
		// Only parse it to validate: compiling it would be wasted work, as just
		// the full pattern below is used for matching.
		if _, err := syntax.Parse("^"+patt+"$", syntax.Perl); err != nil {
			return "", err
		}

		// Build the regexp pattern.
		pattern.WriteString(regexp.QuoteMeta(raw))
		pattern.WriteByte('(')
		pattern.WriteString(patt)
		pattern.WriteByte(')')
	}

	// Add the remaining.
	pattern.WriteString(regexp.QuoteMeta(tpl[end:]))
	pattern.WriteByte('$')

	return pattern.String(), nil
}

// compileRegexTemplate parses a template and returns a Regexp.
//
// You can define your own delimiters. It is e.g. common to use curly braces {} but I recommend using characters
// which have no special meaning in Regex, e.g.: <, >
//
//	reg, err := compiler.CompileRegex("foo:bar.baz:<[0-9]{2,10}>", '<', '>')
//	// if err != nil ...
//	reg.MatchString("foo:bar.baz:123")
func compileRegexTemplate(tpl string, delimiterStart, delimiterEnd byte) (*regexp.Regexp, error) {
	pattern, err := generateRegexTemplate(tpl, delimiterStart, delimiterEnd)
	if err != nil {
		return nil, err
	}

	// Compile full regexp.
	return regexp.Compile(pattern)
}
