package main

import "testing"

func TestReplacementChar(t *testing.T) {
	if _, err := Parse("", []byte("a\xef\xbf\xbdb")); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse("", []byte("a\xffb")); err == nil {
		t.Fatal("want error on invalid byte sequence")
	} else {
		t.Log(err)
	}
}
