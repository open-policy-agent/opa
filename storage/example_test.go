// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/open-policy-agent/opa/storage"
)

func ExampleStorage_Read() {
	// Initialize context for the example. Normally the caller would obtain the
	// context from an input parameter or instantiate their own.
	ctx := context.Background()

	// Define some dummy data to initialize the built-in store with.
	exampleInput := `
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

	var data map[string]interface{}

	// OPA uses Go's standard JSON library but assumes that numbers have been
	// decoded as json.Number instead of float64. You MUST decode with UseNumber
	// enabled.
	decoder := json.NewDecoder(bytes.NewBufferString(exampleInput))
	decoder.UseNumber()

	if err := decoder.Decode(&data); err != nil {
		// Handle error.
	}

	// Instantiate the storage layer.
	store := storage.New(storage.InMemoryWithJSONConfig(data))

	txn, err := store.NewTransaction(ctx)
	if err != nil {
		// Handle error.
	}

	defer store.Close(ctx, txn)

	// Read values out of storage.
	v1, err1 := store.Read(ctx, txn, storage.MustParsePath("/users/1/likes/1"))
	v2, err2 := store.Read(ctx, txn, storage.MustParsePath("/users/0/age"))

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
	// err2: storage error (code: 1): bad path: /users/0/age, document does not exist
	// err2 is not found: true
}

func ExampleStorage_Write() {
	// Initialize context for the example. Normally the caller would obtain the
	// context from an input parameter or instantiate their own.
	ctx := context.Background()

	// Define some dummy data to initialize the DataStore with.
	exampleInput := `
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

	var data map[string]interface{}

	// OPA uses Go's standard JSON library but assumes that numbers have been
	// decoded as json.Number instead of float64. You MUST decode with UseNumber
	// enabled.
	decoder := json.NewDecoder(bytes.NewBufferString(exampleInput))
	decoder.UseNumber()

	if err := decoder.Decode(&data); err != nil {
		// Handle error.
	}

	// Create the new DataStore with the dummy data.
	store := storage.New(storage.InMemoryWithJSONConfig(data))

	// Define dummy data to add to the DataStore.
	examplePatch := `{
        "longitude": 82.501389,
        "latitude": -62.338889
    }`

	var patch interface{}

	// See comment above regarding decoder usage.
	decoder = json.NewDecoder(bytes.NewBufferString(examplePatch))
	decoder.UseNumber()

	if err := decoder.Decode(&patch); err != nil {
		// Handle error.
	}

	txn, err := store.NewTransaction(ctx)
	if err != nil {
		// Handle error.
	}

	defer store.Close(ctx, txn)

	// Write values into storage and read result.
	err0 := store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/users/0/location"), patch)
	v1, err1 := store.Read(ctx, txn, storage.MustParsePath("/users/0/location/latitude"))
	err2 := store.Write(ctx, txn, storage.ReplaceOp, storage.MustParsePath("/users/1/color"), "red")

	// Inspect the return values.
	fmt.Println("err0:", err0)
	fmt.Println("v1:", v1)
	fmt.Println("err1:", err1)
	fmt.Println("err2:", err2)

	// Rollback transaction because write failed.

	// Output:
	// err0: <nil>
	// v1: -62.338889
	// err1: <nil>
	// err2: storage error (code: 1): bad path: /users/1/color, document does not exist

}

func ExampleStorage_Open() {
	// Initialize context for the example. Normally the caller would obtain the
	// context from an input parameter or instantiate their own.
	ctx := context.Background()

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
		// Handle error.
	}

	defer os.RemoveAll(path)

	if err = ioutil.WriteFile(filepath.Join(path, "ex1.rego"), []byte(ex1), 0644); err != nil {
		// Handle error.
	}

	if err = ioutil.WriteFile(filepath.Join(path, "ex2.rego"), []byte(ex2), 0644); err != nil {
		// Handle error.
	}

	// Instantiate storage layer and configure with a directory to persist policy modules.
	store := storage.New(storage.InMemoryConfig().WithPolicyDir(path))

	if err = store.Open(ctx); err != nil {
		// Handle error.
	}

	// Inspect one of the loaded policies.
	mod, _, err := storage.GetPolicy(ctx, store, "ex1.rego")

	if err != nil {
		// Handle error.
	}

	fmt.Println("Expr:", mod.Rules[0].Body[0])

	// Output:
	// Expr: neq(q.r, 0)

}
