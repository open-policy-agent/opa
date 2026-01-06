package inmem_test

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/util"
)

type (
	target struct {
		store           storage.Store
		txn             storage.Transaction
		name            string
		isAST           bool
		isRoundTripping bool
	}
	targets []*target
	benchFn func(ctx context.Context, target *target) error
)

var (
	path        = storage.Path{"foo"}
	readTxn     = storage.TransactionParams{Write: false}
	writeTxn    = storage.TransactionParams{Write: true}
	paramsRead  = []storage.TransactionParams{readTxn}
	paramsWrite = []storage.TransactionParams{writeTxn}
)

func AllStores(data map[string]any) targets {
	return []*target{
		{
			name:  "Go",
			store: inmem.NewFromObjectWithOpts(data, inmem.OptRoundTripOnWrite(false)),
		},
		{
			name:            "Go (roundtrip)",
			store:           inmem.NewFromObjectWithOpts(data, inmem.OptRoundTripOnWrite(true)),
			isRoundTripping: true,
		},
		{
			name:  "AST",
			store: inmem.NewFromObjectWithOpts(data, inmem.OptReturnASTValuesOnRead(true)),
			isAST: true,
		},
	}
}

// Read:     18.29 ns/op      48 B/op       1 allocs/op
// Write:    18.41 ns/op      48 B/op       1 allocs/op
func BenchmarkNewTransaction(b *testing.B) {
	store := inmem.NewFromObject(map[string]any{})

	for name, params := range map[string][]storage.TransactionParams{"read": paramsRead, "write": paramsWrite} {
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				txn, _ := store.NewTransaction(b.Context(), params...)
				store.Abort(b.Context(), txn)
			}
		})
	}
}

// Go:     27.86 ns/op      48 B/op       1 allocs/op (cost of txn)
// AST:    47.84 ns/op      64 B/op       2 allocs/op (cost of txn + ast.Value boxing of path element)
func BenchmarkReadOne(b *testing.B) {
	operation := func(ctx context.Context, target *target) error {
		return onlyError(storage.ReadOne(b.Context(), target.store, path))
	}

	AllStores(map[string]any{"foo": "bar"}).Bench(b, operation)
}

// Go:     27.86 ns/op      48 B/op       1 allocs/op (cost of txn)
// AST:    99.63 ns/op      96 B/op       4 allocs/op (cost of txn + ast.Value boxing of each path element)
func BenchmarkReadOneNested(b *testing.B) {
	path := storage.Path{"foo", "bar", "baz"}
	operation := func(ctx context.Context, target *target) error {
		return onlyError(storage.ReadOne(b.Context(), target.store, path))
	}

	AllStores(map[string]any{"foo": map[string]any{"bar": map[string]any{"baz": "qux"}}}).Bench(b, operation)
}

// Go:     10.67 ns/op       0 B/op       0 allocs/op
// AST:    30.64 ns/op      16 B/op       1 allocs/op (ast.Value boxing of path element)
func BenchmarkRead(b *testing.B) {
	operation := func(ctx context.Context, target *target) error {
		return onlyError(target.store.Read(ctx, target.txn, path))
	}

	AllStores(map[string]any{"foo": "bar"}).BenchWithTxn(b, readTxn, operation)
}

// Go:     71.29 ns/op       0 B/op       0 allocs/op
// AST:    94.69 ns/op      16 B/op       1 allocs/op allocs/op (ast.Value boxing path element)
func BenchmarkReadNotFound(b *testing.B) {
	operation := func(ctx context.Context, target *target) error {
		_ = onlyError(target.store.Read(ctx, target.txn, path))
		return nil
	}

	AllStores(map[string]any{}).BenchWithTxn(b, readTxn, operation)
}

// (roundtrip not needed to write string)
// Go:     123.4 ns/op       288 B/op       6 allocs/op
// AST:    262.6 ns/op       408 B/op      12 allocs/op
func BenchmarkWriteOneString(b *testing.B) {
	operation := func(ctx context.Context, target *target) error {
		return storage.WriteOne(ctx, target.store, storage.AddOp, path, "bar")
	}

	AllStores(map[string]any{}).Bench(b, operation).VerifyRead(b, path, "bar")
}

// All stores        10.95 ns/op        0 B/op       0 allocs/op
func BenchmarkWriteSameValue(b *testing.B) {
	operation := func(ctx context.Context, target *target) error {
		return target.store.Write(ctx, target.txn, storage.ReplaceOp, path, "bar")
	}

	AllStores(map[string]any{"foo": "bar"}).BenchWithTxn(b, writeTxn, operation).VerifyRead(b, path, "bar")
}

