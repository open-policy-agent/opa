// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package topdown

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/internal/version"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/open-policy-agent/opa/tracing"

	iCache "github.com/open-policy-agent/opa/topdown/cache"

	"github.com/open-policy-agent/opa/ast"
)

// The person Type
type Person struct {
	ID        string `json:"id,omitempty"`
	Firstname string `json:"firstname,omitempty"`
}

// TestHTTPGetRequest returns the list of persons
func TestHTTPGetRequest(t *testing.T) {
	var people []Person

	// test data
	people = append(people, Person{ID: "1", Firstname: "John"})

	// test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := w.Header()
		headers["test-header"] = []string{"test-value"}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(people)
	}))

	defer ts.Close()

	// expected result
	expectedResult := make(map[string]interface{})
	expectedResult["status"] = "200 OK"
	expectedResult["status_code"] = http.StatusOK

	var body []interface{}
	bodyMap := map[string]string{"id": "1", "firstname": "John"}
	body = append(body, bodyMap)
	expectedResult["body"] = body
	expectedResult["raw_body"] = "[{\"id\":\"1\",\"firstname\":\"John\"}]\n"
	expectedResult["headers"] = map[string]interface{}{
		"content-length": []interface{}{"32"},
		"content-type":   []interface{}{"text/plain; charset=utf-8"},
		"test-header":    []interface{}{"test-value"},
	}

	resultObj := ast.MustInterfaceToValue(expectedResult)

	// run the test
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"http.send", []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "force_json_decode": true}, resp); x := clean_headers(resp) }`, ts.URL)}, resultObj.String()},
		{"http.send skip verify no HTTPS", []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "force_json_decode": true, "tls_insecure_skip_verify": true}, resp); x := clean_headers(resp) }`, ts.URL)}, resultObj.String()},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, append(tc.rules, httpSendHelperRules...), tc.expected)
	}
}

// TestHTTPGetRequest returns the list of persons
func TestHTTPGetRequestTlsInsecureSkipVerify(t *testing.T) {
	var people []Person

	// test data
	people = append(people, Person{ID: "1", Firstname: "John"})

	// test server
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(people)
	}))
	defer ts.Close()

	// expected result
	expectedResult := make(map[string]interface{})
	expectedResult["status"] = "200 OK"
	expectedResult["status_code"] = http.StatusOK

	var body []interface{}
	bodyMap := map[string]string{"id": "1", "firstname": "John"}
	body = append(body, bodyMap)
	expectedResult["body"] = body
	expectedResult["raw_body"] = "[{\"id\":\"1\",\"firstname\":\"John\"}]\n"
	expectedResult["headers"] = map[string]interface{}{
		"content-length": []interface{}{"32"},
		"content-type":   []interface{}{"text/plain; charset=utf-8"},
	}

	resultObj := ast.MustInterfaceToValue(expectedResult)

	type httpsStruct struct {
		note     string
		rules    []string
		expected interface{}
	}

	// run the test
	tests := []httpsStruct{}
	tests = append(tests, httpsStruct{note: "http.send", rules: []string{fmt.Sprintf(
		`p = x { http.send({"method": "get", "url": "%s", "force_json_decode": true, "tls_insecure_skip_verify": true}, resp); x := clean_headers(resp) }`, ts.URL)}, expected: resultObj.String()})

	// This case verifies that `tls_insecure_skip_verify`
	// is still applied, even if other TLS settings are
	// present.
	tests = append(tests, httpsStruct{note: "http.send", rules: []string{fmt.Sprintf(
		`p = x { http.send({"method": "get", "url": "%s", "force_json_decode": true, "tls_insecure_skip_verify": true, "tls_use_system_certs": true,}, resp); x := clean_headers(resp) }`, ts.URL)}, expected: resultObj.String()})

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, append(tc.rules, httpSendHelperRules...), tc.expected)
	}
}

func TestHTTPEnableJSONOrYAMLDecode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json-no-header":
			fmt.Fprintf(w, `{"foo":"bar"}`)
		case "/yaml-no-header":
			fmt.Fprintf(w, `foo: bar`)
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"foo":"bar"}`)
		case "/yaml":
			w.Header().Set("Content-Type", "application/yaml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `foo: bar`)
		case "/x-yaml":
			w.Header().Set("Content-Type", "application/x-yaml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `foo: bar`)
		case "/text-no-header":
			fmt.Fprintf(w, "*Hello World®")
		}
	}))

	defer ts.Close()

	body := func(b interface{}) func(map[string]interface{}) {
		return func(x map[string]interface{}) {
			x["body"] = b
		}
	}
	rawBody := func(b interface{}) func(map[string]interface{}) {
		return func(x map[string]interface{}) {
			x["raw_body"] = b
		}
	}

	headers := func(xs ...string) func(map[string]interface{}) {
		hdrs := map[string]interface{}{}
		for i := 0; i < len(xs)/2; i++ {
			hdrs[xs[2*i]] = []interface{}{xs[2*i+1]}
		}
		return func(x map[string]interface{}) {
			x["headers"] = hdrs
		}
	}

	ok := func(and ...func(map[string]interface{})) ast.Value {
		o := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
		}
		for _, a := range and {
			a(o)
		}
		return ast.MustInterfaceToValue(o)
	}

	resultObjText := ok(
		body(nil),
		rawBody("*Hello World®"),
		headers("content-length", "14", "content-type", "text/plain; charset=utf-8"),
	)

	tests := []struct {
		note     string
		rule     string
		expected ast.Value
	}{
		{
			note:     "text response, force json",
			rule:     fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s/text-no-header", "force_json_decode": true}, resp); x := clean_headers(resp) }`, ts.URL),
			expected: resultObjText,
		},
		{
			note:     "text response, force yaml",
			rule:     fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s/text-no-header", "force_yaml_decode": true}, resp); x := clean_headers(resp) }`, ts.URL),
			expected: resultObjText,
		},
		{
			note: "json response, proper header",
			rule: fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s/json"}, resp); x := clean_headers(resp) }`, ts.URL),
			expected: ok(
				body(map[string]interface{}{"foo": "bar"}),
				rawBody(`{"foo":"bar"}`),
				headers("content-length", "13", "content-type", "application/json"),
			),
		},
		{
			note: "yaml response, proper header",
			rule: fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s/yaml"}, resp); x := clean_headers(resp) }`, ts.URL),
			expected: ok(
				body(map[string]interface{}{"foo": "bar"}),
				rawBody(`foo: bar`),
				headers("content-length", "8", "content-type", "application/yaml"),
			),
		},
		{
			note: "yaml response, x-yaml header",
			rule: fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s/x-yaml"}, resp); x := clean_headers(resp) }`, ts.URL),
			expected: ok(
				body(map[string]interface{}{"foo": "bar"}),
				rawBody(`foo: bar`),
				headers("content-length", "8", "content-type", "application/x-yaml"),
			),
		},
		{
			note: "json response, no header",
			rule: fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s/json-no-header", "force_json_decode": true}, resp); x := clean_headers(resp) }`, ts.URL),
			expected: ok(
				body(map[string]interface{}{"foo": "bar"}),
				rawBody(`{"foo":"bar"}`),
				headers("content-length", "13", "content-type", "text/plain; charset=utf-8"),
			),
		},
		{
			note: "yaml response, no header",
			rule: fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s/yaml-no-header", "force_yaml_decode": true}, resp); x := clean_headers(resp) }`, ts.URL),
			expected: ok(
				body(map[string]interface{}{"foo": "bar"}),
				rawBody(`foo: bar`),
				headers("content-length", "8", "content-type", "text/plain; charset=utf-8"),
			),
		},
		{
			note: "json response, no header, yaml decode",
			rule: fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s/json-no-header", "force_yaml_decode": true}, resp); x := clean_headers(resp) }`, ts.URL),
			expected: ok(
				body(map[string]interface{}{"foo": "bar"}),
				rawBody(`{"foo":"bar"}`),
				headers("content-length", "13", "content-type", "text/plain; charset=utf-8"),
			),
		},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, append([]string{tc.rule}, httpSendHelperRules...), tc.expected.String())
	}
}

func echoCustomHeaders(w http.ResponseWriter, r *http.Request) {
	headers := make(map[string][]string)
	w.Header().Set("Content-Type", "application/json")
	for k, v := range r.Header {
		if strings.HasPrefix(k, "X-") || k == "User-Agent" {
			headers[k] = v
		}
	}
	_ = json.NewEncoder(w).Encode(headers)
}

// TestHTTPSendCustomRequestHeaders adds custom headers to request
func TestHTTPSendCustomRequestHeaders(t *testing.T) {
	// test server
	ts := httptest.NewServer(http.HandlerFunc(echoCustomHeaders))
	defer ts.Close()

	// expected result with default User-Agent
	expectedResult := make(map[string]interface{})
	expectedResult["status"] = "200 OK"
	expectedResult["status_code"] = http.StatusOK

	bodyMap := map[string][]string{"X-Foo": {"ISO-8859-1,utf-8;q=0.7,*;q=0.7"}, "X-Opa": {"server"}, "User-Agent": {version.UserAgent}}
	expectedResult["body"] = bodyMap
	expectedResult["raw_body"] = fmt.Sprintf("{\"User-Agent\":[\"%s\"],\"X-Foo\":[\"ISO-8859-1,utf-8;q=0.7,*;q=0.7\"],\"X-Opa\":[\"server\"]}\n", version.UserAgent)

	jsonString, err := json.Marshal(expectedResult)
	if err != nil {
		panic(err)
	}
	s := string(jsonString[:])

	// expected result with custom User-Agent

	bodyMap = map[string][]string{"X-Opa": {"server"}, "User-Agent": {"AuthZPolicy/0.0.1"}}
	expectedResult["body"] = bodyMap
	expectedResult["raw_body"] = "{\"User-Agent\":[\"AuthZPolicy/0.0.1\"],\"X-Opa\":[\"server\"]}\n"

	jsonString, err = json.Marshal(expectedResult)
	if err != nil {
		panic(err)
	}
	s2 := string(jsonString[:])

	// run the test
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"http.send custom headers", []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "headers": {"X-Foo": "ISO-8859-1,utf-8;q=0.7,*;q=0.7", "X-Opa": "server"}}, resp); x := remove_headers(resp) }`, ts.URL)}, s},
		{"http.send custom UA", []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "headers": {"User-Agent": "AuthZPolicy/0.0.1", "X-Opa": "server"}}, resp); x := remove_headers(resp) }`, ts.URL)}, s2},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, append(tc.rules, httpSendHelperRules...), tc.expected)
	}
}

