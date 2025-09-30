package inmem_test

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

type (
	txnType bool
	target  struct {
		name  string
		store storage.Store
	}
	targets []target

	withTxnFn func(b *testing.B, target storage.Store, txn storage.Transaction)
	noTxnFn   func(b *testing.B, target storage.Store)
)

const (
	readTxn  txnType = false
	writeTxn txnType = true
)

func BenchmarkNewTransaction(b *testing.B) {
	store := inmem.NewFromObject(map[string]any{})

	for name, typ := range map[string]txnType{"Read": readTxn, "Write": writeTxn} {
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				txn, err := store.NewTransaction(b.Context(), transactionParams(typ)...)
				if err != nil {
					b.Fatal(err)
				}
				store.Abort(b.Context(), txn)
			}
		})
	}
}

func BenchmarkReadOne(b *testing.B) {
	data := map[string]any{"foo": "bar"}
	path := storage.Path{"foo"}

	AllStores(data).Bench(b, func(b *testing.B, store storage.Store) {
		if _, err := storage.ReadOne(b.Context(), store, path); err != nil {
			b.Fatal(err)
		}
	})
}

func BenchmarkRead(b *testing.B) {
	data := map[string]any{"foo": "bar"}
	path := storage.Path{"foo"}

	AllStores(data).BenchWithTxn(b, readTxn, func(b *testing.B, s storage.Store, txn storage.Transaction) {
		if _, err := s.Read(b.Context(), txn, path); err != nil {
			b.Fatal(err)
		}
	})
}

func BenchmarkWriteOne(b *testing.B) {
	data := map[string]any{}
	path := storage.Path{"foo"}

	AllStores(data).Bench(b, func(b *testing.B, store storage.Store) {
		if err := storage.WriteOne(b.Context(), store, storage.AddOp, path, "bar"); err != nil {
			b.Fatal(err)
		}
	})
}

func transactionParams(mode txnType) (params []storage.TransactionParams) {
	if mode == writeTxn {
		params = append(params, storage.WriteParams)
	}
	return params
}

func AllStores(data map[string]any) targets {
	return []target{
		{
			"Go store (roundtrip)",
			inmem.NewFromObjectWithOpts(data, inmem.OptRoundTripOnWrite(true)),
		},
		{
			"Go store (no roundtrip)",
			inmem.NewFromObjectWithOpts(data, inmem.OptRoundTripOnWrite(false)),
		},
		{
			"AST store (roundtrip)",
			inmem.NewFromObjectWithOpts(data, inmem.OptReturnASTValuesOnRead(true), inmem.OptRoundTripOnWrite(true)),
		},
		{
			"AST store (no roundtrip)",
			inmem.NewFromObjectWithOpts(data, inmem.OptReturnASTValuesOnRead(true), inmem.OptRoundTripOnWrite(false)),
		},
	}
}

func (t targets) Bench(b *testing.B, fn noTxnFn) {
	b.Helper()

	for _, target := range t {
		b.Run(target.name, func(b *testing.B) {
			for b.Loop() {
				fn(b, target.store)
			}
		})
	}
}

func (t targets) BenchWithTxn(b *testing.B, mode txnType, fn withTxnFn) {
	b.Helper()

	for _, target := range t {
		b.Run(target.name, func(b *testing.B) {
			txn := storage.NewTransactionOrDie(b.Context(), target.store, transactionParams(mode)...)

			for b.Loop() {
				fn(b, target.store, txn)
			}

			target.store.Abort(b.Context(), txn)
		})
	}
}
