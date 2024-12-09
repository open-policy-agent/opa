package test

// This file collects some helpers for generating data used in
// benchmarks,
// - topdown/topdown_bench_test.go

import (
	v1 "github.com/open-policy-agent/opa/v1/util/test"
)

// PartialObjectBenchmarkCrossModule returns a module with n "bench_test_" prefixed rules
// that each refer to another "cond_bench_" prefixed rule
func PartialObjectBenchmarkCrossModule(n int) []string {
	return v1.PartialObjectBenchmarkCrossModule(n)
}

// ArrayIterationBenchmarkModule returns a module that iterates an array
// with `n` elements
func ArrayIterationBenchmarkModule(n int) string {
	return v1.ArrayIterationBenchmarkModule(n)
}

// SetIterationBenchmarkModule returns a module that iterates a set
// with `n` elements
func SetIterationBenchmarkModule(n int) string {
	return v1.SetIterationBenchmarkModule(n)
}

// ObjectIterationBenchmarkModule returns a module that iterates an object
// with `n` key/val pairs
func ObjectIterationBenchmarkModule(n int) string {
	return v1.ObjectIterationBenchmarkModule(n)
}

// GenerateLargeJSONBenchmarkData returns a map of 100 keys and 100.000 key/value
// pairs.
func GenerateLargeJSONBenchmarkData() map[string]interface{} {
	return v1.GenerateLargeJSONBenchmarkData()
}

// GenerateJSONBenchmarkData returns a map of `k` keys and `v` key/value pairs.
func GenerateJSONBenchmarkData(k, v int) map[string]interface{} {
	return v1.GenerateJSONBenchmarkData(k, v)
}

// GenerateConcurrencyBenchmarkData returns a module and data; the module
// checks some input parameters against that data in a simple API authz
// scheme.
func GenerateConcurrencyBenchmarkData() (string, map[string]interface{}) {
	return v1.GenerateConcurrencyBenchmarkData()
}

// GenerateVirtualDocsBenchmarkData generates a module and input; the
// numTotalRules and numHitRules create as many rules in the module to
// match/miss the returned input.
func GenerateVirtualDocsBenchmarkData(numTotalRules, numHitRules int) (string, map[string]interface{}) {
	return v1.GenerateVirtualDocsBenchmarkData(numTotalRules, numHitRules)
}
