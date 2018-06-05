// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package watch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

const (
	testPaths = 4
	testOps   = 100
)

func TestWatchSimple(t *testing.T) {
	ctx := context.Background()
	store := inmem.NewFromObject(loadSmallTestData())
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	watcher, err := New(ctx, store, ast.NewCompiler(), txn)
	if err != nil {
		t.Fatalf("Failed to create watch: %v", err)
	}
	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	modifiedPath := storage.MustParsePath("/a")

	h, err := watcher.Query("x = data.a")
	if err != nil {
		t.Fatalf("Unexpected error setting watch: %v", err)
	}
	time.Sleep(time.Millisecond)

	changes := []interface{}{
		"hello",
		"bye",
		"foo",
		json.Number("5"),
		[]interface{}{json.Number("1"), json.Number("2"), json.Number("3")},
	}

	exp := []Event{
		{
			Query: "x = data.a",
			Value: rego.ResultSet{
				{
					Bindings: rego.Vars{"x": []interface{}{json.Number("1"), json.Number("2"), json.Number("3"), json.Number("4")}},
				},
			},
			Error: nil,
		},
	}
	for _, data := range changes {
		exp = append(exp, Event{
			Query: "x = data.a",
			Value: rego.ResultSet{
				{
					Bindings: rego.Vars{"x": data},
				},
			},
			Error: nil,
		})
	}

	var wg sync.WaitGroup
	wg.Add(len(exp))

	notifyRead := make(chan struct{}, 1)
	defer close(notifyRead)

	done := make(chan struct{})
	defer close(done)

	go func() {
		for i := 0; i < len(exp); i++ {
			e, ok := <-h.C
			if !ok {
				break
			}

			if len(e.Value) != 1 {
				t.Errorf("Expected result set length 1, got %d", len(e.Value))
			}

			if len(e.Metrics.All()) == 0 {
				t.Errorf("Expected non-empty metrics")
			}

			e.Metrics = nil

			if len(e.Tracer) != 5 {
				t.Errorf("Expected explanation to have length 5, got %d", len(e.Tracer))
			}

			e.Tracer = nil

			if len(e.Value) > 0 {
				e.Value[0].Expressions = nil
				if !reflect.DeepEqual(exp[i], e) {
					t.Errorf("Expected notification %v, got %v", exp[i], e)
				}
			}

			notifyRead <- struct{}{}
			wg.Done()
		}

		for {
			select {
			case e, ok := <-h.C:
				if !ok {
					h.C = nil
					continue
				}
				t.Errorf("Unexpected notification %v", e)
			case <-done:
				return
			}
		}
	}()

	for _, data := range changes {
		// Since the notifications are delivered in a goroutine, we need to wait
		// to allow the current one to arrive. If we don't wait, the next transaction below
		// could execute before the notification was processed, which is fine in real
		// life, but for testing purposes we want to see every change.
		<-notifyRead

		txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
		if err := store.Write(ctx, txn, storage.AddOp, modifiedPath, data); err != nil {
			store.Abort(ctx, txn)
			t.Fatalf("Unexpected error writing to store: %v", err)
		}

		if err := store.Commit(ctx, txn); err != nil {
			store.Abort(ctx, txn)
			t.Fatalf("Unexpected error committing to store: %v", err)
		}
	}

	wg.Wait()
	h.Stop()
	if _, ok := watcher.handles[h]; ok {
		t.Fatal(`Query was not unregistered`)
	}
	for _, notifiers := range watcher.dataWatch {
		if _, ok := notifiers[h.notify]; ok {
			t.Fatal(`Query's notify channel was not deleted`)
		}
	}
}

