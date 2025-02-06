// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cache

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestParseCachingConfig(t *testing.T) {
	t.Parallel()

	maxSize := new(int64)
	*maxSize = defaultMaxSizeBytes
	period := new(int64)
	*period = defaultStaleEntryEvictionPeriodSeconds
	threshold := new(int64)
	*threshold = defaultForcedEvictionThresholdPercentage
	maxNumEntriesInterQueryValueCache := new(int)
	*maxNumEntriesInterQueryValueCache = defaultInterQueryBuiltinValueCacheSize

	expected := &Config{
		InterQueryBuiltinCache: InterQueryBuiltinCacheConfig{
			MaxSizeBytes:                      maxSize,
			StaleEntryEvictionPeriodSeconds:   period,
			ForcedEvictionThresholdPercentage: threshold,
		},
		InterQueryBuiltinValueCache: InterQueryBuiltinValueCacheConfig{
			MaxNumEntries: maxNumEntriesInterQueryValueCache,
		},
	}

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
		"default_num_entries": {
			input:   []byte(`{"inter_query_builtin_value_cache": {},}`),
			wantErr: false,
		},
		"bad_limit": {
			input:   []byte(`{"inter_query_builtin_cache": {"max_size_bytes": "100"},}`),
			wantErr: true,
		},
		"bad_num_entries": {
			input:   []byte(`{"inter_query_builtin_value_cache": {"max_num_entries": "100"},}`),
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

func TestInterValueCache_DefaultConfiguration(t *testing.T) {
	t.Run("default config not set", func(t *testing.T) {
		config := Config{
			InterQueryBuiltinValueCache: InterQueryBuiltinValueCacheConfig{},
		}

		c := NewInterQueryValueCache(context.Background(), &config)
		if c.GetCache("foo") != nil {
			t.Fatal("Expected cache to be disabled")
		}
	})

	t.Run("default config set", func(t *testing.T) {
		config := Config{
			InterQueryBuiltinValueCache: InterQueryBuiltinValueCacheConfig{},
		}

		RegisterDefaultInterQueryBuiltinValueCacheConfig("bar", &NamedValueCacheConfig{
			MaxNumEntries: &[]int{5}[0],
		})

		c := NewInterQueryValueCache(context.Background(), &config)
		if act := *c.GetCache("bar").(*interQueryValueCacheBucket).config.MaxNumEntries; act != 5 {
			t.Fatalf("Expected 5 max entries, got %d", act)
		}
	})

	t.Run("explicitly disabled", func(t *testing.T) {
		cacheConfig := Config{
			InterQueryBuiltinValueCache: InterQueryBuiltinValueCacheConfig{
				NamedCacheConfigs: map[string]*NamedValueCacheConfig{
					"baz": nil,
				},
			},
		}

		RegisterDefaultInterQueryBuiltinValueCacheConfig("baz", nil)

		c := NewInterQueryValueCache(context.Background(), &cacheConfig)
		if c.GetCache("baz") != nil {
			t.Fatal("Expected cache to be disabled")
		}
	})

	t.Run("override", func(t *testing.T) {
		cacheConfig := Config{
			InterQueryBuiltinValueCache: InterQueryBuiltinValueCacheConfig{
				NamedCacheConfigs: map[string]*NamedValueCacheConfig{
					"box": {
						MaxNumEntries: &[]int{5}[0],
					},
				},
			},
		}

		RegisterDefaultInterQueryBuiltinValueCacheConfig("box", &NamedValueCacheConfig{
			MaxNumEntries: &[]int{10}[0],
		})

		c := NewInterQueryValueCache(context.Background(), &cacheConfig)
		if act := *c.GetCache("box").(*interQueryValueCacheBucket).config.MaxNumEntries; act != 5 {
			t.Fatalf("Expected 5 max entries, got %d", act)
		}
	})
}

func TestInterValueCache_NamedCaches(t *testing.T) {
	t.Parallel()

	t.Run("configured max is respected", func(t *testing.T) {
		config := Config{
			InterQueryBuiltinValueCache: InterQueryBuiltinValueCacheConfig{
				NamedCacheConfigs: map[string]*NamedValueCacheConfig{
					"foo": {
						MaxNumEntries: &[]int{2}[0],
					},
				},
			},
		}

		c := NewInterQueryValueCache(context.Background(), &config)

		nc := c.GetCache("foo").(*interQueryValueCacheBucket)
		if act := *nc.config.MaxNumEntries; act != 2 {
			t.Fatalf("Expected 2 max entries, got %d", act)
		}

		if nc.items.Len() != 0 {
			t.Fatalf("Expected cache to be empty")
		}

		nc.Insert(ast.StringTerm("a").Value, "b")
		if nc.items.Len() != 1 {
			t.Fatalf("Expected cache to have 1 entry")
		}
		if v, found := nc.Get(ast.StringTerm("a").Value); !found && v != "b" {
			t.Fatalf("Expected cache hit")
		}

		nc.Insert(ast.StringTerm("c").Value, "d")
		if nc.items.Len() != 2 {
			t.Fatalf("Expected cache to have 2 entries")
		}
		if v, found := nc.Get(ast.StringTerm("c").Value); !found && v != "d" {
			t.Fatalf("Expected cache hit")
		}

		nc.Insert(ast.StringTerm("e").Value, "f")
		if nc.items.Len() != 2 {
			t.Fatalf("Expected cache to still have 2 entries")
		}
		if v, found := nc.Get(ast.StringTerm("e").Value); !found && v != "f" {
			t.Fatalf("Expected cache hit")
		}
	})

	t.Run("named caches are separate", func(t *testing.T) {
		config := Config{
			InterQueryBuiltinValueCache: InterQueryBuiltinValueCacheConfig{
				MaxNumEntries: &[]int{2}[0],
				NamedCacheConfigs: map[string]*NamedValueCacheConfig{
					"foo": {
						MaxNumEntries: &[]int{2}[0],
					},
					"bar": {
						MaxNumEntries: &[]int{2}[0],
					},
				},
			},
		}

		c := NewInterQueryValueCache(context.Background(), &config)

		c.Insert(ast.StringTerm("foo").Value, "bar")

		nc1 := c.GetCache("foo").(*interQueryValueCacheBucket)
		nc2 := c.GetCache("bar").(*interQueryValueCacheBucket)

		nc1.Insert(ast.StringTerm("a").Value, "b")
		nc2.Insert(ast.StringTerm("c").Value, "d")

		if _, found := c.Get(ast.StringTerm("foo").Value); !found {
			t.Fatal("Expected cache hit")
		}
		if _, found := c.Get(ast.StringTerm("a").Value); found {
			t.Fatal("Expected cache miss")
		}
		if _, found := c.Get(ast.StringTerm("c").Value); found {
			t.Fatal("Expected cache miss")
		}

		if _, found := nc1.Get(ast.StringTerm("a").Value); !found {
			t.Fatal("Expected cache hit")
		}
		if _, found := nc1.Get(ast.StringTerm("c").Value); found {
			t.Fatal("Expected cache miss")
		}
		if _, found := nc1.Get(ast.StringTerm("foo").Value); found {
			t.Fatal("Expected cache miss")
		}

		if _, found := nc2.Get(ast.StringTerm("c").Value); !found {
			t.Fatal("Expected cache hit")
		}
		if _, found := nc2.Get(ast.StringTerm("a").Value); found {
			t.Fatal("Expected cache miss")
		}
		if _, found := nc2.Get(ast.StringTerm("foo").Value); found {
			t.Fatal("Expected cache miss")
		}
	})
}

func TestInsert(t *testing.T) {
	t.Parallel()

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
	cacheValue3 := newInterQueryCacheValue(ast.StringTerm("bar3").Value, 10)
	cache.Insert(ast.StringTerm("foo3").Value, cacheValue3)
	cacheValue4 := newInterQueryCacheValue(ast.StringTerm("bar4").Value, 10)
	cache.Insert(ast.StringTerm("foo4").Value, cacheValue4)
	cacheValue5 := newInterQueryCacheValue(ast.StringTerm("bar5").Value, 20)
	dropped = cache.Insert(ast.StringTerm("foo5").Value, cacheValue5)
	if dropped != 2 {
		t.Fatal("Expected dropped to be two")
	}
	_, found = cache.Get(ast.StringTerm("foo3").Value)
	if found {
		t.Fatal("Unexpected key \"foo3\" in cache")
	}
	_, found = cache.Get(ast.StringTerm("foo4").Value)
	if found {
		t.Fatal("Unexpected key \"foo4\" in cache")
	}
	_, found = cache.Get(ast.StringTerm("foo5").Value)
	if !found {
		t.Fatal("Expected key \"foo5\" in cache")
	}
	verifyCacheList(t, cache)

	// replacing an existing key should not affect cache size
	cache = NewInterQueryCache(config)

	cacheValue6 := newInterQueryCacheValue(ast.String("bar6"), 10)
	cache.Insert(ast.String("foo6"), cacheValue6)
	cache.Insert(ast.String("foo6"), cacheValue6)
	verifyCacheList(t, cache)

	cacheValue7 := newInterQueryCacheValue(ast.String("bar7"), 10)
	dropped = cache.Insert(ast.StringTerm("foo7").Value, cacheValue7)
	verifyCacheList(t, cache)

	if dropped != 0 {
		t.Fatal("Expected dropped to be zero")
	}
}

func TestInterQueryValueCache(t *testing.T) {
	t.Parallel()

	in := `{"inter_query_builtin_value_cache": {"max_num_entries": 4},}`

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	cache := NewInterQueryValueCache(context.Background(), config)

	cache.Insert(ast.StringTerm("foo").Value, "bar")
	cache.Insert(ast.StringTerm("foo2").Value, "bar2")
	cache.Insert(ast.StringTerm("hello").Value, "world")
	dropped := cache.Insert(ast.StringTerm("hello2").Value, "world2")

	if dropped != 0 {
		t.Fatal("Expected dropped to be zero")
	}

	value, found := cache.Get(ast.StringTerm("foo").Value)
	if !found {
		t.Fatal("Expected key \"foo\" in cache")
	}

	actual, ok := value.(string)
	if !ok {
		t.Fatal("Expected string value")
	}

	if actual != "bar" {
		t.Fatalf("Expected value \"bar\" but got %v", actual)
	}

	dropped = cache.Insert(ast.StringTerm("foo3").Value, "bar3")
	if dropped != 1 {
		t.Fatal("Expected dropped to be one")
	}

	_, found = cache.Get(ast.StringTerm("foo3").Value)
	if !found {
		t.Fatal("Expected key \"foo3\" in cache")
	}

	// update the cache config
	in = `{"inter_query_builtin_value_cache": {"max_num_entries": 0},}` // unlimited
	config, err = ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	cache.UpdateConfig(config)

	cache.Insert(ast.StringTerm("a").Value, "b")
	cache.Insert(ast.StringTerm("c").Value, "d")
	cache.Insert(ast.StringTerm("e").Value, "f")
	dropped = cache.Insert(ast.StringTerm("g").Value, "h")

	if dropped != 0 {
		t.Fatal("Expected dropped to be zero")
	}

	// at this point the cache should have 8 entries
	// update the cache size and verify multiple items dropped
	in = `{"inter_query_builtin_value_cache": {"max_num_entries": 6},}`
	config, err = ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	cache.UpdateConfig(config)

	dropped = cache.Insert(ast.StringTerm("i").Value, "j")

	if dropped != 3 {
		t.Fatal("Expected dropped to be three")
	}

	_, found = cache.Get(ast.StringTerm("i").Value)
	if !found {
		t.Fatal("Expected key \"i\" in cache")
	}

	cache.Delete(ast.StringTerm("i").Value)

	_, found = cache.Get(ast.StringTerm("i").Value)
	if found {
		t.Fatal("Unexpected key \"i\" in cache")
	}
}

func TestConcurrentInsert(t *testing.T) {
	t.Parallel()

	in := `{"inter_query_builtin_cache": {"max_size_bytes": 20},}` // 20 byte limit for test purposes

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	cache := NewInterQueryCache(config)

	cacheValue := newInterQueryCacheValue(ast.String("bar"), 10)
	cache.Insert(ast.String("foo"), cacheValue)

	wg := sync.WaitGroup{}

	for range 5 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			cacheValue2 := newInterQueryCacheValue(ast.String("bar2"), 5)
			cache.Insert(ast.String("foo2"), cacheValue2)

		}()
	}
	wg.Wait()

	cacheValue3 := newInterQueryCacheValue(ast.String("bar3"), 5)
	dropped := cache.Insert(ast.String("foo3"), cacheValue3)
	verifyCacheList(t, cache)

	if dropped != 0 {
		t.Fatal("Expected dropped to be zero")
	}

	_, found := cache.Get(ast.String("foo"))
	if !found {
		t.Fatal("Expected key \"foo\" in cache")
	}

	_, found = cache.Get(ast.String("foo2"))
	if !found {
		t.Fatal("Expected key \"foo2\" in cache")
	}

	_, found = cache.Get(ast.String("foo3"))
	if !found {
		t.Fatal("Expected key \"foo3\" in cache")
	}
}

