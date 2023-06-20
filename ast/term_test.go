// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/open-policy-agent/opa/util"
)

func TestInterfaceToValue(t *testing.T) {
	// Test util package unmarshalled inputs
	input := `
	{
		"x": [
			1,
			true,
			false,
			null,
			"hello",
			["goodbye", 1],
			{"y": 3.1}
		]
	}
	`
	var x interface{}
	if err := util.UnmarshalJSON([]byte(input), &x); err != nil {
		t.Fatal(err)
	}

	expected := MustParseTerm(input).Value

	v, err := InterfaceToValue(x)
	if err != nil {
		t.Fatal(err)
	}

	if v.Compare(expected) != 0 {
		t.Fatalf("Expected %v but got: %v", expected, v)
	}

	// Test standard JSON package unmarshalled inputs
	if err := json.Unmarshal([]byte(input), &x); err != nil {
		t.Fatal(err)
	}

	expected = MustParseTerm(input).Value
	if v, err = InterfaceToValue(x); err != nil {
		t.Fatal(err)
	}

	if expected.Compare(v) != 0 {
		t.Fatalf("Expected %v but got: %v", expected, v)
	}

	// Test misc. types
	tests := []struct {
		input    interface{}
		expected string
	}{
		{int64(100), "100"},
		{float64(100), "100"},
		{int(100), "100"},
		{map[string]string{"foo": "bar"}, `{"foo": "bar"}`},
		{uint64(100), "100"},
	}

	for _, tc := range tests {
		expected := MustParseTerm(tc.expected).Value
		v, err := InterfaceToValue(tc.input)
		if err != nil {
			t.Fatal(err)
		}
		if v.Compare(expected) != 0 {
			t.Fatalf("Expected %v but got: %v", expected, v)
		}
	}
}

func TestInterfaceToValueStructs(t *testing.T) {
	var x struct {
		Foo struct {
			Baz string `json:"baz"`
		} `json:"foo"`
		bar string
	}

	x.Foo.Baz = "a"
	x.bar = "b"

	result, err := InterfaceToValue(x)
	if err != nil {
		t.Fatal(err)
	}

	exp := MustParseTerm(`{"foo": {"baz": "a"}}`)

	if result.Compare(exp.Value) != 0 {
		t.Fatalf("expected %v but got %v", exp, result)
	}

	var m brokenMarshaller

	_, err = InterfaceToValue(m)
	if err == nil || err.Error() != "ast: interface conversion: json: error calling MarshalJSON for type ast.brokenMarshaller: broken" {
		t.Fatal("expected error but got:", err)
	}
}

type brokenMarshaller struct{}

func (brokenMarshaller) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("broken")
}

func TestObjectInsertGetLen(t *testing.T) {
	tests := []struct {
		insert   [][2]string
		expected map[string]string
	}{
		{[][2]string{{`null`, `value1`}, {`null`, `value2`}}, map[string]string{`null`: `value2`}},
		{[][2]string{{`false`, `value`}, {`true`, `value1`}, {`true`, `value2`}}, map[string]string{`false`: `value`, `true`: `value2`}},
		{[][2]string{{`0`, `value`}, {`1`, `value1`}, {`1`, `value2`}, {`1.5`, `value`}}, map[string]string{`0`: `value`, `1`: `value2`, `1.5`: `value`}},
		{[][2]string{{`"string"`, `value1`}, {`"string"`, `value2`}}, map[string]string{`"string"`: `value2`}},
		{[][2]string{{`["other"]`, `value1`}, {`["other"]`, `value2`}}, map[string]string{`["other"]`: `value2`}},
	}

	for _, tc := range tests {
		o := NewObject()
		for _, kv := range tc.insert {
			o.Insert(MustParseTerm(kv[0]), MustParseTerm(kv[1]))

			if v := o.Get(MustParseTerm(kv[0])); v == nil || !MustParseTerm(kv[1]).Equal(v) {
				t.Errorf("Expected the object to contain %v", v)
			}
		}

		if o.Len() != len(tc.expected) {
			t.Errorf("Expected the object to have %v entries", len(tc.expected))
		}

		for k, v := range tc.expected {
			if x := o.Get(MustParseTerm(k)); x == nil || !MustParseTerm(v).Equal(x) {
				t.Errorf("Expected the object to contain %v", k)
			}
		}
	}
}

func TestObjectSetOperations(t *testing.T) {
	a := MustParseTerm(`{"a": "b", "c": "d"}`).Value.(Object)
	b := MustParseTerm(`{"c": "q", "d": "e"}`).Value.(Object)

	r1 := a.Diff(b)
	if r1.Compare(MustParseTerm(`{"a": "b"}`).Value) != 0 {
		t.Errorf(`Expected a.Diff(b) to equal {"a": "b"} but got: %v`, r1)
	}

	r2 := a.Intersect(b)
	var expectedTerms []*Term
	MustParseTerm(`["c", "d", "q"]`).Value.(*Array).Foreach(func(t *Term) {
		expectedTerms = append(expectedTerms, t)
	})
	if len(r2) != 1 || !termSliceEqual(r2[0][:], expectedTerms) {
		t.Errorf(`Expected a.Intersect(b) to equal [["a", "d", "q"]] but got: %v`, r2)
	}

	if r3, ok := a.Merge(b); ok {
		t.Errorf("Expected a.Merge(b) to fail but got: %v", r3)
	}

	c := MustParseTerm(`{"a": {"b": [1], "c": {"d": 2}}}`).Value.(Object)
	d := MustParseTerm(`{"a": {"x": [3], "c": {"y": 4}}}`).Value.(Object)
	r3, ok := c.Merge(d)
	expected := MustParseTerm(`{"a": {"b": [1], "x": [3], "c": {"d": 2, "y": 4}}}`).Value.(Object)

	if !ok || r3.Compare(expected) != 0 {
		t.Errorf("Expected c.Merge(d) to equal %v but got: %v", expected, r3)
	}
}