// TestHTTPHostHeader tests Host header support
func TestHTTPHostHeader(t *testing.T) {
	// test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(r.Host)
	}))

	defer ts.Close()

	expectedResult, err := json.Marshal(map[string]interface{}{
		"status":      "200 OK",
		"status_code": http.StatusOK,
		"body":        t.Name(),
		"raw_body":    fmt.Sprintf("\"%s\"\n", t.Name()),
		"headers": map[string]interface{}{
			"content-length": []interface{}{"21"},
			"content-type":   []interface{}{"application/json"},
		},
	})
	if err != nil {
		panic(err)
	}

	data := loadSmallTestData()

	for _, h := range []string{"HOST", "Host", "host"} {
		runTopDownTestCase(t,
			data,
			fmt.Sprintf("http.send custom Host header %q", h),
			append(httpSendHelperRules, fmt.Sprintf(
				`p = x { http.send({ "method": "get", "url": "%s", "headers": {"%s": "%s"}}, resp); x := clean_headers(resp) }`, ts.URL, h, t.Name()),
			),
			string(expectedResult))
	}
}

// TestHTTPPostRequest adds a new person
func TestHTTPPostRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")

		bs, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(bs)
		if err != nil {
			t.Fatal(err)
		}
	}))

	defer ts.Close()

	tests := []struct {
		note        string
		params      string
		respHeaders string
		expected    interface{}
	}{
		{
			note: "basic",
			params: `{
				"method": "post",
				"headers": {"Content-Type": "application/json"},
				"body": {"id": "2", "firstname": "Joe"}
			}`,
			expected: `{
				"status": "200 OK",
				"status_code": 200,
				"body": {"id": "2", "firstname": "Joe"},
				"raw_body": "{\"firstname\":\"Joe\",\"id\":\"2\"}",
				"headers": {"content-type": ["application/json"], "content-length": ["28"]}
			}`,
		},
		{
			note: "raw_body",
			params: `{
				"method": "post",
				"headers": {"content-type": "application/x-www-form-encoded"},
				"raw_body": "username=foobar&password=baz"
			}`,
			expected: `{
				"status": "200 OK",
				"status_code": 200,
				"body": null,
				"raw_body": "username=foobar&password=baz",
				"headers": {"content-type": ["application/x-www-form-encoded"], "content-length": ["28"]}
			}`,
		},
		{
			note: "raw_body overrides body",
			params: `{
				"method": "post",
				"headers": {"content-type": "application/x-www-form-encoded"},
				"body": {"foo": 1},
				"raw_body": "username=foobar&password=baz"
			}`,
			expected: `{
				"status": "200 OK",
				"status_code": 200,
				"body": null,
				"raw_body": "username=foobar&password=baz",
				"headers": {"content-type": ["application/x-www-form-encoded"], "content-length": ["28"]}
			}`,
		},
		{
			note: "raw_body bad type",
			params: `{
				"method": "post",
				"headers": {"content-type": "application/x-www-form-encoded"},
				"raw_body": {"bar": "bar"}
			}`,
			expected: &Error{Code: BuiltinErr, Message: "\"raw_body\" must be a string"},
		},
	}

	data := map[string]interface{}{}

	for _, tc := range tests {

		// Automatically set the URL because it's generated when the test server
		// is started. If needed, the test cases can override in the future.
		term := ast.MustParseTerm(tc.params)
		term.Value.(ast.Object).Insert(ast.StringTerm("url"), ast.StringTerm(ts.URL))

		rules := append(
			httpSendHelperRules,
			fmt.Sprintf(`p = x { http.send(%s, resp); x := clean_headers(resp) }`, term),
		)

		runTopDownTestCase(t, data, tc.note, rules, tc.expected)
	}
}

func TestHTTPDeleteRequest(t *testing.T) {
	var people []Person

	// test data
	people = append(people, Person{ID: "1", Firstname: "John"})
	people = append(people, Person{ID: "2", Firstname: "Joe"})

	// test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var person Person
		if r.Body == nil {
			http.Error(w, "Please send a request body", 400)
			return
		}
		err := json.NewDecoder(r.Body).Decode(&person)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		// delete person
		for index, item := range people {
			if item.ID == person.ID {
				people = append(people[:index], people[index+1:]...)
				break
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(people)
	}))

	defer ts.Close()

	// expected result
	expectedResult := make(map[string]interface{})
	expectedResult["status"] = "200 OK"
	expectedResult["status_code"] = http.StatusOK

	var body []interface{}
	bodyMap := map[string]string{"id": "1", "firstname": "John"}
	body = append(body, bodyMap)
	expectedResult["body"] = body
	expectedResult["raw_body"] = "[{\"id\":\"1\",\"firstname\":\"John\"}]\n"
	expectedResult["headers"] = map[string]interface{}{
		"content-length": []interface{}{"32"},
		"content-type":   []interface{}{"application/json"},
	}

	resultObj := ast.MustInterfaceToValue(expectedResult)

	// delete a new person
	personToDelete := Person{ID: "2", Firstname: "Joe"}
	b := new(bytes.Buffer)
	_ = json.NewEncoder(b).Encode(personToDelete)

	// run the test
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"http.send", []string{fmt.Sprintf(
			`p = x { http.send({"method": "delete", "url": "%s", "body": %s}, resp); x := clean_headers(resp) }`, ts.URL, b)}, resultObj.String()},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, append(tc.rules, httpSendHelperRules...), tc.expected)
	}
}

// TestInvalidKeyError returns an error when an invalid key is passed in the
// http.send builtin
func TestInvalidKeyError(t *testing.T) {
	// run the test
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"invalid keys", []string{`p = x { http.send({"method": "get", "url": "http://127.0.0.1:51113", "bad_key": "bad_value"}, x) }`}, &Error{Code: TypeErr, Message: `invalid request parameters(s): {"bad_key"}`}},
		{"missing keys", []string{`p = x { http.send({"method": "get"}, x) }`}, &Error{Code: TypeErr, Message: `missing required request parameters(s): {"url"}`}},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestParseTimeout(t *testing.T) {
	tests := []struct {
		note     string
		raw      ast.Value
		expected interface{}
	}{
		{
			note:     "zero string",
			raw:      ast.String("0"),
			expected: time.Duration(0),
		},
		{
			note:     "zero number",
			raw:      ast.Number(strconv.FormatInt(0, 10)),
			expected: time.Duration(0),
		},
		{
			note:     "number",
			raw:      ast.Number(strconv.FormatInt(1234, 10)),
			expected: time.Duration(1234),
		},
		{
			note:     "number with invalid float",
			raw:      ast.Number("1.234"),
			expected: errors.New("invalid timeout number value"),
		},
		{
			note:     "string no units",
			raw:      ast.String("1000"),
			expected: time.Duration(1000),
		},
		{
			note:     "string with units",
			raw:      ast.String("10ms"),
			expected: time.Duration(10000000),
		},
		{
			note:     "string with complex units",
			raw:      ast.String("1s10ms5us"),
			expected: time.Second + (10 * time.Millisecond) + (5 * time.Microsecond),
		},
		{
			note:     "string with invalid duration format",
			raw:      ast.String("1xyz 2"),
			expected: errors.New("invalid timeout value"),
		},
		{
			note:     "string with float",
			raw:      ast.String("1.234"),
			expected: errors.New("invalid timeout value"),
		},
		{
			note:     "invalid value type object",
			raw:      ast.NewObject(),
			expected: builtins.NewOperandErr(1, "'timeout' must be one of {string, number} but got object"),
		},
		{
			note:     "invalid value type set",
			raw:      ast.NewSet(),
			expected: builtins.NewOperandErr(1, "'timeout' must be one of {string, number} but got set"),
		},
		{
			note:     "invalid value type array",
			raw:      ast.NewArray(),
			expected: builtins.NewOperandErr(1, "'timeout' must be one of {string, number} but got array"),
		},
		{
			note:     "invalid value type boolean",
			raw:      ast.Boolean(true),
			expected: builtins.NewOperandErr(1, "'timeout' must be one of {string, number} but got boolean"),
		},
		{
			note:     "invalid value type null",
			raw:      ast.Null{},
			expected: builtins.NewOperandErr(1, "'timeout' must be one of {string, number} but got null"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			actual, err := parseTimeout(tc.raw)
			switch e := tc.expected.(type) {
			case error:
				assertError(t, tc.expected, err)
			case time.Duration:
				if e != actual {
					t.Fatalf("Expected %d but got %d", e, actual)
				}
			}
		})
	}
}

// TestHTTPRedirectDisable tests redirects are not enabled by default
func TestHTTPRedirectDisable(t *testing.T) {
	// test server
	baseURL, teardown := getTestServer()
	defer teardown()

	// expected result
	expectedResult := make(map[string]interface{})
	expectedResult["body"] = nil
	expectedResult["raw_body"] = "<a href=\"/test\">Moved Permanently</a>.\n\n"
	expectedResult["status"] = "301 Moved Permanently"
	expectedResult["status_code"] = http.StatusMovedPermanently
	expectedResult["headers"] = map[string]interface{}{
		"content-length": []interface{}{"40"},
		"content-type":   []interface{}{"text/html; charset=utf-8"},
		"location":       []interface{}{"/test"},
	}

	resultObj := ast.MustInterfaceToValue(expectedResult)

	data := loadSmallTestData()
	rules := append(
		httpSendHelperRules,
		fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s"}, resp); x := clean_headers(resp) }`, baseURL),
	)

	// run the test
	runTopDownTestCase(t, data, "http.send", rules, resultObj.String())
}

