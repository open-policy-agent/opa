// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opalog

import "fmt"
import "regexp"
import "strconv"
import "strings"

// Location records a position in source code
type Location struct {
	Text []byte // The original text fragment from the source.
	File string // The name of the source file (which may be empty).
	Row  int    // The line in the source.
	Col  int    // The column in the row.
}

// NewLocation returns a new Location object.
func NewLocation(text []byte, file string, row int, col int) *Location {
	return &Location{Text: text, File: file, Row: row, Col: col}
}

// Value declares the common interface for all Term values. Every kind of Term value
// in the language is represented as a type that implements this interface:
//
// - Null, Boolean, Number, String
// - Object, Array
// - Variables
// - References
//
type Value interface {
	// Equal returns true if this value equals the other value.
	Equal(other Value) bool

	// String returns a human readable string representation of the value.
	String() string
}

// Term is an argument to a function.
type Term struct {
	Value    Value     // the value of the Term as represented in Go
	Location *Location // the location of the Term in the source
}

// Equal returns true if this term equals the other term. Equality is
// defined for each kind of term.
func (term *Term) Equal(other *Term) bool {
	if term == other {
		return true
	}
	return term.Value.Equal(other.Value)
}

func (term *Term) String() string {
	return term.Value.String()
}

type Null struct{}

func NullTerm() *Term {
	return &Term{Value: Null{}}
}

func NullTermWithLoc(loc *Location) *Term {
	return &Term{Value: Null{}, Location: loc}
}

func (null Null) Equal(other Value) bool {
	switch other.(type) {
	case Null:
		return true
	default:
		return false
	}
}

func (null Null) String() string {
	return "null"
}

type Boolean bool

func BooleanTerm(b bool) *Term {
	return &Term{Value: Boolean(b)}
}

func BooleanTermWithLoc(b bool, loc *Location) *Term {
	return &Term{Value: Boolean(b), Location: loc}
}

func (bol Boolean) Equal(other Value) bool {
	switch other := other.(type) {
	case Boolean:
		return bol == other
	default:
		return false
	}
}

func (bol Boolean) String() string {
	return strconv.FormatBool(bool(bol))
}

type Number float64

func NumberTerm(n float64) *Term {
	return &Term{Value: Number(n)}
}

func NumberTermWithLoc(n float64, loc *Location) *Term {
	return &Term{Value: Number(n), Location: loc}
}

func (num Number) Equal(other Value) bool {
	switch other := other.(type) {
	case Number:
		return num == other
	default:
		return false
	}
}

func (num Number) String() string {
	return strconv.FormatFloat(float64(num), 'G', -1, 64)
}

type String string

func StringTerm(s string) *Term {
	return &Term{Value: String(s)}
}

func StringTermWithLoc(s string, loc *Location) *Term {
	return &Term{Value: String(s), Location: loc}
}

func (str String) Equal(other Value) bool {
	switch other := other.(type) {
	case String:
		return str == other
	default:
		return false
	}
}

func (str String) String() string {
	return strconv.Quote(string(str))
}

type Var string

func VarTerm(v string) *Term {
	return &Term{Value: Var(v)}
}

func VarTermWithLoc(v string, loc *Location) *Term {
	return &Term{Value: Var(v), Location: loc}
}

func (variable Var) Equal(other Value) bool {
	switch other := other.(type) {
	case Var:
		return variable == other
	default:
		return false
	}
}

func (variable Var) String() string {
	return string(variable)
}

type Ref []*Term

func RefTerm(r ...*Term) *Term {
	return &Term{Value: Ref(r)}
}

func RefTermWithLoc(r []*Term, loc *Location) *Term {
	return &Term{Value: Ref(r), Location: loc}
}

func (ref Ref) Equal(other Value) bool {
	switch other := other.(type) {
	case Ref:
		if len(ref) == len(other) {
			for i := range ref {
				if !ref[i].Equal(other[i]) {
					return false
				}
			}
			return true
		}
	}
	return false
}

var varRegexp = regexp.MustCompile("[[:alpha:]_][[:alpha:][:digit:]_]+")

func (ref Ref) String() string {
	buf := []string{string(ref[0].Value.(Var))}
	for _, p := range ref[1:] {
		switch p := p.Value.(type) {
		case String:
			str := string(p)
			if varRegexp.MatchString(str) {
				buf = append(buf, "."+str)
			} else {
				buf = append(buf, "["+p.String()+"]")
			}
		default:
			buf = append(buf, "["+p.String()+"]")
		}
	}
	return strings.Join(buf, "")
}

type Array []*Term

func ArrayTerm(a ...*Term) *Term {
	return &Term{Value: Array(a)}
}

func ArrayTermWithLoc(a []*Term, loc *Location) *Term {
	return &Term{Value: Array(a), Location: loc}
}

func (arr Array) Equal(other Value) bool {
	switch other := other.(type) {
	case Array:
		if len(arr) == len(other) {
			for i := range arr {
				if !arr[i].Equal(other[i]) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func (arr Array) String() string {
	var buf []string
	for _, e := range arr {
		buf = append(buf, e.String())
	}
	return "[" + strings.Join(buf, ", ") + "]"
}

type Object [][2]*Term

func Item(key, value *Term) [2]*Term {
	return [2]*Term{key, value}
}

func ObjectTerm(o ...[2]*Term) *Term {
	return &Term{Value: Object(o)}
}

func ObjectTermWithLoc(o [][2]*Term, loc *Location) *Term {
	return &Term{Value: Object(o), Location: loc}
}

func (obj Object) Equal(other Value) bool {
	switch other := other.(type) {
	case Object:
		if len(obj) == len(other) {
			for i := range obj {
				if !obj[i][0].Equal(other[i][0]) {
					return false
				}
				if !obj[i][1].Equal(other[i][1]) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func (obj Object) String() string {
	var buf []string
	for _, p := range obj {
		buf = append(buf, fmt.Sprintf("%s: %s", p[0], p[1]))
	}
	return "{" + strings.Join(buf, ", ") + "}"
}