func TestClone(t *testing.T) {
	t.Parallel()

	in := `{"inter_query_builtin_cache": {"max_size_bytes": 40},}`

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	cache := NewInterQueryCache(config)

	cacheValue := newInterQueryCacheValue(ast.StringTerm("bar").Value, 20)
	dropped := cache.Insert(ast.StringTerm("foo").Value, cacheValue)
	if dropped != 0 {
		t.Fatal("Expected dropped to be zero")
	}

	val, found := cache.Get(ast.StringTerm("foo").Value)
	if !found {
		t.Fatal("Expected key \"foo\" in cache")
	}

	dup, err := cache.Clone(val)
	if err != nil {
		t.Fatal(err)
	}

	original, ok := val.(*testInterQueryCacheValue)
	if !ok {
		t.Fatal("unexpected type")
	}

	cloned, ok := dup.(*testInterQueryCacheValue)
	if !ok {
		t.Fatal("unexpected type")
	}

	if !reflect.DeepEqual(*original, *cloned) {
		t.Fatalf("Expected to get %v, but got %v", *original, *cloned)
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

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
	verifyCacheList(t, cache)

	cache.Delete(ast.StringTerm("foo").Value)

	_, found := cache.Get(ast.StringTerm("foo").Value)
	if found {
		t.Fatal("Unexpected key \"foo\" in cache")
	}
	verifyCacheList(t, cache)
}

func TestInsertWithExpiryAndEviction(t *testing.T) {
	t.Parallel()

	// 50 byte max size
	// 1s stale cleanup period
	// 80% threshold to for FIFO eviction (eviction after 40 bytes)
	in := `{"inter_query_builtin_cache": {"max_size_bytes": 50, "stale_entry_eviction_period_seconds": 1, "forced_eviction_threshold_percentage": 80},}`

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// This starts a background ticker at stale_entry_eviction_period_seconds to clean up items.
	ctx, cancel := context.WithCancel(context.Background())
	cache := NewInterQueryCacheWithContext(ctx, config)
	t.Cleanup(cancel)

	cacheValue := newInterQueryCacheValue(ast.StringTerm("bar").Value, 20)
	cache.InsertWithExpiry(ast.StringTerm("force_evicted_foo").Value, cacheValue, time.Now().Add(100*time.Second))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("force_evicted_foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v, found %v", cacheValue, fetchedCacheValue)
	}
	cache.InsertWithExpiry(ast.StringTerm("expired_foo").Value, cacheValue, time.Now().Add(900*time.Millisecond))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("expired_foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v, found %v", cacheValue, fetchedCacheValue)
	}
	cache.InsertWithExpiry(ast.StringTerm("foo").Value, cacheValue, time.Now().Add(10*time.Second))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v, found %v", cacheValue, fetchedCacheValue)
	}

	// Ensure stale entries clean up routine runs at least once
	time.Sleep(1100 * time.Millisecond)

	// Entry deleted even though not expired because force evicted when foo is inserted
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("force_evicted_foo").Value); found {
		t.Fatalf("Didn't expect cache entry for force_evicted_foo, found entry with value %v", fetchedCacheValue)
	}
	// Stale clean up routine runs and deletes expired entry
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("expired_foo").Value); found {
		t.Fatalf("Didn't expect cache entry for expired_foo, found entry with value %v", fetchedCacheValue)
	}
	// Stale clean up routine runs but doesn't delete the entry
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v for foo, found %v", cacheValue, fetchedCacheValue)
	}
}

