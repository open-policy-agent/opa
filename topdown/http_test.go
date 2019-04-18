// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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
		if strings.HasPrefix(k, "X-") {
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

	// expected result
	expectedResult := make(map[string]interface{})
	expectedResult["status"] = "200 OK"
	expectedResult["status_code"] = http.StatusOK

	bodyMap := map[string][]string{"X-Foo": {"ISO-8859-1,utf-8;q=0.7,*;q=0.7"}, "X-Opa": {"server"}}
	expectedResult["body"] = bodyMap
	expectedResult["raw_body"] = "{\"X-Foo\":[\"ISO-8859-1,utf-8;q=0.7,*;q=0.7\"],\"X-Opa\":[\"server\"]}\n"

	jsonString, err := json.Marshal(expectedResult)
	if err != nil {
		panic(err)
	}
	s := string(jsonString[:])

	// run the test
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"http.send", []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "headers": {"X-Foo":"ISO-8859-1,utf-8;q=0.7,*;q=0.7", "X-Opa": "server"}}, x) }`, ts.URL)}, s},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

// TestHTTPostRequest adds a new person
func TestHTTPostRequest(t *testing.T) {

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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(person)
	}))

	defer ts.Close()

	// expected result
	expectedResult := map[string]interface{}{
		"status":      "200 OK",
		"status_code": http.StatusOK,
		"body":        map[string]string{"id": "2", "firstname": "Joe"},
		"raw_body":    "{\"id\":\"2\",\"firstname\":\"Joe\"}\n",
	}

	resultObj, err := ast.InterfaceToValue(expectedResult)
	if err != nil {
		panic(err)
	}

	// create a new person object
	person2 := Person{ID: "2", Firstname: "Joe"}
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(person2)

	// run the test
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"http.send", []string{fmt.Sprintf(
			`p = x { http.send({"method": "post", "url": "%s", "body": %s}, x) }`, ts.URL, b)}, resultObj.String()},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
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
		{"invalid keys", []string{`p = x { http.send({"method": "get", "url": "http://127.0.0.1:51113", "bad_key": "bad_value"}, x) }`}, fmt.Errorf(`invalid request parameters(s): {"bad_key"}`)},
		{"missing keys", []string{`p = x { http.send({"method": "get"}, x) }`}, fmt.Errorf(`missing required request parameters(s): {"url"}`)},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
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

		expectedResult := Error{Code: BuiltinErr, Message: "x509: certificate signed by unknown authority", Location: nil}
		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s"}, x) }`, s.URL, localClientCertFile, localClientKeyFile)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, expectedResult)
	})

	t.Run("Negative Test: Wrong Cert/Key Pair", func(t *testing.T) {

		expectedResult := Error{Code: BuiltinErr, Message: "tls: private key does not match public key", Location: nil}
		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_ca_cert_file": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s"}, x) }`, s.URL, localCaFile, localClientCert2File, localClientKeyFile)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, expectedResult)
	})

	t.Run("Negative Test: System Certs do not include local rootCA", func(t *testing.T) {

		expectedResult := Error{Code: BuiltinErr, Message: "x509: certificate signed by unknown authority", Location: nil}
		data := loadSmallTestData()
		rule := []string{fmt.Sprintf(
			`p = x { http.send({"method": "get", "url": "%s", "tls_client_cert_file": "%s", "tls_client_key_file": "%s", "tls_use_system_certs": true}, x) }`, s.URL, localClientCertFile, localClientKeyFile)}

		// run the test
		runTopDownTestCase(t, data, "http.send", rule, expectedResult)
	})
}