// TestHTTPRedirectEnable tests redirects are enabled
func TestHTTPRedirectEnable(t *testing.T) {
	// test server
	baseURL, teardown := getTestServer()
	defer teardown()

	// expected result
	expectedResult := make(map[string]interface{})
	expectedResult["status"] = "200 OK"
	expectedResult["status_code"] = http.StatusOK
	expectedResult["body"] = nil
	expectedResult["raw_body"] = ""
	expectedResult["headers"] = map[string]interface{}{
		"content-length": []interface{}{"0"},
	}

	resultObj := ast.MustInterfaceToValue(expectedResult)

	data := loadSmallTestData()
	rules := append(
		httpSendHelperRules,
		fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s", "enable_redirect": true}, resp); x := clean_headers(resp) }`, baseURL),
	)

	// run the test
	runTopDownTestCase(t, data, "http.send", rules, resultObj.String())
}

func TestHTTPRedirectAllowNet(t *testing.T) {
	// test server
	baseURL, teardown := getTestServer()
	defer teardown()

	// host
	serverURL, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	serverHost := strings.Split(serverURL.Host, ":")[0]

	// expected result
	expectedResult := make(map[string]interface{})
	expectedResult["status"] = "200 OK"
	expectedResult["status_code"] = http.StatusOK
	expectedResult["body"] = nil
	expectedResult["raw_body"] = ""

	resultObj := ast.MustInterfaceToValue(expectedResult)

	expectedError := &Error{Code: "eval_builtin_error", Message: fmt.Sprintf("http.send: unallowed host: %s", serverHost)}

	rules := []string{fmt.Sprintf(
		`p = x { http.send({"method": "get", "url": "%s", "enable_redirect": true, "force_json_decode": true}, resp); x := remove_headers(resp) }`, baseURL)}

	// run the test
	tests := []struct {
		note     string
		rules    []string
		options  func(*Query) *Query
		expected interface{}
	}{
		{
			"http.send allow_net nil",
			rules,
			setAllowNet(nil),
			resultObj.String(),
		},
		{
			"http.send allow_net match",
			rules,
			setAllowNet([]string{serverHost}),
			resultObj.String(),
		},
		{
			"http.send allow_net empty",
			rules,
			setAllowNet([]string{}),
			expectedError,
		},
		{
			"http.send allow_net no match",
			rules,
			setAllowNet([]string{"example.com"}),
			expectedError,
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, append(tc.rules, httpSendHelperRules...), tc.expected, tc.options)
	}
}

func TestHTTPSendRaiseError(t *testing.T) {
	// test server
	baseURL, teardown := getTestServer()
	defer teardown()

	networkErrObj := make(map[string]interface{})
	networkErrObj["code"] = HTTPSendNetworkErr
	networkErrObj["message"] = "Get \"foo://foo.com\": unsupported protocol scheme \"foo\""

	networkErr := ast.MustInterfaceToValue(networkErrObj)

	internalErrObj := make(map[string]interface{})
	internalErrObj["code"] = HTTPSendInternalErr
	internalErrObj["message"] = fmt.Sprintf(`http.send({"method": "get", "url": "%s", "force_json_decode": true, "raise_error": false, "force_cache": true}): eval_builtin_error: http.send: 'force_cache' set but 'force_cache_duration_seconds' parameter is missing`, baseURL)

	internalErr := ast.MustInterfaceToValue(internalErrObj)

	responseObj := make(map[string]interface{})
	responseObj["status_code"] = 0
	responseObj["error"] = internalErrObj

	response := ast.MustInterfaceToValue(responseObj)

	tests := []struct {
		note         string
		ruleTemplate string
		body         string
		response     interface{}
	}{
		{
			note: "http.send invalid url (don't raise error, check response body)",
			ruleTemplate: `p = x {
									r = http.send({"method": "get", "url": "%URL%.com", "force_json_decode": true, "raise_error": false})
									x = r.body
								}`,
			response: ``,
		},
		{
			note: "http.send invalid url (don't raise error, check response status code)",
			ruleTemplate: `p = x {
									r = http.send({"method": "get", "url": "%URL%.com", "force_json_decode": true, "raise_error": false})
									x = r.status_code
								}`,
			response: `0`,
		},
		{
			note: "http.send invalid url (don't raise error, network error)",
			ruleTemplate: `p = x {
									r = http.send({"method": "get", "url": "foo://foo.com", "force_json_decode": true, "raise_error": false})
									x = r.error
								}`,
			response: networkErr.String(),
		},
		{
			note: "http.send missing param (don't raise error, internal error)",
			ruleTemplate: `p = x {
									r = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "raise_error": false, "force_cache": true})
									x = r.error
								}`,
			response: internalErr.String(),
		},
		{
			note: "http.send missing param (don't raise error,  check response)",
			ruleTemplate: `p = x {
									r = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "raise_error": false, "force_cache": true})
									x = r
								}`,
			response: response.String(),
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			runTopDownTestCase(t, data, tc.note, []string{strings.ReplaceAll(tc.ruleTemplate, "%URL%", baseURL)}, tc.response)
		})
	}
}

