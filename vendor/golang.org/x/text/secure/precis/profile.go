// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package precis

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/width"
)

var (
	disallowedRune = errors.New("disallowed rune encountered")
)

var dpTrie = newDerivedPropertiesTrie(0)

// A Profile represents a set of rules for normalizing and validating strings in
// the PRECIS framework.
type Profile struct {
	options
	class *class
	transform.NopResetter
}

// NewIdentifier creates a new PRECIS profile based on the Identifier string
// class. Profiles created from this class are suitable for use where safety is
// prioritized over expressiveness like network identifiers, user accounts, chat
// rooms, and file names.
func NewIdentifier(opts ...Option) Profile {
	return Profile{
		options: getOpts(opts...),
		class:   identifier,
	}
}

// NewFreeform creates a new PRECIS profile based on the Freeform string class.
// Profiles created from this class are suitable for use where expressiveness is
// prioritized over safety like passwords, and display-elements such as
// nicknames in a chat room.
func NewFreeform(opts ...Option) Profile {
	return Profile{
		options: getOpts(opts...),
		class:   freeform,
	}
}

// NewTransformer creates a new transform.Transformer that performs the PRECIS
// preparation and enforcement steps on the given UTF-8 encoded bytes.
func (p Profile) NewTransformer() *Transformer {
	var ts []transform.Transformer

	if p.options.allowwidechars {
		ts = append(ts, width.Fold)
	}

	ts = append(ts, checker{p: p})

	if p.options.width != nil {
		ts = append(ts, width.Fold)
	}

	for _, f := range p.options.additional {
		ts = append(ts, f())
	}

	if p.options.cases != nil {
		ts = append(ts, p.options.cases)
	}

	ts = append(ts, p.options.norm)

	// TODO: Apply directionality rule (blocking on the Bidi package)
	// TODO: Add the disallow empty rule with a dummy transformer?

	return &Transformer{transform.Chain(ts...)}
}

// Bytes returns a new byte slice with the result of applying the profile to b.
func (p Profile) Bytes(b []byte) ([]byte, error) {
	b, _, err := transform.Bytes(p.NewTransformer(), b)
	if err == nil && p.options.disallowEmpty && len(b) == 0 {
		return b, errors.New("enforce resulted in empty string")
	}
	return b, err
}

// String returns a string with the result of applying the profile to s.
func (p Profile) String(s string) (string, error) {
	s, _, err := transform.String(p.NewTransformer(), s)
	if err == nil && p.options.disallowEmpty && len(s) == 0 {
		return s, errors.New("enforce resulted in empty string")
	}
	return s, err
}

// Compare enforces both strings, and then compares them for bit-string identity
// (byte-for-byte equality). If either string cannot be enforced, the comparison
// is false.
func (p Profile) Compare(a, b string) bool {
	a, err := p.String(a)
	if err != nil {
		return false
	}
	b, err = p.String(b)
	if err != nil {
		return false
	}

	// TODO: This is out of order. Need to extract the transformation logic and
	// put this in where the normal case folding would go (but only for
	// comparison).
	if p.options.ignorecase {
		a = width.Fold.String(a)
		b = width.Fold.String(a)
	}

	return a == b
}

// Allowed returns a runes.Set containing every rune that is a member of the
// underlying profile's string class and not disallowed by any profile specific
// rules.
func (p Profile) Allowed() runes.Set {
	return runes.Predicate(func(r rune) bool {
		if p.options.disallow != nil {
			return p.class.Contains(r) && !p.options.disallow.Contains(r)
		} else {
			return p.class.Contains(r)
		}
	})
}

type checker struct {
	p Profile
	transform.NopResetter
}

func (c checker) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for nSrc < len(src) {
		r, size := utf8.DecodeRune(src[nSrc:])
		if size == 0 { // Incomplete UTF-8 encoding
			if !atEOF {
				return nDst, nSrc, transform.ErrShortSrc
			}
			size = 1
		}
		if c.p.Allowed().Contains(r) {
			if size != copy(dst[nDst:], src[nSrc:nSrc+size]) {
				return nDst, nSrc, transform.ErrShortDst
			}
			nDst += size
		} else {
			return nDst, nSrc, disallowedRune
		}
		nSrc += size
	}
	return nDst, nSrc, nil
}
