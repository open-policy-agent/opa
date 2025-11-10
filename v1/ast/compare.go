// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"cmp"
	"fmt"
	"math/big"
	"strings"
)

// Compare returns an integer indicating whether two AST values are less than,
// equal to, or greater than each other.
//
// If a is less than b, the return value is negative. If a is greater than b,
// the return value is positive. If a is equal to b, the return value is zero.
//
// Different types are never equal to each other. For comparison purposes, types
// are sorted as follows:
//
// nil < Null < Boolean < Number < String < Var < Ref < Array < Object < Set <
// ArrayComprehension < ObjectComprehension < SetComprehension < Expr < SomeDecl
// < With < Body < Rule < Import < Package < Module.
//
// Arrays and Refs are equal if and only if both a and b have the same length
// and all corresponding elements are equal. If one element is not equal, the
// return value is the same as for the first differing element. If all elements
// are equal but a and b have different lengths, the shorter is considered less
// than the other.
//
// Objects are considered equal if and only if both a and b have the same sorted
// (key, value) pairs and are of the same length. Other comparisons are
// consistent but not defined.
//
// Sets are considered equal if and only if the symmetric difference of a and b
// is empty.
// Other comparisons are consistent but not defined.
func Compare(a, b any) int {

	if t, ok := a.(*Term); ok {
		if t == nil {
			a = nil
		} else {
			a = t.Value
		}
	}

	if t, ok := b.(*Term); ok {
		if t == nil {
			b = nil
		} else {
			b = t.Value
		}
	}

	if a == nil {
		if b == nil {
			return 0
		}
		return -1
	}
	if b == nil {
		return 1
	}

	sortA := sortOrder(a)
	sortB := sortOrder(b)

	if sortA < sortB {
		return -1
	} else if sortB < sortA {
		return 1
	}

	switch a := a.(type) {
	case Null:
		return 0
	case Boolean:
		if a == b.(Boolean) {
			return 0
		}
		if !a {
			return -1
		}
		return 1
	case Number:
		return NumberCompare(a, b.(Number))
	case String:
		b := b.(String)
		if a == b {
			return 0
		}
		if a < b {
			return -1
		}
		return 1
	case Var:
		return VarCompare(a, b.(Var))
	case Ref:
		return termSliceCompare(a, b.(Ref))
	case *Array:
		b := b.(*Array)
		return termSliceCompare(a.elems, b.elems)
	case *lazyObj:
		return Compare(a.force(), b)
	case *object:
		if x, ok := b.(*lazyObj); ok {
			b = x.force()
		}
		return a.Compare(b.(*object))
	case Set:
		return a.Compare(b.(Set))
	case *ArrayComprehension:
		b := b.(*ArrayComprehension)
		if cmp := Compare(a.Term, b.Term); cmp != 0 {
			return cmp
		}
		return a.Body.Compare(b.Body)
	case *ObjectComprehension:
		b := b.(*ObjectComprehension)
		if cmp := Compare(a.Key, b.Key); cmp != 0 {
			return cmp
		}
		if cmp := Compare(a.Value, b.Value); cmp != 0 {
			return cmp
		}
		return a.Body.Compare(b.Body)
	case *SetComprehension:
		b := b.(*SetComprehension)
		if cmp := Compare(a.Term, b.Term); cmp != 0 {
			return cmp
		}
		return a.Body.Compare(b.Body)
	case Call:
		return termSliceCompare(a, b.(Call))
	case *Expr:
		return a.Compare(b.(*Expr))
	case *SomeDecl:
		return a.Compare(b.(*SomeDecl))
	case *Every:
		return a.Compare(b.(*Every))
	case *With:
		return a.Compare(b.(*With))
	case Body:
		return a.Compare(b.(Body))
	case *Head:
		return a.Compare(b.(*Head))
	case *Rule:
		return a.Compare(b.(*Rule))
	case Args:
		return termSliceCompare(a, b.(Args))
	case *Import:
		return a.Compare(b.(*Import))
	case *Package:
		return a.Compare(b.(*Package))
	case *Annotations:
		return a.Compare(b.(*Annotations))
	case *Module:
		return a.Compare(b.(*Module))
	}
	panic(fmt.Sprintf("illegal value: %T", a))
}

type termSlice []*Term

func (s termSlice) Less(i, j int) bool { return Compare(s[i].Value, s[j].Value) < 0 }
func (s termSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s termSlice) Len() int           { return len(s) }

func sortOrder(x any) int {
	switch x.(type) {
	case Null:
		return 0
	case Boolean:
		return 1
	case Number:
		return 2
	case String:
		return 3
	case Var:
		return 4
	case Ref:
		return 5
	case *Array:
		return 6
	case Object:
		return 7
	case Set:
		return 8
	case *ArrayComprehension:
		return 9
	case *ObjectComprehension:
		return 10
	case *SetComprehension:
		return 11
	case Call:
		return 12
	case Args:
		return 13
	case *Expr:
		return 100
	case *SomeDecl:
		return 101
	case *Every:
		return 102
	case *With:
		return 110
	case *Head:
		return 120
	case Body:
		return 200
	case *Rule:
		return 1000
	case *Import:
		return 1001
	case *Package:
		return 1002
	case *Annotations:
		return 1003
	case *Module:
		return 10000
	}
	panic(fmt.Sprintf("illegal value: %T", x))
}