func TestWatchMigrateInvalidate(t *testing.T) {
	ctx := context.Background()
	store := inmem.NewFromObject(loadSmallTestData())

	c := ast.NewCompiler()
	c.Compile(map[string]*ast.Module{
		"test": ast.MustParseModule(`package y

		r["foo"] = y {
			y = data.a[0]
		}`),
	})
	if c.Failed() {
		t.Fatalf("compilation failed: %v", c.Errors.Error())
	}
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	watcher, err := New(ctx, store, c, txn)
	if err != nil {
		t.Fatalf("Failed to create watch: %v", err)
	}
	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	handle, err := watcher.Query(`plus(data.y.r["foo"], 1, x)`)
	if err != nil {
		t.Fatalf("Unexpected error setting watch: %v", err)
	}
	first := <-handle.C
	expQuery := `plus(data.y.r["foo"], 1, x)`
	if first.Query != expQuery {
		t.Fatalf("Unexpected query value in first event, expected `%s`, got `%s`", expQuery, first.Query)
	}

	expBindings := rego.Vars{"x": json.Number("2")}
	if !reflect.DeepEqual(expBindings, first.Value[0].Bindings) {
		t.Fatalf("Unexpected bindings in first event, expected %v, got %v", expBindings, first.Value[0].Bindings)
	}

	c = ast.NewCompiler()
	c.Compile(map[string]*ast.Module{
		"test": ast.MustParseModule(`package y

		r["foo"] = y {
			y = "bar"
		}`),
	})

	if c.Failed() {
		t.Fatalf("compilation failed: %v", c.Errors.Error())
	}

	// Old watcher channels should still be working on the new compiler.
	txn = storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	watcher, err = watcher.Migrate(c, txn)
	if err != nil {
		t.Fatalf("Unexpected error migrating watcher: %v", err)
	}
	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	second := <-handle.C
	expSecond := Event{
		Query: `plus(data.y.r["foo"], 1, x)`,
		Error: errors.New(`watch invalidated: 1 error occurred: 1:1: rego_type_error: plus: invalid argument(s)
	have: (string, number, ???)
	want: (number, number, number)`),
	}

	if !reflect.DeepEqual(expSecond, second) {
		t.Fatalf("Expected second event to be %v, got %v", expSecond, second)
	}

	if _, ok := <-handle.C; ok {
		t.Fatalf("Invalid watch channel was not closed")
	}
	if _, ok := watcher.handles[handle]; ok {
		t.Fatalf("Invalid watch channel present in new watcher")
	}
	for _, notifiers := range watcher.dataWatch {
		if _, ok := notifiers[handle.notify]; ok {
			t.Fatalf("Invalid watch channel notify channel present in new watcher")
		}
	}
}

func TestWatchMigrate(t *testing.T) {
	ctx := context.Background()
	store := inmem.NewFromObject(loadSmallTestData())

	c0 := ast.NewCompiler()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	watcher, err := New(ctx, store, c0, txn)
	if err != nil {
		t.Fatalf("Failed to create watch: %v", err)
	}
	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	zero, err := watcher.Query("x0 = data.x0")
	if err != nil {
		t.Fatalf("Unexpected error setting watch: %v", err)
	}

	one, err := watcher.Query("x1 = data.x1")
	if err != nil {
		t.Fatalf("Unexpected error setting watch: %v", err)
	}

	two, err := watcher.Query(`x2 = data.y.r["foo"]`)
	if err != nil {
		t.Fatalf("Unexpected error setting watch: %v", err)
	}

	three, err := watcher.Query("x3 = data.x0+1")
	if err != nil {
		t.Fatalf("Unexpected error setting watch: %v", err)
	}

	c1 := ast.NewCompiler()
	c1.Compile(map[string]*ast.Module{
		"test": ast.MustParseModule(`package y

		r["foo"] = y {
			y = data.x2
		}`),
	})

	if c1.Failed() {
		t.Fatalf("compilation failed: %v", c1.Errors.Error())
	}

	// Old watcher channels should still be working on the new compiler.
	txn = storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	watcher, err = watcher.Migrate(c1, txn)
	if err != nil {
		t.Fatalf("Unexpected error migrating watcher: %v", err)
	}
	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	testWatchRandom(t, watcher, zero, one, two, three)
}

func TestWatchRandom(t *testing.T) {
	ctx := context.Background()
	store := inmem.NewFromObject(loadSmallTestData())

	c := ast.NewCompiler()
	c.Compile(map[string]*ast.Module{
		"test": ast.MustParseModule(`package y

		r["foo"] = y {
			y = data.x2
		}`),
	})

	if c.Failed() {
		t.Fatalf("compilation failed: %v", c.Errors.Error())
	}

	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	watcher, err := New(ctx, store, c, txn)
	if err != nil {
		t.Fatalf("Failed to create watch: %v", err)
	}
	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	zero, err := watcher.Query("x0 = data.x0")
	if err != nil {
		t.Fatalf("Unexpected error setting watch: %v", err)
	}

	one, err := watcher.Query("x1 = data.x1")
	if err != nil {
		t.Fatalf("Unexpected error setting watch: %v", err)
	}

	two, err := watcher.Query(`x2 = data`)
	if err != nil {
		t.Fatalf("Unexpected error setting watch: %v", err)
	}

	three, err := watcher.Query("x3 = data.x0+1")
	if err != nil {
		t.Fatalf("Unexpected error setting watch: %v", err)
	}

	testWatchRandom(t, watcher, zero, one, two, three)
}

