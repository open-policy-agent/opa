# Ristretto
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://pkg.go.dev/github.com/dgraph-io/ristretto/v2)
[![ci-ristretto-tests](https://github.com/dgraph-io/ristretto/actions/workflows/ci-ristretto-tests.yml/badge.svg)](https://github.com/dgraph-io/ristretto/actions/workflows/ci-ristretto-tests.yml)
[![ci-ristretto-lint](https://github.com/dgraph-io/ristretto/actions/workflows/ci-ristretto-lint.yml/badge.svg)](https://github.com/dgraph-io/ristretto/actions/workflows/ci-ristretto-lint.yml)
[![Coverage Status](https://coveralls.io/repos/github/dgraph-io/ristretto/badge.svg?branch=main)](https://coveralls.io/github/dgraph-io/ristretto?branch=main)
[![Go Report Card](https://img.shields.io/badge/go%20report-A%2B-brightgreen)](https://goreportcard.com/report/github.com/dgraph-io/ristretto)

Ristretto is a fast, concurrent cache library built with a focus on performance and correctness.

The motivation to build Ristretto comes from the need for a contention-free cache in [Dgraph][].

[Dgraph]: https://github.com/dgraph-io/dgraph


## Features

* **High Hit Ratios** - with our unique admission/eviction policy pairing, Ristretto's performance is best in class.
	* **Eviction: SampledLFU** - on par with exact LRU and better performance on Search and Database traces.
	* **Admission: TinyLFU** - extra performance with little memory overhead (12 bits per counter).
* **Fast Throughput** - we use a variety of techniques for managing contention and the result is excellent throughput.
* **Cost-Based Eviction** - any large new item deemed valuable can evict multiple smaller items (cost could be anything).
* **Fully Concurrent** - you can use as many goroutines as you want with little throughput degradation.
* **Metrics** - optional performance metrics for throughput, hit ratios, and other stats.
* **Simple API** - just figure out your ideal `Config` values and you're off and running.


## Status

Ristretto is production-ready. See [Projects using Ristretto](#projects-using-ristretto).


## Usage

```go
package main

import (
	"fmt"

	"github.com/dgraph-io/ristretto/v2"
)

func main() {
	cache, err := ristretto.NewCache(&ristretto.Config[string, string]{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
	})
	if err != nil {
		panic(err)
	}
	defer cache.Close()

	// set a value with a cost of 1
	cache.Set("key", "value", 1)

	// wait for value to pass through buffers
	cache.Wait()

	// get value from cache
	value, found := cache.Get("key")
	if !found {
		panic("missing value")
	}
	fmt.Println(value)

	// del value from cache
	cache.Del("key")
}
```


## Benchmarks

The benchmarks can be found in https://github.com/dgraph-io/benchmarks/tree/master/cachebench/ristretto.

### Hit Ratios for Search

This trace is described as "disk read accesses initiated by a large commercial
search engine in response to various web search requests."

<p align="center">
	<img src="https://raw.githubusercontent.com/dgraph-io/ristretto/master/benchmarks/Hit%20Ratios%20-%20Search%20(ARC-S3).svg">
</p>

### Hit Ratio for Database

This trace is described as "a database server running at a commercial site
running an ERP application on top of a commercial database."

<p align="center">
	<img src="https://raw.githubusercontent.com/dgraph-io/ristretto/master/benchmarks/Hit%20Ratios%20-%20Database%20(ARC-DS1).svg">
</p>

### Hit Ratio for Looping

This trace demonstrates a looping access pattern.

<p align="center">
	<img src="https://raw.githubusercontent.com/dgraph-io/ristretto/master/benchmarks/Hit%20Ratios%20-%20Glimpse%20(LIRS-GLI).svg">
</p>

### Hit Ratio for CODASYL

This trace is described as "references to a CODASYL database for a one hour period."

<p align="center">
	<img src="https://raw.githubusercontent.com/dgraph-io/ristretto/master/benchmarks/Hit%20Ratios%20-%20CODASYL%20(ARC-OLTP).svg">
</p>

### Throughput for Mixed Workload

<p align="center">
	<img src="https://raw.githubusercontent.com/dgraph-io/ristretto/master/benchmarks/Throughput%20-%20Mixed.svg">
</p>

### Throughput ffor Read Workload

<p align="center">
	<img src="https://raw.githubusercontent.com/dgraph-io/ristretto/master/benchmarks/Throughput%20-%20Read%20(Zipfian).svg">
</p>

### Through for Write Workload

<p align="center">
	<img src="https://raw.githubusercontent.com/dgraph-io/ristretto/master/benchmarks/Throughput%20-%20Write%20(Zipfian).svg">
</p>


## Projects Using Ristretto

Below is a list of known projects that use Ristretto:

- [Badger](https://github.com/dgraph-io/badger) - Embeddable key-value DB in Go
- [Dgraph](https://github.com/dgraph-io/dgraph) - Horizontally scalable and distributed GraphQL database with a graph backend


## FAQ

### How are you achieving this performance? What shortcuts are you taking?

We go into detail in the [Ristretto blog post](https://blog.dgraph.io/post/introducing-ristretto-high-perf-go-cache/),
but in short: our throughput performance can be attributed to a mix of batching and eventual consistency. Our hit ratio
performance is mostly due to an excellent [admission policy](https://arxiv.org/abs/1512.00727) and SampledLFU eviction policy.

As for "shortcuts," the only thing Ristretto does that could be construed as one is dropping some Set calls. That means
a Set call for a new item (updates are guaranteed) isn't guaranteed to make it into the cache. The new item could be
dropped at two points: when passing through the Set buffer or when passing through the admission policy. However, this
doesn't affect hit ratios much at all as we expect the most popular items to be Set multiple times and eventually make
it in the cache.

### Is Ristretto distributed?

No, it's just like any other Go library that you can import into your project and use in a single process.
