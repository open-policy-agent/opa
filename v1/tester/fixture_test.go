package tester

const fixtureReporterVerboseBenchmark = `data.foo.bar.test_baz	    1000	       123.0 ns/op
data.foo.bar.test_qux: ERROR (0s)
  some err
data.foo.bar.test_corge: FAIL (0s)
data.foo.bar.test_cases_fail: FAIL (0s)
  one: PASS
  two: FAIL
data.foo.bar.test_cases_ok	    2000	        61.50 ns/op
--------------------------------------------------------------------------------
FAILURES
--------------------------------------------------------------------------------
data.foo.bar.test_corge: FAIL (0s)

  query:1       | Fail true = false  

data.foo.bar.test_cases_fail: FAIL (0s)

  query:1       | Fail true = false  

  two: FAIL

--------------------------------------------------------------------------------
PASS: 4/7
FAIL: 2/7
ERROR: 1/7
`

const fixtureReporterVerboseBenchmarkShowAllocations = `data.foo.bar.test_baz	    1000	       123.0 ns/op	       123.0 timer_rego_query_eval_ns/op	      91 B/op	       0 allocs/op
data.foo.bar.test_qux: ERROR (0s)
  some err
data.foo.bar.test_corge: FAIL (0s)
--------------------------------------------------------------------------------
FAILURES
--------------------------------------------------------------------------------
data.foo.bar.test_corge: FAIL (0s)


--------------------------------------------------------------------------------
PASS: 1/3
FAIL: 1/3
ERROR: 1/3
`

const fixtureReporterVerboseBenchmarkShowAllocationsGoBenchFormat = `BenchmarkDataFooBarTestBaz	    1000	       123.0 ns/op	       123.0 timer_rego_query_eval_ns/op	      91 B/op	       0 allocs/op
data.foo.bar.test_qux: ERROR (0s)
  some err
data.foo.bar.test_corge: FAIL (0s)
--------------------------------------------------------------------------------
FAILURES
--------------------------------------------------------------------------------
data.foo.bar.test_corge: FAIL (0s)


--------------------------------------------------------------------------------
PASS: 1/3
FAIL: 1/3
ERROR: 1/3
`
