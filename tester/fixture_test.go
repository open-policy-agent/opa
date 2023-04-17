package tester

const fixtureReporterVerboseBenchmark = `FAILURES
--------------------------------------------------------------------------------
data.foo.bar.test_corge: FAIL (0s)


SUMMARY
--------------------------------------------------------------------------------
data.foo.bar.test_baz	    1000	       123.0 ns/op
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
data.foo.bar.test_baz	    1000	       123.0 ns/op	       123.0 timer_rego_query_eval_ns/op	      91 B/op	       0 allocs/op
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
BenchmarkDataFooBarTestBaz	    1000	       123.0 ns/op	       123.0 timer_rego_query_eval_ns/op	      91 B/op	       0 allocs/op
data.foo.bar.test_qux: ERROR (0s)
  some err
data.foo.bar.test_corge: FAIL (0s)
--------------------------------------------------------------------------------
PASS: 1/3
FAIL: 1/3
ERROR: 1/3
`