func TestInsertHighTTLWithStaleEntryCleanup(t *testing.T) {
	t.Parallel()

	// 40 byte max size
	// 1s stale cleanup period
	// 100% threshold to for FIFO eviction (eviction after 40 bytes)
	in := `{"inter_query_builtin_cache": {"max_size_bytes": 40, "stale_entry_eviction_period_seconds": 1, "forced_eviction_threshold_percentage": 100},}`

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// This starts a background ticker at stale_entry_eviction_period_seconds to clean up items.
	ctx, cancel := context.WithCancel(context.Background())
	cache := NewInterQueryCacheWithContext(ctx, config)
	t.Cleanup(cancel)

	cacheValue := newInterQueryCacheValue(ast.StringTerm("bar").Value, 20)
	cache.InsertWithExpiry(ast.StringTerm("high_ttl_foo").Value, cacheValue, time.Now().Add(100*time.Second))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("high_ttl_foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v, found %v", cacheValue, fetchedCacheValue)
	}
	cache.InsertWithExpiry(ast.StringTerm("expired_foo").Value, cacheValue, time.Now().Add(900*time.Millisecond))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("expired_foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v, found no entry", fetchedCacheValue)
	}

	// Ensure stale entries clean up routine runs at least once
	time.Sleep(1100 * time.Millisecond)

	cache.InsertWithExpiry(ast.StringTerm("foo").Value, cacheValue, time.Now().Add(10*time.Second))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v, found %v", cacheValue, fetchedCacheValue)
	}

	// Since expired_foo is deleted by stale cleanup routine, high_ttl_foo is not evicted when foo is inserted
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("high_ttl_foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v for high_ttl_foo, found %v", cacheValue, fetchedCacheValue)
	}
	// Stale clean up routine runs and deletes expired entry
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("expired_foo").Value); found {
		t.Fatalf("Didn't expect cache entry for expired_foo, found entry with value %v", fetchedCacheValue)
	}
}

