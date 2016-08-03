// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/open-policy-agent/opa/storage"
)

func ExampleDataStore_Get() {

	// Define some dummy data to initialize the DataStore with.
	exampleData := `
    {
        "users": [
            {
                "name": "alice",
                "color": "red",
                "likes": ["clouds", "ships"] 
            },
            {
                "name": "burt",
                "likes": ["cheese", "wine"]
            }
        ]
    }
    `

	var d map[string]interface{}

	if err := json.Unmarshal([]byte(exampleData), &d); err != nil {
		fmt.Println("Unmarshal error:", err)
	}

	// Create the new DataStore with the dummy data.
	ds := storage.NewDataStoreFromJSONObject(d)

	// Read values out of the DataStore.
	v1, err1 := ds.Get([]interface{}{"users", float64(1), "likes", float64(1)})
	v2, err2 := ds.Get([]interface{}{"users", float64(0), "age"})

	// Inspect the return values.
	fmt.Println("v1:", v1)
	fmt.Println("err1:", err1)
	fmt.Println("v2:", v2)
	fmt.Println("err2:", err2)
	fmt.Println("err2 is not found:", storage.IsNotFound(err2))

	// Output:
	// v1: wine
	// err1: <nil>
	// v2: <nil>
	// err2: storage error (code: 1): bad path: [users 0 age], document does not exist
	// err2 is not found: true
}

func ExampleDataStore_Patch() {

	// Define some dummy data to initialize the DataStore with.
	exampleData := `
    {
        "users": [
            {
                "name": "alice",
                "color": "red",
                "likes": ["clouds", "ships"] 
            },
            {
                "name": "burt",
                "likes": ["cheese", "wine"]
            }
        ]
    }
    `

	var d1 map[string]interface{}

	if err := json.Unmarshal([]byte(exampleData), &d1); err != nil {
		fmt.Println("Unmarshal error:", err)
	}

	// Create the new DataStore with the dummy data.
	ds := storage.NewDataStoreFromJSONObject(d1)

	// Define dummy data to add to the DataStore.
	exampleAdd := `{
        "longitude": 82.501389,
        "latitude": -62.338889
    }`

	var d2 interface{}

	if err := json.Unmarshal([]byte(exampleAdd), &d2); err != nil {
		fmt.Println("Unmarshal error:", err)
	}

	// Write values into storage and read result.
	err0 := ds.Patch(storage.AddOp, []interface{}{"users", float64(0), "location"}, d2)
	v1, err1 := ds.Get([]interface{}{"users", float64(0), "location", "latitude"})
	err2 := ds.Patch(storage.ReplaceOp, []interface{}{"users", float64(1), "color"}, "red")

	// Inspect the return values.
	fmt.Println("err0:", err0)
	fmt.Println("v1:", v1)
	fmt.Println("err1:", err1)
	fmt.Println("err2:", err2)

	// Output:
	// err0: <nil>
	// v1: -62.338889
	// err1: <nil>
	// err2: storage error (code: 1): bad path: [users 1 color], document does not exist

}

func ExamplePolicyStore_Open() {

	// Define two example modules and write them to disk in a temporary directory.
	ex1 := `

        package opa.example

        p :- q.r != 0
        
    `

	ex2 := `

        package opa.example

        q = {"r": 100}

    `

	path, err := ioutil.TempDir("", "")
	if err != nil {
		fmt.Println("TempDir error:", err)
	}

	defer os.RemoveAll(path)

	if err = ioutil.WriteFile(filepath.Join(path, "ex1.rego"), []byte(ex1), 0644); err != nil {
		fmt.Println("WriteFile error:", err)
	}

	if err = ioutil.WriteFile(filepath.Join(path, "ex2.rego"), []byte(ex2), 0644); err != nil {
		fmt.Println("WriteFile error:", err)
	}

	// Create a new policy store and use the temporary directory for persistence.
	ds := storage.NewDataStore()
	ps := storage.NewPolicyStore(ds, path)

	// Open the policy store and load the existing modules.
	//
	// The LoadPolicies function provides a default implementation of callback
	// used to load the modules. If necessary, you can provide your own implementation
	// of the callback function to customize the policy store initialization.
	err = ps.Open(storage.LoadPolicies)
	if err != nil {
		fmt.Println("Open error:", err)
	}

	// Inspect one of the loaded policies.
	mod, err := ps.Get("ex1.rego")
	if err != nil {
		fmt.Println("Get error:", err)
	}

	fmt.Println("Expr:", mod.Rules[0].Body[0])

	// Output:
	// Expr: neq(data.opa.example.q.r, 0)

}
