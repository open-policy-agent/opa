// +build !go1.16

// The outputs here change with Go 1.16, see
// https://github.com/golang/go/commit/afe7c8d0b25f26f0abd749ca52c7e1e7dfdee8cb
// But since we've been using 1.14 by the time these split-off fixtures
// were introduced, the filename mentions go1.14.

package tester

const fixtureReporterVerboseBenchmark = `FAILURES
--------------------------------------------------------------------------------
data.foo.bar.test_corge: FAIL (0s)


SUMMARY
--------------------------------------------------------------------------------
data.foo.bar.test_baz	    1000	       123 ns/op
data.foo.bar.test_qux: ERROR (0s)
  some err
data.foo.bar.test_corge: FAIL (0s)
--------------------------------------------------------------------------------
PASS: 1/3
FAIL: 1/3
ERROR: 1/3
`

const fixtureReporterVerboseBenchmarkShowAllocations = `FAILURES
--------------------------------------------------------------------------------
data.foo.bar.test_corge: FAIL (0s)


SUMMARY
--------------------------------------------------------------------------------
data.foo.bar.test_baz	    1000	       123 ns/op	       123 timer_rego_query_eval_ns/op	      91 B/op	       0 allocs/op
data.foo.bar.test_qux: ERROR (0s)
  some err
data.foo.bar.test_corge: FAIL (0s)
--------------------------------------------------------------------------------
PASS: 1/3
FAIL: 1/3
ERROR: 1/3
`

const fixtureReporterVerboseBenchmarkShowAllocationsGoBenchFormat = `FAILURES
--------------------------------------------------------------------------------
data.foo.bar.test_corge: FAIL (0s)


SUMMARY
--------------------------------------------------------------------------------
BenchmarkDataFooBarTestBaz	    1000	       123 ns/op	       123 timer_rego_query_eval_ns/op	      91 B/op	       0 allocs/op
data.foo.bar.test_qux: ERROR (0s)
  some err
data.foo.bar.test_corge: FAIL (0s)
--------------------------------------------------------------------------------
PASS: 1/3
FAIL: 1/3
ERROR: 1/3
`
