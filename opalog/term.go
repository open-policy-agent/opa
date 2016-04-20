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

	// IsGround returns true if this value is not a variable or contains no variables.
	IsGround() bool

	// String returns a human readable string representation of the value.
	String() string

	// Returns hash code of the value.
	Hash() int
}

// Term is an argument to a function.
type Term struct {
	Value    Value     // the value of the Term as represented in Go
	Location *Location // the location of the Term in the source
}

// Equal returns true if this term equals the other term. Equality is
// defined for each kind of term.
func (term *Term) Equal(other *Term) bool {
	if term == nil && other != nil {
		return false
	}
	if term != nil && other == nil {
		return false
	}
	if term == other {
		return true
	}
	return term.Value.Equal(other.Value)
}

// IsGround returns true if this terms' Value is ground.
func (term *Term) IsGround() bool {
	return term.Value.IsGround()
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

// Equal returns true if the other term Value is also Null.
func (null Null) Equal(other Value) bool {
	switch other.(type) {
	case Null:
		return true
	default:
		return false
	}
}

// Hash returns the hash code for the Value.
func (null Null) Hash() int {
	return 0
}

// IsGround always returns true.
func (null Null) IsGround() bool {
	return true
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

// Equal returns true if the other Value is a Boolean and is equal.
func (bol Boolean) Equal(other Value) bool {
	switch other := other.(type) {
	case Boolean:
		return bol == other
	default:
		return false
	}
}

// Hash returns the hash code for the Value.
func (bol Boolean) Hash() int {
	if bol {
		return 1
	}
	return 0
}

// IsGround always returns true.
func (bol Boolean) IsGround() bool {
	return true
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

// Equal returns true if the other Value is a Number and is equal.
func (num Number) Equal(other Value) bool {
	switch other := other.(type) {
	case Number:
		return num == other
	default:
		return false
	}
}

// Hash returns the hash code for the Value.
func (num Number) Hash() int {
	return int(num)
}

// IsGround always returns true.
func (num Number) IsGround() bool {
	return true
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

// Equal returns true if the other Value is a String and is equal.
func (str String) Equal(other Value) bool {
	switch other := other.(type) {
	case String:
		return str == other
	default:
		return false
	}
}

// IsGround always returns true.
func (str String) IsGround() bool {
	return true
}

func (str String) String() string {
	return strconv.Quote(string(str))
}

// Hash returns the hash code for the Value.
func (str String) Hash() int {
	return stringHash(string(str))
}

// Var represents a variable as defined by Opalog.
type Var string

// VarTerm creates a new Term with a Variable value.
func VarTerm(v string) *Term {
	return &Term{Value: Var(v)}
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

// Hash returns the hash code for the Value.
func (variable Var) Hash() int {
	return stringHash(string(variable))
}

// IsGround always returns false.
func (variable Var) IsGround() bool {
	return false
}

func (variable Var) String() string {
	return string(variable)
}

// Ref represents a reference as defined by Opalog.
type Ref []*Term

// EmptyRef returns a new, empty reference.
func EmptyRef() Ref {
	return Ref([]*Term{})
}

// RefTerm creates a new Term with a Ref value.
func RefTerm(r ...*Term) *Term {
	return &Term{Value: Ref(r)}
}

// Equal returns true if the other Value is a Ref and the elements of the
// other Ref are equal to the this Ref.
func (ref Ref) Equal(other Value) bool {
	switch other := other.(type) {
	case Ref:
		return termSliceEqual(ref, other)
	}
	return false
}

// Hash returns the hash code for the Value.
func (ref Ref) Hash() int {
	return termSliceHash(ref)
}

// IsGround returns true if all of the parts of the Ref are ground.
func (ref Ref) IsGround() bool {
	if len(ref) == 0 {
		return true
	}
	return termSliceIsGround(ref[1:])
}

var varRegexp = regexp.MustCompile("^[[:alpha:]_][[:alpha:][:digit:]_]*$")

func (ref Ref) String() string {
	var buf []string
	path := ref
	switch v := ref[0].Value.(type) {
	case Var:
		buf = append(buf, string(v))
		path = path[1:]
	}
	for _, p := range path {
		switch p := p.Value.(type) {
		case String:
			str := string(p)
			if varRegexp.MatchString(str) && len(buf) > 0 {
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

// Underlying returns a slice of underlying Go values.
// If the slice is not ground, an error is returned.
func (ref Ref) Underlying() ([]interface{}, error) {

	if !ref.IsGround() {
		return nil, fmt.Errorf("cannot get underlying value for non-ground ref: %v", ref)
	}

	r := []interface{}{}
	if len(ref) == 0 {
		return r, nil
	}

	// The reference head is typically a variable, however,
	// in some cases, we use the slice operator to process the reference
	// in which ase the slice result is still a reference but the head
	// may be a string, number, etc.
	switch head := ref[0].Value.(type) {
	case Var:
		r = append(r, string(head))
	case String:
		r = append(r, string(head))
	case Number:
		r = append(r, float64(head))
	case Boolean:
		r = append(r, bool(head))
	case Null:
		r = append(r, nil)
	default:
		panic(fmt.Sprintf("illegal value: %v", head))
	}

	for _, v := range ref[1:] {
		switch v := v.Value.(type) {
		case String:
			r = append(r, string(v))
		case Number:
			r = append(r, float64(v))
		case Boolean:
			r = append(r, bool(v))
		case Null:
			r = append(r, nil)
		default:
			panic(fmt.Sprintf("illegal value: %v", v))
		}
	}

	return r, nil
}

// QueryIterator defines the interface for querying AST documents with references.
type QueryIterator func(map[Var]Value, Value) error

// Array represents an array as defined by Opalog. Arrays are similar to the
// same types as defined by JSON with the exception that they can contain Vars
// and References.
type Array []*Term

// ArrayTerm creates a new Term with an Array value.
func ArrayTerm(a ...*Term) *Term {
	return &Term{Value: Array(a)}
}

// Equal returns true if the other Value is an Array and the elements of the
// other Array are equal to the elements of this Array. The elements are
// ordered.
func (arr Array) Equal(other Value) bool {
	switch other := other.(type) {
	case Array:
		return termSliceEqual(arr, other)
	}
	return false
}

// Hash returns the hash code for the Value.
func (arr Array) Hash() int {
	return termSliceHash(arr)
}

// IsGround returns true if all of the Array elements are ground.
func (arr Array) IsGround() bool {
	return termSliceIsGround(arr)
}

// Query invokes the iterator for each referenced value inside the array.
func (arr Array) Query(ref Ref, iter QueryIterator) error {
	return arr.queryRec(ref, make(map[Var]Value), iter)
}

func (arr Array) String() string {
	var buf []string
	for _, e := range arr {
		buf = append(buf, e.String())
	}
	return "[" + strings.Join(buf, ", ") + "]"
}

func (arr Array) queryRec(ref Ref, keys map[Var]Value, iter QueryIterator) error {
	if len(ref) == 0 {
		return iter(keys, arr)
	}
	switch head := ref[0].Value.(type) {
	case Var:
		tail := ref[1:]
		for i, v := range arr {
			keys[head] = Number(i)
			if err := queryRec(v.Value, ref, tail, keys, iter, true); err != nil {
				return err
			}
		}
		return nil
	case Number:
		idx := int(head)
		if len(arr) < idx {
			return fmt.Errorf("unexpected index in %v: out of bounds: %v", ref, idx)
		}
		tail := ref[1:]
		return queryRec(arr[idx].Value, ref, tail, keys, iter, false)
	default:
		return fmt.Errorf("unexpected non-numeric index in %v: %v (%T)", ref, head, head)
	}
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

// Hash returns the hash code for the Value.
func (obj Object) Hash() int {
	var hash int
	for i := range obj {
		hash += obj[i][0].Value.Hash()
		hash += obj[i][1].Value.Hash()
	}
	return hash
}

// IsGround returns true if all of the Object key/value pairs are ground.
func (obj Object) IsGround() bool {
	for i := range obj {
		if !obj[i][0].IsGround() {
			return false
		}
		if !obj[i][1].IsGround() {
			return false
		}
	}
	return true
}

// ObjectTerm creates a new Term with an Object value.
func ObjectTerm(o ...[2]*Term) *Term {
	return &Term{Value: Object(o)}
}

// Query invokes the iterator for each referenced value inside the object.
func (obj Object) Query(ref Ref, iter QueryIterator) error {
	return obj.queryRec(ref, make(map[Var]Value), iter)
}

func (obj Object) String() string {
	var buf []string
	for _, p := range obj {
		buf = append(buf, fmt.Sprintf("%s: %s", p[0], p[1]))
	}
	return "{" + strings.Join(buf, ", ") + "}"
}

func (obj Object) queryRec(ref Ref, keys map[Var]Value, iter QueryIterator) error {
	if len(ref) == 0 {
		return iter(keys, obj)
	}
	switch head := ref[0].Value.(type) {
	case Var:
		tail := ref[1:]
		for _, i := range obj {
			keys[head] = i[0].Value
			if err := queryRec(i[1].Value, ref, tail, keys, iter, true); err != nil {
				return err
			}
		}
		return nil

	default:
		found := false
		tail := ref[1:]
		for _, i := range obj {
			if i[0].Value.Equal(head) {
				if err := queryRec(i[1].Value, ref, tail, keys, iter, false); err != nil {
					return err
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing key %v: %v", head, ref)
		}
		return nil
	}
}

func queryRec(v Value, ref Ref, tail Ref, keys map[Var]Value, iter QueryIterator, skipScalar bool) error {
	if len(tail) == 0 {
		if err := iter(keys, v); err != nil {
			return err
		}
	} else {
		switch v := v.(type) {
		case Array:
			if err := v.queryRec(tail, keys, iter); err != nil {
				return err
			}
		case Object:
			if err := v.queryRec(tail, keys, iter); err != nil {
				return err
			}
		default:
			if !skipScalar {
				return fmt.Errorf("unexpected non-composite at %v: %v", ref, v)
			}
		}
	}
	return nil
}

func stringHash(s string) int {
	// FNV-1a hashing
	var hash uint32
	for i := 0; i < len(s); i++ {
		hash ^= uint32(s[i])
		hash *= 16777619
	}
	return int(hash)
}

func termSliceEqual(a, b []*Term) bool {
	if len(a) == len(b) {
		for i := range a {
			if !a[i].Equal(b[i]) {
				return false
			}
		}
		return true
	}
	return false
}

func termSliceHash(a []*Term) int {
	var hash int
	for _, v := range a {
		hash += v.Value.Hash()
	}
	return hash
}

func termSliceIsGround(a []*Term) bool {
	for _, v := range a {
		if !v.IsGround() {
			return false
		}
	}
	return true
}
