package main

import "testing"

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

// Since go1.7: The Method and NumMethod methods of Type and Value no longer return or count unexported methods.
func TestIssue1(t *testing.T) {
	var cur interface{} = &current{}
	_, ok := cur.(interface {
		onTableRef1(interface{}, interface{}) (interface{}, error)
		onID1() (interface{}, error)
	})
	if !ok {
		t.Errorf("want *current to have expected methods")
	}
}
