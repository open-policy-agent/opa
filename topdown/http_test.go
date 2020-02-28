// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/internal/version"
	"github.com/open-policy-agent/opa/topdown/builtins"

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
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(people)
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

	resultObj, err := ast.InterfaceToValue(expectedResult)
	if err != nil {
		panic(err)
	}

	// run the test
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"http.send", []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "force_json_decode": true}, x) }`, ts.URL)}, resultObj.String()},
		{"http.send", []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "force_json_decode": true, "tls_insecure_skip_verify": true}, x) }`, ts.URL)}, resultObj.String()},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
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
		json.NewEncoder(w).Encode(people)
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

	resultObj, err := ast.InterfaceToValue(expectedResult)
	if err != nil {
		panic(err)
	}

	// run the test
	tests := []struct {
		note          string
		rules         []string
		expected      interface{}
		expectedError error
	}{
		{note: "http.send", rules: []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "force_json_decode": true}, x) }`, ts.URL)}, expected: &Error{Message: "x509: certificate signed by unknown authority"}},
		{note: "http.send", rules: []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "force_json_decode": true, "tls_insecure_skip_verify": true}, x) }`, ts.URL)}, expected: resultObj.String()},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestHTTPEnableJSONDecode(t *testing.T) {

	// test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := "*Hello World®"
		fmt.Fprint(w, body)
	}))

	defer ts.Close()

	// expected result
	expectedResult := make(map[string]interface{})
	expectedResult["status"] = "200 OK"
	expectedResult["status_code"] = http.StatusOK
	expectedResult["body"] = nil
	expectedResult["raw_body"] = "*Hello World®"

	resultObj, err := ast.InterfaceToValue(expectedResult)
	if err != nil {
		panic(err)
	}

	// run the test
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"http.send", []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "force_json_decode": true}, x) }`, ts.URL)}, resultObj.String()},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
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
	json.NewEncoder(w).Encode(headers)
	return
}

// TestHTTPCustomHeaders adds custom headers to request
func TestHTTPCustomHeaders(t *testing.T) {

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
			`p = x { http.send({"method": "get", "url": "%s", "headers": {"X-Foo": "ISO-8859-1,utf-8;q=0.7,*;q=0.7", "X-Opa": "server"}}, x) }`, ts.URL)}, s},
		{"http.send custom UA", []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "headers": {"User-Agent": "AuthZPolicy/0.0.1", "X-Opa": "server"}}, x) }`, ts.URL)}, s2},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

