// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opalog

import (
	"fmt"
	"reflect"
	"testing"
)

var _ = fmt.Printf

func TestScalarTerms(t *testing.T) {
	assertParseOneTerm(t, "null", "null", reflectTerm(nil))
	assertParseOneTerm(t, "true", "true", reflectTerm(true))
	assertParseOneTerm(t, "false", "false", reflectTerm(false))
	assertParseOneTerm(t, "integer", "53", reflectTerm(53))
	assertParseOneTerm(t, "integer2", "-53", reflectTerm(-53))
	assertParseOneTerm(t, "float", "16.7", reflectTerm(16.7))
	assertParseOneTerm(t, "float2", "-16.7", reflectTerm(-16.7))
	assertParseOneTerm(t, "exponent", "6e7", reflectTerm(6e7))
	assertParseOneTerm(t, "string", "\"a string\"", reflectTerm("a string"))
	assertParseOneTerm(t, "string", "\"a string u6abc7def8abc0def with unicode\"",
		reflectTerm("a string u6abc7def8abc0def with unicode"))
	assertParseOneTermFail(t, "hex", "6abc")
	assertParseOneTermFail(t, "non-string", "'a string'")
	assertParseOneTermFail(t, "non-number", "6zxy")
	assertParseOneTermFail(t, "non-number2", "6d7")
	assertParseOneTermFail(t, "non-number3", "6\"foo\"")
	assertParseOneTermFail(t, "non-number4", "6true")
	assertParseOneTermFail(t, "non-number5", "6false")
	assertParseOneTermFail(t, "non-number6", "6[null, null]")
	assertParseOneTermFail(t, "non-number7", "6{\"foo\": \"bar\"}")
    assertParseOneTermFail(t, "out-of-range", "1e1000")
}

func TestVarTerms(t *testing.T) {
	assertParseOneTerm(t, "var", "foo", reflectTerm(NewVar("foo")))
	assertParseOneTerm(t, "var", "foo_bar", reflectTerm(NewVar("foo_bar")))
	assertParseOneTerm(t, "var", "foo0", reflectTerm(NewVar("foo0")))

	assertParseOneTermFail(t, "non-var", "foo-bar")
	assertParseOneTermFail(t, "non-var2", "foo-7")
}

func TestObjectWithScalars(t *testing.T) {
	assertParseOneTerm(t, "number", "{\"abc\": 7, \"def\": 8}", reflectTerm(map[string]int{"abc": 7, "def": 8}))
	assertParseOneTerm(t, "bool", "{\"abc\": false, \"def\": true}", reflectTerm(map[string]bool{"abc": false, "def": true}))
	assertParseOneTerm(t, "string", "{\"abc\": \"foo\", \"def\": \"bar\"}", reflectTerm(map[string]string{"abc": "foo", "def": "bar"}))
	assertParseOneTerm(t, "mixed", "{\"abc\": 7, \"def\": null}", reflectTerm(map[string]interface{}{"abc": 7, "def": nil}))
    assertParseOneTerm(t, "number key", "{8: 7, \"def\": null}", reflectTerm(map[interface{}]interface{}{8: 7, "def": nil}))
    assertParseOneTerm(t, "number key 2", "{8.5: 7, \"def\": null}", reflectTerm(map[interface{}]interface{}{8.5: 7, "def": nil}))
    assertParseOneTerm(t, "bool key", "{true: false}", reflectTerm(map[bool]bool{true: false}))
}

func TestObjectWithVars(t *testing.T) {

	assertParseOneTerm(t, "var keys", "{foo: \"bar\", bar: 64}", newObjectTerm([]*KeyValue{
		NewKeyValue(reflectTerm(NewVar("foo")), reflectTerm("bar")),
		NewKeyValue(reflectTerm(NewVar("bar")), reflectTerm(64)),
	}))

	assertParseOneTerm(t, "nested var keys", "{baz: {foo: \"bar\", bar: qux}}", newObjectTerm([]*KeyValue{
		NewKeyValue(reflectTerm(NewVar("baz")), newObjectTerm([]*KeyValue{
			NewKeyValue(reflectTerm(NewVar("foo")), reflectTerm("bar")),
			NewKeyValue(reflectTerm(NewVar("bar")), reflectTerm(NewVar("qux"))),
		})),
	}))
}

func TestArrayWithScalars(t *testing.T) {
	assertParseOneTerm(t, "number", "[1,2,3,4.5]", reflectTerm([]float64{1, 2, 3, 4.5}))
	assertParseOneTerm(t, "bool", "[true, false, true]", reflectTerm([]bool{true, false, true}))
	assertParseOneTerm(t, "string", "[\"foo\", \"bar\"]", reflectTerm([]string{"foo", "bar"}))
	assertParseOneTerm(t, "mixed", "[null, true, 42]", reflectTerm([]interface{}{nil, true, 42}))
}

