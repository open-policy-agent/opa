// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/open-policy-agent/opa/internal/prefixtrie"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
	"github.com/open-policy-agent/opa/v1/util"
)

var (
	errEmptySearchCharacter = errors.New("empty search character")
)

// Named inter-query value caches for the tries built over the base strings of
// strings.any_prefix_match and strings.any_suffix_match, keyed by collection,
// and for the hashes of the collections matched against once, kept apart so
// that marking a collection never evicts a trie.
const (
	anyPrefixMatchCacheName     = "any_prefix_match"
	anySuffixMatchCacheName     = "any_suffix_match"
	anyPrefixMatchSeenCacheName = "any_prefix_match_seen"
	anySuffixMatchSeenCacheName = "any_suffix_match_seen"
)

type affixCaches struct {
	tries, seen string
}

var (
	anyPrefixMatchCaches = affixCaches{tries: anyPrefixMatchCacheName, seen: anyPrefixMatchSeenCacheName}
	anySuffixMatchCaches = affixCaches{tries: anySuffixMatchCacheName, seen: anySuffixMatchSeenCacheName}
)

func builtinAnyPrefixMatch(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	found, err := anyAffixMatch(bctx, operands[0].Value, operands[1].Value, anyPrefixMatchCaches, false)
	if err != nil {
		return err
	}
	return iter(ast.InternedTerm(found))
}

func builtinAnySuffixMatch(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	found, err := anyAffixMatch(bctx, operands[0].Value, operands[1].Value, anySuffixMatchCaches, true)
	if err != nil {
		return err
	}
	return iter(ast.InternedTerm(found))
}

// affixSeen marks a collection that has been matched against once. The first
// call to see it again builds the collection's trie, while any others wait.
type affixSeen struct {
	once sync.Once
}

// anyAffixMatch reports whether any string in a starts with (or with suffix set,
// ends with) any string in base.
//
// A collection's trie is built the second time it is seen, so that one built
// per query (from input, say) never pays for it. Collections are marked by hash,
// so as not to hold on to them: a collision only builds a trie a call early.
// Cached tries are shared by concurrent evaluations and never written to.
func anyAffixMatch(bctx BuiltinContext, a, base ast.Value, caches affixCaches, suffix bool) (bool, error) {
	match := strings.HasPrefix
	if suffix {
		match = strings.HasSuffix
	}

	// The search strings are checked first, so that their type errors come first.
	s, single := a.(ast.String)
	var strs []string
	switch a := a.(type) {
	case ast.String:
	case *ast.Array, ast.Set:
		var err error
		if strs, err = builtins.StringSliceOperand(a, 1); err != nil {
			return false, err
		}
	default:
		return false, builtins.NewOperandTypeErr(1, a, "string", "set", "array")
	}

	switch base := base.(type) {
	case ast.String:
		if single {
			return match(string(s), string(base)), nil
		}
		return slices.ContainsFunc(strs, func(s string) bool { return match(s, string(base)) }), nil
	case *ast.Array, ast.Set:
	default:
		return false, builtins.NewOperandTypeErr(2, base, "string", "set", "array")
	}

	var tries, seen cache.InterQueryValueCacheBucket
	if bctx.InterQueryBuiltinValueCache != nil {
		tries = bctx.InterQueryBuiltinValueCache.GetCache(caches.tries)
		seen = bctx.InterQueryBuiltinValueCache.GetCache(caches.seen)
	}

	var key ast.Value
	var marker *affixSeen
	if tries != nil && seen != nil {
		if v, ok := tries.Get(base); ok {
			if trie, ok := v.(*prefixtrie.Trie[struct{}]); ok {
				return matchAffixTrie(trie, s, single, strs, suffix), nil
			}
		}
		key = ast.Number(strconv.Itoa(base.Hash()))
		if v, ok := seen.Get(key); ok {
			marker, _ = v.(*affixSeen)
		}
	}

	if single && marker == nil {
		// Declined only for a non-string member, reported below.
		if found, ok := anyStringMatch(string(s), base, match); ok {
			if key != nil {
				seen.Insert(key, &affixSeen{})
			}
			return found, nil
		}
	}

	bases, err := builtins.StringSliceOperand(base, 2)
	if err != nil {
		return false, err
	}

	if marker != nil {
		var trie *prefixtrie.Trie[struct{}]
		marker.once.Do(func() {
			trie = buildAffixTrie(bases, suffix)
			tries.Insert(base, trie)
		})
		if trie == nil {
			// Built by another call, for this collection or one with the same hash.
			if v, ok := tries.Get(base); ok {
				trie, _ = v.(*prefixtrie.Trie[struct{}])
			}
		}
		if trie != nil {
			return matchAffixTrie(trie, s, single, strs, suffix), nil
		}
		// Since evicted, or never built for this collection: built next time.
	}

	if key != nil {
		seen.Insert(key, &affixSeen{})
	}

	if suffix {
		// Bytes, not runes, so that invalid UTF-8 matches like endswith.
		return anyStartsWithAny(util.Map(strs, prefixtrie.Reverse), util.Map(bases, prefixtrie.Reverse)), nil
	}
	return anyStartsWithAny(strs, bases), nil
}

