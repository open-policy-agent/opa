// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"net/http"
	"os"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

const defaultHTTPRequestTimeout = time.Second * 5

var allowedKeys = ast.NewSet(ast.StringTerm("method"), ast.StringTerm("url"), ast.StringTerm("body"), ast.StringTerm("enable_redirect"), ast.StringTerm("headers"))
var requiredKeys = ast.NewSet(ast.StringTerm("method"), ast.StringTerm("url"))

var client *http.Client

func builtinHTTPSend(bctx BuiltinContext, args []*ast.Term, iter func(*ast.Term) error) error {

	req, err := validateHTTPRequestOperand(args[0], 1)
	if err != nil {
		return handleBuiltinErr(ast.HTTPSend.Name, bctx.Location, err)
	}

	resp, err := executeHTTPRequest(bctx, req)
	if err != nil {
		return handleBuiltinErr(ast.HTTPSend.Name, bctx.Location, err)
	}

	return iter(ast.NewTerm(resp))
}

func init() {
	createHTTPClient()
	RegisterBuiltinFunc(ast.HTTPSend.Name, builtinHTTPSend)
}

func createHTTPClient() {
	timeout := defaultHTTPRequestTimeout
	timeoutDuration := os.Getenv("HTTP_SEND_TIMEOUT")
	if timeoutDuration != "" {
		timeout, _ = time.ParseDuration(timeoutDuration)
	}

	// create a http client with redirects disabled
	client = &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func validateHTTPRequestOperand(term *ast.Term, pos int) (ast.Object, error) {

	obj, err := builtins.ObjectOperand(term.Value, pos)
	if err != nil {
		return nil, err
	}

	requestKeys := ast.NewSet(obj.Keys()...)

	invalidKeys := requestKeys.Diff(allowedKeys)
	if invalidKeys.Len() != 0 {
		return nil, builtins.NewOperandErr(pos, "invalid request parameters(s): %v", invalidKeys)
	}

	missingKeys := requiredKeys.Diff(requestKeys)
	if missingKeys.Len() != 0 {
		return nil, builtins.NewOperandErr(pos, "missing required request parameters(s): %v", missingKeys)
	}

	return obj, nil

}

// Adds custom headers to a new HTTP request.
func addHeaders(req *http.Request, headers map[string]interface{}) (bool, error) {
	for k, v := range headers {
		// Type assertion
		header, ok := v.(string)
		if ok {
			req.Header.Add(k, header)
		} else {
			return false, fmt.Errorf("invalid type for headers value")
		}
	}
	return true, nil
}

func executeHTTPRequest(bctx BuiltinContext, obj ast.Object) (ast.Value, error) {
	var url string
	var method string
	var body *bytes.Buffer
	var enableRedirect bool
	var customHeaders map[string]interface{}
	for _, val := range obj.Keys() {
		key, err := ast.JSON(val.Value)
		if err != nil {
			return nil, err
		}
		key = key.(string)

		switch key {
		case "method":
			method = obj.Get(val).String()
			method = strings.Trim(method, "\"")
		case "url":
			url = obj.Get(val).String()
			url = strings.Trim(url, "\"")
		case "enable_redirect":
			enableRedirect, err = strconv.ParseBool(obj.Get(val).String())
			if err != nil {
				return nil, err
			}
		case "body":
			bodyVal := obj.Get(val).Value
			bodyValInterface, err := ast.JSON(bodyVal)
			if err != nil {
				return nil, err
			}

			bodyValBytes, err := json.Marshal(bodyValInterface)
			if err != nil {
				return nil, err
			}
			body = bytes.NewBuffer(bodyValBytes)
		case "headers":
			headersVal := obj.Get(val).Value
			headersValInterface, err := ast.JSON(headersVal)
			if err != nil {
				return nil, err
			}
			var ok bool
			customHeaders, ok = headersValInterface.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid type for headers key")
			}
		default:
			return nil, fmt.Errorf("Invalid Key %v", key)
		}
	}

	// check if redirects are enabled
	if enableRedirect {
		client.CheckRedirect = nil
	}

	if body == nil {
		body = bytes.NewBufferString("")
	}

	// check if cache already has a response for this query
	cachedResponse := checkCache(method, url, bctx)
	if cachedResponse != nil {
		return cachedResponse, nil
	}

	// create the http request
	req, err := http.NewRequest(strings.ToUpper(method), url, body)
	if err != nil {
		return nil, err
	}

	// Add custom headers passed from CLI

	if len(customHeaders) != 0 {
		if ok, err := addHeaders(req, customHeaders); !ok {
			return nil, err
		}
	}

	// execute the http request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// format the http result
	var resultBody interface{}
	json.NewDecoder(resp.Body).Decode(&resultBody)

	result := make(map[string]interface{})
	result["status"] = resp.Status
	result["status_code"] = resp.StatusCode
	result["body"] = resultBody

	resultObj, err := ast.InterfaceToValue(result)
	if err != nil {
		return nil, err
	}

	// add result to cache
	key := getCtxKey(method, url)
	bctx.Cache.Put(key, resultObj)

	return resultObj, nil
}

// getCtxKey returns the cache key.
// Key format: <METHOD>_<url>
func getCtxKey(method string, url string) string {
	keyTerms := []string{strings.ToUpper(method), url}
	return strings.Join(keyTerms, "_")
}

// checkCache checks for the given key's value in the cache
func checkCache(method string, url string, bctx BuiltinContext) ast.Value {
	key := getCtxKey(method, url)

	val, ok := bctx.Cache.Get(key)
	if ok {
		return val.(ast.Value)
	}
	return nil
}
