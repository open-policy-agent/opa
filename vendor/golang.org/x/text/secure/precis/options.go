// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package precis

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"
)

// An Option is used to define the behavior and rules of a Profile.
type Option func(*options)

type options struct {
	// Preparation options
	allowwidechars bool

	// Enforcement options
	cases         transform.Transformer
	disallow      runes.Set
	norm          norm.Form
	additional    []func() transform.Transformer
	width         *width.Transformer
	disallowEmpty bool

	// Comparison options
	ignorecase bool
}

func getOpts(o ...Option) (res options) {
	for _, f := range o {
		f(&res)
	}
	return
}

var (
	// The IgnoreCase option causes the profile to perform a case insensitive
	// comparison during the PRECIS comparison step.
	IgnoreCase Option = ignoreCase

	// The AllowWide option causes the profile to allow full-width and half-width
	// characters by mapping them to their decomposition mappings. This is useful
	// for profiles that are based on the identifier class which would otherwise
	// disallow wide characters.
	AllowWide Option = allowWide

	// The DisallowEmpty option causes the enforcement step to return an error if
	// the resulting string would be empty.
	DisallowEmpty Option = disallowEmpty
)

var (
	ignoreCase = func(o *options) {
		o.ignorecase = true
	}
	allowWide = func(o *options) {
		o.allowwidechars = true
	}
	disallowEmpty = func(o *options) {
		o.disallowEmpty = true
	}
)

// The AdditionalMapping option defines the additional mapping rule for the
// Profile by applying Transformer's in sequence.
func AdditionalMapping(t ...func() transform.Transformer) Option {
	return func(o *options) {
		o.additional = t
	}
}

// The Norm option defines a Profile's normalization rule. Defaults to NFC.
func Norm(f norm.Form) Option {
	return func(o *options) {
		o.norm = f
	}
}

// The Width option defines a Profile's width mapping rule.
func Width(w width.Transformer) Option {
	return func(o *options) {
		o.width = &w
	}
}

// The FoldCase option defines a Profile's case mapping rule. Options can be
// provided to determine the type of case folding used.
func FoldCase(opts ...cases.Option) Option {
	return func(o *options) {
		o.cases = cases.Fold(opts...)
	}
}

// The Disallow option further restricts a Profile's allowed characters beyond
// what is disallowed by the underlying string class.
func Disallow(set runes.Set) Option {
	return func(o *options) {
		o.disallow = set
	}
}

// TODO: Pending finalization of the unicode/bidi API
// // The Dir option defines a Profile's directionality mapping rule. Generally
// // profiles based on the Identifier string class will want to use the "Bidi
// // Rule" defined in RFC5893, and profiles based on the Freeform string class
// // will want to use the Unicode bidirectional algorithm defined in UAX9.
// func Dir() Option {
// 	panic("unimplemented")
// }
