// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/dchest/siphash"

	"github.com/pkg/errors"
)

// Location records a position in source code
type Location struct {
	Text []byte `json:"-"` // The original text fragment from the source.
	File string // The name of the source file (which may be empty).
	Row  int    // The line in the source.
	Col  int    // The column in the row.
}

// NewLocation returns a new Location object.
func NewLocation(text []byte, file string, row int, col int) *Location {
	return &Location{Text: text, File: file, Row: row, Col: col}
}

// Errorf returns a new error value with a message formatted to include the location
// info (e.g., line, column, filename, etc.)
func (loc *Location) Errorf(f string, a ...interface{}) error {
	return errors.New(loc.Format(f, a...))
}

// Wrapf returns a new error value that wraps an existing error with a message formatted
// to include the location info (e.g., line, column, filename, etc.)
func (loc *Location) Wrapf(err error, f string, a ...interface{}) error {
	return errors.Wrap(err, loc.Format(f, a...))
}

// Format returns a formatted string prefixed with the location information.
func (loc *Location) Format(f string, a ...interface{}) string {
	if len(loc.File) > 0 {
		f = fmt.Sprintf("%v:%v: %v", loc.File, loc.Row, f)
	} else {
		f = fmt.Sprintf("%v:%v: %v", loc.Row, loc.Col, f)
	}
	return fmt.Sprintf(f, a...)
}

// Value declares the common interface for all Term values. Every kind of Term value
// in the language is represented as a type that implements this interface:
//
// - Null, Boolean, Number, String
// - Object, Array
// - Variables
// - References
// - Array Comprehensions
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

// InterfaceToValue converts a native Go value x to a Value.
func InterfaceToValue(x interface{}) (Value, error) {
	switch x := x.(type) {
	case nil:
		return Null{}, nil
	case bool:
		return Boolean(x), nil
	case float64:
		return Number(x), nil
	case string:
		return String(x), nil
	case []interface{}:
		r := Array{}
		for _, e := range x {
			e, err := InterfaceToValue(e)
			if err != nil {
				return nil, err
			}
			r = append(r, &Term{Value: e})
		}
		return r, nil
	case map[string]interface{}:
		r := Object{}
		for k, v := range x {
			k, err := InterfaceToValue(k)
			if err != nil {
				return nil, err
			}
			v, err := InterfaceToValue(v)
			if err != nil {
				return nil, err
			}
			r = append(r, Item(&Term{Value: k}, &Term{Value: v}))
		}
		return r, nil
	default:
		return nil, fmt.Errorf("illegal value: %v", x)
	}
}

// Term is an argument to a function.
type Term struct {
	Value    Value     // the value of the Term as represented in Go
	Location *Location `json:"-"` // the location of the Term in the source
}

