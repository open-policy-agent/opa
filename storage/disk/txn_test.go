// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"context"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
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

func fixture(nbKeys int) interface{} {
	i := 1
	var keyValues = []string{}
	for i <= nbKeys {
		keyValues = append(keyValues, "\""+strconv.Itoa(i)+randomString(4)+"\": \""+randomString(3)+"\"")
		i++
	}
	jsonBytes := []byte(`{"foo":{` + strings.Join(keyValues, ",") + `}}`)
	return util.MustUnmarshalJSON(jsonBytes)
}

func TestSetTxnIsTooBigToFitIntoOneRequestWhenUseDiskStore(t *testing.T) {
	test.WithTempFS(map[string]string{}, func(dir string) {
		ctx := context.Background()
		s, err := New(ctx, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo"),
		}})
		if err != nil {
			t.Fatal(err)
		}

		nbKeys := 140_000 // !!! 135_000 it's ok, but 140_000 not
		jsonFixture := fixture(nbKeys)
		errTxn := storage.Txn(ctx, s, storage.WriteParams, func(txn storage.Transaction) error {

			errTxnWrite := s.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/"), jsonFixture)
			if errTxnWrite != nil {
				t.Fatal(errTxnWrite)
			}
			return nil
		})
		if errTxn != nil {
			t.Fatal(errTxn)
		}

		result, errRead := storage.ReadOne(ctx, s, storage.MustParsePath("/foo"))
		if errRead != nil {
			t.Fatal(errRead)
		}
		actualNbKeys := len(result.(map[string]interface{}))
		if nbKeys != actualNbKeys {
			t.Fatalf("Expected %d keys, read %d", nbKeys, actualNbKeys)
		}
	})

}

func TestDeleteTxnIsTooBigToFitIntoOneRequestWhenUseDiskStore(t *testing.T) {
	test.WithTempFS(map[string]string{}, func(dir string) {
		ctx := context.Background()
		s, err := New(ctx, Options{Dir: dir, Partitions: []storage.Path{
			storage.MustParsePath("/foo"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		nbKeys := 200_000
		jsonFixture := fixture(nbKeys)
		errTxn := storage.Txn(ctx, s, storage.WriteParams, func(txn storage.Transaction) error {

			errTxnWrite := s.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/"), jsonFixture)
			if errTxnWrite != nil {
				t.Fatal(errTxnWrite)
			}
			return nil
		})
		if errTxn != nil {
			t.Fatal(errTxn)
		}

		errTxnD := storage.Txn(ctx, s, storage.WriteParams, func(txn storage.Transaction) error {
			errTxnWrite := s.Write(ctx, txn, storage.RemoveOp, storage.MustParsePath("/foo"), jsonFixture)
			if errTxnWrite != nil {
				t.Fatal(errTxnWrite)
			}
			return nil
		})
		if errTxnD != nil {
			t.Fatal(errTxnD)
		}

		results, errRead := storage.ReadOne(ctx, s, storage.MustParsePath("/foo"))
		if !storage.IsNotFound(errRead) {
			t.Fatal(errRead)
		}
		if results != nil {
			t.Fatalf("Unexpected results %v", results)
		}
	})

}
