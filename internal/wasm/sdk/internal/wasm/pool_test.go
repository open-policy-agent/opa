// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build opa_wasm
// +build opa_wasm

package wasm_test

import (
	"context"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/compile"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/internal/wasm"
	wasm_util "github.com/open-policy-agent/opa/internal/wasm/util"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/open-policy-agent/opa/topdown/cache"
	"github.com/open-policy-agent/opa/util"
)

func TestOpaEvalGrowMemoryForLargeInput(t *testing.T) {
	ctx := context.Background()
	module := `package test
	p = true
	`
	data := []byte(`{}`)
	s := strings.Repeat("a", 16*wasm_util.PageSize)
	input := interface{}([]byte(s))

	poolSize := 1
	testPool := initPoolWithData(t, uint32(poolSize), module, "test/p", data)
	expected := `{{"result":true}}`
	ensurePoolResults(t, ctx, testPool, poolSize, &input, expected)
}

func TestPoolCopyParsedDataOnInit(t *testing.T) {
	ctx := context.Background()
	module := `package test

	p = data.a
	`
	data := []byte(`{
  "a": {
    "b": [
      1,
      2,
      3,
      {
        "c": 4,
        "d": {
          "e": {
            "f": 123
          }
        }
      }
    ]
  }
}`)

	poolSize := 4
	testPool := initPoolWithData(t, uint32(poolSize), module, "test/p", data)
	expected := `{{"result":{"b":[1,2,3,{"d":{"e":{"f":123}},"c":4}]}}}`
	ensurePoolResults(t, ctx, testPool, poolSize, nil, expected)
}

func TestPoolCopyParsedDataUpdateFull(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	module := `package test

	p = data.a
	`
	data := []byte(`{"a": 123}`)

	poolSize := 4
	testPool := initPoolWithData(t, uint32(poolSize), module, "test/p", data)

	updated := []byte(`{"a": {"x": 123, "y": "bar"}}`)
	err := testPool.SetPolicyData(ctx, testPool.Policy(), updated)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	expected := `{{"result":{"y":"bar","x":123}}}`
	ensurePoolResults(t, ctx, testPool, poolSize, nil, expected)

	// Change it one more time, now that all VM's in the pool have been
	// initialized and exercised at least once.
	updated = []byte(`{"a": [1, 2, 3]}`)
	err = testPool.SetPolicyData(ctx, testPool.Policy(), updated)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	expected = `{{"result":[1,2,3]}}`
	ensurePoolResults(t, ctx, testPool, poolSize, nil, expected)
}

func TestPoolCopyParsedDataUpdatePartial(t *testing.T) {
	module := `package test

	p = data.a
	`
	data := []byte(`{}`)
	poolSize := 4
	testPool := initPoolWithData(t, uint32(poolSize), module, "test/p", data)

	// Each case is applied in order to the original dataset
	cases := []struct {
		note     string
		update   interface{}
		path     []string
		remove   bool
		expected string
	}{
		{
			note:     "add object",
			update:   util.MustUnmarshalJSON([]byte(`{"foo": 123}`)),
			path:     []string{"a"},
			expected: `{{"result":{"foo":123}}}`,
		},
		{
			note:     "remove path",
			path:     []string{"a", "foo"},
			remove:   true,
			expected: `{{"result":{}}}`,
		},
		{
			note:     "add set",
			update:   ast.MustParseTerm(`{"x": {"y": {"z"}}}`),
			path:     []string{"a", "b", "c"},
			expected: `{{"result":{"b":{"c":{"x":{"y":{"z"}}}}}}}`,
		},
		{
			note:     "remove set",
			path:     []string{"a", "b", "c", "x", "y"},
			remove:   true,
			expected: `{{"result":{"b":{"c":{"x":{}}}}}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var err error
			if tc.remove {
				err = testPool.RemoveDataPath(ctx, tc.path)
			} else {
				err = testPool.SetDataPath(ctx, tc.path, tc.update)
			}

			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			ensurePoolResults(t, ctx, testPool, poolSize, nil, tc.expected)
		})
	}
}

func ensurePoolResults(t *testing.T, ctx context.Context, testPool *wasm.Pool, poolSize int, input *interface{}, expected string) {
	t.Helper()
	var toRelease []*wasm.VM
	for i := 0; i < poolSize; i++ {
		vm, err := testPool.Acquire(ctx, metrics.New())
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		toRelease = append(toRelease, vm)

		cfg, _ := cache.ParseCachingConfig(nil)
		result, err := vm.Eval(ctx, 0, input, metrics.New(), rand.New(rand.NewSource(0)), time.Now(), cache.NewInterQueryCache(cfg), builtins.NDBCache{}, nil, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if string(result) != expected {
			t.Fatalf("Incorrect result for VM %d:\nExpected: %s\nGot: %s", i, expected, string(result))
		}
	}
	for _, vm := range toRelease {
		testPool.Release(vm, metrics.New())
	}
}

func initPoolWithData(t *testing.T, size uint32, module string, entrypoint string, data []byte) *wasm.Pool {
	t.Helper()

	ctx := context.Background()

	compiler := compile.New().
		WithTarget(compile.TargetWasm).
		WithEntrypoints(entrypoint).
		WithBundle(&bundle.Bundle{
			Modules: []bundle.ModuleFile{
				{
					Path:   "policy.rego",
					URL:    "policy.rego",
					Raw:    []byte(module),
					Parsed: ast.MustParseModule(module),
				},
			},
		})

	err := compiler.Build(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	testPool := wasm.NewPool(size, 16, 100)

	err = testPool.SetPolicyData(ctx, compiler.Bundle().WasmModules[0].Raw, data)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if testPool.Size() != 1 {
		t.Fatalf("Expected a single vm to be initialized with data")
	}

	parsedDataAddr, parsedData := testPool.ParsedData()
	if parsedDataAddr == 0 {
		t.Fatalf("Expected parsedDataAddr to be non-nil")
	}

	if len(parsedData) == 0 {
		t.Fatalf("Expected parsedData to be non-nil")
	}

	vm := testPool.Wait(0)
	if vm == nil {
		t.Fatalf("Expected non-nil initial vm")
	}

	testPool.Release(vm, metrics.New())
	return testPool
}
