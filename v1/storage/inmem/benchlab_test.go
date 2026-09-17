// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.
//
// Every case costs the same fixed slice of wall clock, whatever it measures.
// Each family below is measured against all three store representations, which
// is the comparison worth having here and is left intact; what is cut is
// families that re-measure a neighbour, and two of MakeDirSiblings' three
// sizes.

package inmem_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/v1/storage"
)

// --- transactions -------------------------------------------------------

func BenchmarkBenchlabNewTransaction(b *testing.B) {
	BenchmarkNewTransaction(b)
}

// --- reads --------------------------------------------------------------

// The in-transaction read hot path. BenchmarkReadOne is left out: it is this
// plus a transaction, and BenchmarkBenchlabNewTransaction prices that already.
func BenchmarkBenchlabRead(b *testing.B) {
	BenchmarkRead(b)
}

// Path depth, and with it the ast.Value boxing of each path element.
func BenchmarkBenchlabReadOneNested(b *testing.B) {
	BenchmarkReadOneNested(b)
}

func BenchmarkBenchlabReadNotFound(b *testing.B) {
	BenchmarkReadNotFound(b)
}

// --- writes -------------------------------------------------------------

func BenchmarkBenchlabWriteOneString(b *testing.B) {
	BenchmarkWriteOneString(b)
}

// A pair: the same write with and without interning of path and value. Neither
// arm says much without the other.
func BenchmarkBenchlabWriteIncrementingValueSamePath(b *testing.B) {
	BenchmarkWriteIncrementingValueSamePath(b)
}

func BenchmarkBenchlabWriteIncrementingValueSamePathInterned(b *testing.B) {
	BenchmarkWriteIncrementingValueSamePathInterned(b)
}

// Writing a composite value rather than a scalar.
func BenchmarkBenchlabWriteCollection(b *testing.B) {
	BenchmarkWriteCollection(b)
}

// --- commit -------------------------------------------------------------

func BenchmarkBenchlabWriteAndCommit(b *testing.B) {
	BenchmarkWriteAndCommit(b)
}

// What a registered trigger adds to a commit.
func BenchmarkBenchlabWriteAndCommitWithTriggers(b *testing.B) {
	BenchmarkWriteAndCommitWithTriggers(b)
}

// --- directories --------------------------------------------------------

// MakeDir over a parent that already holds many siblings, at the largest of the
// three sizes; the sweep is otherwise linear in the sibling count.
func BenchmarkBenchlabMakeDirSiblings(b *testing.B) {
	const n = 1000

	b.Run(strconv.Itoa(n)+"_existing", func(b *testing.B) {
		existing := make(map[string]any, n)
		for i := range n {
			existing[strconv.Itoa(i)] = map[string]any{"v": i}
		}
		data := map[string]any{"parent": existing}

		paths := make([]storage.Path, 100)
		for i := range paths {
			paths[i] = storage.Path{"parent", "new-" + strconv.Itoa(i)}
		}

		// Abort after each iteration so the store stays unmodified and
		// subsequent iterations exercise the full MakeDir path.
		operation := func(ctx context.Context, target *target) error {
			txn, err := target.store.NewTransaction(ctx, storage.WriteParams)
			if err != nil {
				return err
			}
			for _, p := range paths {
				if err := storage.MakeDir(ctx, target.store, txn, p); err != nil {
					target.store.Abort(ctx, txn)
					return err
				}
			}
			target.store.Abort(ctx, txn)
			return nil
		}

		AllStores(data).Bench(b, operation)
	})
}
