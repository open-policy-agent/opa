// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package cache defines the inter-query cache interface that can cache data across queries
package cache

import (
	"context"

	v1 "github.com/open-policy-agent/opa/v1/topdown/cache"
)

// Config represents the configuration for the inter-query builtin cache.
type Config = v1.Config

// InterQueryBuiltinValueCacheConfig represents the configuration of the inter-query value cache that built-in functions can utilize.
// MaxNumEntries - max number of cache entries
type InterQueryBuiltinValueCacheConfig = v1.InterQueryBuiltinValueCacheConfig

// InterQueryBuiltinCacheConfig represents the configuration of the inter-query cache that built-in functions can utilize.
// MaxSizeBytes - max capacity of cache in bytes
// ForcedEvictionThresholdPercentage - capacity usage in percentage after which forced FIFO eviction starts
// StaleEntryEvictionPeriodSeconds - time period between end of previous and start of new stale entry eviction routine
type InterQueryBuiltinCacheConfig = v1.InterQueryBuiltinCacheConfig

// ParseCachingConfig returns the config for the inter-query cache.
func ParseCachingConfig(raw []byte) (*Config, error) {
	return v1.ParseCachingConfig(raw)
}

// InterQueryCacheValue defines the interface for the data that the inter-query cache holds.
type InterQueryCacheValue = v1.InterQueryCacheValue

// InterQueryCache defines the interface for the inter-query cache.
type InterQueryCache = v1.InterQueryCache

// NewInterQueryCache returns a new inter-query cache.
// The cache uses a FIFO eviction policy when it reaches the forced eviction threshold.
// Parameters:
//
//	config - to configure the InterQueryCache
func NewInterQueryCache(config *Config) InterQueryCache {
	return v1.NewInterQueryCache(config)
}

// NewInterQueryCacheWithContext returns a new inter-query cache with context.
// The cache uses a combination of FIFO eviction policy when it reaches the forced eviction threshold
// and a periodic cleanup routine to remove stale entries that exceed their expiration time, if specified.
// If configured with a zero stale_entry_eviction_period_seconds value, the stale entry cleanup routine is disabled.
//
// Parameters:
//
//	ctx - used to control lifecycle of the stale entry cleanup routine
//	config - to configure the InterQueryCache
func NewInterQueryCacheWithContext(ctx context.Context, config *Config) InterQueryCache {
	return v1.NewInterQueryCacheWithContext(ctx, config)
}

type InterQueryValueCache = v1.InterQueryValueCache

func NewInterQueryValueCache(ctx context.Context, config *Config) InterQueryValueCache {
	return v1.NewInterQueryValueCache(ctx, config)
}