func TestHTTPSendCaching(t *testing.T) {
	// run the test
	tests := []struct {
		note             string
		ruleTemplate     string
		body             string
		response         string
		expectedReqCount int
	}{
		{
			note:             "http.send GET single",
			ruleTemplate:     `p = x { http.send({"method": "get", "url": "%URL%", "force_json_decode": true}, r); x = r.body }`,
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
		{
			note: "http.send GET cache hit",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true})  # cached
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true})  # cached
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
		{
			note: "http.send GET cache miss different method",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true})
									r2 = http.send({"method": "post", "url": "%URL%", "force_json_decode": true})
									r1_2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true})  # cached
									r2_2 = http.send({"method": "post", "url": "%URL%", "force_json_decode": true})  # cached
									x = r1.body
								}`,
			response:         `{"x": 1}`,
			expectedReqCount: 2,
		},
		{
			note: "http.send GET cache miss different url",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%/foo", "force_json_decode": true})
									r2 = http.send({"method": "get", "url": "%URL%/bar", "force_json_decode": true})
									r1_2 = http.send({"method": "get", "url": "%URL%/foo", "force_json_decode": true})  # cached
									r2_2 = http.send({"method": "get", "url": "%URL%/bar", "force_json_decode": true})  # cached
									x = r1.body
								}`,
			response:         `{"x": 1}`,
			expectedReqCount: 2,
		},
		{
			note: "http.send GET cache miss different decode opt",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": false})
									r1_2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true})  # cached
									r2_2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": false})  # cached
									x = r1.body
								}`,
			response:         `{"x": 1}`,
			expectedReqCount: 2,
		},
		{
			note: "http.send GET cache miss different headers",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "headers": {"h1": "v1", "h2": "v2"}})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "headers": {"h2": "v2"}})
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "headers": {"h2": "v3"}})
									r1_2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "headers": {"h1": "v1", "h2": "v2"}})  # cached
									r2_2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "headers": {"h2": "v2"}})  # cached
									r3_2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "headers": {"h2": "v3"}})  # cached
									x = r1.body
								}`,
			response:         `{"x": 1}`,
			expectedReqCount: 3,
		},
		{
			note: "http.send POST cache miss different body",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "post", "url": "%URL%", "force_json_decode": true, "headers": {"h2": "v2"}, "body": "{\"foo\": 42}"})
									r2 = http.send({"method": "post", "url": "%URL%", "force_json_decode": true, "headers": {"h2": "v3"}, "body": "{\"foo\": 23}"})
									r1_2 = http.send({"method": "post", "url": "%URL%", "force_json_decode": true, "headers": {"h2": "v2"}, "body": "{\"foo\": 42}"})  # cached
									r2_2 = http.send({"method": "post", "url": "%URL%", "force_json_decode": true, "headers": {"h2": "v3"}, "body": "{\"foo\": 23}"})  # cached
									x = r1.body
								}`,
			response:         `{"x": 1}`,
			expectedReqCount: 2,
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var requests []*http.Request
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r)
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(tc.response))
				if err != nil {
					t.Fatal(err)
				}
			}))
			defer ts.Close()

			runTopDownTestCase(t, data, tc.note, []string{strings.ReplaceAll(tc.ruleTemplate, "%URL%", ts.URL)}, tc.response)

			// Note: The runTopDownTestCase ends up evaluating twice (once with and once without partial
			// eval first), so expect 2x the total request count the test case specified.
			actualCount := len(requests) / 2
			if actualCount != tc.expectedReqCount {
				t.Fatalf("Expected to get %d requests, got %d", tc.expectedReqCount, actualCount)
			}
		})
	}
}

func TestHTTPSendIntraQueryCaching(t *testing.T) {
	tests := []struct {
		note             string
		ruleTemplate     string
		headers          map[string][]string
		body             string
		response         string
		expectedReqCount int
	}{
		{
			note:             "http.send GET single",
			ruleTemplate:     `p = x { http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}, r); x = r.body }`,
			headers:          map[string][]string{"Cache-Control": {"max-age=290304000, public"}},
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
		{
			note: "http.send GET cache hit (max_age_response_fresh)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})  # cached and fresh
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})  # cached and fresh
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{"Cache-Control": {"max-age=290304000, public"}},
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
		{
			note: "http.send GET cache hit (expires_header_response_fresh)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})  # cached and fresh
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})  # cached and fresh
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{"Expires": {"Wed, 31 Dec 2115 07:28:00 GMT"}},
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
		{
			note: "http.send GET (expires_header_invalid_value)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})  # not cached
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})  # not cached
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{"Expires": {"0"}},
			response:         `{"x": 1}`,
			expectedReqCount: 3,
		},
		{
			note: "http.send GET no-store cache",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})  # not cached
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})  # not cached
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{"Cache-Control": {"no-store"}},
			response:         `{"x": 1}`,
			expectedReqCount: 3,
		},
		{
			note: "http.send GET (response_stale_revalidate_with_etag)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # stale
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # stale
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{"Cache-Control": {"max-age=0, public"}, "Etag": {"1234"}},
			response:         `{"x": 1}`,
			expectedReqCount: 3,
		},
		{
			note: "http.send GET (response_stale_revalidate_with_last_modified)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # stale
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # stale
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{"Cache-Control": {"max-age=0, public"}, "Last-Modified": {"Wed, 31 Dec 2115 07:28:00 GMT"}},
			response:         `{"x": 1}`,
			expectedReqCount: 3,
		},
		{
			note: "http.send GET (response_age_negative_duration)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # stale
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # stale
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{"Cache-Control": {"max-age=0, public"}, "Last-Modified": {"Wed, 31 Dec 2115 07:28:00 GMT"}, "Date": {"Wed, 31 Dec 2115 07:28:00 GMT"}},
			response:         `{"x": 1}`,
			expectedReqCount: 3,
		},
		{
			note: "http.send GET cache hit deserialized mode (max_age_response_fresh)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "caching_mode": "deserialized"})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "caching_mode": "deserialized"})  # cached and fresh
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "caching_mode": "deserialized"})  # cached and fresh
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{"Cache-Control": {"max-age=290304000, public"}},
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
		{
			note: "http.send GET cache hit serialized mode explicit (max_age_response_fresh)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "caching_mode": "serialized"})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "caching_mode": "serialized"})  # cached and fresh
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "caching_mode": "serialized"})  # cached and fresh
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{"Cache-Control": {"max-age=290304000, public"}},
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
		{
			note: "http.send GET cache hit serialized mode explicit (max_age_response_fresh), when parsing a yaml response",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "cache": true, "caching_mode": "serialized"})
									r2 = http.send({"method": "get", "url": "%URL%", "cache": true, "caching_mode": "serialized"})  # cached and fresh
									r3 = http.send({"method": "get", "url": "%URL%", "cache": true, "caching_mode": "serialized"})  # cached and fresh
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers: map[string][]string{
				"Cache-Control": {"max-age=290304000, public"},
				"Content-Type":  {"application/yaml"},
			},
			// NOTE: fed into runTopDownTestCase, so it has to be JSON; but we're making use of YAML being a superset of JSON
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
	}

	data := loadSmallTestData()

	t0 := time.Now()
	opts := setTime(t0)

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var requests []*http.Request
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r)
				headers := w.Header()

				for k, v := range tc.headers {
					headers[k] = v
				}

				headers.Set("Date", t0.Format(time.RFC850))

				etag := w.Header().Get("etag")
				lm := w.Header().Get("last-modified")

				if etag != "" {
					if r.Header.Get("if-none-match") == etag {
						w.WriteHeader(http.StatusNotModified)
					}
				} else if lm != "" {
					if r.Header.Get("if-modified-since") == lm {
						w.WriteHeader(http.StatusNotModified)
					}
				} else {
					w.WriteHeader(http.StatusOK)
				}
				_, _ = w.Write([]byte(tc.response)) // ignore error
			}))
			defer ts.Close()

			runTopDownTestCase(t, data, tc.note, []string{strings.ReplaceAll(tc.ruleTemplate, "%URL%", ts.URL)}, tc.response, opts)

			// Note: The runTopDownTestCase ends up evaluating twice (once with and once without partial
			// eval first), so expect 2x the total request count the test case specified.
			actualCount := len(requests) / 2
			if actualCount != tc.expectedReqCount {
				t.Fatalf("Expected to get %d requests, got %d", tc.expectedReqCount, actualCount)
			}
		})
	}
}

func TestHTTPSendIntraQueryForceCaching(t *testing.T) {
	tests := []struct {
		note             string
		ruleTemplate     string
		headers          map[string][]string
		body             string
		response         string
		expectedReqCount int
	}{
		{
			note: "http.send GET cache hit (force_cache_only)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "force_cache": true, "force_cache_duration_seconds": 300})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "force_cache": true, "force_cache_duration_seconds": 300})  # cached and fresh
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "force_cache": true, "force_cache_duration_seconds": 300})  # cached and fresh
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{"Expires": {"Wed, 31 Dec 2005 07:28:00 GMT"}},
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
		{
			note: "http.send GET cache hit, empty headers (force_cache_only)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "force_cache": true, "force_cache_duration_seconds": 300})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "force_cache": true, "force_cache_duration_seconds": 300})  # cached and fresh
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "force_cache": true, "force_cache_duration_seconds": 300})  # cached and fresh
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{},
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
		{
			note: "http.send GET cache hit (cache_param_override)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "force_cache": true, "force_cache_duration_seconds": 300})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "force_cache": true, "force_cache_duration_seconds": 300})  # cached and fresh
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "force_cache": true, "force_cache_duration_seconds": 300})  # cached and fresh
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{"Expires": {"Wed, 31 Dec 2005 07:28:00 GMT"}},
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
		{
			note: "http.send GET cache hit (force_cache_only_no_store_override)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "force_cache": true, "force_cache_duration_seconds": 300})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "force_cache": true, "force_cache_duration_seconds": 300})  # cached and fresh
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "force_cache": true, "force_cache_duration_seconds": 300})  # cached and fresh
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers: map[string][]string{
				"Expires":       {"Wed, 31 Dec 2005 07:28:00 GMT"},
				"Cache-Control": {"no-store"},
			},
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
		{
			note: "http.send GET cache hit (cache_param_override_no_store_override)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "force_cache": true, "force_cache_duration_seconds": 300})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "force_cache": true, "force_cache_duration_seconds": 300})  # cached and fresh
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "force_cache": true, "force_cache_duration_seconds": 300})  # cached and fresh
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers: map[string][]string{
				"Expires":       {"Wed, 31 Dec 2005 07:28:00 GMT"},
				"Cache-Control": {"no-store", "no-cache", "max-age=0"},
			},
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
		{
			note: "http.send GET cache hit (cache_param_override_no_store_override_invalid_expires_header_value)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "force_cache": true, "force_cache_duration_seconds": 300})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "force_cache": true, "force_cache_duration_seconds": 300})  # cached and fresh
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "force_cache": true, "force_cache_duration_seconds": 300})  # cached and fresh
									r1 == r2
									r2 == r3
									x = r1.body
								}`,
			headers: map[string][]string{
				"Expires":       {"0"},
				"Cache-Control": {"no-store", "no-cache", "max-age=0"},
			},
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
	}

	data := map[string]interface{}{}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t0 := time.Now().UTC()
			opts := setTime(t0)

			var requests []*http.Request
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r)
				headers := w.Header()

				for k, v := range tc.headers {
					headers[k] = v
				}

				headers.Set("Date", t0.Format(http.TimeFormat))

				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(tc.response))
				if err != nil {
					t.Fatal(err)
				}
			}))
			defer ts.Close()

			runTopDownTestCase(t, data, tc.note, []string{strings.ReplaceAll(tc.ruleTemplate, "%URL%", ts.URL)}, tc.response, opts)

			// Note: The runTopDownTestCase ends up evaluating twice (once with and once without partial
			// eval first), so expect 2x the total request count the test case specified.
			actualCount := len(requests) / 2
			if actualCount != tc.expectedReqCount {
				t.Errorf("Expected to get %d requests, got %d", tc.expectedReqCount, actualCount)
			}
		})
	}
}

func TestHTTPSendIntraQueryCachingModifiedResp(t *testing.T) {
	tests := []struct {
		note             string
		ruleTemplate     string
		headers          map[string][]string
		body             string
		response         string
		expectedReqCount int
	}{
		{
			note: "http.send GET (response_stale_revalidate_with_etag)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # stale
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # cached and fresh
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{"Cache-Control": {"max-age=0, public"}, "Etag": {"1234"}, "location": {"/test"}},
			response:         `{"x": 1}`,
			expectedReqCount: 2,
		},
		{
			note: "http.send GET cache deserialized mode (response_stale_revalidate_with_etag)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "caching_mode": "deserialized"})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "caching_mode": "deserialized"}) # stale
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true, "caching_mode": "deserialized"}) # cached and fresh
									r2 == r3
									x = r1.body
								}`,
			headers:          map[string][]string{"Cache-Control": {"max-age=0, public"}, "Etag": {"1234"}, "location": {"/test"}},
			response:         `{"x": 1}`,
			expectedReqCount: 2,
		},
		{
			note: "http.send GET (response_stale_revalidate_with_no_etag)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # stale
									r1 == r2
									x = r1.body
								}`,
			headers:          map[string][]string{"Cache-Control": {"max-age=0, public"}},
			response:         `{"x": 1}`,
			expectedReqCount: 2,
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t0 := time.Now().UTC()
			opts := setTime(t0)

			var requests []*http.Request
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r)
				headers := w.Header()

				for k, v := range tc.headers {
					headers[k] = v
				}

				headers.Set("Date", t0.Format(http.TimeFormat))

				etag := w.Header().Get("etag")

				if r.Header.Get("if-none-match") != "" {
					if r.Header.Get("if-none-match") == etag {
						// add new headers and update existing header value
						headers["Cache-Control"] = []string{"max-age=290304000, public"}
						headers["foo"] = []string{"bar"}
						w.WriteHeader(http.StatusNotModified)
					}
				} else {
					w.WriteHeader(http.StatusOK)
				}
				_, _ = w.Write([]byte(tc.response)) // ignore error
			}))
			defer ts.Close()

			runTopDownTestCase(t, data, tc.note, []string{strings.ReplaceAll(tc.ruleTemplate, "%URL%", ts.URL)}, tc.response, opts)

			// Note: The runTopDownTestCase ends up evaluating twice (once with and once without partial
			// eval first), so expect 2x the total request count the test case specified.
			actualCount := len(requests) / 2
			if actualCount != tc.expectedReqCount {
				t.Fatalf("Expected to get %d requests, got %d", tc.expectedReqCount, actualCount)
			}
		})
	}
}

