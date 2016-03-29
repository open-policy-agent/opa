package main

import (
	"reflect"
	"testing"
)

func TestRunIssue1(t *testing.T) {
	got, err := Parse("", []byte("foo"))
	if err != nil {
		t.Fatal(err)
	}
	want := "<nil>.foo"
	gots := got.(string)
	if gots != want {
		t.Errorf("want %q, got %q", want, gots)
	}
}

func TestIssue1(t *testing.T) {
	methods := map[string][]string{
		"onTableRef1": {"database", "table"},
		"onID1":       {},
	}

	typ := reflect.TypeOf(&current{})
	for nm, args := range methods {
		meth, ok := typ.MethodByName(nm)
		if !ok {
			t.Errorf("want *current to have method %s", nm)
			continue
		}
		if n := meth.Func.Type().NumIn(); n != len(args)+1 {
			t.Errorf("%q: want %d arguments, got %d", nm, len(args)+1, n)
			continue
		}
	}
}