func TestInsertHighTTLWithoutStaleEntryCleanup(t *testing.T) {
	t.Parallel()

	// 40 byte max size
	// 0s stale cleanup period -> no cleanup
	// 100% threshold to for FIFO eviction (eviction after 40 bytes)
	in := `{"inter_query_builtin_cache": {"max_size_bytes": 40, "stale_entry_eviction_period_seconds": 0, "forced_eviction_threshold_percentage": 100},}`

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// This starts a background ticker at stale_entry_eviction_period_seconds to clean up items.
	ctx, cancel := context.WithCancel(context.Background())
	cache := NewInterQueryCacheWithContext(ctx, config)
	t.Cleanup(cancel)

	cacheValue := newInterQueryCacheValue(ast.StringTerm("bar").Value, 20)
	cache.InsertWithExpiry(ast.StringTerm("high_ttl_foo").Value, cacheValue, time.Now().Add(100*time.Second))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("high_ttl_foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v for high_ttl_foo, found no entry", fetchedCacheValue)
	}
	cache.InsertWithExpiry(ast.StringTerm("expired_foo").Value, cacheValue, time.Now().Add(1*time.Second))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("expired_foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v for expired_foo, found no entry", fetchedCacheValue)
	}

	cache.InsertWithExpiry(ast.StringTerm("foo").Value, cacheValue, time.Now().Add(10*time.Second))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v for foo, found no entry", fetchedCacheValue)
	}

	// Since stale cleanup routine is disabled, high_ttl_foo is evicted when foo is inserted
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("high_ttl_foo").Value); found {
		t.Fatalf("Didn't expect cache entry for high_ttl_foo, found entry with value %v", fetchedCacheValue)
	}
	// Stale clean up disabled so expired entry exists
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("expired_foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v for expired_foo, found %v", cacheValue, fetchedCacheValue)
	}
}