func TestHTTPSendIntraQueryCachingNewResp(t *testing.T) {
	tests := []struct {
		note             string
		ruleTemplate     string
		headers          map[string][]string
		body             string
		response         string
		expectedReqCount int
	}{
		{
			note: "http.send GET (response_stale_revalidate_with_etag)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true})
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # stale
									r3 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # cached and fresh
									x = r1.body
								}`,
			headers:          map[string][]string{"Cache-Control": {"max-age=0, public"}, "Etag": {"1234"}, "location": {"/test"}},
			response:         `{"x": 1}`,
			expectedReqCount: 2,
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t0 := time.Now().UTC()
			opts := setTime(t0)

			var requests []*http.Request
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r)
				headers := w.Header()

				for k, v := range tc.headers {
					headers[k] = v
				}

				headers.Set("Date", t0.Format(http.TimeFormat))

				etag := w.Header().Get("etag")

				if r.Header.Get("if-none-match") != "" {
					if r.Header.Get("if-none-match") == etag {
						headers["Cache-Control"] = []string{"max-age=290304000, public"}
					}
				}
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(tc.response))
				if err != nil {
					t.Fatal(err)
				}
			}))
			defer ts.Close()

			runTopDownTestCase(t, data, tc.note, []string{strings.ReplaceAll(tc.ruleTemplate, "%URL%", ts.URL)}, tc.response, opts)

			// Note: The runTopDownTestCase ends up evaluating twice (once with and once without partial
			// eval first), so expect 2x the total request count the test case specified.
			actualCount := len(requests) / 2
			if actualCount != tc.expectedReqCount {
				t.Fatalf("Expected to get %d requests, got %d", tc.expectedReqCount, actualCount)
			}
		})
	}
}

func TestInsertIntoHTTPSendIntraQueryCacheError(t *testing.T) {
	tests := []struct {
		note             string
		ruleTemplate     string
		headers          map[string][]string
		body             string
		response         string
		expectedReqCount int
	}{
		{
			note: "http.send GET (bad_date_header_value)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # fallback to normal cache
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # retrieved from normal cache
									r1 == r2
									x = r1.body
								}`,
			headers:          map[string][]string{"Cache-Control": {"max-age=0, public"}, "Date": {"Wed, 32 Dec 2115 07:28:00 GMT"}},
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
		{
			note: "http.send GET (bad_cache_control_header_value)",
			ruleTemplate: `p = x {
									r1 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # fallback to normal cache
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "cache": true}) # retrieved from normal cache
									r1 == r2
									x = r1.body
								}`,
			headers:          map[string][]string{"Cache-Control": {"max-age=\"foo\", public"}},
			response:         `{"x": 1}`,
			expectedReqCount: 1,
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var requests []*http.Request
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r)
				headers := w.Header()

				for k, v := range tc.headers {
					headers[k] = v
				}

				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(tc.response))
				if err != nil {
					t.Fatal(err)
				}
			}))
			defer ts.Close()

			runTopDownTestCase(t, data, tc.note, []string{strings.ReplaceAll(tc.ruleTemplate, "%URL%", ts.URL)}, tc.response)

			// Note: The runTopDownTestCase ends up evaluating twice (once with and once without partial
			// eval first), so expect 2x the total request count the test case specified.
			actualCount := len(requests) / 2
			if actualCount != tc.expectedReqCount {
				t.Fatalf("Expected to get %d requests, got %d", tc.expectedReqCount, actualCount)
			}
		})
	}
}

func TestGetCachingMode(t *testing.T) {
	tests := []struct {
		note      string
		input     ast.Object
		expected  cachingMode
		wantError bool
		err       error
	}{
		{
			note:      "default caching mode",
			input:     ast.MustParseTerm(`{}`).Value.(ast.Object),
			expected:  defaultCachingMode,
			wantError: false,
		},
		{
			note:      "serialized caching mode",
			input:     ast.MustParseTerm(`{"caching_mode": "serialized"}`).Value.(ast.Object),
			expected:  defaultCachingMode,
			wantError: false,
		},
		{
			note:      "deserialized caching mode",
			input:     ast.MustParseTerm(`{"caching_mode": "deserialized"}`).Value.(ast.Object),
			expected:  cachingModeDeserialized,
			wantError: false,
		},
		{
			note:      "invalid caching mode type",
			input:     ast.MustParseTerm(`{"caching_mode": 1}`).Value.(ast.Object),
			wantError: true,
			err:       fmt.Errorf("invalid value for \"caching_mode\" field"),
		},
		{
			note:      "invalid caching mode value",
			input:     ast.MustParseTerm(`{"caching_mode": "foo"}`).Value.(ast.Object),
			wantError: true,
			err:       fmt.Errorf("invalid value specified for \"caching_mode\" field: foo"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			actual, err := getCachingMode(tc.input)
			if tc.wantError {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}

				if actual != tc.expected {
					t.Fatalf("Expected caching mode %v but got %v", tc.expected, actual)
				}
			}
		})
	}
}

func TestGetResponseHeaderDateEmpty(t *testing.T) {
	_, err := getResponseHeaderDate(http.Header{"Date": {""}})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	expected := "no date header"
	if err.Error() != expected {
		t.Fatalf("Expected error message %v but got %v", expected, err.Error())
	}
}

func TestParseMaxAgeCacheDirective(t *testing.T) {
	tests := []struct {
		note      string
		input     map[string]string
		expected  deltaSeconds
		wantError bool
		err       error
	}{
		{
			note:      "max age not set",
			input:     nil,
			expected:  deltaSeconds(-1),
			wantError: false,
			err:       nil,
		},
		{
			note:      "max age out of range",
			input:     map[string]string{"max-age": "214748364888"},
			expected:  deltaSeconds(math.MaxInt32),
			wantError: false,
			err:       nil,
		},
		{
			note:      "max age greater than MaxInt32",
			input:     map[string]string{"max-age": "2147483648"},
			expected:  deltaSeconds(math.MaxInt32),
			wantError: false,
			err:       nil,
		},
		{
			note:      "max age less than MaxInt32",
			input:     map[string]string{"max-age": "21"},
			expected:  deltaSeconds(21),
			wantError: false,
			err:       nil,
		},
		{
			note:      "max age bad format",
			input:     map[string]string{"max-age": "21,21"},
			expected:  deltaSeconds(-1),
			wantError: true,
			err:       fmt.Errorf("strconv.ParseUint: parsing \"21,21\": invalid syntax"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			actual, err := parseMaxAgeCacheDirective(tc.input)
			if tc.wantError {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}

			if actual != tc.expected {
				t.Fatalf("Expected value for max-age %v but got %v", tc.expected, actual)
			}
		})
	}
}

func TestNewForceCacheParams(t *testing.T) {
	tests := []struct {
		note      string
		input     ast.Object
		expected  *forceCacheParams
		wantError bool
		err       error
	}{
		{
			note:      "non existent key",
			input:     ast.MustParseTerm(`{}`).Value.(ast.Object),
			expected:  nil,
			wantError: true,
			err:       fmt.Errorf("'force_cache' set but 'force_cache_duration_seconds' parameter is missing"),
		},
		{
			note:      "empty input",
			input:     ast.MustParseTerm(`{"force_cache_duration_seconds": ""}`).Value.(ast.Object),
			expected:  nil,
			wantError: true,
			err:       fmt.Errorf("strconv.ParseInt: parsing \"\\\"\\\"\": invalid syntax"),
		},
		{
			note:      "invalid input",
			input:     ast.MustParseTerm(`{"force_cache_duration_seconds": "foo"}`).Value.(ast.Object),
			expected:  nil,
			wantError: true,
			err:       fmt.Errorf("strconv.ParseInt: parsing \"\\\"foo\\\"\": invalid syntax"),
		},
		{
			note:      "valid input",
			input:     ast.MustParseTerm(`{"force_cache_duration_seconds": 300}`).Value.(ast.Object),
			expected:  &forceCacheParams{forceCacheDurationSeconds: int32(300)},
			wantError: false,
			err:       nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			actual, err := newForceCacheParams(tc.input)
			if tc.wantError {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}

				if actual.forceCacheDurationSeconds != tc.expected.forceCacheDurationSeconds {
					t.Fatalf("Expected force cache duration %v but got %v", tc.expected.forceCacheDurationSeconds, actual.forceCacheDurationSeconds)
				}
			}
		})
	}
}

func TestGetBoolValFromReqObj(t *testing.T) {
	validInput := ast.MustParseTerm(`{"cache": true}`)
	validInputObj := validInput.Value.(ast.Object)

	invalidInput := ast.MustParseTerm(`{"cache": "true"}`)
	invalidInputObj := invalidInput.Value.(ast.Object)

	tests := []struct {
		note      string
		input     ast.Object
		key       *ast.Term
		expected  bool
		wantError bool
		err       error
	}{
		{
			note:      "valid input",
			input:     validInputObj,
			key:       ast.StringTerm("cache"),
			expected:  true,
			wantError: false,
			err:       nil,
		},
		{
			note:      "invalid input",
			input:     invalidInputObj,
			key:       ast.StringTerm("cache"),
			expected:  false,
			wantError: true,
			err:       fmt.Errorf("invalid value for \"cache\" field"),
		},
		{
			note:      "non existent key",
			input:     validInputObj,
			key:       ast.StringTerm("foo"),
			expected:  false,
			wantError: false,
			err:       nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			actual, err := getBoolValFromReqObj(tc.input, tc.key)
			if tc.wantError {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}

			if actual != tc.expected {
				t.Fatalf("Expected value for key %v is %v but got %v", tc.key, tc.expected, actual)
			}
		})
	}
}

func TestInterQueryCheckCacheError(t *testing.T) {
	input := ast.MustParseTerm(`{"force_cache": true}`)
	inputObj := input.Value.(ast.Object)

	_, err := newHTTPRequestExecutor(BuiltinContext{Context: context.Background()}, inputObj)
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	errMsg := "eval_builtin_error: http.send: 'force_cache' set but 'force_cache_duration_seconds' parameter is missing"
	if err.Error() != errMsg {
		t.Fatalf("Expected error message %v but got %v", errMsg, err.Error())
	}
}

