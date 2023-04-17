// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/dgraph-io/badger/v3"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util/test"
)

func randomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func fixture(n int) map[string]interface{} {
	foo := map[string]string{}
	for i := 0; i < n; i++ {
		foo[fmt.Sprintf(`"%d%s"`, i, randomString(4))] = randomString(3)
	}
	return map[string]interface{}{"foo": foo}
}

func TestSetTxnIsTooBigToFitIntoOneRequestWhenUseDiskStoreReturnsError(t *testing.T) {
	test.WithTempFS(nil, func(dir string) {
		ctx := context.Background()
		s, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo"),
		}})
		if err != nil {
			t.Fatal(err)
		}

		nbKeys := 140_000 // 135_000 is ok, but 140_000 not
		jsonFixture := fixture(nbKeys)
		err = storage.Txn(ctx, s, storage.WriteParams, func(txn storage.Transaction) error {
			err := s.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/"), jsonFixture)
			if !errors.Is(err, badger.ErrTxnTooBig) {
				t.Errorf("expected %v, got %v", badger.ErrTxnTooBig, err)
			}
			return err
		})
		if !errors.Is(err, badger.ErrTxnTooBig) {
			t.Errorf("expected %v, got %v", badger.ErrTxnTooBig, err)
		}

		_, err = storage.ReadOne(ctx, s, storage.MustParsePath("/foo"))
		var notFound *storage.Error
		ok := errors.As(err, &notFound)
		if !ok {
			t.Errorf("expected %T, got %v", notFound, err)
		}
		if exp, act := storage.NotFoundErr, notFound.Code; exp != act {
			t.Errorf("expected code %v, got %v", exp, act)
		}
	})

}

func TestDeleteTxnIsTooBigToFitIntoOneRequestWhenUseDiskStore(t *testing.T) {
	test.WithTempFS(nil, func(dir string) {
		ctx := context.Background()
		s, err := New(ctx, logging.NewNoOpLogger(), nil, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		nbKeys := 200_000
		jsonFixture := fixture(nbKeys)
		foo := jsonFixture["foo"].(map[string]string)

		// Write data in increments so we don't step over the too-large-txn limit
		for k, v := range foo {
			err := storage.WriteOne(ctx, s, storage.AddOp, storage.MustParsePath("/foo/"+k), v)
			if err != nil {
				t.Fatal(err)
			}
		}

		// check expected state
		res, err := storage.ReadOne(ctx, s, storage.MustParsePath("/foo"))
		if err != nil {
			t.Fatal(err)
		}
		if exp, act := nbKeys, len(res.(map[string]interface{})); exp != act {
			t.Fatalf("expected %d keys, read %d", exp, act)
		}

		err = storage.Txn(ctx, s, storage.WriteParams, func(txn storage.Transaction) error {
			err := s.Write(ctx, txn, storage.RemoveOp, storage.MustParsePath("/foo"), jsonFixture)
			if !errors.Is(err, badger.ErrTxnTooBig) {
				t.Errorf("expected %v, got %v", badger.ErrTxnTooBig, err)
			}
			return err
		})
		if !errors.Is(err, badger.ErrTxnTooBig) {
			t.Errorf("expected %v, got %v", badger.ErrTxnTooBig, err)
		}

		// check expected state again
		res, err = storage.ReadOne(ctx, s, storage.MustParsePath("/foo"))
		if err != nil {
			t.Fatal(err)
		}
		if exp, act := nbKeys, len(res.(map[string]interface{})); exp != act {
			t.Fatalf("expected %d keys, read %d", exp, act)
		}

	})

}
