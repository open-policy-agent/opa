package server

import (
	"strconv"
	"testing"
)

func TestCacheBase(t *testing.T) {
	c := newCache(5)
	foo := struct{}{}
	c.Insert("foo", foo)
	ensureCacheKey(t, c, "foo", foo)
}

func TestCacheLimit(t *testing.T) {
	max := 10
	c := newCache(max)

	// Fill the cache with values
	var i int
	for i = 0; i < max; i++ {
		c.Insert(strconv.Itoa(i), i)
	}

	// Ensure its at the max size
	ensureCacheSize(t, c, max)

	// Ensure they are all stored..
	for j := 0; j < max; j++ {
		ensureCacheKey(t, c, strconv.Itoa(j), j)
	}

	// Continue filling the cache, expect old keys to be dropped
	c.Insert(strconv.Itoa(i), i)
	ensureCacheKey(t, c, strconv.Itoa(i), i)

	// Should still be at max size
	ensureCacheSize(t, c, max)

	// Expect that "0" got dropped
	_, ok := c.Get("0")
	if ok {
		t.Fatal("Expected key '0' to not be found")
	}

	// Load the cache with many more than max
	for ; i < max*20; i++ {
		c.Insert(strconv.Itoa(i), i)
	}
	ensureCacheSize(t, c, max)

	// Ensure the last set of "max" number are available, and everything else is not
	for j := 0; j < i; j++ {
		k := strconv.Itoa(j)
		if j >= (i - max) {
			ensureCacheKey(t, c, k, j)
		} else {
			_, ok := c.Get(k)
			if ok {
				t.Fatalf("Expected key %s to not be found", k)
			}
		}
	}
}

func ensureCacheKey(t *testing.T, c *cache, k string, v interface{}) {
	t.Helper()
	actual, ok := c.Get(k)
	if !ok || v != actual {
		t.Fatalf("expected to retrieve value %v for key %s, got %v ok==%t", v, k, actual, ok)
	}
}

func ensureCacheSize(t *testing.T, c *cache, size int) {
	if len(c.data) != size && len(c.keylist) != size {
		t.Fatalf("Unexpected cache size len(data)=%d len(keylist)=%d, expected %d", size, len(c.data), len(c.keylist))
	}
}
