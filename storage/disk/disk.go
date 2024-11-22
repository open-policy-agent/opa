// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package disk provides disk-based implementation of the storage.Store
// interface.
//
// The disk.Store implementation uses an embedded key-value store to persist
// policies and data. Policy modules are stored as raw byte strings with one
// module per key. Data is mapped to the underlying key-value store with the
// assistance of caller-supplied "partitions". Partitions allow the caller to
// control the portions of the /data namespace that are mapped to individual
// keys. Operations that span multiple keys (e.g., a read against the entirety
// of /data) are more expensive than reads that target a specific key because
// the storage layer has to reconstruct the object from individual key-value
// pairs and page all of the data into memory. By supplying partitions that
// align with lookups in the policies, callers can optimize policy evaluation.
//
// Partitions are specified as a set of storage paths (e.g., {/foo/bar} declares
// a single partition at /foo/bar). Each partition tells the store that values
// under the partition path should be mapped to individual keys. Values that
// fall outside of the partitions are stored at adjacent keys without further
// splitting. For example, given the partition set {/foo/bar}, /foo/bar/abcd and
// /foo/bar/efgh are be written to separate keys. All other values under /foo
// are not split any further (e.g., all values under /foo/baz would be written
// to a single key). Similarly, values that fall outside of partitions are
// stored under individual keys at the root (e.g., the full extent of the value
// at /qux would be stored under one key.)
// There is support for wildcards in partitions: {/foo/*} will cause /foo/bar/abc
// and /foo/buz/def to be written to separate keys. Multiple wildcards are
// supported (/tenants/*/users/*/bindings), and they can also appear at the end
// of a partition (/users/*).
//
// All keys written by the disk.Store implementation are prefixed as follows:
//
//	/<schema_version>/<partition_version>/<type>
//
// The <schema_version> value represents the version of the schema understood by
// this version of OPA. Currently this is always set to 1. The
// <partition_version> value represents the version of the partition layout
// supplied by the caller. Currently this is always set to 1. Currently, the
// disk.Store implementation only supports _additive_ changes to the
// partitioning layout, i.e., new partitions can be added as long as they do not
// overlap with existing unpartitioned data. The <type> value is either "data"
// or "policies" depending on the value being stored.
//
// The disk.Store implementation attempts to be compatible with the inmem.store
// implementation however there are some minor differences:
//
// * Writes that add partitioned values implicitly create an object hierarchy
// containing the value (e.g., `add /foo/bar/abcd` implicitly creates the
// structure `{"foo": {"bar": {"abcd": ...}}}`). This is unavoidable because of
// how nested /data values are mapped to key-value pairs.
//
// * Trigger events do not include a set of changed paths because the underlying
// key-value store does not make them available.
package disk

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/open-policy-agent/opa/logging"
	v1 "github.com/open-policy-agent/opa/v1/storage/disk"
)

// Options contains parameters that configure the disk-based store.
type Options = v1.Options

// Store provides a disk-based implementation of the storage.Store interface.
type Store = v1.Store

// New returns a new disk-based store based on the provided options.
func New(ctx context.Context, logger logging.Logger, prom prometheus.Registerer, opts Options) (*Store, error) {
	return v1.New(ctx, logger, prom, opts)
}
