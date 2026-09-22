// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package conformance

import (
	"fmt"
	"reflect"
	"strings"
)

// RegoVersions are the rego versions a corpus can hold, which is also the set of legal
// version directory names at a corpus root. v0-compat-v1 is its own parsing mode rather
// than either of the versions it names, so it would take a directory of its own; no case
// needs one yet.
//
// Also the accepted values of ParseOptions.RegoVersion, where an absent value means v1:
// a directive can make one want entry v1 without moving the file.
//
// Canonical for both corpora. A version added here has to be mapped onto a parser by every
// consumer, which for OPA means corpusgen.RegoVersion and the runner's caseRegoVersion; the
// tests over this list are what fail until both are.
var RegoVersions = []string{"v0", "v1", "v0-compat-v1"}

// ParseOptionFields are the names of ParseOptions' fields, which is the list a reader has
// to carry onto its own parser in full. Exported so that a reader can be tested against
// it: a field added here fails that test until the reader says how it carries the field.
//
// OPA needs this because it has three readers that cannot import each other: the two
// generators under build/, and the runner in package ast. A field carried by some of them
// and dropped by the rest would otherwise go unnoticed until a fixture was wrong.
func ParseOptionFields() []string {
	t := reflect.TypeFor[ParseOptions]()

	out := make([]string, 0, t.NumField())
	for f := range t.Fields() {
		out = append(out, f.Name)
	}
	return out
}

// ParseOptions is how a corpus's Rego has to be parsed. Stated without reference
// to v1/ast, so a consumer can map it onto its own parser.
//
// Every decision a case implies is made here rather than by each reader: OPA has three of
// those — a generator per corpus, and the runner that checks what they write — and none can
// import another, so anything a reader works out for itself is worked out three times. What
// is left for a reader to do is rename these fields onto its own parser's.
type ParseOptions struct {
	RegoVersion       string
	FutureKeywords    []string
	AllFutureKeywords bool

	// ProcessAnnotations asks for metadata comments to be parsed into annotations.
	ProcessAnnotations bool

	// SkipRules reads the Rego as a body rather than a module. Set for a body case, as
	// rego.New does for a query: without it a body that reads as a rule fails with a Go
	// type name and no position, where the parser has a positioned diagnostic to report.
	SkipRules bool
}

// DirectiveOption folds a directive import path into out, reporting whether imp
// was a directive at all.
//
// Both corpora carry import paths beside Rego that has nowhere to declare them: a
// compiled module the compiler stripped its directives from, a query, a body. at
// names the field for the error message.
func DirectiveOption(at, imp string, out *ParseOptions) (bool, error) {
	switch {
	case imp == "rego.v1":
		// Not v0-compat-v1: that mode requires the import, which the Rego this is
		// in effect for does not carry.
		out.RegoVersion = "v1"

	case imp == "future.keywords":
		out.AllFutureKeywords = true

	case strings.HasPrefix(imp, "future.keywords."):
		kw := strings.TrimPrefix(imp, "future.keywords.")
		if kw == "" || strings.Contains(kw, ".") {
			return false, fmt.Errorf("unrecognised '%s' entry %q", at, imp)
		}
		out.FutureKeywords = append(out.FutureKeywords, kw)

	default:
		return false, nil
	}

	return true, nil
}

// UnknownDirective is the error for an imports entry that is not a directive, in
// the fields where nothing else is accepted.
func UnknownDirective(at, imp string) error {
	return fmt.Errorf("unrecognised '%s' entry %q; "+
		"expected rego.v1, future.keywords or future.keywords.<keyword>", at, imp)
}
