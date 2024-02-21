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

	"github.com/open-policy-agent/opa/ast"
)

func TestParseCachingConfig(t *testing.T) {
	maxSize := new(int64)
	*maxSize = defaultMaxSizeBytes
	period := new(int64)
	*period = defaultStaleEntryEvictionPeriodSeconds
	threshold := new(int64)
	*threshold = defaultForcedEvictionThresholdPercentage
	expected := &Config{InterQueryBuiltinCache: InterQueryBuiltinCacheConfig{MaxSizeBytes: maxSize, StaleEntryEvictionPeriodSeconds: period, ForcedEvictionThresholdPercentage: threshold}}

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

func TestConcurrentInsert(t *testing.T) {
	in := `{"inter_query_builtin_cache": {"max_size_bytes": 20},}` // 20 byte limit for test purposes

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	cache := NewInterQueryCache(config)

	cacheValue := newInterQueryCacheValue(ast.String("bar"), 10)
	cache.Insert(ast.String("foo"), cacheValue)

	wg := sync.WaitGroup{}

	for i := 0; i < 5; i++ {
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
	// 50 byte max size
	// 1s stale cleanup period
	// 80% threshold to for FIFO eviction (eviction after 40 bytes)
	in := `{"inter_query_builtin_cache": {"max_size_bytes": 50, "stale_entry_eviction_period_seconds": 1, "forced_eviction_threshold_percentage": 80},}`

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	cache := NewInterQueryCacheWithContext(context.Background(), config)

	cacheValue := newInterQueryCacheValue(ast.StringTerm("bar").Value, 20)
	cache.InsertWithExpiry(ast.StringTerm("force_evicted_foo").Value, cacheValue, time.Now().Add(100*time.Second))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("force_evicted_foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v, found %v", cacheValue, fetchedCacheValue)
	}
	cache.InsertWithExpiry(ast.StringTerm("expired_foo").Value, cacheValue, time.Now().Add(1*time.Second))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("expired_foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v, found %v", cacheValue, fetchedCacheValue)
	}
	cache.InsertWithExpiry(ast.StringTerm("foo").Value, cacheValue, time.Now().Add(10*time.Second))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v, found %v", cacheValue, fetchedCacheValue)
	}

	// Ensure stale entries clean up routine runs at least once
	time.Sleep(2 * time.Second)

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
	// 40 byte max size
	// 1s stale cleanup period
	// 100% threshold to for FIFO eviction (eviction after 40 bytes)
	in := `{"inter_query_builtin_cache": {"max_size_bytes": 40, "stale_entry_eviction_period_seconds": 1, "forced_eviction_threshold_percentage": 100},}`

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	cache := NewInterQueryCacheWithContext(context.Background(), config)

	cacheValue := newInterQueryCacheValue(ast.StringTerm("bar").Value, 20)
	cache.InsertWithExpiry(ast.StringTerm("high_ttl_foo").Value, cacheValue, time.Now().Add(100*time.Second))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("high_ttl_foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v, found %v", cacheValue, fetchedCacheValue)
	}
	cache.InsertWithExpiry(ast.StringTerm("expired_foo").Value, cacheValue, time.Now().Add(1*time.Second))
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("expired_foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v, found no entry", fetchedCacheValue)
	}

	// Ensure stale entries clean up routine runs at least once
	time.Sleep(2 * time.Second)

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
	// 40 byte max size
	// 0s stale cleanup period -> no cleanup
	// 100% threshold to for FIFO eviction (eviction after 40 bytes)
	in := `{"inter_query_builtin_cache": {"max_size_bytes": 40, "stale_entry_eviction_period_seconds": 0, "forced_eviction_threshold_percentage": 100},}`

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	cache := NewInterQueryCacheWithContext(context.Background(), config)

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
	// 20 byte max size
	// 1s stale cleanup period
	// 100% threshold to for FIFO eviction (eviction after 40 bytes)
	in := `{"inter_query_builtin_cache": {"max_size_bytes": 20, "stale_entry_eviction_period_seconds": 1, "forced_eviction_threshold_percentage": 100},}`

	config, err := ParseCachingConfig([]byte(in))
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	cache := NewInterQueryCacheWithContext(context.Background(), config)
	cacheValue := newInterQueryCacheValue(ast.StringTerm("bar").Value, 20)
	cache.InsertWithExpiry(ast.StringTerm("foo").Value, cacheValue, time.Time{})
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v for foo, found %v", cacheValue, fetchedCacheValue)
	}

	time.Sleep(2 * time.Second)

	// Stale entry cleanup routine skips zero time cache entries
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v for foo, found %v", cacheValue, fetchedCacheValue)
	}
}

func TestCancelNewInterQueryCacheWithContext(t *testing.T) {
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
	time.Sleep(2 * time.Second)

	// Stale entry cleanup routine stopped as context was cancelled
	if fetchedCacheValue, found := cache.Get(ast.StringTerm("foo").Value); !found {
		t.Fatalf("Expected cache entry with value %v for foo, found %v", cacheValue, fetchedCacheValue)
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

func TestDefaultConfigValues(t *testing.T) {
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