func TestZeroExpiryTime(t *testing.T) {
	t.Parallel()

	// 20 byte max size
	// 1s stale cleanup period
	// 100% threshold to for FIFO eviction (eviction after 40 bytes)
	in := `{"inter_query_builtin_cache": {"max_size_bytes": 20, "stale_entry_eviction_period_seconds": 1, "forced_eviction_threshold_percentage": 100},}`

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// This starts a background ticker at stale_entry_eviction_period_seconds to clean up items.
	ctx, cancel := context.WithCancel(context.Background())
	cache := NewInterQueryCacheWithContext(ctx, config)
	t.Cleanup(cancel)
	cacheValue := newInterQueryCacheValue(ast.StringTerm("bar").Value, 20)
	cache.InsertWithExpiry(ast.StringTerm("foo").Value, cacheValue, time.Time{})
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v for foo, found %v", cacheValue, fetchedCacheValue)
	}

	time.Sleep(1100 * time.Millisecond)

	// Stale entry cleanup routine skips zero time cache entries
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v for foo, found %v", cacheValue, fetchedCacheValue)
	}
}

func TestCancelNewInterQueryCacheWithContext(t *testing.T) {
	t.Parallel()

	// 40 byte max size
	// 1s stale cleanup period
	// 100% threshold to for FIFO eviction (eviction after 40 bytes)
	in := `{"inter_query_builtin_cache": {"max_size_bytes": 40, "stale_entry_eviction_period_seconds": 1, "forced_eviction_threshold_percentage": 100},}`

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cache := NewInterQueryCacheWithContext(ctx, config)
	cacheValue := newInterQueryCacheValue(ast.StringTerm("bar").Value, 20)
	cache.InsertWithExpiry(ast.StringTerm("foo").Value, cacheValue, time.Now().Add(100*time.Millisecond))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v for foo, found %v", cacheValue, fetchedCacheValue)
	}

	cancel()
	time.Sleep(1100 * time.Millisecond)

	// Stale entry cleanup routine stopped as context was cancelled
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v for foo, found %v", cacheValue, fetchedCacheValue)
	}

}