func importsCompare(a, b []*Import) int {
	minLen := min(len(b), len(a))
	for i := range minLen {
		if cmp := a[i].Compare(b[i]); cmp != 0 {
			return cmp
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(b) < len(a) {
		return 1
	}
	return 0
}

func annotationsCompare(a, b []*Annotations) int {
	minLen := min(len(b), len(a))
	for i := range minLen {
		if cmp := a[i].Compare(b[i]); cmp != 0 {
			return cmp
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(b) < len(a) {
		return 1
	}
	return 0
}

func rulesCompare(a, b []*Rule) int {
	minLen := min(len(b), len(a))
	for i := range minLen {
		if cmp := a[i].Compare(b[i]); cmp != 0 {
			return cmp
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(b) < len(a) {
		return 1
	}
	return 0
}

func termSliceCompare(a, b []*Term) int {
	minLen := min(len(b), len(a))
	for i := range minLen {
		if cmp := Compare(a[i], b[i]); cmp != 0 {
			return cmp
		}
	}
	if len(a) < len(b) {
		return -1
	} else if len(b) < len(a) {
		return 1
	}
	return 0
}

func withSliceCompare(a, b []*With) int {
	minLen := min(len(b), len(a))
	for i := range minLen {
		if cmp := Compare(a[i], b[i]); cmp != 0 {
			return cmp
		}
	}
	if len(a) < len(b) {
		return -1
	} else if len(b) < len(a) {
		return 1
	}
	return 0
}

func VarCompare(a, b Var) int {
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

func TermValueCompare(a, b *Term) int {
	return a.Value.Compare(b.Value)
}

func TermValueEqual(a, b *Term) bool {
	return ValueEqual(a.Value, b.Value)
}

func ValueEqual(a, b Value) bool {
	// TODO(ae): why doesn't this work the same?
	//
	// case interface{ Equal(Value) bool }:
	// 	   return v.Equal(b)
	//
	// When put on top, golangci-lint even flags the other cases as unreachable..
	// but TestTopdownVirtualCache will have failing test cases when we replace
	// the other cases with the above one.. 🤔
	switch v := a.(type) {
	case Null:
		return v.Equal(b)
	case Boolean:
		return v.Equal(b)
	case Number:
		return v.Equal(b)
	case String:
		return v.Equal(b)
	case Var:
		return v.Equal(b)
	case Ref:
		return v.Equal(b)
	case *Array:
		return v.Equal(b)
	}

	return a.Compare(b) == 0
}

func RefCompare(a, b Ref) int {
	return termSliceCompare(a, b)
}

func RefEqual(a, b Ref) bool {
	return termSliceEqual(a, b)
}

func NumberCompare(x, y Number) int {
	xs, ys := string(x), string(y)

	var xIsF, yIsF bool

	// Treat "1" and "1.0", "1.00", etc as "1"
	if strings.Contains(xs, ".") {
		if tx := strings.TrimRight(xs, ".0"); tx != xs {
			// Still a float after trimming?
			xIsF = strings.Contains(tx, ".")
			xs = tx
		}
	}
	if strings.Contains(ys, ".") {
		if ty := strings.TrimRight(ys, ".0"); ty != ys {
			yIsF = strings.Contains(ty, ".")
			ys = ty
		}
	}
	if xs == ys {
		return 0
	}

	var xi, yi int64
	var xf, yf float64
	var xiOK, yiOK, xfOK, yfOK bool

	if xi, xiOK = x.Int64(); xiOK {
		if yi, yiOK = y.Int64(); yiOK {
			return cmp.Compare(xi, yi)
		}
	}

	if xIsF && yIsF {
		if xf, xfOK = x.Float64(); xfOK {
			if yf, yfOK = y.Float64(); yfOK {
				if xf == yf {
					return 0
				}
				// could still be "equal" depending on precision, so we continue?
			}
		}
	}

	var a *big.Rat
	fa, ok := new(big.Float).SetString(string(x))
	if !ok {
		panic("illegal value")
	}
	if fa.IsInt() {
		if i, _ := fa.Int64(); i == 0 {
			a = new(big.Rat).SetInt64(0)
		}
	}
	if a == nil {
		a, ok = new(big.Rat).SetString(string(x))
		if !ok {
			panic("illegal value")
		}
	}

	var b *big.Rat
	fb, ok := new(big.Float).SetString(string(y))
	if !ok {
		panic("illegal value")
	}
	if fb.IsInt() {
		if i, _ := fb.Int64(); i == 0 {
			b = new(big.Rat).SetInt64(0)
		}
	}
	if b == nil {
		b, ok = new(big.Rat).SetString(string(y))
		if !ok {
			panic("illegal value")
		}
	}

	return a.Cmp(b)
}
