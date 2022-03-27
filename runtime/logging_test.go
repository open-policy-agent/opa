// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package runtime

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestDropInputParam(t *testing.T) {

	// Without other params.
	abc := `a.b.c:{"foo":[1,2,3,4]}`
	abcEncoded := url.QueryEscape(abc)

	uri, err := url.ParseRequestURI(fmt.Sprintf(`http://localhost:8181/v1/data/foo/bar?input=%v`, abcEncoded))
	if err != nil {
		panic(err)
	}

	result := dropInputParam(uri)
	expected := "/v1/data/foo/bar"

	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	// With other params.
	def := `d.e.f:{"bar":{"baz":null}}`
	defEncoded := url.QueryEscape(def)

	uri, err = url.ParseRequestURI(fmt.Sprintf(`http://localhost:8181/v1/data/foo/bar?input=%v&pretty=true&depth=1&input=%v`, abcEncoded, defEncoded))
	if err != nil {
		panic(err)
	}

	result = dropInputParam(uri)
	expected = "/v1/data/foo/bar?depth=1&pretty=true"

	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

}

func TestValidateGzipHeader(t *testing.T) {

	httpHeader := http.Header{}

	httpHeader.Add("Accept", "*/*")
	result := gzipAccepted(httpHeader)
	expected := false

	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	httpHeader.Add("Accept-Encoding", "gzip")

	result = gzipAccepted(httpHeader)
	expected = true

	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	httpHeader.Set("Accept-Encoding", "gzip, deflate, br")
	result = gzipAccepted(httpHeader)
	expected = true

	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	httpHeader.Set("Accept-Encoding", "br;q=1.0, gzip;q=0.8, *;q=0.1")
	result = gzipAccepted(httpHeader)
	expected = true

	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}
}

func TestValidatePprofUrl(t *testing.T) {

	req := http.Request{}

	req.URL = &url.URL{Path: "/metrics"}
	result := isPprofEndpoint(&req)
	expected := false

	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	req.URL = &url.URL{Path: "/debug/pprof/"}
	result = isPprofEndpoint(&req)
	expected = true

	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}
}
