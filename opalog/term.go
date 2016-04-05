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

// Null represents the null value defined by JSON.
type Null struct{}

// NullTerm creates a new Term with a Null value.
func NullTerm() *Term {
	return &Term{Value: Null{}}
}

// NullTermWithLoc creates a new Term with a Null value and a Location.
func NullTermWithLoc(loc *Location) *Term {
	return &Term{Value: Null{}, Location: loc}
}

// Equal returns true if the other term Value is also Null.
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

// Boolean represents a boolean value defined by JSON.
type Boolean bool

// BooleanTerm creates a new Term with a Boolean value.
func BooleanTerm(b bool) *Term {
	return &Term{Value: Boolean(b)}
}

// BooleanTermWithLoc creates a new Term with a Boolean value and a Location.
func BooleanTermWithLoc(b bool, loc *Location) *Term {
	return &Term{Value: Boolean(b), Location: loc}
}

// Equal returns true if the other Value is a Boolean and is equal.
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

// Number represents a numeric value as defined by JSON.
type Number float64

// NumberTerm creates a new Term with a Number value.
func NumberTerm(n float64) *Term {
	return &Term{Value: Number(n)}
}

// NumberTermWithLoc creates a new Term with a Number value and a Location.
func NumberTermWithLoc(n float64, loc *Location) *Term {
	return &Term{Value: Number(n), Location: loc}
}

// Equal returns true if the other Value is a Number and is equal.
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

// String represents a string value as defined by JSON.
type String string

// StringTerm creates a new Term with a String value.
func StringTerm(s string) *Term {
	return &Term{Value: String(s)}
}

// StringTermWithLoc creates a new Term with a String value and a Location.
func StringTermWithLoc(s string, loc *Location) *Term {
	return &Term{Value: String(s), Location: loc}
}

// Equal returns true if the other Value is a String and is equal.
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

// Var represents a variable as defined by Opalog.
type Var string

// VarTerm creates a new Term with a Variable value.
func VarTerm(v string) *Term {
	return &Term{Value: Var(v)}
}

// VarTermWithLoc creates a new Term with a Variable value and a Location.
func VarTermWithLoc(v string, loc *Location) *Term {
	return &Term{Value: Var(v), Location: loc}
}

// Equal returns true if the other Value is a Variable and has the same value
// (name).
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

// Ref represents a variable as defined by Opalog.
type Ref []*Term

// RefTerm creates a new Term with a Ref value.
func RefTerm(r ...*Term) *Term {
	return &Term{Value: Ref(r)}
}

// RefTermWithLoc creates a new Term with a Ref value and a Location.
func RefTermWithLoc(r []*Term, loc *Location) *Term {
	return &Term{Value: Ref(r), Location: loc}
}

// Equal returns true if the other Value is a Ref and the elements of the
// other Ref are equal to the this Ref.
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

// Array represents an array as defined by Opalog. Arrays are similar to the
// same types as defined by JSON with the exception that they can contain Vars
// and References.
type Array []*Term

// ArrayTerm creates a new Term with an Array value.
func ArrayTerm(a ...*Term) *Term {
	return &Term{Value: Array(a)}
}

// ArrayTermWithLoc creates a new Term with an Array value and a Location.
func ArrayTermWithLoc(a []*Term, loc *Location) *Term {
	return &Term{Value: Array(a), Location: loc}
}

// Equal returns true if the other Value is an Array and the elements of the
// other Array are equal to the elements of this Array. The elements are
// ordered.
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

// Object represents an object as defined by Opalog. Objects are similar to
// the same types as defined by JSON with the exception that they can contain
// Vars and References.
type Object [][2]*Term

// Item is a helper for constructing an tuple containing two Terms
// representing a key/value pair in an Object.
func Item(key, value *Term) [2]*Term {
	return [2]*Term{key, value}
}

// ObjectTerm creates a new Term with an Object value.
func ObjectTerm(o ...[2]*Term) *Term {
	return &Term{Value: Object(o)}
}

// ObjectTermWithLoc creates a new Term with an Object value and a Location.
func ObjectTermWithLoc(o [][2]*Term, loc *Location) *Term {
	return &Term{Value: Object(o), Location: loc}
}

// Equal returns true if the other Value is an Object and the key/value pairs
// of the Other object are equal to the key/value pairs of this Object. The
// key/value pairs are ordered.
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
