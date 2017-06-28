// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package watch_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/watch"
)

func ExampleWatcher_Query() {
	ctx := context.Background()
	store := inmem.NewFromObject(loadSmallTestData())

	// This example syncs the reader and writing to make the output deterministic.
	done := make(chan struct{})
	gotNotification := make(chan struct{})

	mod, err := ast.ParseModule("example", "package y\nr = s { s = data.a }")
	if err != nil {
		// Handle error
	}

	compiler := ast.NewCompiler()
	if compiler.Compile(map[string]*ast.Module{"example": mod}); compiler.Failed() {
		// Handle error
	}

	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		// Handle error
	}

	// Create a new Watcher that uses the given store and compiler to monitor
	// queries. The watcher must be creating inside a transaction so that it can
	// properly hook into the store.
	w, err := watch.New(ctx, store, compiler, txn)
	if err != nil {
		// Handle error
	}
	if err := store.Commit(ctx, txn); err != nil {
		// Handle error
	}

	// Create a new watch on the query. Whenever its result changes, the result of
	// the change will be sent to `handle.C`.
	handle, err := w.Query("x = data.y")
	if err != nil {
		// Handle error
	}

	go func() {
		for e := range handle.C {
			fmt.Printf("%s: %v (%v)\n", e.Query, e.Value[0].Bindings, e.Error)

			gotNotification <- struct{}{}
		}
		close(done)
	}()
	<-gotNotification // One notification will be sent on watch creation with the initial query value.

	for i := 0; i < 4; i++ {
		path, _ := storage.ParsePath(fmt.Sprintf("/a/%d", i))
		if err := storage.WriteOne(ctx, store, storage.ReplaceOp, path, json.Number(fmt.Sprint(i))); err != nil {
			// Handle error
		}

		<-gotNotification
	}

	// Ending the query will close `handle.C`.
	handle.Stop()
	<-done

	// Output:
	//
	// x = data.y: map[x:map[r:[1 2 3 4]]] (<nil>)
	// x = data.y: map[x:map[r:[0 2 3 4]]] (<nil>)
	// x = data.y: map[x:map[r:[0 1 3 4]]] (<nil>)
	// x = data.y: map[x:map[r:[0 1 2 4]]] (<nil>)
	// x = data.y: map[x:map[r:[0 1 2 3]]] (<nil>)
}

func ExampleWatcher_Migrate() {
	ctx := context.Background()
	store := inmem.NewFromObject(loadSmallTestData())

	// This example syncs the reader and writing to make the output deterministic.
	var notifyAlert chan struct{}
	done := make(chan struct{})
	gotNotification1 := make(chan struct{})
	gotNotification2 := make(chan struct{})

	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		// Handle error
	}

	// Create a new Watcher that uses the given store and compiler to monitor
	// queries. The watcher must be creating inside a transaction so that it can
	// properly hook into the store.
	w, err := watch.New(ctx, store, ast.NewCompiler(), txn)
	if err != nil {
		// Handle error
	}

	if err := store.Commit(ctx, txn); err != nil {
		// Handle error
	}

	handle1, err := w.Query("x = data.y")
	if err != nil {
		// Handle error
	}

	go func() {
		for e := range handle1.C {
			value := fmt.Sprint(e.Value)
			if len(e.Value) > 0 {
				value = fmt.Sprint(e.Value[0].Bindings)
			}
			fmt.Printf("%s: %s (%v)\n", e.Query, value, e.Error)

			if notifyAlert != nil {
				notifyAlert <- struct{}{}
			}
			gotNotification1 <- struct{}{}
		}
		done <- struct{}{}
	}()

	// One notification will be sent on watch creation with the initial query
	// value. It will be empty since the document we are watching is not yet defined.
	<-gotNotification1

	mod, err := ast.ParseModule("example", "package y\nr = s { s = data.a }")
	if err != nil {
		// Handle error
	}

	compiler := ast.NewCompiler()
	if compiler.Compile(map[string]*ast.Module{"example": mod}); compiler.Failed() {
		// Handle error
	}

	if txn, err = store.NewTransaction(ctx, storage.WriteParams); err != nil {
		// Handle error
	}

	// The handle from before will still be active after we migrate to the
	// new compiler. Changes to data.a will now cause notifications since data.y now
	// exists.
	m, err := w.Migrate(compiler, txn)
	if err != nil {
		// Handle error
	}

	if err := store.Commit(ctx, txn); err != nil {
		// Handle error
	}

	// After migrating, all existing watches will get a notification, as if they had
	// just started.
	<-gotNotification1

	// The old watcher will be closed. Watches can no longer be registered on it.
	_, err = w.Query("foo")
	fmt.Println(err)

	handle2, err := m.Query("y = data.a")
	if err != nil {
		// Handle error
	}

	go func() {
		for e := range handle2.C {
			if notifyAlert != nil {
				<-notifyAlert
			} else {
				notifyAlert = make(chan struct{})
			}

			value := fmt.Sprint(e.Value)
			if len(e.Value) > 0 {
				value = fmt.Sprint(e.Value[0].Bindings)
			}
			fmt.Printf("%s: %s (%v)\n", e.Query, value, e.Error)
			gotNotification2 <- struct{}{}
		}
		done <- struct{}{}
	}()
	<-gotNotification2

	for i := 0; i < 4; i++ {
		path, _ := storage.ParsePath(fmt.Sprintf("/a/%d", i))
		if err := storage.WriteOne(ctx, store, storage.ReplaceOp, path, json.Number(fmt.Sprint(i))); err != nil {
			// Handle error
		}

		<-gotNotification1
		<-gotNotification2
	}

	// Ending the queries will close the notification channels.
	handle1.Stop()
	handle2.Stop()
	<-done
	<-done

	// Output:
	//
	// x = data.y: [] (<nil>)
	// x = data.y: map[x:map[r:[1 2 3 4]]] (<nil>)
	// cannot start query watch with closed Watcher
	// y = data.a: map[y:[1 2 3 4]] (<nil>)
	// x = data.y: map[x:map[r:[0 2 3 4]]] (<nil>)
	// y = data.a: map[y:[0 2 3 4]] (<nil>)
	// x = data.y: map[x:map[r:[0 1 3 4]]] (<nil>)
	// y = data.a: map[y:[0 1 3 4]] (<nil>)
	// x = data.y: map[x:map[r:[0 1 2 4]]] (<nil>)
	// y = data.a: map[y:[0 1 2 4]] (<nil>)
	// x = data.y: map[x:map[r:[0 1 2 3]]] (<nil>)
	// y = data.a: map[y:[0 1 2 3]] (<nil>)
}

func loadSmallTestData() map[string]interface{} {
	var data map[string]interface{}
	err := util.UnmarshalJSON([]byte(`{
        "a": [1,2,3,4],
        "b": {
            "v1": "hello",
            "v2": "goodbye"
        },
        "c": [{
            "x": [true, false, "foo"],
            "y": [null, 3.14159],
            "z": {"p": true, "q": false}
        }],
        "d": {
            "e": ["bar", "baz"]
        },
		"g": {
			"a": [1, 0, 0, 0],
			"b": [0, 2, 0, 0],
			"c": [0, 0, 0, 4]
		},
		"h": [
			[1,2,3],
			[2,3,4]
		]
    }`), &data)
	if err != nil {
		panic(err)
	}
	return data
}
