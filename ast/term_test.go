// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
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

func TestObjectSetOperations(t *testing.T) {

	a := MustParseTerm(`{"a": "b", "c": "d"}`).Value.(Object)
	b := MustParseTerm(`{"c": "q", "d": "e"}`).Value.(Object)

	r1 := a.Diff(b)
	if !r1.Equal(MustParseTerm(`{"a": "b"}`).Value) {
		t.Errorf(`Expected a.Diff(b) to equal {"a": "b"} but got: %v`, r1)
	}

	r2 := a.Intersect(b)
	if len(r2) != 1 || !termSliceEqual(r2[0][:], MustParseTerm(`["c", "d", "q"]`).Value.(Array)) {
		t.Errorf(`Expected a.Intersect(b) to equal [["a", "d", "q"]] but got: %v`, r2)
	}

	if r3, ok := a.Merge(b); ok {
		t.Errorf("Expected a.Merge(b) to fail but got: %v", r3)
	}

	c := MustParseTerm(`{"a": {"b": [1], "c": {"d": 2}}}`).Value.(Object)
	d := MustParseTerm(`{"a": {"x": [3], "c": {"y": 4}}}`).Value.(Object)
	r3, ok := c.Merge(d)
	expected := MustParseTerm(`{"a": {"b": [1], "x": [3], "c": {"d": 2, "y": 4}}}`).Value.(Object)

	if !ok || !r3.Equal(expected) {
		t.Errorf("Expected c.Merge(d) to equal %v but got: %v", expected, r3)
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

func TestHash(t *testing.T) {

	doc := `{"a": [[true, {"b": [null]}, {"c": "d"}]], "e": {100: a[i].b}, "k": ["foo" | true], "o": {"foo": "bar" | true}, "sc": {"foo" | true}, "s": {1, 2, {3, 4}}, "big": 1e+1000}`

	stmt1 := MustParseStatement(doc)
	stmt2 := MustParseStatement(doc)

	obj1 := stmt1.(Body)[0].Terms.(*Term).Value.(Object)
	obj2 := stmt2.(Body)[0].Terms.(*Term).Value.(Object)

	if obj1.Hash() != obj2.Hash() {
		t.Errorf("Expected hash codes to be equal")
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
	assertToString(t, String("foo"), "\"foo\"")
	assertToString(t, String("\"foo\""), "\"\\\"foo\\\"\"")
	assertToString(t, String("foo bar"), "\"foo bar\"")
	assertToString(t, Var("foo"), "foo")
	assertToString(t, RefTerm(VarTerm("foo"), StringTerm("bar")).Value, "foo.bar")
	assertToString(t, RefTerm(VarTerm("foo"), StringTerm("bar"), VarTerm("i"), IntNumberTerm(0), StringTerm("baz")).Value, "foo.bar[i][0].baz")
	assertToString(t, RefTerm(VarTerm("foo"), BooleanTerm(false), NullTerm(), StringTerm("bar")).Value, "foo[false][null].bar")
	assertToString(t, ArrayTerm().Value, "[]")
	assertToString(t, ObjectTerm().Value, "{}")
	assertToString(t, SetTerm().Value, "set()")
	assertToString(t, ArrayTerm(ObjectTerm(Item(VarTerm("foo"), ArrayTerm(RefTerm(VarTerm("bar"), VarTerm("i"))))), StringTerm("foo"), SetTerm(BooleanTerm(true), NullTerm()), FloatNumberTerm(42.1)).Value, "[{foo: [bar[i]]}, \"foo\", {true, null}, 42.1]")
	assertToString(t, ArrayComprehensionTerm(ArrayTerm(VarTerm("x")), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})).Value, `[[x] | a[i]]`)
	assertToString(t, ObjectComprehensionTerm(VarTerm("y"), ArrayTerm(VarTerm("x")), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})).Value, `{y: [x] | a[i]}`)
	assertToString(t, SetComprehensionTerm(ArrayTerm(VarTerm("x")), NewBody(&Expr{Terms: RefTerm(VarTerm("a"), VarTerm("i"))})).Value, `{[x] | a[i]}`)
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

func TestRefDynamic(t *testing.T) {
	a := MustParseRef("foo.bar[baz.qux].corge")
	if a.Dynamic() != 2 {
		t.Fatalf("Expected dynamic offset to be baz.qux for foo.bar[baz.qux].corge")
	}
	if a[:a.Dynamic()].Dynamic() != -1 {
		t.Fatalf("Expected dynamic offset to be -1 for foo.bar")
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

	set := MustParseTerm(`{"foo", "bar", "baz", "qux"}`).Value.(*Set)

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

	if !result.Equal(expected) {
		t.Fatalf("Expected map result to be %v but got: %v", expected, result)
	}

	result, err = set.Map(func(*Term) (*Term, error) {
		return nil, fmt.Errorf("oops")
	})

	if !reflect.DeepEqual(err, fmt.Errorf("oops")) {
		t.Fatalf("Expected oops to be returned but got: %v, %v", result, err)
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
		s1 := MustParseTerm(tc.a).Value.(*Set)
		s2 := MustParseTerm(tc.b).Value.(*Set)
		s3 := MustParseTerm(tc.c).Value.(*Set)
		var result *Set
		if tc.op == "-" {
			result = s1.Diff(s2)
		} else if tc.op == "&" {
			result = s1.Intersect(s2)
		} else if tc.op == "|" {
			result = s1.Union(s2)
		} else {
			panic("bad operation")
		}
		if !result.Equal(s3) {
			t.Errorf("Expected %v for %v %v %v but got: %v", s3, tc.a, tc.op, tc.b, result)
		}
	}
}

func TestArrayOperations(t *testing.T) {

	arr := MustParseTerm(`[1,2,3,4]`).Value.(Array)

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

}

func TestValueToInterface(t *testing.T) {

	// Happy path
	term := MustParseTerm(`{
		"foo": [
			1, "two", true, null, {
				3,
			}
		]
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