// NewTerm returns a new Term object.
func NewTerm(v Value) *Term {
	return &Term{
		Value: v,
	}
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

// Hash returns the hash code of the Term's value.
func (term *Term) Hash() int {
	return term.Value.Hash()
}

// IsGround returns true if this terms' Value is ground.
func (term *Term) IsGround() bool {
	return term.Value.IsGround()
}

// MarshalJSON returns the JSON encoding of the term.
// Specialized marshalling logic is required to include a type hint
// for Value.
func (term *Term) MarshalJSON() ([]byte, error) {
	var typ string
	switch term.Value.(type) {
	case Null:
		typ = "null"
	case Boolean:
		typ = "boolean"
	case Number:
		typ = "number"
	case String:
		typ = "string"
	case Ref:
		typ = "ref"
	case Var:
		typ = "var"
	case Array:
		typ = "array"
	case Object:
		typ = "object"
	case *Set:
		typ = "set"
	case *ArrayComprehension:
		typ = "array-comprehension"
	}
	d := map[string]interface{}{
		"Type":  typ,
		"Value": term.Value,
	}
	return json.Marshal(d)
}

func (term *Term) String() string {
	return term.Value.String()
}

// UnmarshalJSON parses the byte array and stores the result in term.
// Specialized unmarshalling is required to handle Value.
func (term *Term) UnmarshalJSON(bs []byte) error {
	v := map[string]interface{}{}
	if err := json.Unmarshal(bs, &v); err != nil {
		return err
	}
	val, err := unmarshalValue(v)
	if err != nil {
		return err
	}
	term.Value = val
	return nil
}

// Vars returns a VarSet with variables contained in this term.
func (term *Term) Vars() VarSet {
	vis := &varVisitor{vars: VarSet{}}
	Walk(vis, term)
	return vis.vars
}

// IsScalar returns true if the AST value is a scalar.
func IsScalar(v Value) bool {
	switch v.(type) {
	case String:
		return true
	case Number:
		return true
	case Boolean:
		return true
	case Null:
		return true
	}
	return false
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
	h := siphash.Hash(hashSeed0, hashSeed1, *(*[]byte)(unsafe.Pointer(&str)))
	return int(h)
}

// Var represents a variable as defined by the language.
type Var string

// VarTerm creates a new Term with a Variable value.
func VarTerm(v string) *Term {
	return &Term{Value: Var(v)}
}

// Equal returns true if the other Value is a Variable and has the same value
// (name).
func (v Var) Equal(other Value) bool {
	switch other := other.(type) {
	case Var:
		return v == other
	default:
		return false
	}
}

// Hash returns the hash code for the Value.
func (v Var) Hash() int {
	h := siphash.Hash(hashSeed0, hashSeed1, *(*[]byte)(unsafe.Pointer(&v)))
	return int(h)
}

// IsGround always returns false.
func (v Var) IsGround() bool {
	return false
}

// IsWildcard returns true if this is a wildcard variable.
func (v Var) IsWildcard() bool {
	return strings.HasPrefix(string(v), WildcardPrefix)
}

func (v Var) String() string {
	// Special case for wildcard so that string representation is parseable. The
	// parser mangles wildcard variables to make their names unique and uses an
	// illegal variable name character (WildcardPrefix) to avoid conflicts. When
	// we serialize the variable here, we need to make sure it's parseable.
	if v.IsWildcard() {
		return Wildcard.String()
	}
	return string(v)
}

// Ref represents a reference as defined by the language.
type Ref []*Term

// EmptyRef returns a new, empty reference.
func EmptyRef() Ref {
	return Ref([]*Term{})
}

// RefTerm creates a new Term with a Ref value.
func RefTerm(r ...*Term) *Term {
	return &Term{Value: Ref(r)}
}

// Append returns a copy of ref with the term appended to the end.
func (ref Ref) Append(term *Term) Ref {
	n := len(ref)
	dst := make(Ref, n+1)
	copy(dst, ref)
	dst[n] = term
	return dst
}

// Equal returns true if ref is equal to other.
func (ref Ref) Equal(other Value) bool {
	return Compare(ref, other) == 0
}

// Hash returns the hash code for the Value.
func (ref Ref) Hash() int {
	return termSliceHash(ref)
}

// HasPrefix returns true if the other ref is a prefix of this ref.
func (ref Ref) HasPrefix(other Ref) bool {
	if len(other) > len(ref) {
		return false
	}
	for i := range other {
		if !ref[i].Equal(other[i]) {
			return false
		}
	}
	return true
}

// GroundPrefix returns the ground portion of the ref starting from the head. By
// definition, the head of the reference is always ground.
func (ref Ref) GroundPrefix() Ref {
	prefix := make(Ref, 0, len(ref))

	for i, x := range ref {
		if i > 0 && !x.IsGround() {
			break
		}
		prefix = append(prefix, x)
	}

	return prefix
}

// IsGround returns true if all of the parts of the Ref are ground.
func (ref Ref) IsGround() bool {
	if len(ref) == 0 {
		return true
	}
	return termSliceIsGround(ref[1:])
}

// IsNested returns true if this ref contains other Refs.
func (ref Ref) IsNested() bool {
	for _, x := range ref {
		if _, ok := x.Value.(Ref); ok {
			return true
		}
	}
	return false
}

var varRegexp = regexp.MustCompile("^[[:alpha:]_][[:alpha:][:digit:]_]*$")

func (ref Ref) String() string {
	if len(ref) == 0 {
		return ""
	}
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
		panic(fmt.Sprintf("illegal value: %v %v", head, ref))
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
			panic(fmt.Sprintf("illegal value: %v %v", v, ref))
		}
	}

	return r, nil
}