func buildAffixTrie(bases []string, suffix bool) *prefixtrie.Trie[struct{}] {
	trie := &prefixtrie.Trie[struct{}]{}
	for _, b := range bases {
		if suffix {
			b = prefixtrie.Reverse(b)
		}
		trie.Insert(b)
	}
	trie.Compact()
	return trie
}

func matchAffixTrie(trie *prefixtrie.Trie[struct{}], s ast.String, single bool, strs []string, suffix bool) bool {
	hasAffix := trie.HasPrefixOf
	if suffix {
		hasAffix = trie.HasSuffixOf
	}
	if single {
		return hasAffix(string(s))
	}
	return slices.ContainsFunc(strs, hasAffix)
}

// anyStringMatch tests one string against each member of an array or set of
// strings, without copying them out first. ok is false for any other operand,
// which is left to the general path to handle or reject.
func anyStringMatch(s string, b ast.Value, match func(s, affix string) bool) (found, ok bool) {
	// An array or a set; an object's Until takes keys and values.
	col, ok := b.(interface {
		Until(func(*ast.Term) bool) bool
	})
	if !ok {
		return false, false
	}

	// Every member is checked before any is matched, so that a non-string
	// after a match is still the type error it always was.
	if col.Until(func(t *ast.Term) bool {
		_, ok := t.Value.(ast.String)
		return !ok
	}) {
		return false, false
	}

	col.Until(func(t *ast.Term) bool {
		found = match(s, string(t.Value.(ast.String)))
		return found
	})
	return found, true
}

func anyStartsWithAny(strs []string, prefixes []string) bool {
	if len(strs) == 0 || len(prefixes) == 0 {
		return false
	}
	if len(strs) == 1 && len(prefixes) == 1 {
		return strings.HasPrefix(strs[0], prefixes[0])
	}

	// Strings sharing a prefix sort contiguously, directly after the prefix
	// itself, so only the first string at or after each prefix needs to be
	// checked. strs is sorted in place: callers pass slices they own.
	slices.Sort(strs)
	for _, prefix := range prefixes {
		if i, _ := slices.BinarySearch(strs, prefix); i < len(strs) && strings.HasPrefix(strs[i], prefix) {
			return true
		}
	}

	return false
}