func TestNewInterQueryCacheValue(t *testing.T) {
	date := "Wed, 31 Dec 2115 07:28:00 GMT"
	maxAge := 290304000

	headers := make(http.Header)
	headers.Set("test-header", "test-value")
	headers.Set("Cache-Control", fmt.Sprintf("max-age=%d, public", maxAge))
	headers.Set("Date", date)

	// test data
	b := []byte(`[{"ID": "1", "Firstname": "John"}]`)

	response := &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Header:     headers,
		Request:    &http.Request{Method: "Get"},
		Body:       io.NopCloser(bytes.NewBuffer(b)),
	}

	result, err := newInterQueryCacheValue(BuiltinContext{}, response, b, &forceCacheParams{})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	dateTime, _ := http.ParseTime(date)

	cvd := interQueryCacheData{
		RespBody:   b,
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Headers:    headers,
		ExpiresAt:  dateTime.Add(time.Duration(maxAge) * time.Second),
	}

	cvdBytes, err := json.Marshal(cvd)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	expectedResult := &interQueryCacheValue{Data: cvdBytes}

	if !reflect.DeepEqual(result, expectedResult) {
		t.Fatalf("Expected result %v but got %v", expectedResult, result)
	}

	if int64(len(cvdBytes)) != result.SizeInBytes() {
		t.Fatalf("Expected cache item size %v but got %v", len(cvdBytes), result.SizeInBytes())
	}
}

func getTestServer() (baseURL string, teardownFn func()) {
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)

	mux.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/test", http.StatusMovedPermanently)
	})

	return ts.URL, ts.Close
}

func getTLSTestServer() (ts *httptest.Server) {
	mux := http.NewServeMux()
	ts = httptest.NewUnstartedServer(mux)

	mux.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/cert", func(w http.ResponseWriter, req *http.Request) {
		clientCert := req.TLS.PeerCertificates[0]
		commonName := clientCert.Issuer.CommonName
		certificate := struct{ CommonName string }{commonName}
		js, err := json.Marshal(certificate)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(js)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return
}

func TestHTTPSClient(t *testing.T) {
	const (
		localClientCertFile  = "testdata/client-cert.pem"
		localClientCert2File = "testdata/client-cert-2.pem"
		localClientKeyFile   = "testdata/client-key.pem"
		localCaFile          = "testdata/ca.pem"
		localServerCertFile  = "testdata/server-cert.pem"
		localServerKeyFile   = "testdata/server-key.pem"
	)

	caCertPEM, err := os.ReadFile(localCaFile)
	if err != nil {
		t.Fatal(err)
	}
	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caCertPEM); !ok {
		t.Fatal("failed to parse CA cert")
	}

	cert, err := tls.LoadX509KeyPair(localServerCertFile, localServerKeyFile)
	if err != nil {
		t.Fatal(err)
	}

	// Set up Environment
	clientCert, err := readCertFromFile(localClientCertFile)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLIENT_CERT_ENV", string(clientCert))

	clientKey, err := readKeyFromFile(localClientKeyFile)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLIENT_KEY_ENV", string(clientKey))
	t.Setenv("CLIENT_CA_ENV", string(caCertPEM))

	// Replicating some of what happens in the server's HTTPS listener
	s := getTLSTestServer()
	s.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
	}
	s.StartTLS()
	defer s.Close()

	t.Run("Server reflects Certificate CommonName", func(t *testing.T) {
		// expected result
		bodyMap := map[string]string{"CommonName": "my-ca"}
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"raw_body":    "{\"CommonName\":\"my-ca\"}",
		}
		expectedResult["body"] = bodyMap
		expectedResult["headers"] = map[string]interface{}{
			"content-length": []interface{}{"22"},
			"content-type":   []interface{}{"application/json"},
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rules := append(
			httpSendHelperRules,
			fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_file": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s"}, resp); x := clean_headers(resp) }`, s.URL+"/cert", localCaFile, localClientCertFile, localClientKeyFile),
		)
		// run the test
		runTopDownTestCase(t, data, "http.send", rules, resultObj.String())
	})

	t.Run("HTTPS Get with Inline Cert", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
			"headers": map[string]interface{}{
				"content-length": []interface{}{"0"},
			},
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		ca, err := os.ReadFile(localCaFile)
		if err != nil {
			t.Fatal(err)
		}

		cert, err := os.ReadFile(localClientCertFile)
		if err != nil {
			t.Fatal(err)
		}

		key, err := os.ReadFile(localClientKeyFile)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rules := append(
			httpSendHelperRules,
			fmt.Sprintf(
				"p = x { http.send({`method`: `get`, `url`: `%s`, `tls_ca_cert`: `%s`, `tls_client_cert`: `%s`, `tls_client_key`: `%s`}, resp); x := clean_headers(resp) }",
				s.URL, ca, cert, key),
		)

		// run the test
		runTopDownTestCase(t, data, "http.send", rules, resultObj.String())
	})

	t.Run("HTTPS Get with File Cert", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
			"headers": map[string]interface{}{
				"content-length": []interface{}{"0"},
			},
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rules := append(
			httpSendHelperRules,
			fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_file": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s"}, resp); x := clean_headers(resp) }`, s.URL, localCaFile, localClientCertFile, localClientKeyFile),
		)

		// run the test
		runTopDownTestCase(t, data, "http.send", rules, resultObj.String())
	})

	t.Run("HTTPS Get with Env Cert", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
			"headers": map[string]interface{}{
				"content-length": []interface{}{"0"},
			},
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rules := append(
			httpSendHelperRules,
			fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_env_variable": "CLIENT_CA_ENV", "tls_client_cert_env_variable": "CLIENT_CERT_ENV", "tls_client_key_env_variable": "CLIENT_KEY_ENV"}, resp); x := clean_headers(resp) }`, s.URL),
		)

		// run the test
		runTopDownTestCase(t, data, "http.send", rules, resultObj.String())
	})

	t.Run("HTTPS Get with Env and File Cert", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
			"headers": map[string]interface{}{
				"content-length": []interface{}{"0"},
			},
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rules := append(
			httpSendHelperRules,
			fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_env_variable": "CLIENT_CA_ENV", "tls_client_cert_env_variable": "CLIENT_CERT_ENV", "tls_client_key_env_variable": "CLIENT_KEY_ENV", "tls_ca_cert_file": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s"}, resp); x := clean_headers(resp) }`, s.URL, localCaFile, localClientCertFile, localClientKeyFile),
		)

		// run the test
		runTopDownTestCase(t, data, "http.send", rules, resultObj.String())
	})

	t.Run("HTTPS Get with System Certs, Env and File Cert", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
			"headers": map[string]interface{}{
				"content-length": []interface{}{"0"},
			},
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rules := append(
			httpSendHelperRules,
			fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s", "tls_use_system_certs": true, "tls_ca_cert_env_variable": "CLIENT_CA_ENV", "tls_client_cert_env_variable": "CLIENT_CERT_ENV", "tls_client_key_env_variable": "CLIENT_KEY_ENV", "tls_ca_cert_file": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s"}, resp); x := clean_headers(resp) }`, s.URL, localCaFile, localClientCertFile, localClientKeyFile),
		)

		// run the test
		runTopDownTestCase(t, data, "http.send", rules, resultObj.String())
	})

	t.Run("Negative Test: No Root Ca", func(t *testing.T) {
		expectedResult := &Error{Code: BuiltinErr, Message: fixupDarwinGo118("x509: certificate signed by unknown authority", `“my-server” certificate is not standards compliant`), Location: nil}
		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s"}, x) }`, s.URL, localClientCertFile, localClientKeyFile)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, expectedResult)
	})

	t.Run("Negative Test: Wrong Cert/Key Pair", func(t *testing.T) {
		expectedResult := &Error{Code: BuiltinErr, Message: "tls: private key does not match public key", Location: nil}
		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_file": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s"}, x) }`, s.URL, localCaFile, localClientCert2File, localClientKeyFile)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, expectedResult)
	})

	t.Run("Negative Test: System Certs do not include local rootCA", func(t *testing.T) {
		expectedResult := &Error{Code: BuiltinErr, Message: fixupDarwinGo118("x509: certificate signed by unknown authority", `“my-server” certificate is not standards compliant`), Location: nil}
		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s", "tls_use_system_certs": true}, x) }`, s.URL, localClientCertFile, localClientKeyFile)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, expectedResult)
	})

	// Expect that setting the Host header causes TLS server validation
	// to fail because the server sends a different certificate.
	t.Run("Client Host is also ServerName", func(t *testing.T) {
		url := s.URL + "/cert"
		hostname := "notpresent"

		expected := &Error{Code: BuiltinErr, Message: fmt.Sprintf(
			"x509: certificate is valid for localhost, not %s", hostname)}

		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_file": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s", "headers": {"host": "%s"}}, x) }`, url, localCaFile, localClientCertFile, localClientKeyFile, hostname)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, expected)
	})

	// Expect that setting `tls_server_name` causes TLS server validation
	// to fail because the server sends a different certificate.
	t.Run("Client can set ServerName", func(t *testing.T) {
		url := s.URL + "/cert"
		hostname := "notpresent"

		expected := &Error{Code: BuiltinErr, Message: fmt.Sprintf(
			"x509: certificate is valid for localhost, not %s", hostname)}

		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_file": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s", "tls_server_name": "%s"}, x) }`, url, localCaFile, localClientCertFile, localClientKeyFile, hostname)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, expected)
	})
}