func testWatchRandom(t *testing.T, watcher *Watcher, zero, one, two, three *Handle) {
	var wg sync.WaitGroup
	check := func(h *Handle, done chan struct{}, key string, offset int64) {
		defer wg.Done()
		last := offset
		exp := testOps + offset

		var sent bool
		for e := range h.C {
			// Until the data is uploaded, the query will be
			// unsatisfied.
			if len(e.Value) == 0 {
				continue
			}

			if e.Error != nil {
				t.Errorf("error evaluating query: %v", e.Error)
				continue
			}

			v, ok := e.Value[0].Bindings[key]
			if !ok {
				t.Errorf("Expected notification of %s changing, got %v", key, e.Value)
				continue
			}

			// One of the tests watches all of data. We care about
			// data.y.r.foo
			if m, ok := v.(map[string]interface{}); ok {
				tmp := m["y"].(map[string]interface{})
				if len(tmp) != 1 {
					t.Errorf("Expected exactly one value in data.y, found %v", tmp)
				}

				tmp = tmp["r"].(map[string]interface{})
				if len(tmp) == 0 {
					// If we are watching all of data and the
					// relevant data hasn't been uploaded, skip it.
					continue
				}
				v = tmp["foo"]
			}

			n, ok := v.(json.Number)
			if !ok {
				t.Errorf("%v was not a valid json.Number", e.Value)
				continue
			}

			i, err := n.Int64()
			if err != nil {
				t.Errorf("%v was not a valid int", v)
				continue
			}
			// Since the writes are strictly increasing in value and
			// all the queries results are linearly dependent on the
			// data they watch, the result should never increase. It
			// may stay the same or skip values due to multiple store
			// writes occurring after a notification is acted upon,
			// but while or before the new result is computed.
			if i < last {
				t.Errorf("query result decreased, last=%d, cur=%d", last, i)
				continue
			}

			last = i

			if last == exp && !sent {
				done <- struct{}{}
				sent = true
			}
		}

		if last < exp {
			t.Errorf("%s did not receive all notifications, expected %d, got %d", key, exp, last)
		} else if last > exp {
			t.Errorf("%s received extra notifications, expected %d, got %d", key, exp, last)
		}
	}

	wg.Add(testPaths)
	done := make(chan struct{}, testPaths)
	go check(zero, done, "x0", 0)
	go check(one, done, "x1", 0)
	go check(two, done, "x2", 0)
	go check(three, done, "x3", 1)

	writesDone := make(signal)
	xacts := generateTestSet()
	go func() {
		defer close(writesDone)
		for _, xact := range xacts {
			txn := storage.NewTransactionOrDie(watcher.ctx, watcher.store, storage.WriteParams)
			if err := watcher.store.Write(watcher.ctx, txn, storage.AddOp, xact.path, xact.value); err != nil {
				watcher.store.Abort(watcher.ctx, txn)
				t.Errorf("Unexpected error writing to watcher.store: %v", err)
				return
			}

			if err := watcher.store.Commit(watcher.ctx, txn); err != nil {
				watcher.store.Abort(watcher.ctx, txn)
				t.Errorf("Unexpected error committing to watcher.store: %v", err)
				return
			}
		}
	}()

	<-writesDone
	for i := 0; i < testPaths; i++ {
		<-done
	}

	zero.Stop()
	one.Stop()
	two.Stop()
	three.Stop()
	wg.Wait()
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

type transaction struct {
	path  storage.Path
	value json.Number
}

// generates a random test set of transactions, writing to `testPaths` paths `testOps`
// integers, which are guaranteed to strictly increase with respect to a path. These
// separate sets of writes are then randomly interleaved, remaining in the same relative
// order with respect to path.
func generateTestSet() []transaction {
	rand.Seed(time.Now().UnixNano())

	var lists [][]transaction
	for i := 0; i < testPaths; i++ {
		var list []transaction

		path := storage.MustParsePath(fmt.Sprintf("/x%d", i))
		for j := 0; j < testOps; j++ {
			v := json.Number(fmt.Sprint(j + 1))
			list = append(list, transaction{path, v})
		}
		lists = append(lists, list)
	}
	return randMerge(lists)
}

func randMerge(lists [][]transaction) (xacts []transaction) {
	for len(lists) > 0 {
		t := 1.0 / float64(len(lists))
		r := rand.Float64()
		i := int(r / t)

		xacts = append(xacts, lists[i][0])
		lists[i] = lists[i][1:]

		if len(lists[i]) == 0 {
			var tmp [][]transaction
			for j, l := range lists {
				if i == j {
					continue
				}
				tmp = append(tmp, l)
			}
			lists = tmp
		}
	}
	return xacts
}