func builtinFormatInt(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {

	input, err := builtins.NumberOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	base, err := builtins.NumberOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	var format string
	var radix int
	switch base {
	case ast.Number("2"):
		format = "%b"
		radix = 2
	case ast.Number("8"):
		format = "%o"
		radix = 8
	case ast.Number("10"):
		// Fast path: for numbers whose decimal string is already interned (e.g.
		// "0"–"100"), we can skip strconv.ParseInt entirely.
		if term := ast.InternedStringTermFromNumber(input); term != nil {
			return iter(term)
		}
		if i, ok := input.Int(); ok {
			return iter(ast.InternedIntegerString(i))
		}
		format = "%d"
		radix = 10
	case ast.Number("16"):
		format = "%x"
		radix = 16
	default:
		return builtins.NewOperandEnumErr(2, "2", "8", "10", "16")
	}

	// For integer inputs, format the exact big.Int. Routing integers through a
	// float (as the fractional path below does) loses precision for values that
	// need more than a float64's 53-bit mantissa, e.g. 18446744073709551617.
	if i, ok := new(big.Int).SetString(string(input), 10); ok {
		return iter(ast.InternedTerm(i.Text(radix)))
	}

	// Fractional inputs (e.g. 15.9) are truncated toward zero, matching the
	// historical behaviour: format_int(15.9, 16) == "f", format_int(-15.9, 16) == "-f".
	f := builtins.NumberToFloat(input)
	i, _ := f.Int(nil)

	return iter(ast.InternedTerm(fmt.Sprintf(format, i)))
}

func builtinConcat(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	join, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	// fast path for empty or single string array/set, allocates no memory
	if term, ok := zeroOrOneStringTerm(operands[1].Value); ok {
		return iter(term)
	}

	sb := newSink(ast.Concat.Name, 0, bctx.Cancel)

	// NOTE(anderseknert):
	// More or less Go's strings.Join implementation, but where we avoid
	// creating an intermediate []string slice to pass to that function,
	// as that's expensive (3.5x more space allocated). Instead we build
	// the string directly using the sink to concatenate the string
	// values from the array/set with the separator.
	n := 0
	switch b := operands[1].Value.(type) {
	case *ast.Array:
		l := b.Len()
		for i := range l {
			s, ok := b.Elem(i).Value.(ast.String)
			if !ok {
				return builtins.NewOperandElementErr(2, b, b.Elem(i).Value, "string")
			}
			n += len(s)
		}
		sep := string(join)
		n += len(sep) * (l - 1)
		sb.Grow(n)
		if _, err := sb.WriteString(string(b.Elem(0).Value.(ast.String))); err != nil {
			return err
		}
		if sep == "" {
			for i := 1; i < l; i++ {
				if _, err := sb.WriteString(string(b.Elem(i).Value.(ast.String))); err != nil {
					return err
				}
			}
		} else if len(sep) == 1 {
			// when the separator is a single byte, sb.WriteByte is substantially faster
			bsep := sep[0]
			for i := 1; i < l; i++ {
				if err := sb.WriteByte(bsep); err != nil {
					return err
				}
				if _, err := sb.WriteString(string(b.Elem(i).Value.(ast.String))); err != nil {
					return err
				}
			}
		} else {
			// for longer separators, there is no such difference between WriteString and Write
			for i := 1; i < l; i++ {
				if _, err := sb.WriteString(sep); err != nil {
					return err
				}
				if _, err := sb.WriteString(string(b.Elem(i).Value.(ast.String))); err != nil {
					return err
				}
			}
		}
		return iter(ast.InternedTerm(sb.String()))
	case ast.Set:
		for _, v := range b.Slice() {
			s, ok := v.Value.(ast.String)
			if !ok {
				return builtins.NewOperandElementErr(2, b, v.Value, "string")
			}
			n += len(s)
		}
		sep := string(join)
		l := b.Len()
		n += len(sep) * (l - 1)
		sb.Grow(n)
		for i, v := range b.Slice() {
			if _, err := sb.WriteString(string(v.Value.(ast.String))); err != nil {
				return err
			}
			if i < l-1 {
				if _, err := sb.WriteString(sep); err != nil {
					return err
				}
			}
		}
		return iter(ast.InternedTerm(sb.String()))
	}

	return builtins.NewOperandTypeErr(2, operands[1].Value, "set", "array")
}

func zeroOrOneStringTerm(a ast.Value) (*ast.Term, bool) {
	switch b := a.(type) {
	case *ast.Array:
		if b.Len() == 0 {
			return ast.InternedEmptyString, true
		}
		if b.Len() == 1 {
			e := b.Elem(0)
			if _, ok := e.Value.(ast.String); ok {
				return e, true
			}
		}
	case ast.Set:
		if b.Len() == 0 {
			return ast.InternedEmptyString, true
		}
		if b.Len() == 1 {
			e := b.Slice()[0]
			if _, ok := e.Value.(ast.String); ok {
				return e, true
			}
		}
	}
	return nil, false
}

func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func builtinIndexOf(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	base, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	search, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}
	if len(string(search)) == 0 {
		return errEmptySearchCharacter
	}

	if isASCII(string(base)) && isASCII(string(search)) {
		// this is a false positive in the indexAlloc rule that thinks we're converting
		// byte arrays to strings. still a false positive as of 2026-08-19.
		//nolint:gocritic
		return iter(ast.InternedTerm(strings.Index(string(base), string(search))))
	}

	baseRunes := []rune(string(base))
	searchRunes := []rune(string(search))
	searchLen := len(searchRunes)

	for i, r := range baseRunes {
		if len(baseRunes) >= i+searchLen {
			if r == searchRunes[0] && runesEqual(baseRunes[i:i+searchLen], searchRunes) {
				return iter(ast.InternedTerm(i))
			}
		} else {
			break
		}
	}

	return iter(ast.InternedTerm(-1))
}