func TestHTTPSNoClientCerts(t *testing.T) {
	const (
		localCaFile         = "testdata/ca.pem"
		localServerCertFile = "testdata/server-cert.pem"
		localServerKeyFile  = "testdata/server-key.pem"
	)

	caCertPEM, err := os.ReadFile(localCaFile)
	if err != nil {
		t.Fatal(err)
	}
	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caCertPEM); !ok {
		t.Fatal("failed to parse CA cert")
	}

	cert, err := tls.LoadX509KeyPair(localServerCertFile, localServerKeyFile)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("CLIENT_CA_ENV", string(caCertPEM))

	// Replicating some of what happens in the server's HTTPS listener
	s := getTLSTestServer()
	s.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caPool,
	}
	s.StartTLS()
	defer s.Close()

	t.Run("HTTPS Get with Broken CA Cert w/ File", func(t *testing.T) {
		// `tls_ca_cert_file` is valid, but `tls_ca_cert` is not, so we
		// expect and error building the TLS context.
		expectedResult := &Error{Code: BuiltinErr, Message: "could not append certificates"}

		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			"p = x { http.send({`method`: `get`, `url`: `%s`, `tls_ca_cert`: `%s`, `tls_ca_cert_file`: `%s`}, x) }", s.URL, "xxx", localCaFile)}

		runTopDownTestCase(t, data, "http.send", rule, expectedResult)
	})

	t.Run("HTTPS Get with Broken CA Cert w/ Env", func(t *testing.T) {
		// `tls_ca_cert_env_variable` is valid, but `tls_ca_cert` is not, so we
		// expect and error building the TLS context.
		expectedResult := &Error{Code: BuiltinErr, Message: "could not append certificates"}

		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			"p = x { http.send({`method`: `get`, `url`: `%s`, `tls_ca_cert`: `%s`, `tls_ca_cert_env_variable`: `CLIENT_CA_ENV`}, x) }", s.URL, "xxx")}

		runTopDownTestCase(t, data, "http.send", rule, expectedResult)
	})

	t.Run("HTTPS Get with Inline CA Cert", func(t *testing.T) {
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
			"headers": map[string]interface{}{
				"content-length": []interface{}{"0"},
			},
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		ca, err := os.ReadFile(localCaFile)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rules := append(
			httpSendHelperRules,
			fmt.Sprintf("p = x { http.send({`method`: `get`, `url`: `%s`, `tls_ca_cert`: `%s`}, resp); x := clean_headers(resp) }", s.URL, ca),
		)

		runTopDownTestCase(t, data, "http.send", rules, resultObj.String())
	})

	t.Run("HTTPS Get with CA Cert File", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
			"headers": map[string]interface{}{
				"content-length": []interface{}{"0"},
			},
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rules := append(
			httpSendHelperRules,
			fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_file": "%s"}, resp); x := clean_headers(resp) }`, s.URL, localCaFile),
		)

		// run the test
		runTopDownTestCase(t, data, "http.send", rules, resultObj.String())
	})

	t.Run("HTTPS Get with CA Cert ENV", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
			"headers": map[string]interface{}{
				"content-length": []interface{}{"0"},
			},
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rules := append(
			httpSendHelperRules,
			fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_env_variable": "CLIENT_CA_ENV"}, resp); x := clean_headers(resp) }`, s.URL),
		)

		// run the test
		runTopDownTestCase(t, data, "http.send", rules, resultObj.String())
	})

	t.Run("HTTPS Get with System CA Cert Pool", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
			"headers": map[string]interface{}{
				"content-length": []interface{}{"0"},
			},
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rules := append(
			httpSendHelperRules,
			fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_env_variable": "CLIENT_CA_ENV"}, resp); x := clean_headers(resp) }`, s.URL),
		)

		// run the test
		runTopDownTestCase(t, data, "http.send", rules, resultObj.String())
	})

	t.Run("HTTPS Get with System Certs, Env and File Cert", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
			"headers": map[string]interface{}{
				"content-length": []interface{}{"0"},
			},
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rules := append(
			httpSendHelperRules,
			fmt.Sprintf(`p = x { http.send({"method": "get", "url": "%s", "tls_use_system_certs": true, "tls_ca_cert_env_variable": "CLIENT_CA_ENV", "tls_ca_cert_file": "%s"}, resp); x := clean_headers(resp) }`, s.URL, localCaFile),
		)

		// run the test
		runTopDownTestCase(t, data, "http.send", rules, resultObj.String())
	})

	t.Run("Negative Test: System Certs do not include local rootCA", func(t *testing.T) {
		expectedResult := &Error{Code: BuiltinErr, Message: fixupDarwinGo118("x509: certificate signed by unknown authority", `“my-server” certificate is not standards compliant`), Location: nil}
		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_use_system_certs": true}, x) }`, s.URL)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, expectedResult)
	})
}

// Note(philipc): In Go 1.18, the crypto/x509 package deprecated the
// (*CertPool).Subjects() function. The precise reasoning for why this was
// done traces back to:
//
//	https://github.com/golang/go/issues/46287
//
// For now, most projects seem to be working around this deprecation by
// changing how they verify certificates, and when CertPools are needed in
// tests, some larger projects have just slapped linter ignores on the
// offending callsites. Since we only use (*CertPool).Subjects() here for
// tests, we've gone with using linter ignores for now.
func TestCertSelectionLogic(t *testing.T) {
	const (
		localCaFile = "testdata/ca.pem"
	)

	// Set up Environment
	caCertPEM, err := os.ReadFile(localCaFile)
	if err != nil {
		t.Fatal(err)
	}

	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caCertPEM); !ok {
		t.Fatal("failed to parse CA cert")
	}

	ca, err := os.ReadFile(localCaFile)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("CLIENT_CA_ENV", string(caCertPEM))

	getClientTLSConfig := func(obj ast.Object) *tls.Config {
		_, client, err := createHTTPRequest(BuiltinContext{Context: context.Background()}, obj)
		if err != nil {
			t.Fatalf("Unexpected error creating HTTP request %v", err)
		}
		if client.Transport == nil {
			return nil
		}
		return client.Transport.(*http.Transport).TLSClientConfig
	}

	systemCertsPool, err := x509.SystemCertPool()
	if err != nil {
		t.Fatalf("Unexpected error reading system certs %v", err)
	}

	tempSystemCertsPool, err := x509.SystemCertPool()
	if err != nil {
		t.Fatalf("Unexpected error reading system certs %v", err)
	}
	systemCertsAndCaPool, err := addCACertsFromBytes(tempSystemCertsPool, ca)
	if err != nil {
		t.Fatalf("Unexpected error merging system certs and ca certs %v", err)
	}

	tests := []struct {
		note     string
		input    map[*ast.Term]*ast.Term
		expected [][]byte
		msg      string
	}{
		{
			note:     "tls_use_system_certs set to true",
			input:    map[*ast.Term]*ast.Term{ast.StringTerm("tls_use_system_certs"): ast.BooleanTerm(true)},
			expected: systemCertsPool.Subjects(), // nolint:staticcheck // ignoring the deprecated (*CertPool).Subjects() call here because it's in a test.
			msg:      "Expected TLS config to use system certs",
		},
		{
			note:     "tls_use_system_certs set to nil",
			input:    map[*ast.Term]*ast.Term{ast.StringTerm("tls_use_system_certs"): ast.BooleanTerm(false)},
			expected: nil,
			msg:      "Expected no TLS config",
		},
		{
			note:     "no CAs specified",
			input:    nil,
			expected: systemCertsPool.Subjects(), // nolint:staticcheck // ignoring the deprecated (*CertPool).Subjects() call here because it's in a test.
			msg:      "Expected TLS config to use system certs",
		},
		{
			note:     "CA cert provided directly",
			input:    map[*ast.Term]*ast.Term{ast.StringTerm("tls_ca_cert"): ast.StringTerm(string(ca))},
			expected: caPool.Subjects(), // nolint:staticcheck // ignoring the deprecated (*CertPool).Subjects() call here because it's in a test.
			msg:      "Expected TLS config to use provided CA certs",
		},
		{
			note:     "CA cert file path provided",
			input:    map[*ast.Term]*ast.Term{ast.StringTerm("tls_ca_cert_file"): ast.StringTerm(localCaFile)},
			expected: caPool.Subjects(), // nolint:staticcheck // ignoring the deprecated (*CertPool).Subjects() call here because it's in a test.
			msg:      "Expected TLS config to use provided CA certs in file",
		},
		{
			note:     "CA cert provided in env variable",
			input:    map[*ast.Term]*ast.Term{ast.StringTerm("tls_ca_cert_env_variable"): ast.StringTerm("CLIENT_CA_ENV")},
			expected: caPool.Subjects(), // nolint:staticcheck // ignoring the deprecated (*CertPool).Subjects() call here because it's in a test.
			msg:      "Expected TLS config to use provided CA certs in env variable",
		},
		{
			note: "CA cert provided directly and tls_use_system_certs parameter set to false",
			input: map[*ast.Term]*ast.Term{
				ast.StringTerm("tls_ca_cert"):          ast.StringTerm(string(ca)),
				ast.StringTerm("tls_use_system_certs"): ast.BooleanTerm(false),
			},
			expected: caPool.Subjects(), // nolint:staticcheck // ignoring the deprecated (*CertPool).Subjects() call here because it's in a test.
			msg:      "Expected TLS config to use provided CA certs only",
		},
		{
			note: "CA cert provided directly and tls_use_system_certs parameter set to true",
			input: map[*ast.Term]*ast.Term{
				ast.StringTerm("tls_ca_cert"):          ast.StringTerm(string(ca)),
				ast.StringTerm("tls_use_system_certs"): ast.BooleanTerm(true),
			},
			expected: systemCertsAndCaPool.Subjects(), // nolint:staticcheck // ignoring the deprecated (*CertPool).Subjects() call here because it's in a test.
			msg:      "Expected TLS config to use provided CA certs and system certs",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			obj := ast.NewObject()
			for key, value := range tc.input {
				obj.Insert(key, value)
			}
			tlsConfig := getClientTLSConfig(obj)
			if tc.expected == nil {
				if tlsConfig != nil {
					t.Fatalf(tc.msg)
				}
			} else {
				// nolint:staticcheck // ignoring the deprecated (*CertPool).Subjects() call here because it's in a test.
				if !reflect.DeepEqual(tlsConfig.RootCAs.Subjects(), tc.expected) {
					t.Fatal(tc.msg)
				}
			}
		})
	}
}

