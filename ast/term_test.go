// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func TestQuery(t *testing.T) {

	stmt := MustParseStatement(`
		{
			"a": [
				[true, {"b": [4]}, {"c": "d"}]
			],
			"e": {
				100: "true"
			}
		}
	`)

	obj := stmt.(Body)[0].Terms.(*Term).Value.(Object)

	var tests = []struct {
		note     string
		ref      interface{}
		expected interface{}
	}{
		{"object base", `a[0][1]`, []string{`[{"b": [4]}]`, "[{}]"}},
		{"array base", `a[0][1]["b"]`, []string{"[[4]]", "[{}]"}},
		{"array non-base", `a[0][1]["b"][0]`, []string{"[4]", "[{}]"}},
		{"object non-base", `a[0][2]["c"]`, []string{`["d"]`, "[{}]"}},
		{"object nested", `e[100]`, []string{`["true"]`, "[{}]"}},
		{"vars", `a[i][j][k]`, []string{`[[4], "d"]`, `[{i:0, j:1, k:"b"}, {i:0, j:2, k:"c"}]`}},
		{"vars/mixed", `a[0][j][k]`, []string{`[[4], "d"]`, `[{j:1, k:"b"}, {j:2, k:"c"}]`}},
		{"array bad index type", `a["0"]`, fmt.Errorf(`unexpected non-numeric index in ["0"]: "0" (ast.String)`)},
		{"array bad index value", "a[1]", fmt.Errorf(`unexpected index in [1]: out of bounds: 1`)},
		{"array bad element type", "a[0][0][1]", fmt.Errorf(`unexpected non-composite at [0][1]: true`)},
		{"object bad key", `e["hello"]`, fmt.Errorf(`missing key "hello": ["hello"]`)},
		{"object bad value type", "e[100][1]", fmt.Errorf(`unexpected non-composite at [100][1]: "true"`)},
	}

	for i, tc := range tests {

		var ref Ref
		switch r := tc.ref.(type) {
		case Ref:
			ref = r
		case string:
			ref = MustParseStatement(r).(Body)[0].Terms.(*Term).Value.(Ref)
			head := String(ref[0].Value.(Var))
			ref[0] = &Term{Value: head}
		}

		var collectedKeys Array
		var collectedValues Array
		collect := func(keys map[Var]Value, v Value) error {
			collectedValues = append(collectedValues, &Term{Value: v})
			var obj Object
			var tmp []string
			for k := range keys {
				tmp = append(tmp, string(k))
			}
			sort.Strings(tmp)
			for _, k := range tmp {
				obj = append(obj, [2]*Term{&Term{Value: Var(k)}, &Term{Value: keys[Var(k)]}})
			}
			collectedKeys = append(collectedKeys, &Term{Value: obj})
			return nil
		}

		err := obj.Query(ref, collect)

		switch e := tc.expected.(type) {

		case []string:
			if err != nil {
				t.Errorf("Test case %d (%v): unexpected error: %v", i+1, tc.note, err)
				continue
			}

			expectedValues := MustParseStatement(e[0]).(Body)[0].Terms.(*Term).Value

			if !collectedValues.Equal(expectedValues) {
				t.Errorf("Test case %d (%v): expected %v but got: %v", i+1, tc.note, expectedValues, collectedValues)
				continue
			}

			expectedKeys := MustParseStatement(e[1]).(Body)[0].Terms.(*Term).Value

			if !collectedKeys.Equal(expectedKeys) {
				t.Errorf("Test case %d (%v): expected keys %v but got: %v", i+1, tc.note, expectedKeys, collectedKeys)
			}
		case error:
			if !reflect.DeepEqual(e, err) {
				t.Errorf("Test case %d (%v): expected error %v but got: %v", i+1, tc.note, e, err)
				continue
			}
		}

	}
}

func TestEqualTerms(t *testing.T) {
	assertTermEqual(t, NullTerm(), NullTerm())
	assertTermEqual(t, BooleanTerm(true), BooleanTerm(true))
	assertTermEqual(t, NumberTerm(5), NumberTerm(5))
	assertTermEqual(t, StringTerm("a string"), StringTerm("a string"))
	assertTermEqual(t, ObjectTerm(), ObjectTerm())
	assertTermEqual(t, ArrayTerm(), ArrayTerm())
	assertTermEqual(t, ObjectTerm(Item(NumberTerm(1), NumberTerm(2))), ObjectTerm(Item(NumberTerm(1), NumberTerm(2))))
	assertTermEqual(t, ObjectTerm(Item(NumberTerm(1), NumberTerm(2)), Item(NumberTerm(3), NumberTerm(4))), ObjectTerm(Item(NumberTerm(1), NumberTerm(2)), Item(NumberTerm(3), NumberTerm(4))))
	assertTermEqual(t, ArrayTerm(NumberTerm(1), NumberTerm(2), NumberTerm(3)), ArrayTerm(NumberTerm(1), NumberTerm(2), NumberTerm(3)))
	assertTermEqual(t, VarTerm("foo"), VarTerm("foo"))
	assertTermEqual(t, RefTerm(VarTerm("foo"), VarTerm("i"), NumberTerm(2)), RefTerm(VarTerm("foo"), VarTerm("i"), NumberTerm(2)))
	assertTermNotEqual(t, NullTerm(), BooleanTerm(true))
	assertTermNotEqual(t, BooleanTerm(true), BooleanTerm(false))
	assertTermNotEqual(t, NumberTerm(5), NumberTerm(7))
	assertTermNotEqual(t, StringTerm("a string"), StringTerm("abc"))
	assertTermNotEqual(t, ObjectTerm(Item(NumberTerm(3), NumberTerm(2))), ObjectTerm(Item(NumberTerm(1), NumberTerm(2))))
	assertTermNotEqual(t, ObjectTerm(Item(NumberTerm(1), NumberTerm(2)), Item(NumberTerm(3), NumberTerm(7))), ObjectTerm(Item(NumberTerm(1), NumberTerm(2)), Item(NumberTerm(3), NumberTerm(4))))
	assertTermNotEqual(t, NumberTerm(5), StringTerm("a string"))
	assertTermNotEqual(t, NumberTerm(1), BooleanTerm(true))
	assertTermNotEqual(t, ObjectTerm(Item(NumberTerm(1), NumberTerm(2)), Item(NumberTerm(3), NumberTerm(7))), ArrayTerm(NumberTerm(1), NumberTerm(2), NumberTerm(7)))
	assertTermNotEqual(t, ArrayTerm(NumberTerm(1), NumberTerm(2), NumberTerm(3)), ArrayTerm(NumberTerm(1), NumberTerm(2), NumberTerm(4)))
	assertTermNotEqual(t, VarTerm("foo"), VarTerm("bar"))
	assertTermNotEqual(t, RefTerm(VarTerm("foo"), VarTerm("i"), NumberTerm(2)), RefTerm(VarTerm("foo"), StringTerm("i"), NumberTerm(2)))
}