func builtinIndexOfN(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	base, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	search, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}
	if len(string(search)) == 0 {
		return errEmptySearchCharacter
	}

	baseRunes := []rune(string(base))
	searchRunes := []rune(string(search))
	searchLen := len(searchRunes)

	var arr []*ast.Term
	for i, r := range baseRunes {
		if len(baseRunes) >= i+searchLen {
			if r == searchRunes[0] && runesEqual(baseRunes[i:i+searchLen], searchRunes) {
				arr = append(arr, ast.InternedTerm(i))
			}
		} else {
			break
		}
	}

	return iter(ast.ArrayTerm(arr...))
}

func builtinSubstring(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	base, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	startIndex, err := builtins.IntOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	length, err := builtins.IntOperand(operands[2].Value, 3)
	if err != nil {
		return err
	}

	if startIndex < 0 {
		return errors.New("negative offset")
	}

	sbase := string(base)
	if sbase == "" {
		return iter(ast.InternedEmptyString)
	}

	// Optimized path for the likely common case of ASCII strings.
	// This allocates less memory and runs in about 1/3 the time.
	if isASCII(sbase) {
		if startIndex >= len(sbase) {
			return iter(ast.InternedEmptyString)
		}

		if length < 0 {
			return iter(ast.InternedTerm(sbase[startIndex:]))
		}

		if startIndex == 0 && length >= len(sbase) {
			return iter(operands[0])
		}

		upto := min(len(sbase), startIndex+length)
		return iter(ast.InternedTerm(sbase[startIndex:upto]))
	}

	if startIndex == 0 && length >= utf8.RuneCountInString(sbase) {
		return iter(operands[0])
	}

	runes := []rune(base)

	if startIndex >= len(runes) {
		return iter(ast.InternedEmptyString)
	}

	var s string
	if length < 0 {
		s = string(runes[startIndex:])
	} else {
		upto := min(len(runes), startIndex+length)
		s = string(runes[startIndex:upto])
	}

	return iter(ast.InternedTerm(s))
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func builtinContains(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	substr, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	return iter(ast.InternedTerm(strings.Contains(string(s), string(substr))))
}

func builtinStringCount(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	substr, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	baseTerm := string(s)
	searchTerm := string(substr)
	count := strings.Count(baseTerm, searchTerm)

	return iter(ast.InternedTerm(count))
}

func builtinStartsWith(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	prefix, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	return iter(ast.InternedTerm(strings.HasPrefix(string(s), string(prefix))))
}

func builtinEndsWith(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	suffix, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	return iter(ast.InternedTerm(strings.HasSuffix(string(s), string(suffix))))
}

func builtinLower(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	arg := string(s)
	low := strings.ToLower(arg)

	if arg == low {
		return iter(operands[0])
	}

	return iter(ast.InternedTerm(low))
}

func builtinUpper(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	arg := string(s)
	upp := strings.ToUpper(arg)

	if arg == upp {
		return iter(operands[0])
	}

	return iter(ast.InternedTerm(upp))
}

func builtinSplit(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	d, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	text, delim := string(s), string(d)
	if !strings.Contains(text, delim) {
		return iter(ast.ArrayTerm(operands[0]))
	}

	return iter(ast.ArrayTerm(util.SplitMap(text, delim, ast.InternedTerm)...))
}

func builtinSplitN(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	d, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	n, err := builtins.IntOperand(operands[2].Value, 3)
	if err != nil {
		return err
	}

	text, delim := string(s), string(d)

	var result []*ast.Term
	if n >= 0 {
		// n+1 may overflow for very large n; a negative limit means no limit.
		limit := n + 1
		if limit < 0 {
			limit = -1
		}
		parts := strings.SplitN(text, delim, limit)
		result = make([]*ast.Term, min(n, len(parts)))
		for i := range result {
			result[i] = ast.InternedTerm(parts[i])
		}
	} else {
		parts := strings.Split(text, delim)
		start := max(len(parts)+n, 0)
		result = util.Map(parts[start:], ast.InternedTerm)
	}

	return iter(ast.ArrayTerm(result...))
}

func builtinReplace(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	old, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	n, err := builtins.StringOperand(operands[2].Value, 3)
	if err != nil {
		return err
	}

	sink := newSink(ast.Replace.Name, len(s), bctx.Cancel)
	replacer := strings.NewReplacer(string(old), string(n))
	if _, err := replacer.WriteString(sink, string(s)); err != nil {
		return err
	}
	replaced := sink.String()
	if replaced == string(s) {
		return iter(operands[0])
	}

	return iter(ast.InternedTerm(replaced))
}

func builtinReplaceN(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	patterns, err := builtins.ObjectOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	s, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	keys := util.SortedFunc(patterns.Keys(), ast.TermValueCompare)
	pairs := make([]string, 0, len(keys)*2)

	for _, k := range keys {
		keyVal, ok := k.Value.(ast.String)
		if !ok {
			return builtins.NewOperandErr(1, "non-string key found in pattern object")
		}
		strVal, ok := patterns.Get(k).Value.(ast.String)
		if !ok {
			return builtins.NewOperandErr(1, "non-string value found in pattern object")
		}
		pairs = append(pairs, string(keyVal), string(strVal))
	}

	sink := newSink(ast.ReplaceN.Name, len(s), bctx.Cancel)
	replacer := strings.NewReplacer(pairs...)
	if _, err := replacer.WriteString(sink, string(s)); err != nil {
		return err
	}
	return iter(ast.InternedTerm(sink.String()))
}

func builtinTrim(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	c, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	str := string(s)
	trimmed := strings.Trim(str, string(c))
	if trimmed == str {
		return iter(operands[0])
	}

	return iter(ast.InternedTerm(trimmed))
}

func builtinTrimLeft(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	c, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	trimmed := strings.TrimLeft(string(s), string(c))
	if trimmed == string(s) {
		return iter(operands[0])
	}

	return iter(ast.InternedTerm(trimmed))
}

func builtinTrimPrefix(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	pre, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	trimmed := strings.TrimPrefix(string(s), string(pre))
	if trimmed == string(s) {
		return iter(operands[0])
	}

	return iter(ast.InternedTerm(trimmed))
}

func builtinTrimRight(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	c, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	trimmed := strings.TrimRight(string(s), string(c))
	if trimmed == string(s) {
		return iter(operands[0])
	}

	return iter(ast.InternedTerm(trimmed))
}

func builtinTrimSuffix(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	suf, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	trimmed := strings.TrimSuffix(string(s), string(suf))
	if trimmed == string(s) {
		return iter(operands[0])
	}

	return iter(ast.InternedTerm(trimmed))
}

func builtinTrimSpace(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	trimmed := strings.TrimSpace(string(s))
	if trimmed == string(s) {
		return iter(operands[0])
	}

	return iter(ast.InternedTerm(trimmed))
}

func builtinSprintf(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	a, err := builtins.ArrayOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	// Optimized path for where sprintf is used as a "to_string" function for
	// a single integer, i.e. sprintf("%d", [x]) where x is an integer.
	if s == "%d" && a.Len() == 1 {
		if n, ok := a.Elem(0).Value.(ast.Number); ok && !isFloatNumber(string(n)) {
			if i, ok := n.Int(); ok {
				if interned := ast.InternedIntegerString(i); interned != nil {
					return iter(interned)
				}
				return iter(ast.StringTerm(strconv.Itoa(i)))
			}
		}
	}

	args := make([]any, a.Len())

	for i := range args {
		t := a.Elem(i)
		switch v := t.Value.(type) {
		case ast.Number:
			ns := string(v)
			if isFloatNumber(ns) {
				if f, ok := v.Float64(); ok {
					args[i] = f
				} else {
					args[i] = ns
				}
			} else if x, ok := util.Atoi64(ns); ok {
				args[i] = x
			} else if b, ok := new(big.Int).SetString(ns, 10); ok {
				args[i] = b
			} else {
				args[i] = ns
			}
		case ast.String:
			args[i] = string(v)
		default:
			args[i] = t.Value.String()
		}
	}

	return iter(ast.InternedTerm(fmt.Sprintf(string(s), args...)))
}

// isFloatNumber reports whether the textual representation of a number is that
// of a floating point value, i.e. it has a fraction or an exponent. Since
// util.Atoi64 parses numbers with only zeros past the decimal point (1.0) as
// integers, the text, and not the parsed value, decides how a number is
// formatted by sprintf.
func isFloatNumber(s string) bool {
	return strings.ContainsAny(s, ".eE")
}

func builtinReverse(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	s, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	return iter(ast.InternedTerm(reverseString(string(s))))
}

func reverseString(str string) string {
	var buf []byte
	var arr [255]byte
	size := len(str)

	if size < 255 {
		buf = arr[:size:size]
	} else {
		buf = make([]byte, size)
	}

	for start := 0; start < size; {
		r, n := utf8.DecodeRuneInString(str[start:])
		start += n
		utf8.EncodeRune(buf[size-start:], r)
	}

	return util.ByteSliceToString(buf)
}

func init() {
	defaultCacheEntries, defaultSeenCacheEntries := 10, 1000
	for _, c := range []affixCaches{anyPrefixMatchCaches, anySuffixMatchCaches} {
		cache.RegisterDefaultInterQueryBuiltinValueCacheConfig(c.tries, &cache.NamedValueCacheConfig{
			MaxNumEntries: &defaultCacheEntries,
		})
		cache.RegisterDefaultInterQueryBuiltinValueCacheConfig(c.seen, &cache.NamedValueCacheConfig{
			MaxNumEntries: &defaultSeenCacheEntries,
		})
	}

	RegisterBuiltinFunc(ast.FormatInt.Name, builtinFormatInt)
	RegisterBuiltinFunc(ast.Concat.Name, builtinConcat)
	RegisterBuiltinFunc(ast.IndexOf.Name, builtinIndexOf)
	RegisterBuiltinFunc(ast.IndexOfN.Name, builtinIndexOfN)
	RegisterBuiltinFunc(ast.Substring.Name, builtinSubstring)
	RegisterBuiltinFunc(ast.Contains.Name, builtinContains)
	RegisterBuiltinFunc(ast.StringCount.Name, builtinStringCount)
	RegisterBuiltinFunc(ast.StartsWith.Name, builtinStartsWith)
	RegisterBuiltinFunc(ast.EndsWith.Name, builtinEndsWith)
	RegisterBuiltinFunc(ast.Upper.Name, builtinUpper)
	RegisterBuiltinFunc(ast.Lower.Name, builtinLower)
	RegisterBuiltinFunc(ast.Split.Name, builtinSplit)
	RegisterBuiltinFunc(ast.SplitN.Name, builtinSplitN)
	RegisterBuiltinFunc(ast.Replace.Name, builtinReplace)
	RegisterBuiltinFunc(ast.ReplaceN.Name, builtinReplaceN)
	RegisterBuiltinFunc(ast.Trim.Name, builtinTrim)
	RegisterBuiltinFunc(ast.TrimLeft.Name, builtinTrimLeft)
	RegisterBuiltinFunc(ast.TrimPrefix.Name, builtinTrimPrefix)
	RegisterBuiltinFunc(ast.TrimRight.Name, builtinTrimRight)
	RegisterBuiltinFunc(ast.TrimSuffix.Name, builtinTrimSuffix)
	RegisterBuiltinFunc(ast.TrimSpace.Name, builtinTrimSpace)
	RegisterBuiltinFunc(ast.Sprintf.Name, builtinSprintf)
	RegisterBuiltinFunc(ast.AnyPrefixMatch.Name, builtinAnyPrefixMatch)
	RegisterBuiltinFunc(ast.AnySuffixMatch.Name, builtinAnySuffixMatch)
	RegisterBuiltinFunc(ast.StringReverse.Name, builtinReverse)
}