func BenchmarkWriteIncrementingValueSamePath(b *testing.B) {
	operation := func(ctx context.Context, target *target) error {
		for i := range 100 {
			if err := target.store.Write(ctx, target.txn, storage.AddOp, path, i); err != nil {
				return err
			}
		}
		return nil
	}

	AllStores(map[string]any{}).BenchWithTxn(b, writeTxn, operation).VerifyRead(b, path, 99)
}

// Half as expensive as BenchmarkWriteIncrementingValueSamePath due to interned string term
func BenchmarkWriteIncrementingValueSamePathInterned(b *testing.B) {
	path := storage.Path{"var"} // known interned string term
	operation := func(ctx context.Context, target *target) error {
		for i := range 100 {
			if err := target.store.Write(ctx, target.txn, storage.AddOp, path, i); err != nil {
				return err
			}
		}
		return nil
	}

	AllStores(map[string]any{}).BenchWithTxn(b, writeTxn, operation).VerifyRead(b, path, 99)
}

// All stores should perform identically as the only remaining cost
// is that of the data update getting appended to the updates list
func BenchmarkPathAndValueInternedAndNoRoundtripRequired(b *testing.B) {
	paths := make([]storage.Path, 100)
	for i := range 100 {
		paths[i] = storage.Path{strconv.Itoa(i)}
	}

	operation := func(ctx context.Context, target *target) error {
		for i := range 100 {
			if err := target.store.Write(ctx, target.txn, storage.AddOp, paths[i], paths[i][0]); err != nil {
				return err
			}
		}
		return nil
	}

	AllStores(map[string]any{}).BenchWithTxn(b, writeTxn, operation).VerifyRead(b, paths[99], "99")
}

// Go no_roundtrip             -            -                 - (not relevant as input needs roundtrip)
// Go roundtrip       1144 ns/op    2297 B/op      35 allocs/op
// AST                1811 ns/op    3554 B/op      63 allocs/op
func BenchmarkWriteCollection(b *testing.B) {
	value := map[string]any{"a": 1, "b": []any{1, 2, 3}, "c": map[string]any{"d": "e"}}
	operation := func(ctx context.Context, target *target) error {
		return target.store.Write(b.Context(), target.txn, storage.AddOp, path, value)
	}

	AllStores(map[string]any{}).BenchWithTxn(b, writeTxn, operation).VerifyRead(b, path, value)
}

// Go          48750 ns/op   27040 B/op    311 allocs/op
// AST        188462 ns/op   31121 B/op    515 allocs/op
func BenchmarkWriteAndCommit(b *testing.B) {
	paths := make([]storage.Path, 100)
	for i := range 100 {
		paths[i] = storage.Path{strconv.Itoa(i)}
	}

	operation := func(ctx context.Context, target *target) error {
		txn, _ := target.store.NewTransaction(b.Context(), storage.WriteParams)
		for i := range 100 {
			val := paths[i][0]
			if err := target.store.Write(b.Context(), txn, storage.AddOp, paths[i], val); err != nil {
				return err
			}
		}

		return target.store.Commit(b.Context(), txn)
	}

	AllStores(map[string]any{}).Bench(b, operation)
}

// Go          48399 ns/op   16209 B/op     304 allocs/op (no additional cost of triggers)
// AST        191501 ns/op   26673 B/op     605 allocs/op (extra cost due to converting back to Go values)
func BenchmarkWriteAndCommitWithTriggers(b *testing.B) {
	paths := make([]storage.Path, 100)
	for i := range 100 {
		paths[i] = storage.Path{strconv.Itoa(i)}
	}

	operation := func(ctx context.Context, target *target) error {
		txn, _ := target.store.NewTransaction(b.Context(), storage.WriteParams)
		for i := range 100 {
			if err := target.store.Write(b.Context(), txn, storage.AddOp, paths[i], paths[i][0]); err != nil {
				return err
			}
		}

		return target.store.Commit(b.Context(), txn)
	}

	trigger := storage.TriggerConfig{OnCommit: func(context.Context, storage.Transaction, storage.TriggerEvent) {}}

	AllStores(map[string]any{}).
		SetupWithTxn(b, writeTxn, func(ctx context.Context, target *target) error {
			return onlyError(target.store.Register(b.Context(), target.txn, trigger))
		}).
		Bench(b, operation)
}