func TestHash(t *testing.T) {

	doc := `
		{
			"a": [
				[true, {"b": [null]}, {"c": "d"}]
			],
			"e": {
				100: a[i].b
			}
		}
	`

	stmt1 := MustParseStatement(doc)
	stmt2 := MustParseStatement(doc)

	obj1 := stmt1.(Body)[0].Terms.(*Term).Value.(Object)
	obj2 := stmt2.(Body)[0].Terms.(*Term).Value.(Object)
	if obj1.Hash() != obj2.Hash() {
		t.Errorf("Expected hash codes to be equal")
	}
}

func TestTermsToString(t *testing.T) {
	assertToString(t, Null{}, "null")
	assertToString(t, Boolean(true), "true")
	assertToString(t, Boolean(false), "false")
	assertToString(t, Number(4), "4")
	assertToString(t, Number(42.1), "42.1")
	assertToString(t, Number(6e7), "6E+07")
	assertToString(t, String("foo"), "\"foo\"")
	assertToString(t, String("\"foo\""), "\"\\\"foo\\\"\"")
	assertToString(t, String("foo bar"), "\"foo bar\"")
	assertToString(t, Var("foo"), "foo")
	assertToString(t, RefTerm(VarTerm("foo"), StringTerm("bar")).Value, "foo.bar")
	assertToString(t, RefTerm(VarTerm("foo"), StringTerm("bar"), VarTerm("i"), NumberTerm(0), StringTerm("baz")).Value, "foo.bar[i][0].baz")
	assertToString(t, RefTerm(VarTerm("foo"), BooleanTerm(false), NullTerm(), StringTerm("bar")).Value, "foo[false][null].bar")
	assertToString(t, ArrayTerm().Value, "[]")
	assertToString(t, ObjectTerm().Value, "{}")
	assertToString(t, ArrayTerm(ObjectTerm(Item(VarTerm("foo"), ArrayTerm(RefTerm(VarTerm("bar"), VarTerm("i"))))), StringTerm("foo"), BooleanTerm(true), NullTerm(), NumberTerm(42.1)).Value, "[{foo: [bar[i]]}, \"foo\", true, null, 42.1]")
}

func TestRefUnderlying(t *testing.T) {

	assertUnderlying(t, RefTerm().Value.(Ref), []interface{}{})
	assertUnderlying(t, RefTerm(VarTerm("a")).Value.(Ref), []interface{}{"a"})
	assertUnderlying(t, RefTerm(StringTerm("a")).Value.(Ref), []interface{}{"a"})
	assertUnderlying(t, RefTerm(NullTerm()).Value.(Ref), []interface{}{nil})
	assertUnderlying(t, RefTerm(BooleanTerm(false)).Value.(Ref), []interface{}{false})
	assertUnderlying(t, RefTerm(NumberTerm(3)).Value.(Ref), []interface{}{float64(3)})
	assertUnderlying(t, RefTerm(VarTerm("a"), StringTerm("b"), NumberTerm(4)).Value.(Ref), []interface{}{"a", "b", float64(4)})
	assertUnderlyingError(t, RefTerm(VarTerm("a"), VarTerm("i")).Value.(Ref), fmt.Errorf("cannot get underlying value for non-ground ref: a[i]"))

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

func assertToString(t *testing.T, val Value, expected string) {
	result := val.String()
	if result != expected {
		t.Errorf("Expected %v but got %v", expected, result)
	}
}

func assertUnderlying(t *testing.T, ref Ref, expected []interface{}) {
	u, err := ref.Underlying()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	if !reflect.DeepEqual(u, expected) {
		t.Errorf("Expected %v but got %v", expected, u)
	}
}

func assertUnderlyingError(t *testing.T, ref Ref, expected error) {
	u, err := ref.Underlying()
	if err == nil {
		t.Errorf("Expected error but got %v", u)
		return
	}
	if !reflect.DeepEqual(err, expected) {
		t.Errorf("Expected %v but got %v", expected, err)
	}
}
