// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package precis

import (
	"unicode/utf8"
)

// TODO: Add contextual character rules from Appendix A of RFC5892.

// A class is a set of characters that match certain derived properties. The
// PRECIS framework defines two classes: The Freeform class and the Identifier
// class. The freeform class should be used for profiles where expressiveness is
// prioritized over safety such as nicknames or passwords. The identifier class
// should be used for profiles where safety is the first priority such as
// addressable network labels and usernames.
type class struct {
	extraAllowed    property
	extraDisallowed property
}

// Contains satisfies the runes.Set interface and returns whether the given rune
// is a member of the class.
func (c class) Contains(r rune) bool {
	b := make([]byte, 4)
	n := utf8.EncodeRune(b, r)

	trieval, _ := dpTrie.lookup(b[:n])
	switch p := property(trieval); {
	case p&c.extraDisallowed != 0:
		return false
	case p&c.extraAllowed != 0:
		return true
	case p&disallowed != 0, p&unassigned != 0:
		return false
	case p&pValid != 0:
		return true
	default:
		return false
	}
}

var (
	identifier = &class{
		extraDisallowed: idDis,
	}
	freeform = &class{
		extraAllowed: freePVal,
	}
)