func TestUpdateConfig(t *testing.T) {
	t.Parallel()

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

func TestDefaultConfigValues(t *testing.T) {
	t.Parallel()

	c := NewInterQueryCache(nil)
	actualC, ok := c.(*cache)
	if !ok {
		t.Fatal("Unexpected error converting InterQueryCache to cache struct")
	}
	if actualC.maxSizeBytes() != defaultMaxSizeBytes {
		t.Fatal("Expected maxSizeBytes() to return default when config is nil")
	}
	if actualC.forcedEvictionThresholdPercentage() != defaultForcedEvictionThresholdPercentage {
		t.Fatal("Expected forcedEvictionThresholdPercentage() to return default when config is nil")
	}
	if actualC.staleEntryEvictionTimePeriodSeconds() != defaultStaleEntryEvictionPeriodSeconds {
		t.Fatal("Expected staleEntryEvictionTimePeriodSeconds() to return default when config is nil")
	}
}

// Verifies that the size of c.l is identical to the size of c.items
// Since the size of c.items is limited by c.usage, this helps us
// avoid a situation where c.l can grow indefinitely causing a memory leak
func verifyCacheList(t *testing.T, c InterQueryCache) {
	actualC, ok := c.(*cache)
	if !ok {
		t.Fatal("Unexpected error converting InterQueryCache to cache struct")
	}
	if len(actualC.items) != actualC.l.Len() {
		t.Fatal("actualC.l should contain equally many elements as actualC.items")
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

func (p testInterQueryCacheValue) Clone() (InterQueryCacheValue, error) {
	return &testInterQueryCacheValue{value: p.value, size: p.size}, nil
}
