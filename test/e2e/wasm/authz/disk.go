// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build bench_disk
// +build bench_disk

// nolint: deadcode,unused // build tags confuse these linters
package authz

import (
	"os"

	"github.com/open-policy-agent/opa/storage/disk"
)

func diskStorage() (*disk.Options, func() error) {
	dir, err := os.CreateTemp("", "disk-store")
	if err != nil {
		panic(err)
	}

	return &disk.Options{Dir: dir, Partitions: nil}, func() error { return os.RemoveAll(dir) }
}
