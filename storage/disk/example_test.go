// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk_test

import (
	"context"
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/disk"
	"github.com/open-policy-agent/opa/util"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func Example_store() {

	ctx := context.Background()

	// Create a temporary directory for store.
	dir, err := os.MkdirTemp("", "opa_disk_example")
	check(err)

	// Cleanup temporary directory after finishing.
	defer os.RemoveAll(dir)

	// Create a new disk-based store.
	store, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
		Dir: dir,
		Partitions: []storage.Path{
			storage.MustParsePath("/authz/tenants"),
		},
	})
	check(err)

	// Insert data into the store. The `storage.WriteOne` function automatically
	// opens a write transaction, applies the operation, and commits the
	// transaction in one-shot.
	err = storage.WriteOne(ctx, store, storage.AddOp, storage.MustParsePath("/"), util.MustUnmarshalJSON([]byte(`{
		"authz": {
			"tenants": {
				"acmecorp.openpolicyagent.org": {
					"tier": "gold"
				},
				"globex.openpolicyagent.org" :{
					"tier": "silver"
				}
			}
		}
	}`)))
	check(err)

	// Close the store so that it can be reopened.
	err = store.Close(ctx)
	check(err)

	// Re-create the disk-based store using the same options.
	store2, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
		Dir: dir,
		Partitions: []storage.Path{
			storage.MustParsePath("/authz/tenants"),
		},
	})
	check(err)

	// Read value persisted above and inspect the result.
	value, err := storage.ReadOne(ctx, store2, storage.MustParsePath("/authz/tenants/acmecorp.openpolicyagent.org"))
	check(err)

	err = store2.Close(ctx)
	check(err)

	fmt.Println(value)

	// Output:
	//
	// map[tier:gold]
}
