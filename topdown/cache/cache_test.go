// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cache

import (
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestParseCachingConfig(t *testing.T) {
	maxSize := new(int64)
	*maxSize = defaultMaxSizeBytes
	expected := &Config{InterQueryBuiltinCache: InterQueryBuiltinCacheConfig{MaxSizeBytes: maxSize}}

	tests := map[string]struct {
		input   []byte
		wantErr bool
	}{
		"empty_config": {
			input:   nil,
			wantErr: false,
		},
		"default_limit": {
			input:   []byte(`{"inter_query_builtin_cache": {},}`),
			wantErr: false,
		},
		"bad_limit": {
			input:   []byte(`{"inter_query_builtin_cache": {"max_size_bytes": "100"},}`),
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			config, err := ParseCachingConfig(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}

			if !tc.wantErr && !reflect.DeepEqual(config, expected) {
				t.Fatalf("want %v got %v", expected, config)
			}
		})
	}

	// cache limit specified
	in := `{"inter_query_builtin_cache": {"max_size_bytes": 100},}`

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	limit := int64(100)
	expected.InterQueryBuiltinCache.MaxSizeBytes = &limit

	if !reflect.DeepEqual(config, expected) {
		t.Fatalf("want %v got %v", expected, config)
	}
}

func TestInsert(t *testing.T) {

	in := `{"inter_query_builtin_cache": {"max_size_bytes": 20},}` // 20 byte limit for test purposes

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	cache := NewInterQueryCache(config)

	// large cache value that exceeds limit
	cacheValueLarge := newInterQueryCacheValue(ast.StringTerm("bar").Value, 40)
	dropped := cache.Insert(ast.StringTerm("foo").Value, cacheValueLarge)

	if dropped != 1 {
		t.Fatal("Expected dropped to be one")
	}

	_, found := cache.Get(ast.StringTerm("foo").Value)
	if found {
		t.Fatal("Unexpected key \"foo\" in cache")
	}

	cacheValue := newInterQueryCacheValue(ast.StringTerm("bar").Value, 20)
	dropped = cache.Insert(ast.StringTerm("foo").Value, cacheValue)

	if dropped != 0 {
		t.Fatal("Expected dropped to be zero")
	}

	// exceed cache limit
	cacheValue2 := newInterQueryCacheValue(ast.StringTerm("bar2").Value, 20)
	dropped = cache.Insert(ast.StringTerm("foo2").Value, cacheValue2)

	if dropped != 1 {
		t.Fatal("Expected dropped to be one")
	}

	_, found = cache.Get(ast.StringTerm("foo2").Value)
	if !found {
		t.Fatal("Expected key \"foo2\" in cache")
	}

	_, found = cache.Get(ast.StringTerm("foo").Value)
	if found {
		t.Fatal("Unexpected key \"foo\" in cache")
	}
}

func TestDelete(t *testing.T) {
	config, err := ParseCachingConfig(nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	cache := NewInterQueryCache(config)
	cacheValue := newInterQueryCacheValue(ast.StringTerm("bar").Value, 20)

	dropped := cache.Insert(ast.StringTerm("foo").Value, cacheValue)

	if dropped != 0 {
		t.Fatal("Expected dropped to be zero")
	}

	cache.Delete(ast.StringTerm("foo").Value)

	_, found := cache.Get(ast.StringTerm("foo").Value)
	if found {
		t.Fatal("Unexpected key \"foo\" in cache")
	}
}

func TestUpdateConfig(t *testing.T) {
	config, err := ParseCachingConfig(nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	c := NewInterQueryCache(config)
	actualC, ok := c.(*cache)
	if !ok {
		t.Fatal("Unexpected error converting InterQueryCache to cache struct")
	}
	if actualC.config != config {
		t.Fatal("Cache config is different than expected")
	}
	actualC.UpdateConfig(nil)
	if actualC.config != config {
		t.Fatal("Cache config is different than expected after a nil update")
	}
	config2, err := ParseCachingConfig(nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	actualC.UpdateConfig(config2)
	if actualC.config != config2 {
		t.Fatal("Cache config is different than expected after update")
	}
}

func TestDefaultMaxSizeBytes(t *testing.T) {
	c := NewInterQueryCache(nil)
	actualC, ok := c.(*cache)
	if !ok {
		t.Fatal("Unexpected error converting InterQueryCache to cache struct")
	}
	if actualC.maxSizeBytes() != defaultMaxSizeBytes {
		t.Fatal("Expected maxSizeBytes() to return default when config is nil")
	}
}

type testInterQueryCacheValue struct {
	value ast.Value
	size  int
}

func newInterQueryCacheValue(value ast.Value, size int) *testInterQueryCacheValue {
	return &testInterQueryCacheValue{value: value, size: size}
}

func (p testInterQueryCacheValue) SizeInBytes() int64 {
	return int64(p.size)
}