// TestHTTPPostRequest adds a new person
func TestHTTPPostRequest(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		contentType := r.Header.Get("Content-Type")

		bs, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		w.Write(bs)
	}))

	defer ts.Close()

	tests := []struct {
		note     string
		params   string
		expected interface{}
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
				"raw_body": "{\"firstname\":\"Joe\",\"id\":\"2\"}"
			}`,
		},
		{
			note: "raw_body",
			params: `{
				"method": "post",
				"headers": {"Content-Type": "application/x-www-form-encoded"},
				"raw_body": "username=foobar&password=baz"
			}`,
			expected: `{
				"status": "200 OK",
				"status_code": 200,
				"body": null,
				"raw_body": "username=foobar&password=baz"
			}`,
		},
		{
			note: "raw_body overrides body",
			params: `{
				"method": "post",
				"headers": {"Content-Type": "application/x-www-form-encoded"},
				"body": {"foo": 1},
				"raw_body": "username=foobar&password=baz"
			}`,
			expected: `{
				"status": "200 OK",
				"status_code": 200,
				"body": null,
				"raw_body": "username=foobar&password=baz"
			}`,
		},
		{
			note: "raw_body bad type",
			params: `{
				"method": "post",
				"headers": {"Content-Type": "application/x-www-form-encoded"},
				"raw_body": {"bar": "bar"}
			}`,
			expected: &Error{Code: BuiltinErr, Message: "raw_body must be a string"},
		},
	}

	data := map[string]interface{}{}

	for _, tc := range tests {

		// Automatically set the URL because it's generated when the test server
		// is started. If needed, the test cases can override in the future.
		term := ast.MustParseTerm(tc.params)
		term.Value.(ast.Object).Insert(ast.StringTerm("url"), ast.StringTerm(ts.URL))

		rules := []string{
			fmt.Sprintf(`p = x { http.send(%s, x) }`, term),
		}

		runTopDownTestCase(t, data, tc.note, rules, tc.expected)
	}
}

// TestHTTDeleteRequest deletes a person
func TestHTTDeleteRequest(t *testing.T) {

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
		json.NewEncoder(w).Encode(people)
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

	resultObj, err := ast.InterfaceToValue(expectedResult)
	if err != nil {
		panic(err)
	}

	// delete a new person
	personToDelete := Person{ID: "2", Firstname: "Joe"}
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(personToDelete)

	// run the test
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"http.send", []string{fmt.Sprintf(
			`p = x { http.send({"method": "delete", "url": "%s", "body": %s}, x) }`, ts.URL, b)}, resultObj.String()},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
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

func TestHTTPSendTimeout(t *testing.T) {

	// Each test can tweak the response delay, default is 0 with no delay
	var responseDelay time.Duration

	tsMtx := sync.Mutex{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tsMtx.Lock()
		defer tsMtx.Unlock()
		time.Sleep(responseDelay)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`hello`))
	}))
	// Note: We don't Close() the test server as it will block waiting for the
	// timed out clients connections to shut down gracefully (they wont).
	// We don't need to clean it up nicely for the unit test.

	tests := []struct {
		note           string
		rule           string
		input          string
		defaultTimeout time.Duration
		evalTimeout    time.Duration
		serverDelay    time.Duration
		expected       interface{}
	}{
		{
			note:     "no timeout",
			rule:     `p = x { http.send({"method": "get", "url": "%URL%" }, x) }`,
			expected: `{"body": null, "raw_body": "hello", "status": "200 OK", "status_code": 200}`,
		},
		{
			note:           "default timeout",
			rule:           `p = x { http.send({"method": "get", "url": "%URL%" }, x) }`,
			evalTimeout:    1 * time.Minute,
			serverDelay:    5 * time.Second,
			defaultTimeout: 500 * time.Millisecond,
			expected:       &Error{Code: BuiltinErr, Message: "http.send: Get %URL%: request timed out"},
		},
		{
			note:           "eval timeout",
			rule:           `p = x { http.send({"method": "get", "url": "%URL%" }, x) }`,
			evalTimeout:    500 * time.Millisecond,
			serverDelay:    5 * time.Second,
			defaultTimeout: 1 * time.Minute,
			expected:       &Error{Code: BuiltinErr, Message: "http.send: Get %URL%: context deadline exceeded"},
		},
		{
			note:           "param timeout less than default",
			rule:           `p = x { http.send({"method": "get", "url": "%URL%", "timeout": "500ms"}, x) }`,
			evalTimeout:    1 * time.Minute,
			serverDelay:    5 * time.Second,
			defaultTimeout: 1 * time.Minute,
			expected:       &Error{Code: BuiltinErr, Message: "http.send: Get %URL%: request timed out"},
		},
		{
			note:           "param timeout greater than default",
			rule:           `p = x { http.send({"method": "get", "url": "%URL%", "timeout": "500ms"}, x) }`,
			evalTimeout:    1 * time.Minute,
			serverDelay:    5 * time.Second,
			defaultTimeout: 1 * time.Millisecond,
			expected:       &Error{Code: BuiltinErr, Message: "http.send: Get %URL%: request timed out"},
		},
		{
			note:           "eval timeout less than param",
			rule:           `p = x { http.send({"method": "get", "url": "%URL%", "timeout": "1m" }, x) }`,
			evalTimeout:    500 * time.Millisecond,
			serverDelay:    5 * time.Second,
			defaultTimeout: 1 * time.Minute,
			expected:       &Error{Code: BuiltinErr, Message: "http.send: Get %URL%: context deadline exceeded"},
		},
	}

	for _, tc := range tests {
		responseDelay = tc.serverDelay

		ctx := context.Background()
		var cancel context.CancelFunc
		if tc.evalTimeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, tc.evalTimeout)
		}

		// TODO(patrick-east): Remove this along with the environment variable so that the "default" can't change
		originalDefaultTimeout := defaultHTTPRequestTimeout
		if tc.defaultTimeout > 0 {
			defaultHTTPRequestTimeout = tc.defaultTimeout
		}

		rule := strings.ReplaceAll(tc.rule, "%URL%", ts.URL)
		if e, ok := tc.expected.(*Error); ok {
			e.Message = strings.ReplaceAll(e.Message, "%URL%", ts.URL)
		}

		runTopDownTestCaseWithContext(ctx, t, map[string]interface{}{}, tc.note, []string{rule}, nil, tc.input, tc.expected)

		// Put back the default (may not have changed)
		defaultHTTPRequestTimeout = originalDefaultTimeout
		if cancel != nil {
			cancel()
		}
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
			raw:      &ast.Array{},
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

	resultObj, err := ast.InterfaceToValue(expectedResult)
	if err != nil {
		panic(err)
	}

	data := loadSmallTestData()
	rule := []string{fmt.Sprintf(
		`p = x { http.send({"method": "get", "url": "%s"}, x) }`, baseURL)}

	// run the test
	runTopDownTestCase(t, data, "http.send", rule, resultObj.String())

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

	resultObj, err := ast.InterfaceToValue(expectedResult)
	if err != nil {
		panic(err)
	}

	data := loadSmallTestData()
	rule := []string{fmt.Sprintf(
		`p = x { http.send({"method": "get", "url": "%s", "enable_redirect": true}, x) }`, baseURL)}

	// run the test
	runTopDownTestCase(t, data, "http.send", rule, resultObj.String())
}

func TestHTTPSendCaching(t *testing.T) {
	// test server
	nextResponse := "{}"
	var requests []*http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(nextResponse))
	}))
	defer ts.Close()

	// expected result

	var body []interface{}
	bodyMap := map[string]string{"id": "1", "firstname": "John"}
	body = append(body, bodyMap)

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
									r2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "headers": {"h2": "v3"}})
									r1_2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "headers": {"h1": "v1", "h2": "v2"}})  # cached
									r2_2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "headers": {"h2": "v2"}})  # cached
									r2_2 = http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "headers": {"h2": "v3"}})  # cached
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
		nextResponse = tc.response
		requests = nil
		runTopDownTestCase(t, data, tc.note, []string{strings.ReplaceAll(tc.ruleTemplate, "%URL%", ts.URL)}, tc.response)

		// Note: The runTopDownTestCase ends up evaluating twice (once with and once without partial
		// eval first), so expect 2x the total request count the test case specified.
		actualCount := len(requests) / 2
		if actualCount != tc.expectedReqCount {
			t.Fatalf("Expected to only get %d requests, got %d", tc.expectedReqCount, actualCount)
		}
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
		w.Write(js)
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

	caCertPEM, err := ioutil.ReadFile(localCaFile)
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
	err = os.Setenv("CLIENT_CERT_ENV", string(clientCert))
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := readKeyFromFile(localClientKeyFile)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Setenv("CLIENT_KEY_ENV", string(clientKey))
	if err != nil {
		t.Fatal(err)
	}
	err = os.Setenv("CLIENT_CA_ENV", string(caCertPEM))
	if err != nil {
		t.Fatal(err)
	}

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

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_file": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s"}, x) }`, s.URL+"/cert", localCaFile, localClientCertFile, localClientKeyFile)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, resultObj.String())
	})

	t.Run("HTTPS Get with File Cert", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_file": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s"}, x) }`, s.URL, localCaFile, localClientCertFile, localClientKeyFile)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, resultObj.String())
	})

	t.Run("HTTPS Get with Env Cert", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_env_variable": "CLIENT_CA_ENV", "tls_client_cert_env_variable": "CLIENT_CERT_ENV", "tls_client_key_env_variable": "CLIENT_KEY_ENV"}, x) }`, s.URL)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, resultObj.String())
	})

	t.Run("HTTPS Get with Env and File Cert", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_env_variable": "CLIENT_CA_ENV", "tls_client_cert_env_variable": "CLIENT_CERT_ENV", "tls_client_key_env_variable": "CLIENT_KEY_ENV", "tls_ca_cert_file": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s"}, x) }`, s.URL, localCaFile, localClientCertFile, localClientKeyFile)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, resultObj.String())
	})

	t.Run("HTTPS Get with System Certs, Env and File Cert", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_use_system_certs": true, "tls_ca_cert_env_variable": "CLIENT_CA_ENV", "tls_client_cert_env_variable": "CLIENT_CERT_ENV", "tls_client_key_env_variable": "CLIENT_KEY_ENV", "tls_ca_cert_file": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s"}, x) }`, s.URL, localCaFile, localClientCertFile, localClientKeyFile)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, resultObj.String())
	})

	t.Run("Negative Test: No Root Ca", func(t *testing.T) {

		expectedResult := &Error{Code: BuiltinErr, Message: "x509: certificate signed by unknown authority", Location: nil}
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

		expectedResult := &Error{Code: BuiltinErr, Message: "x509: certificate signed by unknown authority", Location: nil}
		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s", "tls_use_system_certs": true}, x) }`, s.URL, localClientCertFile, localClientKeyFile)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, expectedResult)
	})
}

