// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// storage read transactions never delete or write
	keysReadPerStoreRead  = newHist("keys_read_per_store_read_txn", "How many database reads had to occur for a storage read transaction")
	bytesReadPerStoreRead = newHist("key_bytes_read_per_store_read_txn", "How many bytes of data were read for a storage read transaction")

	keysReadPerStoreWrite    = newHist("keys_read_per_store_write_txn", "How many database reads had to occur for a storage write transaction")
	keysWrittenPerStoreWrite = newHist("keys_written_per_store_write_txn", "How many database writes had to occur for a storage write transaction")
	keysDeletedPerStoreWrite = newHist("keys_deleted_per_store_write_txn", "How many database writes had to occur for a storage write transaction")
	bytesReadPerStoreWrite   = newHist("key_bytes_read_per_store_write_txn", "How many bytes of data were read for a storage write transaction")
)

func initPrometheus(reg prometheus.Registerer) error {
	for _, hist := range []prometheus.Histogram{
		keysReadPerStoreRead,
		bytesReadPerStoreRead,
		keysReadPerStoreWrite,
		keysWrittenPerStoreWrite,
		keysDeletedPerStoreWrite,
		bytesReadPerStoreWrite,
	} {
		if err := reg.Register(hist); err != nil {
			return err
		}
	}
	return nil
}

func newHist(name, desc string) prometheus.Histogram {
	return prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    name,
		Help:    desc,
		Buckets: prometheus.LinearBuckets(1, 1, 10), // TODO different buckets? exp?
	})
}

func forwardMetric(m map[string]interface{}, counter string, hist prometheus.Histogram) {
	key := "counter_" + counter
	if s, ok := m[key]; ok {
		hist.Observe(float64(s.(uint64)))
	}
}
