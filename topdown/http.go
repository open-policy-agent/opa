// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"encoding/json"
	"fmt"

	"net/http"
	"os"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

// DefaultHTTPRequestTimeoutSec Default timeout value for http client
const defaultHTTPRequestTimeoutSec = 5

var allowedKeys = ast.NewSet(ast.StringTerm("method"), ast.StringTerm("url"), ast.StringTerm("body"))
var requiredKeys = ast.NewSet(ast.StringTerm("method"), ast.StringTerm("url"))

var client *http.Client

func builtinHTTPReq(bctx BuiltinContext, a ast.Value) (ast.Value, error) {
	switch a.(type) {
	case ast.Object:
		obj, ok := a.(ast.Object)
		if !ok {
			return nil, fmt.Errorf("type assertion error")
		}

		return evaluateHTTPRequest(obj, bctx)
	default:
		return nil, builtins.NewOperandTypeErr(1, a, ast.ObjectTypeName)
	}

}

func init() {
	createHTTPClient()
	RegisterFunctionalBuiltin1WithCtxt(ast.HTTPSend.Name, builtinHTTPReq)
}

// createHTTPClient creates a HTTP client
func createHTTPClient() {
	var timeout time.Duration

	timeoutDuration := os.Getenv("HTTP_SEND_TIMEOUT")

	if timeoutDuration == "" {
		timeout = time.Duration(defaultHTTPRequestTimeoutSec) * time.Second
	} else {
		timeout, _ = time.ParseDuration(timeoutDuration)
	}

	client = &http.Client{
		Timeout: timeout,
	}
}

// evaluateHTTPRequest executes the HTTP request and processes the response
func evaluateHTTPRequest(obj ast.Object, bctx BuiltinContext) (ast.Value, error) {
	var url string
	var method string
	var body *bytes.Buffer

	requestKeys := ast.NewSet(obj.Keys()...)

	// check invalid keys
	invalidKeys := requestKeys.Diff(allowedKeys)
	if invalidKeys.Len() != 0 {
		return nil, fmt.Errorf("invalid key %s", invalidKeys)
	}

	// check missing keys
	missingKeys := requiredKeys.Diff(requestKeys)
	if missingKeys.Len() != 0 {
		return nil, fmt.Errorf("missing keys %s", missingKeys)
	}

	for _, val := range obj.Keys() {
		key, err := ast.JSON(val.Value)
		if err != nil {
			return nil, fmt.Errorf("error while converting value to json %v", err)
		}
		key = key.(string)

		if key == "method" {
			method = obj.Get(val).String()
			method = strings.Trim(method, "\"")
		} else if key == "url" {
			url = obj.Get(val).String()
			url = strings.Trim(url, "\"")
		} else {
			bodyVal := obj.Get(val).Value
			bodyValInterface, err := ast.JSON(bodyVal)
			if err != nil {
				return nil, fmt.Errorf("error while converting value to json %v", err)
			}

			bodyValBytes, err := json.Marshal(bodyValInterface)
			if err != nil {
				return nil, fmt.Errorf("error while json marshalling %v", err)
			}
			body = bytes.NewBuffer(bodyValBytes)
		}
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