func TestHTTPSNoClientCerts(t *testing.T) {

	const (
		localCaFile         = "testdata/ca.pem"
		localServerCertFile = "testdata/server-cert.pem"
		localServerKeyFile  = "testdata/server-key.pem"
	)

	caCertPEM, err := ioutil.ReadFile(localCaFile)
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

	err = os.Setenv("CLIENT_CA_ENV", string(caCertPEM))
	if err != nil {
		t.Fatal(err)
	}

	// Replicating some of what happens in the server's HTTPS listener
	s := getTLSTestServer()
	s.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caPool,
	}
	s.StartTLS()
	defer s.Close()

	t.Run("HTTPS Get with CA Cert File", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_file": "%s"}, x) }`, s.URL, localCaFile)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, resultObj.String())
	})

	t.Run("HTTPS Get with CA Cert ENV", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_env_variable": "CLIENT_CA_ENV"}, x) }`, s.URL)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, resultObj.String())
	})

	t.Run("HTTPS Get with System CA Cert Pool", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_env_variable": "CLIENT_CA_ENV"}, x) }`, s.URL)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, resultObj.String())
	})

	t.Run("HTTPS Get with System Certs, Env and File Cert", func(t *testing.T) {
		// expected result
		expectedResult := map[string]interface{}{
			"status":      "200 OK",
			"status_code": http.StatusOK,
			"body":        nil,
			"raw_body":    "",
		}

		resultObj, err := ast.InterfaceToValue(expectedResult)
		if err != nil {
			t.Fatal(err)
		}

		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_use_system_certs": true, "tls_ca_cert_env_variable": "CLIENT_CA_ENV", "tls_ca_cert_file": "%s"}, x) }`, s.URL, localCaFile)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, resultObj.String())
	})

	t.Run("Negative Test: System Certs do not include local rootCA", func(t *testing.T) {

		expectedResult := &Error{Code: BuiltinErr, Message: "x509: certificate signed by unknown authority", Location: nil}
		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_use_system_certs": true}, x) }`, s.URL)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, expectedResult)
	})
}
