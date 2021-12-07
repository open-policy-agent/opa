// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util/test"
)

func TestReadAValidFile(t *testing.T) {
	files := map[string]string{
		"token": `ZXlKaGJHY2lPaUpTV`,
	}
	test.WithTempFS(files, func(root string) {
		ctx := context.Background()
		compiler := ast.NewCompiler()
		query, err := compiler.QueryCompiler().Compile(ast.MustParseBody(`x = fs.readfile(data.file)`))
		if err != nil {
			panic(err)
		}
		var data map[string]interface{}
		decoder := json.NewDecoder(bytes.NewBufferString(fmt.Sprintf(`{"file": "%s/token"}`, root)))

		if err := decoder.Decode(&data); err != nil {
			panic(err)
		}
		store := inmem.NewFromObject(data)
		txn, err := store.NewTransaction(ctx)
		if err != nil {
			panic(err)
		}
		defer store.Abort(ctx, txn)
		q := topdown.NewQuery(query).
			WithCompiler(compiler).
			WithStore(store).
			WithTransaction(txn)

		result := []interface{}{}

		err = q.Iter(ctx, func(qr topdown.QueryResult) error {
			x := qr[ast.Var("x")]
			v, err := ast.JSON(x.Value)
			if err != nil {
				panic(err)
			}

			result = append(result, v)
			return nil
		})
		if err != nil {
			panic(err)
		}

		if result[0].(string) != files["token"] {
			t.Errorf("Variable x should be the file content %s", files["token"])
		}
	})
}
