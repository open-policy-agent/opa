// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"fmt"
	"net/url"
	"reflect"
	"testing"
)

func TestDropGlobals(t *testing.T) {

	// Without other params.
	abc := `a.b.c:{"foo":[1,2,3,4]}`
	abcEncoded := url.QueryEscape(abc)

	uri, err := url.ParseRequestURI(fmt.Sprintf(`http://localhost:8181/v1/data/foo/bar?global=%v`, abcEncoded))
	if err != nil {
		panic(err)
	}

	result := dropGlobals(uri)
	expected := "/v1/data/foo/bar"

	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	// With other params.
	def := `d.e.f:{"bar":{"baz":null}}`
	defEncoded := url.QueryEscape(def)

	uri, err = url.ParseRequestURI(fmt.Sprintf(`http://localhost:8181/v1/data/foo/bar?global=%v&pretty=true&depth=1&global=%v`, abcEncoded, defEncoded))
	if err != nil {
		panic(err)
	}

	result = dropGlobals(uri)
	expected = "/v1/data/foo/bar?depth=1&pretty=true"

	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

}

func TestGetGlobals(t *testing.T) {

	abc := `a.b.c:{"foo":[1,2,3,4]}`
	def := `d.e.f:{"bar":{"baz":null}}`
	abcEncoded := url.QueryEscape(abc)
	defEncoded := url.QueryEscape(def)

	uri, err := url.ParseRequestURI(fmt.Sprintf(`http://localhost:8181/v1/data/foo/bar?global=%v&pretty=true&global=%v`, abcEncoded, defEncoded))
	if err != nil {
		panic(err)
	}

	result := getGlobals(uri)
	expected := []string{abc, def}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v but got: %v", expected, result)
	}
}