// OutputVars returns a VarSet containing variables that would be bound by evaluating
//  this expression in isolation.
func (ref Ref) OutputVars() VarSet {
	vis := &varVisitor{
		vars:        VarSet{},
		skipRefHead: true,
	}
	Walk(vis, ref)
	return vis.vars
}

// QueryIterator defines the interface for querying AST documents with references.
type QueryIterator func(map[Var]Value, Value) error

// Array represents an array as defined by the language. Arrays are similar to the
// same types as defined by JSON with the exception that they can contain Vars
// and References.
type Array []*Term

// ArrayTerm creates a new Term with an Array value.
func ArrayTerm(a ...*Term) *Term {
	return &Term{Value: Array(a)}
}

// Equal returns true if arr is equal to other.
func (arr Array) Equal(other Value) bool {
	return Compare(arr, other) == 0
}

// Hash returns the hash code for the Value.
func (arr Array) Hash() int {
	return termSliceHash(arr)
}

// IsGround returns true if all of the Array elements are ground.
func (arr Array) IsGround() bool {
	return termSliceIsGround(arr)
}

func (arr Array) String() string {
	var buf []string
	for _, e := range arr {
		buf = append(buf, e.String())
	}
	return "[" + strings.Join(buf, ", ") + "]"
}

// Set represents a set as defined by the language.
type Set []*Term

// SetTerm returns a new Term representing a set containing terms t.
func SetTerm(t ...*Term) *Term {
	s := &Set{}
	for i := range t {
		s.Add(t[i])
	}
	return &Term{
		Value: s,
	}
}

// IsGround returns true if all terms in s are ground.
func (s *Set) IsGround() bool {
	return termSliceIsGround(*s)
}

// Hash returns a hash code for s.
func (s *Set) Hash() int {
	return termSliceHash(*s)
}

func (s *Set) String() string {

	sl := *s

	if len(sl) == 0 {
		return "set()"
	}

	buf := make([]string, len(sl))

	for i := range sl {
		buf[i] = sl[i].String()
	}

	return "{" + strings.Join(buf, ", ") + "}"
}

// Equal returns true if s is equal to v.
func (s *Set) Equal(v Value) bool {
	return Compare(s, v) == 0
}

// Diff returns elements in s that are not in other.
func (s *Set) Diff(other *Set) *Set {
	r := &Set{}
	for _, x := range *s {
		if !other.Contains(x) {
			r.Add(x)
		}
	}
	return r
}

// Add updates s to include t.
func (s *Set) Add(t *Term) {
	if s.Contains(t) {
		return
	}
	*s = append(*s, t)
}

// Map returns a new Set obtained by applying f to each value in s.
func (s *Set) Map(f func(*Term) (*Term, error)) (*Set, error) {
	sl := *s
	set := &Set{}
	for i := range sl {
		term, err := f(sl[i])
		if err != nil {
			return nil, err
		}
		set.Add(term)
	}
	return set, nil
}

// Contains returns true if t is in s.
func (s Set) Contains(t *Term) bool {
	for i := range s {
		if s[i].Equal(t) {
			return true
		}
	}
	return false
}

// Object represents an object as defined by the language. Objects are similar to
// the same types as defined by JSON with the exception that they can contain
// Vars and References.
type Object [][2]*Term

// Item is a helper for constructing an tuple containing two Terms
// representing a key/value pair in an Object.
func Item(key, value *Term) [2]*Term {
	return [2]*Term{key, value}
}

// Equal returns true if obj is equal to other.
func (obj Object) Equal(other Value) bool {
	return Compare(obj, other) == 0
}

