// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"math/bits"
	"slices"
)

// pathSet is a fixed set of path ids, held as the words of a bitset that have
// a bit set. It takes one of two layouts, whichever takes less room:
//
//   - sparse (base < 0): data holds pairs, a word's index and its bits, in
//     increasing order of the index;
//   - dense (base >= 0): data holds every word from word base on, zeros
//     included, so that a word is found by its position.
type pathSet struct {
	data []uint64
	base int32
}

var emptyPathSet = pathSet{base: -1}

// newPathSet returns the set of ids, which have to be in increasing order, out
// of a universe of n words.
func newPathSet(ids []int32, n int) pathSet {
	s := emptyPathSet
	for _, id := range ids {
		w, bit := uint64(id>>6), uint64(1)<<(uint(id)&63)
		if k := len(s.data); k == 0 || s.data[k-2] != w {
			s.data = append(s.data, w, 0)
		}
		s.data[len(s.data)-1] |= bit
	}
	words := len(s.data) / 2
	if words == 0 || 3*words < n {
		s.data = slices.Clip(s.data)
		return s
	}
	lo, hi := s.data[0], s.data[len(s.data)-2]
	dense := make([]uint64, hi-lo+1)
	for j := 0; j < len(s.data); j += 2 {
		dense[s.data[j]-lo] = s.data[j+1]
	}
	return pathSet{data: dense, base: int32(lo)}
}

// words is how many words the set holds, which is what going through it costs.
func (s pathSet) words() int {
	if s.base >= 0 {
		return len(s.data)
	}
	return len(s.data) / 2
}

// word returns the j-th word the set holds, and its index.
func (s pathSet) word(j int) (int32, uint64) {
	if s.base >= 0 {
		return s.base + int32(j), s.data[j]
	}
	return int32(s.data[2*j]), s.data[2*j+1]
}

// at returns word w of the set.
func (s pathSet) at(w int32) uint64 {
	if s.base >= 0 {
		if j := w - s.base; j >= 0 && int(j) < len(s.data) {
			return s.data[j]
		}
		return 0
	}
	lo, hi := 0, len(s.data)/2
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		switch pw := int32(s.data[2*mid]); {
		case pw == w:
			return s.data[2*mid+1]
		case pw < w:
			lo = mid + 1
		default:
			hi = mid
		}
	}
	return 0
}

// idSet is a set of ids being built during one lookup: a bitset over all of
// them, and the words written to, so that going through it or clearing it
// costs what was set rather than what could be.
type idSet struct {
	hits    []uint64
	touched []int32
}

func (s *idSet) grow(n int) { s.hits = growTo(s.hits, (n+63)/64) }

func (s *idSet) or(w int32, m uint64) {
	if m == 0 {
		return
	}
	if s.hits[w] == 0 {
		s.touched = append(s.touched, w)
	}
	s.hits[w] |= m
}

func (s *idSet) set(id int32) { s.or(id>>6, 1<<(uint(id)&63)) }

func (s *idSet) has(id int32) bool { return s.hits[id>>6]&(1<<(uint(id)&63)) != 0 }

func (s *idSet) reset() {
	for _, w := range s.touched {
		s.hits[w] = 0
	}
	s.touched = s.touched[:0]
}

// orIn adds the ids of p, keeping only those also in within if it is not nil.
// Where within has fewer words than p, it is p that is looked up.
func (s *idSet) orIn(p pathSet, within *idSet) {
	if within != nil && len(within.touched) < p.words() {
		for _, w := range within.touched {
			s.or(w, p.at(w)&within.hits[w])
		}
		return
	}
	for j := range p.words() {
		w, b := p.word(j)
		if within != nil {
			b &= within.hits[w]
		}
		s.or(w, b)
	}
}

// meets reports whether s and p share an id. A set of one word, which is what a
// level few rules test has, is checked here, where the call is inlined.
func (s *idSet) meets(p *pathSet) bool {
	if p.base < 0 && len(p.data) == 2 {
		return s.hits[p.data[0]]&p.data[1] != 0
	}
	return s.intersects(*p)
}

// intersects reports whether s and p share an id, going through whichever has
// fewer words.
func (s *idSet) intersects(p pathSet) bool {
	if len(s.touched) < p.words() {
		for _, w := range s.touched {
			if s.hits[w]&p.at(w) != 0 {
				return true
			}
		}
		return false
	}
	for j := range p.words() {
		if w, b := p.word(j); s.hits[w]&b != 0 {
			return true
		}
	}
	return false
}

// removeUnless removes the ids of p that are not in keep, going through
// whichever of s and p has fewer words.
func (s *idSet) removeUnless(p pathSet, keep *idSet) {
	if len(s.touched) < p.words() {
		for _, w := range s.touched {
			if s.hits[w] != 0 {
				s.hits[w] &^= p.at(w) &^ keep.hits[w]
			}
		}
		return
	}
	for j := range p.words() {
		w, b := p.word(j)
		s.hits[w] &^= b &^ keep.hits[w]
	}
}

// each calls f with every id in the set, more than once for an id whose word
// was cleared and written to again.
func (s *idSet) each(f func(id int32)) {
	for _, w := range s.touched {
		for word := s.hits[w]; word != 0; word &= word - 1 {
			f(w<<6 | int32(bits.TrailingZeros64(word)))
		}
	}
}

// postings are the path sets of the leaves of a level holding many paths, kept
// in one place rather than as a slice header per leaf.
type postings struct {
	data []uint64
	at   []postingAt
}

// postingAt is where one posting starts in data; it ends where the next one
// starts.
type postingAt struct {
	start, base int32
}

// add adds a posting of the ids, which have to be in increasing order, out of a
// universe of n words, if it takes less room than listing the ids does, and
// returns its index, or -1 if it does not.
func (p *postings) add(ids []int32, n int) int32 {
	words := 0
	for j, id := range ids {
		if j == 0 || id>>6 != ids[j-1]>>6 {
			words++
		}
	}
	lo, hi := ids[0]>>6, ids[len(ids)-1]>>6
	dense := 3*words >= n
	size := 2 * words
	if dense {
		size = int(hi - lo + 1)
	}
	// A word is two ids' worth of room.
	if 2*size >= len(ids) {
		return -1
	}

	at := postingAt{start: int32(len(p.data)), base: -1}
	if dense {
		at.base = lo
		p.data = append(p.data, make([]uint64, size)...)
		for _, id := range ids {
			p.data[int(at.start)+int(id>>6-lo)] |= 1 << (uint(id) & 63)
		}
	} else {
		for j, id := range ids {
			if j == 0 || id>>6 != ids[j-1]>>6 {
				p.data = append(p.data, uint64(id>>6), 0)
			}
			p.data[len(p.data)-1] |= 1 << (uint(id) & 63)
		}
	}
	p.at = append(p.at, at)
	return int32(len(p.at) - 1)
}

func (p *postings) get(k int32) pathSet {
	at, end := p.at[k], int32(len(p.data))
	if int(k)+1 < len(p.at) {
		end = p.at[k+1].start
	}
	return pathSet{data: p.data[at.start:end], base: at.base}
}

func (p *postings) clip() {
	p.data, p.at = slices.Clip(p.data), slices.Clip(p.at)
}