func TestObjectFilter(t *testing.T) {
	cases := []struct {
		note     string
		object   string
		filter   string
		expected string
	}{
		{
			note:     "base",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			filter:   `{"a": {"b": {"c": null}}}`,
			expected: `{"a": {"b": {"c": 7}}}`,
		},
		{
			note:     "multiple roots",
			object:   `{"a": {"b": {"c": 7, "d": 8}}, "e": 9}`,
			filter:   `{"a": {"b": {"c": null}}, "e": null}`,
			expected: `{"a": {"b": {"c": 7}}, "e": 9}`,
		},
		{
			note:     "shared roots",
			object:   `{"a": {"b": {"c": 7, "d": 8}, "e": 9}}`,
			filter:   `{"a": {"b": {"c": null}, "e": null}}`,
			expected: `{"a": {"b": {"c": 7}, "e": 9}}`,
		},
		{
			note:     "empty filter",
			object:   `{"a": 7}`,
			filter:   `{}`,
			expected: `{}`,
		},
		{
			note:     "empty object",
			object:   `{}`,
			filter:   `{"a": {"b": null}}`,
			expected: `{}`,
		},
		{
			note:     "arrays",
			object:   `{"a": [{"b": 7, "c": 8}, {"d": 9}]}`,
			filter:   `{"a": {"0": {"b": null}, "1": null}}`,
			expected: `{"a": [{"b": 7}, {"d": 9}]}`,
		},
		{
			note:     "object with number keys",
			object:   `{"a": [{"1":["b", "c", "d"]}, {"x": "y"}]}`,
			filter:   `{"a": {"0": {"1": {"2": null}}}}`,
			expected: `{"a": [{"1": ["d"]}]}`,
		},
		{
			note:     "sets",
			object:   `{"a": {"b", "c", "d"}, "x": {"y"}}`,
			filter:   `{"a": {"b": null, "d": null}, "x": null}`,
			expected: `{"a": {"b", "d"}, "x": {"y"}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			obj := MustParseTerm(tc.object).Value.(Object)
			filterObj := MustParseTerm(tc.filter).Value.(Object)
			expected := MustParseTerm(tc.expected).Value.(Object)
			actual, err := obj.Filter(filterObj)
			if err != nil {
				t.Errorf("unexpected error: %s", err)
			}
			if actual.Compare(expected) != 0 {
				t.Errorf("Expected:\n\n\t%s\n\nGot:\n\n\t%s\n\n", expected, actual)
			}
		})
	}
}

func TestTermBadJSON(t *testing.T) {
	input := `{
		"Value": [[
			{"Value": [{"Value": "a", "Type": "var"}, {"Value": "x", "Type": "string"}], "Type": "ref"},
			{"Value": [{"Value": "x", "Type": "var"}], "Type": "array"}
		], [
			{"Value": 100, "Type": "array"},
			{"Value": "foo", "Type": "string"}
		]],
		"Type": "object"
	}`

	term := Term{}
	err := util.UnmarshalJSON([]byte(input), &term)
	expected := fmt.Errorf("ast: unable to unmarshal term")
	if !reflect.DeepEqual(expected, err) {
		t.Errorf("Expected %v but got: %v", expected, err)
	}
}

func TestTermEqual(t *testing.T) {
	assertTermEqual(t, NullTerm(), NullTerm())
	assertTermEqual(t, BooleanTerm(true), BooleanTerm(true))
	assertTermEqual(t, IntNumberTerm(5), IntNumberTerm(5))
	assertTermEqual(t, NumberTerm(json.Number("1e6")), NumberTerm("1000000"))
	assertTermEqual(t, StringTerm("a string"), StringTerm("a string"))
	assertTermEqual(t, ObjectTerm(), ObjectTerm())
	assertTermEqual(t, ArrayTerm(), ArrayTerm())
	assertTermEqual(t, ObjectTerm(Item(IntNumberTerm(1), IntNumberTerm(2))), ObjectTerm(Item(IntNumberTerm(1), IntNumberTerm(2))))
	assertTermEqual(t, ObjectTerm(Item(IntNumberTerm(1), IntNumberTerm(2)), Item(IntNumberTerm(3), IntNumberTerm(4))), ObjectTerm(Item(IntNumberTerm(1), IntNumberTerm(2)), Item(IntNumberTerm(3), IntNumberTerm(4))))
	assertTermEqual(t, ArrayTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3)), ArrayTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3)))
	assertTermEqual(t, VarTerm("foo"), VarTerm("foo"))
	assertTermEqual(t, RefTerm(VarTerm("foo"), VarTerm("i"), IntNumberTerm(2)), RefTerm(VarTerm("foo"), VarTerm("i"), IntNumberTerm(2)))
	assertTermEqual(t, ArrayComprehensionTerm(VarTerm("x"), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})), ArrayComprehensionTerm(VarTerm("x"), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})))
	assertTermEqual(t, ObjectComprehensionTerm(VarTerm("x"), VarTerm("y"), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})), ObjectComprehensionTerm(VarTerm("x"), VarTerm("y"), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})))
	assertTermEqual(t, SetComprehensionTerm(VarTerm("x"), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})), SetComprehensionTerm(VarTerm("x"), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})))

	assertTermNotEqual(t, NullTerm(), BooleanTerm(true))
	assertTermNotEqual(t, BooleanTerm(true), BooleanTerm(false))
	assertTermNotEqual(t, IntNumberTerm(5), IntNumberTerm(7))
	assertTermNotEqual(t, StringTerm("a string"), StringTerm("abc"))
	assertTermNotEqual(t, ObjectTerm(Item(IntNumberTerm(3), IntNumberTerm(2))), ObjectTerm(Item(IntNumberTerm(1), IntNumberTerm(2))))
	assertTermNotEqual(t, ObjectTerm(Item(IntNumberTerm(1), IntNumberTerm(2)), Item(IntNumberTerm(3), IntNumberTerm(7))), ObjectTerm(Item(IntNumberTerm(1), IntNumberTerm(2)), Item(IntNumberTerm(3), IntNumberTerm(4))))
	assertTermNotEqual(t, IntNumberTerm(5), StringTerm("a string"))
	assertTermNotEqual(t, IntNumberTerm(1), BooleanTerm(true))
	assertTermNotEqual(t, ObjectTerm(Item(IntNumberTerm(1), IntNumberTerm(2)), Item(IntNumberTerm(3), IntNumberTerm(7))), ArrayTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(7)))
	assertTermNotEqual(t, ArrayTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3)), ArrayTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(4)))
	assertTermNotEqual(t, VarTerm("foo"), VarTerm("bar"))
	assertTermNotEqual(t, RefTerm(VarTerm("foo"), VarTerm("i"), IntNumberTerm(2)), RefTerm(VarTerm("foo"), StringTerm("i"), IntNumberTerm(2)))
	assertTermNotEqual(t, ArrayComprehensionTerm(VarTerm("x"), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("j"))})), ArrayComprehensionTerm(VarTerm("x"), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})))
	assertTermNotEqual(t, ObjectComprehensionTerm(VarTerm("x"), VarTerm("y"), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("j"))})), ObjectComprehensionTerm(VarTerm("x"), VarTerm("y"), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})))
	assertTermNotEqual(t, SetComprehensionTerm(VarTerm("x"), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("j"))})), SetComprehensionTerm(VarTerm("x"), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})))
}

func TestFind(t *testing.T) {
	term := MustParseTerm(`{"foo": [1,{"bar": {2,3,4}}], "baz": {"qux": ["hello", "world"]}}`)

	tests := []struct {
		path     *Term
		expected interface{}
	}{
		{RefTerm(StringTerm("foo"), IntNumberTerm(1), StringTerm("bar")), MustParseTerm(`{2, 3, 4}`)},
		{RefTerm(StringTerm("foo"), IntNumberTerm(1), StringTerm("bar"), IntNumberTerm(4)), MustParseTerm(`4`)},
		{RefTerm(StringTerm("foo"), IntNumberTerm(2)), fmt.Errorf("not found")},
		{RefTerm(StringTerm("baz"), StringTerm("qux"), IntNumberTerm(0)), MustParseTerm(`"hello"`)},
	}

	for _, tc := range tests {
		result, err := term.Value.Find(tc.path.Value.(Ref))
		switch expected := tc.expected.(type) {
		case *Term:
			if err != nil {
				t.Fatalf("Unexpected error occurred for %v: %v", tc.path, err)
			}
			if result.Compare(expected.Value) != 0 {
				t.Fatalf("Expected value %v for %v but got: %v", expected, tc.path, result)
			}
		case error:
			if err == nil {
				t.Fatalf("Expected error but got: %v", result)
			}
			if !strings.Contains(err.Error(), expected.Error()) {
				t.Fatalf("Expected error to contain %v but got: %v", expected, err)
			}
		default:
			panic("bad expected type")
		}
	}
}

func TestHashObject(t *testing.T) {
	doc := `{"a": [[true, {"b": [null]}, {"c": "d"}]], "e": {100: a[i].b}, "k": ["foo" | true], "o": {"foo": "bar" | true}, "sc": {"foo" | true}, "s": {1, 2, {3, 4}}, "big": 1e+1000}`

	stmt1 := MustParseStatement(doc)
	stmt2 := MustParseStatement(doc)

	obj1 := stmt1.(Body)[0].Terms.(*Term).Value.(Object)
	obj2 := stmt2.(Body)[0].Terms.(*Term).Value.(Object)

	if obj1.Hash() != obj2.Hash() {
		t.Errorf("Expected hash codes to be equal")
	}

	// Calculate hash like we did before moving the caching to create/update:
	obj := obj1.(*object)
	exp := 0
	for h, curr := range obj.elems {
		for ; curr != nil; curr = curr.next {
			exp += h
			exp += curr.value.Hash()
		}
	}

	if act := obj1.Hash(); exp != act {
		t.Errorf("expected %v, got %v", exp, act)
	}
}

func TestHashArray(t *testing.T) {
	doc := `[{"a": [[true, {"b": [null]}, {"c": "d"}]]}, 100, true, [a[i].b], {100: a[i].b}, ["foo" | true], {"foo": "bar" | true}, {"foo" | true}, {1, 2, {3, 4}}, 1e+1000]`

	stmt1 := MustParseStatement(doc)
	stmt2 := MustParseStatement(doc)

	arr1 := stmt1.(Body)[0].Terms.(*Term).Value.(*Array)
	arr2 := stmt2.(Body)[0].Terms.(*Term).Value.(*Array)

	if arr1.Hash() != arr2.Hash() {
		t.Errorf("Expected hash codes to be equal")
	}

	// Calculate hash like we did before moving the caching to create/update:
	exp := termSliceHash(arr1.elems)

	if act := arr1.Hash(); exp != act {
		t.Errorf("expected %v, got %v", exp, act)
	}

	for j := 0; j < arr1.Len(); j++ {
		for i := 0; i <= j; i++ {
			slice := arr1.Slice(i, j)
			exp := termSliceHash(slice.elems)
			if act := slice.Hash(); exp != act {
				t.Errorf("arr1[%d:%d]: expected %v, got %v", i, j, exp, act)
			}
		}
	}
}

func TestHashSet(t *testing.T) {
	doc := `{{"a": [[true, {"b": [null]}, {"c": "d"}]]}, 100, 100, 100, true, [a[i].b], {100: a[i].b}, ["foo" | true], {"foo": "bar" | true}, {"foo" | true}, {1, 2, {3, 4}}, 1e+1000}`

	stmt1 := MustParseStatement(doc)
	stmt2 := MustParseStatement(doc)

	set1 := stmt1.(Body)[0].Terms.(*Term).Value.(Set)
	set2 := stmt2.(Body)[0].Terms.(*Term).Value.(Set)

	if set1.Hash() != set2.Hash() {
		t.Errorf("Expected hash codes to be equal")
	}

	// Calculate hash like we did before moving the caching to create/update:
	exp := 0
	set1.Foreach(func(x *Term) {
		exp += x.Hash()
	})

	if act := set1.Hash(); exp != act {
		t.Errorf("expected %v, got %v", exp, act)
	}
}

func TestTermIsGround(t *testing.T) {
	tests := []struct {
		note     string
		term     string
		expected bool
	}{
		{"null", "null", true},
		{"string", `"foo"`, true},
		{"number", "42.1", true},
		{"boolean", "false", true},
		{"var", "x", false},
		{"ref ground", "a.b[0]", true},
		{"ref non-ground", "a.b[i].x", false},
		{"array ground", "[1,2,3]", true},
		{"array non-ground", "[1,2,x]", false},
		{"set ground", "{1,2,3}", true},
		{"Set non-ground", "{1,2,x}", false},
		{"object ground", `{"a": 1}`, true},
		{"object non-ground key", `{"x": 1, y: 2}`, false},
		{"object non-ground value", `{"x": 1, "y": y}`, false},
		{"array compr ground", `["a" | true]`, true},
		{"array compr non-ground", `[x | x = a[i]]`, false},
	}

	for i, tc := range tests {
		term := MustParseTerm(tc.term)
		if term.IsGround() != tc.expected {
			expected := "ground"
			if !tc.expected {
				expected = "non-ground"
			}
			t.Errorf("Expected term %v to be %s (test case %d: %v)", term, expected, i, tc.note)
		}
	}
}

func TestObjectRemainsGround(t *testing.T) {
	tests := []struct {
		key    string
		value  string
		ground bool
	}{
		{`"a"`, `"value1"`, true},
		{`"b"`, `"value2"`, true},
		{`"a"`, `x`, false},
		{`"a"`, `"value1"`, true},
		{`"b"`, `y`, false},
		{`"c"`, `value3`, false},
	}

	obj := NewObject()

	for i, tc := range tests {
		obj.Insert(MustParseTerm(tc.key), MustParseTerm(tc.value))
		if obj.IsGround() != tc.ground {
			t.Errorf("Unexpected object is ground (test case %d)", i)
		}
	}
}

func TestIsConstant(t *testing.T) {
	tests := []struct {
		term     string
		expected bool
	}{
		{`[{"foo": {true, false, [1, 2]}}]`, true},
		{`[{"foo": {x}}]`, false},
	}
	for _, tc := range tests {
		term := MustParseTerm(tc.term)
		if IsConstant(term.Value) != tc.expected {
			t.Fatalf("Expected IsConstant(%v) = %v", term, tc.expected)
		}
	}
}

func TestIsScalar(t *testing.T) {
	tests := []struct {
		term     string
		expected bool
	}{
		{"null", true},
		{`"string"`, true},
		{"3.14", true},
		{"false", true},
		{"[1,2,3]", false},
		{"{1,2,3}", false},
		{`{"a": 1}`, false},
		{`[x | x = 0]`, false},
	}
	for _, tc := range tests {
		term := MustParseTerm(tc.term)
		if IsScalar(term.Value) != tc.expected {
			t.Errorf("Expected IsScalar(%v) = %v", term, tc.expected)
		}
	}
}

func TestTermString(t *testing.T) {
	assertToString(t, Null{}, "null")
	assertToString(t, Boolean(true), "true")
	assertToString(t, Boolean(false), "false")
	assertToString(t, Number("4"), "4")
	assertToString(t, Number("42.1"), "42.1")
	assertToString(t, Number("6e7"), "6e7")
	assertToString(t, UIntNumberTerm(uint64(1)).Value, "1")
	assertToString(t, String("foo"), "\"foo\"")
	assertToString(t, String("\"foo\""), "\"\\\"foo\\\"\"")
	assertToString(t, String("foo bar"), "\"foo bar\"")
	assertToString(t, Var("foo"), "foo")
	assertToString(t, RefTerm(VarTerm("foo"), StringTerm("bar")).Value, "foo.bar")
	assertToString(t, RefTerm(VarTerm("foo"), StringTerm("bar"), VarTerm("i"), IntNumberTerm(0), StringTerm("baz")).Value, "foo.bar[i][0].baz")
	assertToString(t, RefTerm(VarTerm("foo"), BooleanTerm(false), NullTerm(), StringTerm("bar")).Value, "foo[false][null].bar")
	assertToString(t, RefTerm(VarTerm("p"), StringTerm("not")).Value, `p["not"]`)
	assertToString(t, RefTerm(CallTerm(VarTerm("f"), VarTerm("x")), IntNumberTerm(0)).Value, "f(x)[0]")
	assertToString(t, RefTerm(ArrayTerm(StringTerm("a"), StringTerm("b")), IntNumberTerm(0)).Value, "[\"a\", \"b\"][0]")
	assertToString(t, ArrayTerm().Value, "[]")
	assertToString(t, ObjectTerm().Value, "{}")
	assertToString(t, SetTerm().Value, "set()")
	assertToString(t, ArrayTerm(ObjectTerm(Item(VarTerm("foo"), ArrayTerm(RefTerm(VarTerm("bar"), VarTerm("i"))))), StringTerm("foo"), SetTerm(BooleanTerm(true), NullTerm()), FloatNumberTerm(42.1)).Value, "[{foo: [bar[i]]}, \"foo\", {null, true}, 42.1]")
	assertToString(t, ArrayComprehensionTerm(ArrayTerm(VarTerm("x")), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})).Value, `[[x] | a[i]]`)
	assertToString(t, ObjectComprehensionTerm(VarTerm("y"), ArrayTerm(VarTerm("x")), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})).Value, `{y: [x] | a[i]}`)
	assertToString(t, SetComprehensionTerm(ArrayTerm(VarTerm("x")), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})).Value, `{[x] | a[i]}`)

	// ensure that objects and sets have deterministic String() results
	assertToString(t, SetTerm(VarTerm("y"), VarTerm("x")).Value, "{x, y}")
	assertToString(t, ObjectTerm([2]*Term{VarTerm("y"), VarTerm("b")}, [2]*Term{VarTerm("x"), VarTerm("a")}).Value, "{x: a, y: b}")
}

func TestRefHasPrefix(t *testing.T) {
	a := MustParseRef("foo.bar.baz")
	b := MustParseRef("foo.bar")
	c := MustParseRef("foo.bar[0][x]")

	if !a.HasPrefix(b) {
		t.Error("Expected a.HasPrefix(b)")
	}

	if b.HasPrefix(a) {
		t.Error("Expected !b.HasPrefix(a)")
	}

	if !c.HasPrefix(b) {
		t.Error("Expected c.HasPrefix(b)")
	}
}

func TestRefAppend(t *testing.T) {
	a := MustParseRef("foo.bar.baz")
	b := a.Append(VarTerm("x"))
	if !b.Equal(MustParseRef("foo.bar.baz[x]")) {
		t.Error("Expected foo.bar.baz[x]")
	}
}

func TestRefInsert(t *testing.T) {
	ref := MustParseRef("test.ex")
	cases := []struct {
		pos      int
		term     *Term
		expected string
	}{
		{0, VarTerm("foo"), `foo[test].ex`},
		{1, StringTerm("foo"), `test.foo.ex`},
		{2, StringTerm("foo"), `test.ex.foo`},
	}
	for i := range cases {
		result := ref.Insert(cases[i].term, cases[i].pos)
		expected := MustParseRef(cases[i].expected)
		if !expected.Equal(result) {
			t.Fatalf("Expected %v (len: %d) but got: %v (len: %d)", expected, len(expected), result, len(result))
		}
	}
}

func TestRefDynamic(t *testing.T) {
	a := MustParseRef("foo.bar[baz.qux].corge")
	if a.Dynamic() != 2 {
		t.Fatalf("Expected dynamic offset to be baz.qux for foo.bar[baz.qux].corge")
	}
	if a[:a.Dynamic()].Dynamic() != -1 {
		t.Fatalf("Expected dynamic offset to be -1 for foo.bar")
	}

	if MustParseRef("f(x)[0]").Dynamic() != 0 {
		t.Fatalf("Expected dynamic offset to be f(x) for foo.bar[baz.qux].corge")
	}
}

func TestRefExtend(t *testing.T) {
	a := MustParseRef("foo.bar.baz")
	b := MustParseRef("qux.corge")
	c := MustParseRef("data")
	result := a.Extend(b)
	expected := MustParseRef("foo.bar.baz.qux.corge")
	if !result.Equal(expected) {
		t.Fatalf("Expected %v but got %v", expected, result)
	}
	result = result.Extend(c)
	expected = MustParseRef("foo.bar.baz.qux.corge.data")
	if !result.Equal(expected) {
		t.Fatalf("Expected %v but got %v", expected, result)
	}
}

func TestRefConcat(t *testing.T) {
	a := MustParseRef("foo.bar.baz")
	terms := []*Term{}
	if !a.Concat(terms).Equal(a) {
		t.Fatal("Expected no change")
	}
	terms = append(terms, StringTerm("qux"))
	exp := MustParseTerm("foo.bar.baz.qux")
	result := a.Concat(terms)
	if !result.Equal(exp.Value) {
		t.Fatalf("Expected %v but got %v", exp, result)
	}
	exp = MustParseTerm("foo.bar.baz.qux[0]")
	terms = append(terms, IntNumberTerm(0))
	result = a.Concat(terms)
	if !result.Equal(exp.Value) {
		t.Fatalf("Expected %v but got %v", exp, result)
	}
	exp = MustParseTerm("foo.bar.baz")
	if !a.Equal(exp.Value) {
		t.Fatalf("Expected %v but got %v (want a to be unchanged)", exp, a)
	}
}

func TestRefPtr(t *testing.T) {
	cases := []string{
		"",
		"a",
		"a/b",
		"/a/b",
		"/a/b/",
		"a%2Fb",
	}

	for _, tc := range cases {
		ref, err := PtrRef(DefaultRootDocument.Copy(), tc)
		if err != nil {
			t.Fatal("Unexpected error:", err)
		}

		ptr, err := ref.Ptr()
		if err != nil {
			t.Fatal("Unexpected error:", err)
		}

		roundtrip, err := PtrRef(DefaultRootDocument.Copy(), ptr)
		if err != nil {
			t.Fatal("Unexpected error:", err)
		}

		if !ref.Equal(roundtrip) {
			t.Fatalf("Expected roundtrip of %q to be equal but got %v and %v", tc, ref, roundtrip)
		}
	}

	if _, err := PtrRef(DefaultRootDocument.Copy(), "2%"); err == nil {
		t.Fatalf("Expected error from %q", "2%")
	}

	ref := Ref{VarTerm("x"), IntNumberTerm(1)}

	if _, err := ref.Ptr(); err == nil {
		t.Fatal("Expected error from x[1]")
	}
}

func TestSetEqual(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected bool
	}{
		{"set()", "set()", true},
		{"{1,{2,3},4}", "{1,{2,3},4}", true},
		{"{1,{2,3},4}", "{4,{3,2},1}", true},
		{"{1,2,{3,4}}", "{1,2,{3,4},1,2,{3,4}}", true},
		{"{1,2,3,4}", "{1,2,3}", false},
		{"{1,2,3}", "{1,2,3,4}", false},
	}
	for _, tc := range tests {
		a := MustParseTerm(tc.a)
		b := MustParseTerm(tc.b)
		if a.Equal(b) != tc.expected {
			var msg string
			if tc.expected {
				msg = fmt.Sprintf("Expected %v to equal %v", a, b)
			} else {
				msg = fmt.Sprintf("Expected %v to NOT equal %v", a, b)
			}
			t.Errorf(msg)
		}
	}
}

func TestSetMap(t *testing.T) {
	set := MustParseTerm(`{"foo", "bar", "baz", "qux"}`).Value.(Set)

	result, err := set.Map(func(term *Term) (*Term, error) {
		s := string(term.Value.(String))
		if strings.Contains(s, "a") {
			return &Term{Value: String(strings.ToUpper(s))}, nil
		}
		return term, nil
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := MustParseTerm(`{"foo", "BAR", "BAZ", "qux"}`).Value

	if result.Compare(expected) != 0 {
		t.Fatalf("Expected map result to be %v but got: %v", expected, result)
	}

	result, err = set.Map(func(*Term) (*Term, error) {
		return nil, fmt.Errorf("oops")
	})

	if !reflect.DeepEqual(err, fmt.Errorf("oops")) {
		t.Fatalf("Expected oops to be returned but got: %v, %v", result, err)
	}
}

func TestSetAddContainsLen(t *testing.T) {
	tests := []struct {
		add      []string
		expected []string
	}{
		{[]string{`null`, `null`}, []string{`null`}},
		{[]string{`true`, `true`, `false`}, []string{`true`, `false`}},
		{[]string{`0`, `1`, `1`, `1.5`}, []string{`0`, `1`, `1.5`}},
		{[]string{`"string"`, `"string"`}, []string{`"string"`}},
		{[]string{`["other"]`, `["other"]`}, []string{`["other"]`}},
	}

	for _, tc := range tests {
		s := NewSet()
		for _, v := range tc.add {
			x := MustParseTerm(v)
			s.Add(x)

			if !s.Contains(x) {
				t.Errorf("Expected the set to contain %v", v)
			}
		}

		if s.Len() != len(tc.expected) {
			t.Errorf("Expected the set to have %v entries", len(tc.expected))
		}

		for _, v := range tc.expected {
			if !s.Contains(MustParseTerm(v)) {
				t.Errorf("Expected the set to contain %v", v)
			}
		}
	}
}

func TestSetOperations(t *testing.T) {
	tests := []struct {
		a  string
		b  string
		c  string
		op string
	}{
		{`{1,2,3,4}`, `{1,3,5}`, `{2,4}`, "-"},
		{`{1,3,5}`, `{1,2,3,4}`, `{5,}`, "-"},
		{`{1,2,3,4}`, `{1,3,5}`, `{1,3}`, "&"},
		{`{1,3,5}`, `{1,2,3,4}`, `{1,3}`, "&"},
		{`{1,2,3,4}`, `{1,3,5}`, `{1,2,3,4,5}`, "|"},
		{`{1,3,5}`, `{1,2,3,4}`, `{1,2,3,4,5}`, "|"},
	}

	for _, tc := range tests {
		s1 := MustParseTerm(tc.a).Value.(Set)
		s2 := MustParseTerm(tc.b).Value.(Set)
		s3 := MustParseTerm(tc.c).Value.(Set)
		var result Set
		if tc.op == "-" {
			result = s1.Diff(s2)
		} else if tc.op == "&" {
			result = s1.Intersect(s2)
		} else if tc.op == "|" {
			result = s1.Union(s2)
		} else {
			panic("bad operation")
		}
		if result.Compare(s3) != 0 {
			t.Errorf("Expected %v for %v %v %v but got: %v", s3, tc.a, tc.op, tc.b, result)
		}
	}
}

func TestSetCopy(t *testing.T) {
	orig := MustParseTerm("{1,2,3}")
	cpy := orig.Copy()
	vis := NewGenericVisitor(func(x interface{}) bool {
		if Compare(IntNumberTerm(2), x) == 0 {
			// NOTE(sr): If we mess up the rank, our sort-on-insert approach fails us
			x.(*Term).Value = Number("2.5")
		}
		return false
	})
	vis.Walk(orig)
	expOrig := MustParseTerm(`{1,2.5,3}`)
	expCpy := MustParseTerm(`{1,2,3}`)
	if !expOrig.Equal(orig) {
		t.Errorf("Expected %v but got %v", expOrig, orig)
	}
	if !expCpy.Equal(cpy) {
		t.Errorf("Expected %v but got %v", expCpy, cpy)
	}
}

// Constructs a set, and then has several reader goroutines attempt to
// concurrently iterate across it. This should pretty consistently
// hit a race condition around sorting the underlying key slice if
// the sorting isn't guarded properly.
func TestSetConcurrentReads(t *testing.T) {
	// Create array of numbers.
	numbers := make([]*Term, 10000)
	for i := 0; i < 10000; i++ {
		numbers[i] = IntNumberTerm(i)
	}
	// Shuffle numbers array for random insertion order.
	rand.New(rand.NewSource(10000)) // Seed the PRNG.
	rand.Shuffle(len(numbers), func(i, j int) {
		numbers[i], numbers[j] = numbers[j], numbers[i]
	})
	// Build set with numbers in unsorted order.
	s := NewSet()
	for i := 0; i < len(numbers); i++ {
		s.Add(numbers[i])
	}
	// In-place sort on numbers.
	sort.Sort(termSlice(numbers))

	// Check if race condition on key sorting is present.
	var wg sync.WaitGroup
	num := runtime.NumCPU()
	wg.Add(num)
	for i := 0; i < num; i++ {
		go func() {
			defer wg.Done()
			var retrieved []*Term
			s.Foreach(func(v *Term) {
				retrieved = append(retrieved, v)
			})
			// Check for sortedness of retrieved results.
			// This will hit a race condition around `s.sortedKeys`.
			for n := 0; n < len(retrieved); n++ {
				if retrieved[n] != numbers[n] {
					t.Errorf("Expected: %v at iteration %d but got %v instead", numbers[n], n, retrieved[n])
				}
			}
		}()
	}
	wg.Wait()
}

func TestObjectConcurrentReads(t *testing.T) {
	// Create array of numbers.
	numbers := make([]*Term, 10000)
	for i := 0; i < 10000; i++ {
		numbers[i] = IntNumberTerm(i)
	}
	// Shuffle numbers array for random insertion order.
	rand.New(rand.NewSource(10000)) // Seed the PRNG.
	rand.Shuffle(len(numbers), func(i, j int) {
		numbers[i], numbers[j] = numbers[j], numbers[i]
	})
	// Build an object with numbers in unsorted order.
	o := NewObject()
	for i := 0; i < len(numbers); i++ {
		o.Insert(numbers[i], NullTerm())
	}
	// In-place sort on numbers.
	sort.Sort(termSlice(numbers))

	// Check if race condition on key sorting is present.
	var wg sync.WaitGroup
	num := runtime.NumCPU()
	wg.Add(num)
	for i := 0; i < num; i++ {
		go func() {
			defer wg.Done()
			var retrieved []*Term
			o.Foreach(func(k, v *Term) {
				retrieved = append(retrieved, k)
			})
			// Check for sortedness of retrieved results.
			// This will hit a race condition around `s.sortedKeys`.
			for n := 0; n < len(retrieved); n++ {
				if retrieved[n] != numbers[n] {
					t.Errorf("Expected: %v at iteration %d but got %v instead", numbers[n], n, retrieved[n])
				}
			}
		}()
	}
	wg.Wait()
}

func TestArrayOperations(t *testing.T) {
	arr := MustParseTerm(`[1,2,3,4]`).Value.(*Array)

	getTests := []struct {
		input    string
		expected string
	}{
		{"x", ""},
		{"4.1", ""},
		{"-1", ""},
		{"4", ""},
		{"0", "1"},
		{"3", "4"},
	}

	for _, tc := range getTests {
		input := MustParseTerm(tc.input)
		result := arr.Get(input)

		if result != nil {
			if tc.expected != "" {
				expected := MustParseTerm(tc.expected)
				if expected.Equal(result) {
					continue
				}
			}
		} else if tc.expected == "" {
			continue
		}

		t.Errorf("Expected %v.get(%v) => %v but got: %v", arr, input, tc.expected, result)
	}

	// Iteration, append and slice tests

	var results []*Term
	tests := []struct {
		note     string
		input    string
		expected []string
		iterator func(arr *Array)
	}{
		{
			"for",
			`[1, 2, 3, 4]`,
			[]string{"1", "2", "3", "4"},
			func(arr *Array) {
				for i := 0; i < arr.Len(); i++ {
					results = append(results, arr.Elem(i))
				}
			},
		},
		{
			"foreach",
			"[1, 2, 3, 4]",
			[]string{"1", "2", "3", "4"},
			func(arr *Array) {
				arr.Foreach(func(v *Term) {
					results = append(results, v)
				})
			},
		},
		{
			"until",
			"[1, 2, 3, 4]",
			[]string{"1"},
			func(arr *Array) {
				arr.Until(func(v *Term) bool {
					results = append(results, v)
					return len(results) == 1
				})
			},
		},
		{
			"append",
			"[1, 2]",
			[]string{"1", "2", "3"},
			func(arr *Array) {
				arr.Append(MustParseTerm("3")).Foreach(func(v *Term) {
					results = append(results, v)
				})
			},
		},
		{
			"slice",
			"[1, 2, 3, 4]",
			[]string{"3", "4"},
			func(arr *Array) {
				arr.Slice(2, 4).Foreach(func(v *Term) {
					results = append(results, v)
				})
			},
		},
		{
			"slice",
			"[1, 2, 3, 4]",
			[]string{"3", "4"},
			func(arr *Array) {
				arr.Slice(2, -1).Foreach(func(v *Term) {
					results = append(results, v)
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			arr := MustParseTerm(tc.input).Value.(*Array)

			var expected []*Term
			for _, e := range tc.expected {
				expected = append(expected, MustParseTerm(e))
			}

			results = nil
			tc.iterator(arr)

			if !termSliceEqual(results, expected) {
				t.Errorf("Expected iteration to return %v but got %v", expected, results)
			}
		})
	}
}

func TestValueToInterface(t *testing.T) {
	// Happy path
	term := MustParseTerm(`{
		"foo": [1, "two", true, null, {3,
			}]
	}`)

	value, err := JSON(term.Value)
	if err != nil {
		t.Fatalf("Unexpected error while converting term %v to JSON: %v", term, err)
	}

	var expected interface{}
	if err := util.UnmarshalJSON([]byte(`{"foo": [1, "two", true, null, [3]]}`), &expected); err != nil {
		panic(err)
	}

	if util.Compare(value, expected) != 0 {
		t.Fatalf("Expected %v but got: %v", expected, value)
	}

	// Nested ref value
	term = MustParseTerm(`{
		"foo": [{data.a.b.c,}]
	}`)

	_, err = JSON(term.Value)

	if err == nil {
		t.Fatalf("Expected error from JSON(%v)", term)
	}

	// Ref key
	term = MustParseTerm(`{
		data.foo.a: 1
	}`)

	_, err = JSON(term.Value)

	if err == nil {
		t.Fatalf("Expected error from JSON(%v)", term)
	}

	// Requires evaluation
	term = MustParseTerm(`{
		"foo": [x | x = 1]
	}`)

	_, err = JSON(term.Value)

	if err == nil {
		t.Fatalf("Expected error from JSON(%v)", term)
	}

	// Ordering option
	//
	// These inputs exercise all of the cases (i.e., sets nested in arrays, object keys, and object values.)
	//
	a, err := JSONWithOpt(MustParseTerm(`[{{3, 4}: {1, 2}}]`).Value, JSONOpt{SortSets: true})
	if err != nil {
		t.Fatal(err)
	}

	b, err := JSONWithOpt(MustParseTerm(`[{{4, 3}: {2, 1}}]`).Value, JSONOpt{SortSets: true})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(a, b) {
		t.Fatalf("expcted %v = %v", a, b)
	}
}

// NOTE(sr): Without the opt-out, we don't allocate another object for
// the conversion back to interface{} if it can be avoided. As a result,
// the value held by the store could be changed.
func TestJSONWithOptLazyObjDefault(t *testing.T) {
	// would live in the store
	m := map[string]interface{}{
		"foo": "bar",
	}
	o := LazyObject(m)

	n, err := JSONWithOpt(o, JSONOpt{})
	if err != nil {
		t.Fatal(err)
	}
	n0, ok := n.(map[string]interface{})
	if !ok {
		t.Fatalf("expected %T, got %T: %[2]v", n0, n)
	}
	n0["baz"] = true

	if v, ok := m["baz"]; !ok || !v.(bool) {
		t.Errorf("expected change in m, found none: %v", m)
	}
}

func TestJSONWithOptLazyObjOptOut(t *testing.T) {
	// would live in the store
	m := map[string]interface{}{
		"foo": "bar",
	}
	o := LazyObject(m)

	n, err := JSONWithOpt(o, JSONOpt{CopyMaps: true})
	if err != nil {
		t.Fatal(err)
	}
	n0, ok := n.(map[string]interface{})
	if !ok {
		t.Fatalf("expected %T, got %T: %[2]v", n0, n)
	}
	n0["baz"] = true

	if _, ok := m["baz"]; ok {
		t.Errorf("expected no change in m, found one: %v", m)
	}
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

func TestLazyObjectGet(t *testing.T) {
	x := LazyObject(map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": true,
			},
		},
	})
	y := x.Get(StringTerm("a"))
	_, ok := y.Value.(*lazyObj)
	if !ok {
		t.Errorf("expected Get() to return another lazy object, got %v %[1]T", y.Value)
	}
	assertForced(t, x, false)
}

func TestLazyObjectGetCache(t *testing.T) {
	x := LazyObject(map[string]interface{}{
		"a": true,
		"b": false,
		"d": map[string]interface{}{
			"e": "f",
			"f": "g",
		},
	})

	// Assert that non-objects are cached

	y := x.Get(StringTerm("a"))

	if x.(*lazyObj).cache["a"].Compare(y.Value) != 0 {
		t.Errorf("expected cache to be populated with retreived value")
	}

	if x.(*lazyObj).cache["b"] != nil {
		t.Errorf("expected cache to not be populated with non-retrieved value")
	}

	// Assert that objects are cached as lazy objects

	y = x.Get(StringTerm("d"))

	expected := NewObject(Item(StringTerm("e"), StringTerm("f")), Item(StringTerm("f"), StringTerm("g")))
	if y.Value.Compare(expected) != 0 {
		t.Errorf("expected returned value to be %v, got %v", expected, y)
	}

	d := x.(*lazyObj).cache["d"]
	ld, ok := d.(*lazyObj)
	if !ok {
		t.Errorf("expected cache to be populated with lazy object, got %v", d)
	}
	if ld.Compare(expected) != 0 {
		t.Errorf("expected cached intermediate value to be %v, got %v", expected, y)
	}
}

func TestLazyObjectFind(t *testing.T) {
	x := LazyObject(map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": true,
			},
			"d": []interface{}{true, true, true},
		},
	})
	// retrieve object via Find
	y, err := x.Find(Ref{StringTerm("a"), StringTerm("b")})
	if err != nil {
		t.Fatal(err)
	}
	_, ok := y.(*lazyObj)
	if !ok {
		t.Errorf("expected Find() to return another lazy object, got %v %[1]T", y)
	}
	assertForced(t, x, false)

	// retrieve array via Find
	z, err := x.Find(Ref{StringTerm("a"), StringTerm("d")})
	if err != nil {
		t.Fatal(err)
	}
	_, ok = z.(*Array)
	if !ok {
		t.Errorf("expected Find() to return array, got %v %[1]T", z)
	}
}

func TestLazyObjectFindCache(t *testing.T) {
	x := LazyObject(map[string]interface{}{
		"a": []string{
			"b", "c", "d",
		},
		"c": []string{
			"d", "e", "f",
		},
		"d": map[string]interface{}{
			"e": "f",
			"f": "g",
		},
	})

	// Assert that non-objects are cached

	y, err := x.Find(Ref{StringTerm("a"), IntNumberTerm(1)})
	if err != nil {
		t.Fatal(err)
	}

	if y.Compare(String("c")) != 0 {
		t.Errorf("expected returned value to be 'c', got %v", y)
	}

	expected := NewArray(StringTerm("b"), StringTerm("c"), StringTerm("d"))
	if x.(*lazyObj).cache["a"].Compare(expected) != 0 {
		t.Errorf("expected cache to be populated with type-converted intermediate value, got %v",
			x.(*lazyObj).cache["a"])
	}

	if x.(*lazyObj).cache["b"] != nil {
		t.Errorf("expected cache to not be populated non-retrieved intermediate value, got %v",
			x.(*lazyObj).cache["b"])
	}

	// Assert that objects are cached as lazy objects

	y, err = x.Find(Ref{StringTerm("d"), StringTerm("e")})
	if err != nil {
		t.Fatal(err)
	}

	if y.Compare(String("f")) != 0 {
		t.Errorf("expected returned value to be 'c', got %v", y)
	}

	d := x.(*lazyObj).cache["d"]
	ld, ok := d.(*lazyObj)
	if !ok {
		t.Errorf("expected cache to be populated with lazy object, got %v", d)
	}
	if ld.cache["e"].Compare(String("f")) != 0 {
		t.Errorf("expected cache of intermediate lazyObj to be populated with type-converted intermediate value, got %v",
			ld.cache["e"])
	}
}

func TestLazyObjectCopy(t *testing.T) {
	x := LazyObject(map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": true,
			},
		},
	})
	y := x.Copy()
	_, ok := y.(*lazyObj)
	if !ok {
		t.Errorf("expected Get() to return another lazy object, got %v %[1]T", y)
	}
	assertForced(t, x, false)
}

func TestLazyObjectLen(t *testing.T) {
	x := LazyObject(map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": true,
			},
		},
	})
	if exp, act := 1, x.Len(); exp != act {
		t.Errorf("expected Len() %v, got %v", exp, act)
	}
	assertForced(t, x, false)
}

func TestLazyObjectIsGround(t *testing.T) {
	x := LazyObject(map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": true,
			},
		},
	})
	if exp, act := true, x.IsGround(); exp != act {
		t.Errorf("expected IsGround() %v, got %v", exp, act)
	}
	assertForced(t, x, false)
}

func TestLazyObjectInsert(t *testing.T) {
	x := LazyObject(map[string]interface{}{
		"a": "b",
	})
	x.Insert(StringTerm("c"), StringTerm("d"))
	assertForced(t, x, true)

	// NOTE(sr): We compare after asserting that it was forced, since comparison
	// forces the lazy object, too.
	if act, exp := x, NewObject(Item(StringTerm("a"), StringTerm("b")), Item(StringTerm("c"), StringTerm("d"))); exp.Compare(act) != 0 {
		t.Errorf("expected %v to be equal to %v", act, exp)
	}
}

func TestLazyObjectKeys(t *testing.T) {
	x := LazyObject(map[string]interface{}{
		"a": "A",
		"c": "C",
		"b": "B",
	})
	act := x.Keys()
	exp := []*Term{StringTerm("a"), StringTerm("b"), StringTerm("c")}
	if !reflect.DeepEqual(exp, act) {
		t.Errorf("expected Keys() %v, got %v", exp, act)
	}
	assertForced(t, x, false)
}

func TestLazyObjectKeysIterator(t *testing.T) {
	x := LazyObject(map[string]interface{}{
		"a": "A",
		"c": "C",
		"b": "B",
	})
	ki := x.KeysIterator()
	act := make([]*Term, 0, x.Len())
	for k, next := ki.Next(); next; k, next = ki.Next() {
		act = append(act, k)
	}
	exp := []*Term{StringTerm("a"), StringTerm("b"), StringTerm("c")}
	if !reflect.DeepEqual(exp, act) {
		t.Errorf("expected Keys() %v, got %v", exp, act)
	}
	assertForced(t, x, false)
}

func TestLazyObjectCompare(t *testing.T) {
	x := LazyObject(map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": true,
			},
		},
	})
	if exp, act := 1, x.Compare(NewObject()); exp != act {
		t.Errorf("expected Compare() => %v, got %v", exp, act)
	}
	assertForced(t, x, true)
}

func assertForced(t *testing.T, x Object, forced bool) {
	t.Helper()
	l, ok := x.(*lazyObj)
	switch {
	case !ok:
		t.Errorf("expected lazy object, got %v %[1]T", x)
	case !forced && l.strict != nil:
		t.Errorf("expected %v to not be forced", l)
	case forced && l.strict == nil:
		t.Errorf("expected %v to be forced", l)
	}
}