func TestHTTPSendCacheDefaultStatusCodesIntraQueryCache(t *testing.T) {

	// run test server
	var requests []*http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		if len(requests)%2 == 0 {
			headers := w.Header()
			headers["Cache-Control"] = []string{"max-age=290304000, public"}
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))

	defer ts.Close()

	t.Run("non-cacheable status code: intra-query cache", func(t *testing.T) {
		base := fmt.Sprintf(`http.send({"method": "get", "url": %q, "cache": true})`, ts.URL)
		query := fmt.Sprintf("%v;%v;%v", base, base, base)

		q := NewQuery(ast.MustParseBody(query))

		// Execute three http.send calls within a query.
		// Since the server returns a http.StatusInternalServerError on the first request, this should NOT be cached as
		// http.StatusInternalServerError is not a cacheable status code. The second request should result in OPA reaching
		// out to the server again and getting a http.StatusOK response status code.
		// The third request should now be served from the cache.

		_, err := q.Run(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		expectedReqCount := 2
		if len(requests) != expectedReqCount {
			t.Fatalf("Expected to get %d requests, got %d", expectedReqCount, len(requests))
		}
	})
}

func TestHTTPSendCacheDefaultStatusCodesInterQueryCache(t *testing.T) {

	// run test server
	var requests []*http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		if len(requests)%2 == 0 {
			headers := w.Header()
			headers["Cache-Control"] = []string{"max-age=290304000, public"}
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))

	defer ts.Close()

	t.Run("non-cacheable status code: inter-query cache", func(t *testing.T) {

		// add an inter-query cache
		config, _ := iCache.ParseCachingConfig(nil)
		interQueryCache := iCache.NewInterQueryCache(config)

		m := metrics.New()

		q := NewQuery(ast.MustParseBody(fmt.Sprintf(`http.send({"method": "get", "url": %q, "cache": true})`, ts.URL))).
			WithMetrics(m).WithInterQueryBuiltinCache(interQueryCache)

		// Execute three queries.
		// Since the server returns a http.StatusInternalServerError on the first request, this should NOT be cached as
		// http.StatusInternalServerError is not a cacheable status code. The second request should result in OPA reaching
		// out to the server again and getting a http.StatusOK response status code.
		// The third request should now be served from the cache.

		_, err := q.Run(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		_, err = q.Run(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		_, err = q.Run(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		expectedReqCount := 2
		if len(requests) != expectedReqCount {
			t.Fatalf("Expected to get %d requests, got %d", expectedReqCount, len(requests))
		}

		// verify http.send inter-query cache hit metric is incremented due to the third request.
		if exp, act := uint64(1), m.Counter(httpSendInterQueryCacheHits).Value(); exp != act {
			t.Fatalf("expected %d cache hits, got %d", exp, act)
		}
	})
}

func TestHTTPSendMetrics(t *testing.T) {
	// run test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	defer ts.Close()

	t.Run("latency", func(t *testing.T) {
		// Execute query and verify http.send latency shows up in metrics registry.
		m := metrics.New()
		q := NewQuery(ast.MustParseBody(fmt.Sprintf(`http.send({"method": "get", "url": %q})`, ts.URL))).WithMetrics(m)
		_, err := q.Run(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if m.Timer(httpSendLatencyMetricKey).Int64() == 0 {
			t.Fatal("expected non-zero value for http.send latency metric")
		}
	})

	t.Run("cache hits", func(t *testing.T) {
		// add an inter-query cache
		config, _ := iCache.ParseCachingConfig(nil)
		interQueryCache := iCache.NewInterQueryCache(config)

		// Execute query twice and verify http.send inter-query cache hit metric is incremented.
		m := metrics.New()
		q := NewQuery(ast.MustParseBody(fmt.Sprintf(`http.send({"method": "get", "url": %q, "cache": true})`, ts.URL))).
			WithInterQueryBuiltinCache(interQueryCache).
			WithMetrics(m)
		_, err := q.Run(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		// cache hit
		_, err = q.Run(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if exp, act := uint64(1), m.Counter(httpSendInterQueryCacheHits).Value(); exp != act {
			t.Fatalf("expected %d cache hits, got %d", exp, act)
		}
	})
}

func TestInitDefaults(t *testing.T) {
	t.Setenv("HTTP_SEND_TIMEOUT", "300mss")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected function to panic")
		}
	}()
	initDefaults()
}

var httpSendHelperRules = []string{
	`clean_headers(resp) = cleaned {
		cleaned = json.remove(resp, ["headers/date"])
	}`,
	`remove_headers(resp) = no_headers {
		no_headers = object.remove(resp, ["headers"])
	}`,
}

func TestSocketHTTPGetRequest(t *testing.T) {
	var people []Person

	// test data
	people = append(people, Person{ID: "1", Firstname: "John"})

	// Create a local socket
	tmpF, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}

	socketPath := tmpF.Name()
	tmpF.Close()
	_ = os.Remove(socketPath)

	socket, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}

	rs := http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headers := w.Header()
			headers["test-header"] = []string{"test-value"}
			headers["echo-query-string"] = []string{r.URL.RawQuery}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(people)
		}),
	}

	go func() {
		_ = rs.Serve(socket)
	}()
	defer rs.Close()

	path := fmt.Sprintf("socket=%s", url.PathEscape(socketPath))
	rawURL := fmt.Sprintf("unix://localhost/end/point?%s&param1=value1&param2=value2", path) // Send a request to the server over the socket

	// expected result
	expectedResult := make(map[string]interface{})
	expectedResult["status"] = "200 OK"
	expectedResult["status_code"] = http.StatusOK

	var body []interface{}
	bodyMap := map[string]string{"id": "1", "firstname": "John"}
	body = append(body, bodyMap)
	expectedResult["body"] = body
	expectedResult["raw_body"] = "[{\"id\":\"1\",\"firstname\":\"John\"}]\n"
	expectedResult["headers"] = map[string]interface{}{
		"content-length":    []interface{}{"32"},
		"content-type":      []interface{}{"text/plain; charset=utf-8"},
		"test-header":       []interface{}{"test-value"},
		"echo-query-string": []interface{}{"param1=value1&param2=value2"},
	}

	resultObj := ast.MustInterfaceToValue(expectedResult)

	// run the test
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"http.send", []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "force_json_decode": true}, resp); x := clean_headers(resp) }`, rawURL)}, resultObj.String()},
		{"http.send skip verify no HTTPS", []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "force_json_decode": true, "tls_insecure_skip_verify": true}, resp); x := clean_headers(resp) }`, rawURL)}, resultObj.String()},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, append(tc.rules, httpSendHelperRules...), tc.expected)
	}
}

type tracemock struct {
	called int
}

func (m *tracemock) NewTransport(rt http.RoundTripper, _ tracing.Options) http.RoundTripper {
	m.called++
	return rt
}

func (*tracemock) NewHandler(http.Handler, string, tracing.Options) http.Handler {
	panic("unreachable")
}

func TestDistributedTracingEnabled(t *testing.T) {
	mock := tracemock{}
	tracing.RegisterHTTPTracing(&mock)

	builtinContext := BuiltinContext{
		Context:                context.Background(),
		DistributedTracingOpts: tracing.NewOptions(true), // any option means it's enabled
	}

	_, client, err := createHTTPRequest(builtinContext, ast.NewObject())
	if err != nil {
		t.Fatalf("Unexpected error creating HTTP request %v", err)
	}
	if client.Transport == nil {
		t.Fatal("No Transport defined")
	}

	if exp, act := 1, mock.called; exp != act {
		t.Errorf("calls to NewTransported: expected %d, got %d", exp, act)
	}
}

func TestDistributedTracingDisabled(t *testing.T) {
	mock := tracemock{}
	tracing.RegisterHTTPTracing(&mock)

	builtinContext := BuiltinContext{
		Context: context.Background(),
	}

	_, client, err := createHTTPRequest(builtinContext, ast.NewObject())
	if err != nil {
		t.Fatalf("Unexpected error creating HTTP request %v", err)
	}
	if client.Transport == nil {
		t.Fatal("No Transport defined")
	}

	if exp, act := 0, mock.called; exp != act {
		t.Errorf("calls to NewTransported: expected %d, got %d", exp, act)
	}
}

func TestHTTPGetRequestAllowNet(t *testing.T) {
	// test data
	body := map[string]bool{"ok": true}

	// test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(body)
	}))

	defer ts.Close()

	// host
	serverURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	serverHost := strings.Split(serverURL.Host, ":")[0]

	// expected result
	expectedResult := make(map[string]interface{})
	expectedResult["status"] = "200 OK"
	expectedResult["status_code"] = http.StatusOK

	expectedResult["body"] = body
	expectedResult["raw_body"] = "{\"ok\":true}\n"

	resultObj := ast.MustInterfaceToValue(expectedResult)

	expectedError := &Error{Code: "eval_builtin_error", Message: fmt.Sprintf("http.send: unallowed host: %s", serverHost)}

	rules := []string{fmt.Sprintf(
		`p = x { http.send({"method": "get", "url": "%s", "force_json_decode": true}, resp); x := remove_headers(resp) }`, ts.URL)}

	// run the test
	tests := []struct {
		note     string
		rules    []string
		options  func(*Query) *Query
		expected interface{}
	}{
		{
			"http.send allow_net nil",
			rules,

			setAllowNet(nil),
			resultObj.String(),
		},
		{
			"http.send allow_net match",
			rules,
			setAllowNet([]string{serverHost}),
			resultObj.String(),
		},
		{
			"http.send allow_net match + additional host",
			rules,
			setAllowNet([]string{serverHost, "example.com"}),
			resultObj.String(),
		},
		{
			"http.send allow_net empty",
			rules,
			setAllowNet([]string{}),
			expectedError,
		},
		{
			"http.send allow_net no match",
			rules,
			setAllowNet([]string{"example.com"}),
			expectedError,
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, append(tc.rules, httpSendHelperRules...), tc.expected, tc.options)
	}
}
