// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/open-policy-agent/opa/internal/version"
	"io"
	"io/ioutil"
	"strconv"

	"net/http"
	"os"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

const defaultHTTPRequestTimeout = time.Second * 5

var allowedKeyNames = [...]string{
	"method",
	"url",
	"body",
	"enable_redirect",
	"force_json_decode",
	"headers",
	"tls_use_system_certs",
	"tls_ca_cert_file",
	"tls_ca_cert_env_variable",
	"tls_client_cert_env_variable",
	"tls_client_key_env_variable",
	"tls_client_cert_file",
	"tls_client_key_file",
}
var allowedKeys = ast.NewSet()

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
	createAllowedKeys()
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
	var tlsCaCertEnvVar []byte
	var tlsCaCertFile string
	var tlsClientKeyEnvVar []byte
	var tlsClientCertEnvVar []byte
	var tlsClientCertFile string
	var tlsClientKeyFile string
	var body *bytes.Buffer
	var enableRedirect bool
	var forceJSONDecode bool
	var tlsUseSystemCerts bool
	var tlsConfig tls.Config
	var clientCerts []tls.Certificate
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
		case "force_json_decode":
			forceJSONDecode, err = strconv.ParseBool(obj.Get(val).String())
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
		case "tls_use_system_certs":
			tlsUseSystemCerts, err = strconv.ParseBool(obj.Get(val).String())
			if err != nil {
				return nil, err
			}
		case "tls_ca_cert_file":
			tlsCaCertFile = obj.Get(val).String()
			tlsCaCertFile = strings.Trim(tlsCaCertFile, "\"")
		case "tls_ca_cert_env_variable":
			caCertEnv := obj.Get(val).String()
			caCertEnv = strings.Trim(caCertEnv, "\"")
			tlsCaCertEnvVar = []byte(os.Getenv(caCertEnv))
		case "tls_client_cert_env_variable":
			clientCertEnv := obj.Get(val).String()
			clientCertEnv = strings.Trim(clientCertEnv, "\"")
			tlsClientCertEnvVar = []byte(os.Getenv(clientCertEnv))
		case "tls_client_key_env_variable":
			clientKeyEnv := obj.Get(val).String()
			clientKeyEnv = strings.Trim(clientKeyEnv, "\"")
			tlsClientKeyEnvVar = []byte(os.Getenv(clientKeyEnv))
		case "tls_client_cert_file":
			tlsClientCertFile = obj.Get(val).String()
			tlsClientCertFile = strings.Trim(tlsClientCertFile, "\"")
		case "tls_client_key_file":
			tlsClientKeyFile = obj.Get(val).String()
			tlsClientKeyFile = strings.Trim(tlsClientKeyFile, "\"")
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

	if tlsClientCertFile != "" && tlsClientKeyFile != "" {
		clientCertFromFile, err := tls.LoadX509KeyPair(tlsClientCertFile, tlsClientKeyFile)
		if err != nil {
			return nil, err
		}
		clientCerts = append(clientCerts, clientCertFromFile)
	}

	if len(tlsClientCertEnvVar) > 0 && len(tlsClientKeyEnvVar) > 0 {
		clientCertFromEnv, err := tls.X509KeyPair(tlsClientCertEnvVar, tlsClientKeyEnvVar)
		if err != nil {
			return nil, err
		}
		clientCerts = append(clientCerts, clientCertFromEnv)
	}

	if len(clientCerts) > 0 {
		// this is a TLS connection
		connRootCAs, err := createRootCAs(tlsCaCertFile, tlsCaCertEnvVar, tlsUseSystemCerts)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, clientCerts...)
		tlsConfig.RootCAs = connRootCAs
		client.Transport = &http.Transport{
			TLSClientConfig: &tlsConfig,
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
		// Don't overwrite or append to one that was set in the custom headers
		if _, hasUA := customHeaders["User-Agent"]; !hasUA {
			req.Header.Add("User-Agent", version.UserAgent)
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
	var resultRawBody []byte

	var buf bytes.Buffer
	tee := io.TeeReader(resp.Body, &buf)
	resultRawBody, err = ioutil.ReadAll(tee)
	if err != nil {
		return nil, err
	}

	// If the response body cannot be JSON decoded,
	// an error will not be returned. Instead the "body" field
	// in the result will be null.
	if isContentTypeJSON(resp.Header) || forceJSONDecode {
		json.NewDecoder(&buf).Decode(&resultBody)
	}

	result := make(map[string]interface{})
	result["status"] = resp.Status
	result["status_code"] = resp.StatusCode
	result["body"] = resultBody
	result["raw_body"] = string(resultRawBody)

	resultObj, err := ast.InterfaceToValue(result)
	if err != nil {
		return nil, err
	}

	// add result to cache
	key := getCtxKey(method, url)
	bctx.Cache.Put(key, resultObj)

	return resultObj, nil
}

func isContentTypeJSON(header http.Header) bool {
	return strings.Contains(header.Get("Content-Type"), "application/json")
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

func createAllowedKeys() {
	for _, element := range allowedKeyNames {
		allowedKeys.Add(ast.StringTerm(element))
	}
}