// Get returns the value of k in obj if k exists, otherwise nil.
func (obj Object) Get(k *Term) *Term {
	for _, pair := range obj {
		if pair[0].Equal(k) {
			return pair[1]
		}
	}
	return nil
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

// Diff returns a new Object that contains only the key/value pairs that exist in obj.
func (obj Object) Diff(other Object) Object {
	r := Object{}
	for _, i := range obj {
		found := false
		for _, j := range other {
			if j[0].Equal(i[0]) {
				found = true
				break
			}
		}
		if !found {
			r = append(r, i)
		}
	}
	return r
}

// Intersect returns a slice of term triplets that represent the intersection of keys
// between obj and other. For each intersecting key, the values from obj and other are included
// as the last two terms in the triplet (respectively).
func (obj Object) Intersect(other Object) [][3]*Term {
	r := [][3]*Term{}
	for _, i := range obj {
		for _, j := range other {
			if i[0].Equal(j[0]) {
				r = append(r, [...]*Term{&Term{Value: i[0].Value}, i[1], j[1]})
			}
		}
	}
	return r
}

// Keys returns the keys of obj.
func (obj Object) Keys() []*Term {
	keys := make([]*Term, len(obj))
	for i, pair := range obj {
		keys[i] = pair[0]
	}
	return keys
}

// Merge returns a new Object containing the non-overlapping keys of obj and other. If there are
// overlapping keys between obj and other, the values of associated with the keys are merged. Only
// objects can be merged with other objects. If the values cannot be merged, the second turn value
// will be false.
func (obj Object) Merge(other Object) (Object, bool) {
	r := Object{}
	r = append(r, obj.Diff(other)...)
	r = append(r, other.Diff(obj)...)
	for _, vs := range obj.Intersect(other) {
		var merged Value
		switch v1 := vs[1].Value.(type) {
		case Object:
			switch v2 := vs[2].Value.(type) {
			case Object:
				m, ok := v1.Merge(v2)
				if !ok {
					return nil, false
				}
				merged = m
			}
		}
		if merged == nil {
			return nil, false
		}
		r = append(r, [2]*Term{vs[0], &Term{Value: merged}})
	}
	return r, true
}

func (obj Object) String() string {
	var buf []string
	for _, p := range obj {
		buf = append(buf, fmt.Sprintf("%s: %s", p[0], p[1]))
	}
	return "{" + strings.Join(buf, ", ") + "}"
}

// ArrayComprehension represents an array comprehension as defined in the language.
type ArrayComprehension struct {
	Term *Term
	Body Body
}

// ArrayComprehensionTerm creates a new Term with an ArrayComprehension value.
func ArrayComprehensionTerm(term *Term, body Body) *Term {
	return &Term{
		Value: &ArrayComprehension{
			Term: term,
			Body: body,
		},
	}
}

// Equal returns true if ac is equal to other.
func (ac *ArrayComprehension) Equal(other Value) bool {
	return Compare(ac, other) == 0
}

// Hash returns the hash code of the Value.
func (ac *ArrayComprehension) Hash() int {
	return ac.Term.Hash() + ac.Body.Hash()
}

// IsGround returns true if the Term and Body are ground.
func (ac *ArrayComprehension) IsGround() bool {
	return ac.Term.IsGround() && ac.Body.IsGround()
}

func (ac *ArrayComprehension) String() string {
	return "[" + ac.Term.String() + " | " + ac.Body.String() + "]"
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

// NOTE(tsandall): The unmarshalling errors in these functions are not
// helpful for callers because they do not identify the source of the
// unmarshalling error. Because OPA doesn't accept JSON describing ASTs
// from callers, this is acceptable (for now). If that changes in the future,
// the error messages should be revisited. The current approach focuses
// on the happy path and treats all errors the same. If better error
// reporting is needed, the error paths will need to be fleshed out.

func unmarshalBody(b []interface{}) (Body, error) {
	buf := Body{}
	for _, e := range b {
		if m, ok := e.(map[string]interface{}); ok {
			expr := &Expr{}
			if err := unmarshalExpr(expr, m); err == nil {
				buf = append(buf, expr)
				continue
			}
		}
		goto unmarshal_error
	}
	return buf, nil
unmarshal_error:
	return nil, fmt.Errorf("ast: unable to unmarshal body")
}

func unmarshalExpr(expr *Expr, v map[string]interface{}) error {
	if x, ok := v["Negated"]; ok {
		if b, ok := x.(bool); ok {
			expr.Negated = b
		} else {
			return fmt.Errorf("ast: unable to unmarshal Negated field with type: %T (expected true or false)", v["Negated"])
		}
	}
	if err := unmarshalExprIndex(expr, v); err != nil {
		return err
	}
	switch ts := v["Terms"].(type) {
	case map[string]interface{}:
		t, err := unmarshalTerm(ts)
		if err != nil {
			return err
		}
		expr.Terms = t
	case []interface{}:
		terms, err := unmarshalTermSlice(ts)
		if err != nil {
			return err
		}
		expr.Terms = terms
	default:
		return fmt.Errorf(`ast: unable to unmarshal Terms field with type: %T (expected {"Value": ..., "Type": ...} or [{"Value": ..., "Type": ...}, ...])`, v["Terms"])
	}
	return nil
}

func unmarshalExprIndex(expr *Expr, v map[string]interface{}) error {
	if x, ok := v["Index"]; ok {
		if f, ok := x.(float64); ok {
			i := int(f)
			if float64(i) == f {
				expr.Index = i
				return nil
			}
		}
	}
	return fmt.Errorf("ast: unable to unmarshal Index field with type: %T (expected integer)", v["Index"])
}

func unmarshalTerm(m map[string]interface{}) (*Term, error) {
	v, err := unmarshalValue(m)
	if err != nil {
		return nil, err
	}
	return &Term{Value: v}, nil
}

func unmarshalTermSlice(s []interface{}) ([]*Term, error) {
	buf := []*Term{}
	for _, x := range s {
		if m, ok := x.(map[string]interface{}); ok {
			if t, err := unmarshalTerm(m); err == nil {
				buf = append(buf, t)
				continue
			}
		}
		return nil, fmt.Errorf("ast: unable to unmarshal term")
	}
	return buf, nil
}

func unmarshalTermSliceValue(d map[string]interface{}) ([]*Term, error) {
	if s, ok := d["Value"].([]interface{}); ok {
		return unmarshalTermSlice(s)
	}
	return nil, fmt.Errorf(`ast: unable to unmarshal term (expected {"Value": [...], "Type": ...} where type is one of: array, reference)`)
}

func unmarshalValue(d map[string]interface{}) (Value, error) {
	v := d["Value"]
	switch d["Type"] {
	case "null":
		return Null{}, nil
	case "boolean":
		if b, ok := v.(bool); ok {
			return Boolean(b), nil
		}
	case "number":
		if n, ok := v.(float64); ok {
			return Number(n), nil
		}
	case "string":
		if s, ok := v.(string); ok {
			return String(s), nil
		}
	case "var":
		if s, ok := v.(string); ok {
			return Var(s), nil
		}
	case "ref":
		if s, err := unmarshalTermSliceValue(d); err == nil {
			return Ref(s), nil
		}
	case "array":
		if s, err := unmarshalTermSliceValue(d); err == nil {
			return Array(s), nil
		}
	case "set":
		if s, err := unmarshalTermSliceValue(d); err == nil {
			set := &Set{}
			for _, x := range s {
				set.Add(x)
			}
			return set, nil
		}
	case "object":
		if s, ok := v.([]interface{}); ok {
			buf := Object{}
			for _, x := range s {
				if i, ok := x.([]interface{}); ok && len(i) == 2 {
					p, err := unmarshalTermSlice(i)
					if err == nil {
						buf = append(buf, Item(p[0], p[1]))
						continue
					}
				}
				goto unmarshal_error
			}
			return buf, nil
		}
	case "array-comprehension":
		if m, ok := v.(map[string]interface{}); ok {
			if t, ok := m["Term"].(map[string]interface{}); ok {
				if term, err := unmarshalTerm(t); err == nil {
					if b, ok := m["Body"].([]interface{}); ok {
						if body, err := unmarshalBody(b); err == nil {
							buf := &ArrayComprehension{
								Term: term,
								Body: body,
							}
							return buf, nil
						}
					}
				}
			}
		}
	}
unmarshal_error:
	return nil, fmt.Errorf("ast: unable to unmarshal term")
}

var hashSeed0 uint64
var hashSeed1 uint64

func initHashSeed() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	hashSeed0 = (uint64(r.Uint32()) << 32) | uint64(r.Uint32())
	hashSeed1 = (uint64(r.Uint32()) << 32) | uint64(r.Uint32())
}

func init() {
	initHashSeed()
}
