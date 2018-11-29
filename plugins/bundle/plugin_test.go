// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/download"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

func TestPluginOneShot(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: &Status{}}

	module := "package foo\n\ncorge=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
		Modules: []bundle.ModuleFile{
			bundle.ModuleFile{
				Path:   "/foo/bar",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	plugin.oneShot(ctx, download.Update{Bundle: &b})

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	ids, err := manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	} else if len(ids) != 1 {
		t.Fatal("Expected 1 policy")
	}

	bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
	exp := []byte("package foo\n\ncorge=1")
	if err != nil {
		t.Fatal(err)
	} else if !bytes.Equal(bs, exp) {
		t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{})
	expData := util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}, "system": {"bundle": {"manifest": {"revision": "quickbrownfaux"}}}}`))
	if err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(data, expData) {
		t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
	}

}

func TestPluginOneShotCompileError(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: &Status{}}

	b1 := &bundle.Bundle{
		Data: map[string]interface{}{"a": "b"},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/example.rego",
				Parsed: ast.MustParseModule("package foo\n\np[x] { x = 1 }"),
			},
		},
	}

	plugin.oneShot(ctx, download.Update{Bundle: b1})

	b2 := &bundle.Bundle{
		Modules: []bundle.ModuleFile{
			{
				Path:   "/example.rego",
				Parsed: ast.MustParseModule("package foo\n\np[x]"),
			},
		},
	}

	plugin.oneShot(ctx, download.Update{Bundle: b2})

	txn := storage.NewTransactionOrDie(ctx, manager.Store)

	_, err := manager.Store.GetPolicy(ctx, txn, "/example.rego")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{})
	if err != nil || data == nil {
		t.Fatalf("Expected data to be intact but got: %v, err: %v", data, err)
	}
}

func TestPluginOneShotActivatationRemovesOld(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: &Status{}}

	module1 := `package example

		p = 1`

	b1 := bundle.Bundle{
		Data: map[string]interface{}{
			"foo": "bar",
		},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/example.rego",
				Raw:    []byte(module1),
				Parsed: ast.MustParseModule(module1),
			},
		},
	}

	plugin.oneShot(ctx, download.Update{Bundle: &b1})

	module2 := `package example

		p = 2`

	b2 := bundle.Bundle{
		Data: map[string]interface{}{
			"baz": "qux",
		},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/example2.rego",
				Raw:    []byte(module2),
				Parsed: ast.MustParseModule(module2),
			},
		},
	}

	plugin.oneShot(ctx, download.Update{Bundle: &b2})

	err := storage.Txn(ctx, manager.Store, storage.TransactionParams{}, func(txn storage.Transaction) error {
		ids, err := manager.Store.ListPolicies(ctx, txn)
		if err != nil {
			return err
		} else if !reflect.DeepEqual([]string{"/example2.rego"}, ids) {
			return fmt.Errorf("expected updated policy ids")
		}
		data, err := manager.Store.Read(ctx, txn, storage.Path{})
		// remove system key to make comparison simpler
		delete(data.(map[string]interface{}), "system")
		if err != nil {
			return err
		} else if !reflect.DeepEqual(data, map[string]interface{}{"baz": "qux"}) {
			return fmt.Errorf("expected updated data")
		}
		return nil
	})

	if err != nil {
		t.Fatal("Unexpected:", err)
	}
}

func TestPluginListener(t *testing.T) {

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: &Status{}}
	ch := make(chan Status)

	plugin.Register("test", func(status Status) {
		ch <- status
	})

	module := "package gork\np[x] { x = 1 }"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "quickbrownfaux",
		},
		Data: map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	// Test that initial bundle is ok. Defer to separate goroutine so we can
	// check result with channel.
	go plugin.oneShot(ctx, download.Update{Bundle: &b})
	s1 := <-ch

	if s1.ActiveRevision != "quickbrownfaux" || s1.Code != "" {
		t.Fatal("Unexpected status update, got:", s1)
	}

	module = "package gork\np[x]"

	b.Manifest.Revision = "slowgreenburd"
	b.Modules[0] = bundle.ModuleFile{
		Path:   "/foo.rego",
		Raw:    []byte(module),
		Parsed: ast.MustParseModule(module),
	}

	// Test that next update is failed.
	go plugin.oneShot(ctx, download.Update{Bundle: &b})
	s2 := <-ch

	if s2.ActiveRevision != "quickbrownfaux" || s2.Code == "" || s2.Message == "" || len(s2.Errors) == 0 {
		t.Fatal("Unexpected status update, got:", s2)
	}

	module = "package gork\np[1]"
	b.Manifest.Revision = "fancybluederg"
	b.Modules[0] = bundle.ModuleFile{
		Path:   "/foo.rego",
		Raw:    []byte(module),
		Parsed: ast.MustParseModule(module),
	}

	// Test that new update is successful.
	go plugin.oneShot(ctx, download.Update{Bundle: &b})
	s3 := <-ch

	if s3.ActiveRevision != "fancybluederg" || s3.Code != "" || s3.Message != "" || len(s3.Errors) != 0 {
		t.Fatal("Unexpected status update, got:", s3)
	}

	// Test that empty download update results in status update.
	go plugin.oneShot(ctx, download.Update{})
	s4 := <-ch

	if !reflect.DeepEqual(s3, s4) {
		t.Fatalf("Expected: %v but got: %v", s3, s4)
	}

}

func TestPluginListenerErrorClearedOn304(t *testing.T) {
	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{manager: manager, status: &Status{}}
	ch := make(chan Status)

	plugin.Register("test", func(status Status) {
		ch <- status
	})

	b := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "quickbrownfaux",
		},
		Data: map[string]interface{}{"foo": "bar"},
	}

	// Test that initial bundle is ok.
	go plugin.oneShot(ctx, download.Update{Bundle: &b})
	s1 := <-ch

	if s1.ActiveRevision != "quickbrownfaux" || s1.Code != "" {
		t.Fatal("Unexpected status update, got:", s1)
	}

	// Test that service error triggers failure notification.
	go plugin.oneShot(ctx, download.Update{Error: fmt.Errorf("some error")})
	s2 := <-ch

	if s2.ActiveRevision != "quickbrownfaux" || s2.Code == "" {
		t.Fatal("Unexpected status update, got:", s2)
	}

	// Test that service recovery triggers healthy notification.
	go plugin.oneShot(ctx, download.Update{})
	s3 := <-ch

	if s3.ActiveRevision != "quickbrownfaux" || s3.Code != "" {
		t.Fatal("Unexpected status update, got:", s3)
	}
}

func getTestManager() *plugins.Manager {
	store := inmem.New()
	manager, err := plugins.New(nil, "test-instance-id", store)
	if err != nil {
		panic(err)
	}
	return manager
}
