// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
			`p = x { http.send({"method": "get", "url": "%s"}, x) }`, ts.URL)}, resultObj.String()},
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

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(person)
	}))

	defer ts.Close()

	// expected result
	expectedResult := map[string]interface{}{
		"status":      "200 OK",
		"status_code": http.StatusOK,
		"body":        map[string]string{"id": "2", "firstname": "Joe"},
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