func TestArrayWithVars(t *testing.T) {
	assertParseOneTerm(t, "var elements", "[foo, bar, 42]", newArrayTerm([]*Term{reflectTerm(NewVar("foo")), reflectTerm(NewVar("bar")), reflectTerm(42)}))
	assertParseOneTerm(t, "nested var elements", "[[foo, true], [null, bar], 42]", newArrayTerm(
		[]*Term{
			newArrayTerm([]*Term{reflectTerm(NewVar("foo")), reflectTerm(true)}),
			newArrayTerm([]*Term{reflectTerm(nil), reflectTerm(NewVar("bar"))}),
			reflectTerm(42),
		},
	))
}

func TestNestedComposites(t *testing.T) {
    assertParseOneTerm(t, "nested composites", "[{foo: [\"bar\", baz]}]", newArrayTerm([]*Term{
        newObjectTerm([]*KeyValue{
            NewKeyValue(reflectTerm(NewVar("foo")), newArrayTerm([]*Term{
                reflectTerm("bar"), reflectTerm(NewVar("baz")),
            })),
        }),
    }))
}

func assertTermEqual(t *testing.T, x *Term, y *Term) {
	if !x.Equal(y) {
		t.Errorf("Failure on equality: \n%s and \n%s\n", x, y)
	}
}

func assertTermNotEqual(t *testing.T, x *Term, y *Term) {
	if x.Equal(y) {
		t.Errorf("Failure on non-equality: \n%s and \n%s\n", x, y)
	}
}

func assertParseOneTerm(t *testing.T, msg string, expr string, correct *Term) interface{} {
	p, err := Parse("", []byte(expr))
	if err != nil {
		t.Errorf("Error on test %s: parse error on %s: %s", msg, expr, err)
		return nil
	}
	parsed := p.([]interface{})
	if len(parsed) != 1 {
		t.Errorf("Error on test %s: failed to parse 1 element from %s: %v",
			msg, expr, parsed)
		return nil
	}
	term := parsed[0].(*Term)
	if !term.Equal(correct) {
		t.Errorf("Error on test %s: wrong result on %s.  Actual = %v; Correct = %v",
			msg, expr, term, correct)
		return nil
	}
	return parsed[0]
}

func assertParseOneTermFail(t *testing.T, msg string, expr string) {
	p, err := Parse("", []byte(expr))
	if err != nil {
		return
	}
	parsed := p.([]interface{})
	if len(parsed) != 1 {
		t.Errorf("Error on test %s: failed to parse 1 element from %s: %v", msg, expr, parsed)
	} else {
		t.Errorf("Error on test %s: failed to error when parsing %v: %v", msg, expr, parsed)
	}
}

func newObjectTerm(o []*KeyValue) *Term {
	set := NewKeyValueSet()
	for _, v := range o {
		set.Add(v)
	}
	return NewTerm(set, OBJECT, []byte(""), "", 0, 0)
}

func newArrayTerm(arr []*Term) *Term {
	return NewTerm(arr, ARRAY, []byte(""), "", 0, 0)
}

func reflectTerm(x interface{}) *Term {

	if x == nil {
		return NewTerm(nil, NULL, []byte(""), "", 0, 0)
	}

	if v, ok := x.(*Var); ok {
		return NewTerm(v, VAR, []byte(""), "", 0, 0)
	}

	var val interface{}
	var typ int
	switch reflect.TypeOf(x).Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val = float64(reflect.ValueOf(x).Int())
		typ = NUMBER
	case reflect.Float32, reflect.Float64:
		val = float64(reflect.ValueOf(x).Float())
		typ = NUMBER
	case reflect.String:
		val = x
		typ = STRING
	case reflect.Bool:
		val = x
		typ = BOOLEAN
	case reflect.Map:
		kvset := NewKeyValueSet()
		xval := reflect.ValueOf(x)
		for _, key := range xval.MapKeys() {
			kvset.Add(NewKeyValue(reflectTerm(key.Interface()), reflectTerm(xval.MapIndex(key).Interface())))
		}
		val = kvset
		typ = OBJECT
	case reflect.Slice, reflect.Array:
		xval := reflect.ValueOf(x)
		length := xval.Len()
		arr := make([]*Term, length)
		for i := 0; i < length; i++ {
			arr[i] = reflectTerm(xval.Index(i).Interface())
		}
		val = arr
		typ = ARRAY
	default:
		panic(fmt.Sprintf("Unexpected type of term: %v", x))
	}

	return NewTerm(val, typ, []byte(""), "", 0, 0)
}