// AST        189485 ns/op   17011 B/op     304 allocs/op
func BenchmarkWriteAndCommitWithTriggersSkipConversion(b *testing.B) {
	paths := make([]storage.Path, 100)
	for i := range 100 {
		paths[i] = storage.Path{strconv.Itoa(i)}
	}
	values := make([]ast.Value, 100)
	for i := range 100 {
		values[i] = ast.String(paths[i][0])
	}

	operation := func(ctx context.Context, target *target) error {
		txn, _ := target.store.NewTransaction(b.Context(), storage.WriteParams)
		for i := range 100 {
			if err := target.store.Write(b.Context(), txn, storage.AddOp, paths[i], values[i]); err != nil {
				return err
			}
		}

		return target.store.Commit(b.Context(), txn)
	}

	triggerCount := 0

	trigger := storage.TriggerConfig{
		SkipDataConversion: true,
		OnCommit: func(ctx context.Context, txn storage.Transaction, event storage.TriggerEvent) {
			if event.DataChanged() {
				if len(event.Data) != 100 {
					b.Fatalf("Expected 100 data changes but got: %d", len(event.Data))
				}
				if _, ok := event.Data[0].Data.(ast.Value); !ok {
					b.Fatalf("Expected ast.Value data but got: %T", event.Data[0].Data)
				}
				triggerCount++
			}
		},
	}

	onlyAstStores := targets{{
		name:  "AST",
		store: inmem.NewFromObjectWithOpts(map[string]any{}, inmem.OptReturnASTValuesOnRead(true)),
		isAST: true,
	}}

	onlyAstStores.
		SetupWithTxn(b, writeTxn, func(ctx context.Context, target *target) error {
			return onlyError(target.store.Register(b.Context(), target.txn, trigger))
		}).
		Bench(b, operation)

	if triggerCount == 0 {
		b.Fatalf("Expected trigger to be called at least once")
	}
}

func (t targets) VerifyRead(b *testing.B, path storage.Path, expected any) targets {
	b.Helper()
	for _, target := range t {
		target.VerifyRead(b, path, expected)
	}

	return t
}

func (t targets) Bench(b *testing.B, fn benchFn) targets {
	b.Helper()
	for _, target := range t {
		b.Run(target.name, func(b *testing.B) {
			for b.Loop() {
				must(b, fn(b.Context(), target))
			}
		})
	}

	return t
}

func (t targets) BenchWithTxn(b *testing.B, params storage.TransactionParams, fn benchFn) targets {
	b.Helper()
	for _, target := range t {
		b.Run(target.name, func(b *testing.B) {
			must(b, storage.Txn(b.Context(), target.store, params, func(txn storage.Transaction) error {
				target.txn = txn
				for b.Loop() {
					must(b, fn(b.Context(), target))
				}
				return nil
			}))
		})
	}

	return t
}

func (t targets) SetupWithTxn(b *testing.B, params storage.TransactionParams, fn benchFn) targets {
	b.Helper()
	for _, target := range t {
		must(b, storage.Txn(b.Context(), target.store, params, func(txn storage.Transaction) error {
			target.txn = txn
			must(b, fn(b.Context(), target))
			return nil
		}))
	}

	return t
}

func (t *target) VerifyRead(b *testing.B, path storage.Path, expected any) {
	b.Helper()

	must(b, storage.Txn(b.Context(), t.store, readTxn, func(txn storage.Transaction) error {
		act, err := t.store.Read(b.Context(), txn, path)
		if err != nil {
			b.Fatalf("store: %s: unexpected error: %v", t.name, err)
		}

		if t.isAST {
			expectedAST := ast.MustInterfaceToValue(expected)
			if actAST := act.(ast.Value); actAST.Compare(expectedAST) != 0 {
				b.Fatalf("store: %s: expected: %v, got: %v", t.name, expectedAST, actAST)
			}
		} else {
			if t.isRoundTripping {
				cpy := expected
				if must(b, util.RoundTrip(&cpy)); !reflect.DeepEqual(act, cpy) {
					b.Fatalf("store: %s: expected: %v, got: %v", t.name, cpy, act)
				}
			} else if !reflect.DeepEqual(act, expected) {
				b.Fatalf("store: %s: expected: %v, got: %v", t.name, expected, act)
			}
		}

		return nil
	}))

}

func onlyError[T any](_ T, err error) error {
	return err
}

func must(b *testing.B, err error) {
	b.Helper()
	if err != nil {
		b.Fatal(err)
	}
}
