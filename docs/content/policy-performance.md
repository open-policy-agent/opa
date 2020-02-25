---
title: Policy Performance
kind: documentation
weight: 3
---
## High Performance Policy Decisions

For low-latency/high-performance use-cases, e.g. microservice API authorization, policy evaluation has a budget on the order of 1 millisecond.  Not all use cases require that kind of performance, and OPA is powerful enough that you can write expressive policies that take longer than 1 millisecond to evaluate.  But for high-performance use cases, there is a fragment of the policy language that has been engineered to evaluate quickly.  Even as the size of the policies grow, the performance for this fragment can be nearly constant-time.

### Linear fragment

 The *linear fragment* of the language is all of those policies where evaluation amounts to walking over the policy once.  This means there is no search required to make a policy decision.  Any variables you use can be assigned at most one value.

For example, the following rule has one local variable `user`, and that variable can only be assigned one value.  Intuitively, evaluating this rule requires checking each of the conditions in the body, and if there were N of these rules, evaluation would only require walking over each of them as well.

```live:linear:module:read_only,openable
package linear

allow {
  some user
  input.method = "GET"
  input.path = ["accounts", user]
  input.user = user
}
```

### Use objects over arrays

One common mistake people make is using arrays when they could use objects.  For example, below is an array of ID/first-name/last-names where ID is unique, and you're looking up the first-name/last-name given the ID.

```live:prefer_objects/bad:query
# DO NOT DO THIS.
# Array of objects where each object has a unique identifier
d = [{"id": "a123", "first": "alice", "last": "smith"},
     {"id": "a456", "first": "bob", "last": "jones"},
     {"id": "a789", "first": "clarice", "last": "johnson"}
     ]
# search through all elements of the array to find the ID
d[i].id == "a789"
d[i].first ...
```

Instead, use a dictionary where the key is the ID and the value is the first-name/last-name.  Given the ID, you can lookup the name information directly.

```live:prefer_objects/good:query
# DO THIS INSTEAD OF THE ABOVE
# Use object whose keys are the IDs for the objects.
#   Looking up an object given its ID requires NO search
d = {"a123": {"first": "alice", "last": "smith"},
     "a456": {"first": "bob", "last": "jones"},
     "a789": {"first": "clarice", "last": "johnson"}
    }
# no search required
d["a789"].first ...
```

### Use indexed statements

The linear-time fragment ensures that the cost of evaluation is no larger than the size of the policy.  OPA lets you write non-linear policies, because sometimes you need to, and because sometimes it's convenient.  The blog on [partial evaluation](https://blog.openpolicyagent.org/partial-evaluation-162750eaf422) describes one mechanism for converting non-linear policies into linear policies.

But as the size of the policy grows, the cost of evaluation grows with it.  Sometimes the policy can grow large enough that even the linear-fragment fails to meet the performance budget.

In the linear fragment, OPA includes special algorithms that **index rules efficiently**, sometimes making evaluation constant-time, even as the policy grows. The more effective the indexing is the fewer rules need to be evaluated.

Here is an example policy from the [rule-indexing blog](https://blog.openpolicyagent.org/optimizing-opa-rule-indexing-59f03f17caf3) giving the details for these algorithms. See the rest of this section for details on indexed statements.

```live:indexed:module:openable
package indexed

default allow = false

allow {
  some user
  input.method = "GET"
  input.path = ["accounts", user]
  input.user = user
}

allow {
  input.method = "GET"
  input.path = ["accounts", "report"]
  roles[input.user][_] = "admin"
}

allow {
  input.method = "POST"
  input.path = ["accounts"]
  roles[input.user][_] = "admin"
}

roles = {
  "bob": ["admin", "hr"],
  "alice": ["procurement"],
}
```

```live:indexed:query:hidden
allow
```

```live:indexed:input
{
  "user": "bob",
  "path": ["accounts", "bob"],
  "method": "GET"
}
```

```live:indexed:output
```

#### Equality statements

For simple equality statements (`=` and `==`) to be indexed one side must be a non-nested reference that does not contain any variables and the other side must be a variable, scalar, or array (which may contain scalars and variables). For example:

| Expression | Indexed | Reason |
| --- | --- | --- |
| `input.x = "foo"` | yes | n/a |
| `input.x.y = "bar"` | yes | n/a |
| `input.x = ["foo", i]` | yes | n/a |
| `input.x[i] = "foo"` | no | reference contains variables |
| `input.x[input.y] = "foo"` | no | reference is nested |

#### Glob statements

For `glob.match(pattern, delimiter, match)` statements to be indexed the pattern must be recognized by the indexer and the match be a non-nested reference that does not contain any variables. The indexer recognizes patterns containing the normal glob (`*`) operator but not the super glob (`**`) or character pattern matching operators.

| Expression | Indexed | Reason |
| --- | --- | --- |
| `glob.match("foo:*:bar", [":"], input.x)` | yes | n/a |
| `glob.match("foo:**:bar", [":"], input.x)` | no | pattern contains `**` |
| `glob.match("foo:*:bar", [":"], input.x[i])` | no | match contains variable(s) |

### Profiling

You can also _profile_ your policies using `opa eval`. The profiler is useful if you need to understand
why policy evaluation is slow.

The `opa eval` command provides the following profiler options:

| Option | Detail | Default |
| --- | --- | --- |
| <span class="opa-keep-it-together">`--profile`</span> | Enables expression profiling and outputs profiler results. | off |
| <span class="opa-keep-it-together">`--profile-sort`</span> | Criteria to sort the expression profiling results. This options implies `--profile`. | total_time_ns => num_eval => num_redo => file => line |
| <span class="opa-keep-it-together">`--profile-limit`</span> | Desired number of profiling results sorted on the given criteria. This options implies `--profile`. | 10 |

#### Sort criteria for the profile results

  * `total_time_ns` - Results are displayed is decreasing order of _expression evaluation time_
  * `num_eval`  - Results are displayed is decreasing order of _number of times an expression is evaluated_
  * `num_redo`  - Results are displayed is decreasing order of _number of times an expression is re-evaluated(redo)_
  * `file`  - Results are sorted in reverse alphabetical order based on the _rego source filename_
  * `line`  - Results are displayed is decreasing order of _expression line number_ in the source file

When the sort criteria is not provided `total_time_ns` has the **highest** priority
while `line` has the **lowest**.

#### Example Policy

The different profiling examples shown later on this page use the below
sample policy.

```live:profile:module:read_only,openable
package rbac

# Example input request

input = {
	"subject": "bob",
	"resource": "foo123",
	"action": "write",
}

# Example RBAC configuration.
bindings = [
	{
		"user": "alice",
		"roles": ["dev", "test"],
	},
	{
		"user": "bob",
		"roles": ["test"],
	},
]

roles = [
	{
		"name": "dev",
		"permissions": [
			{"resource": "foo123", "action": "write"},
			{"resource": "foo123", "action": "read"},
		],
	},
	{
		"name": "test",
		"permissions": [{"resource": "foo123", "action": "read"}],
	},
]

# Example RBAC policy implementation.

default allow = false

allow {
    some role_name
    user_has_role[role_name]
    role_has_permission[role_name]
}

user_has_role[role_name] {
    binding := bindings[_]
    binding.user == input.subject
    role_name := binding.roles[_]
}

role_has_permission[role_name] {
    role := roles[_]
    role_name := role.name
    perm := role.permissions[_]
    perm.resource == input.resource
    perm.action == input.action
}
```

#### Example: Display `ALL` profile results with `default` ordering criteria

```bash
opa eval --data rbac.rego --profile --format=pretty 'data.rbac.allow'
```

**Sample Output**
```ruby
false

+----------+----------+----------+-----------------+
|   TIME   | NUM EVAL | NUM REDO |    LOCATION     |
+----------+----------+----------+-----------------+
| 47.148µs | 1        | 1        | data.rbac.allow |
| 28.965µs | 1        | 1        | rbac.rego:11    |
| 24.384µs | 1        | 1        | rbac.rego:41    |
| 23.064µs | 2        | 1        | rbac.rego:47    |
| 15.525µs | 1        | 1        | rbac.rego:38    |
| 14.137µs | 1        | 2        | rbac.rego:46    |
| 13.927µs | 1        | 0        | rbac.rego:42    |
| 13.568µs | 1        | 1        | rbac.rego:55    |
| 12.982µs | 1        | 0        | rbac.rego:56    |
| 12.763µs | 1        | 2        | rbac.rego:52    |
+----------+----------+----------+-----------------+

+------------------------------+----------+
|            METRIC            |  VALUE   |
+------------------------------+----------+
| timer_rego_module_compile_ns | 1871613  |
| timer_rego_query_compile_ns  | 82290    |
| timer_rego_query_eval_ns     | 257952   |
| timer_rego_query_parse_ns    | 12337169 |
+------------------------------+----------+
```
As seen from the above table, all results are displayed. The profile results are
sorted on the default sort criteria.

##### Example: Display top `5` profile results

```bash
opa eval --data rbac.rego --profile-limit 5 --format=pretty 'data.rbac.allow'
```

**Sample Output**
```ruby
+----------+----------+----------+-----------------+
|   TIME   | NUM EVAL | NUM REDO |    LOCATION     |
+----------+----------+----------+-----------------+
| 46.329µs | 1        | 1        | data.rbac.allow |
| 26.656µs | 1        | 1        | rbac.rego:11    |
| 24.206µs | 2        | 1        | rbac.rego:47    |
| 23.235µs | 1        | 1        | rbac.rego:41    |
| 18.242µs | 1        | 1        | rbac.rego:38    |
+----------+----------+----------+-----------------+
```
The profile results are sorted on the default sort criteria.
Also `--profile` option is implied and does not need to be provided.

##### Example: Display top `5` profile results based on the `number of times an expression is evaluated`

```bash
opa  eval --data rbac.rego --profile-limit 5 --profile-sort num_eval --format=pretty 'data.rbac.allow'
```

**Sample Profile Output**
```ruby
+----------+----------+----------+-----------------+
|   TIME   | NUM EVAL | NUM REDO |    LOCATION     |
+----------+----------+----------+-----------------+
| 26.675µs | 2        | 1        | rbac.rego:47    |
| 9.274µs  | 2        | 1        | rbac.rego:53    |
| 43.356µs | 1        | 1        | data.rbac.allow |
| 22.467µs | 1        | 1        | rbac.rego:41    |
| 22.425µs | 1        | 1        | rbac.rego:11    |
+----------+----------+----------+-----------------+
```
As seen from the above table, the results are arranged first in decreasing
order of number of evaluations and if two expressions have been evaluated
the same number of times, the default criteria is used since no other sort criteria is provided.
In this case, total_time_ns => num_redo => file => line.
Also `--profile` option is implied and does not need to be provided.

##### Example: Display top `5` profile results based on the `number of times an expression is evaluated` and `number of times an expression is re-evaluated`

```bash
opa eval --data rbac.rego --profile-limit 5 --profile-sort num_eval,num_redo --format=pretty 'data.rbac.allow'
```

**Sample Profile Output**
```ruby
+----------+----------+----------+-----------------+
|   TIME   | NUM EVAL | NUM REDO |    LOCATION     |
+----------+----------+----------+-----------------+
| 22.892µs | 2        | 1        | rbac.rego:47    |
| 8.831µs  | 2        | 1        | rbac.rego:53    |
| 13.767µs | 1        | 2        | rbac.rego:46    |
| 10.78µs  | 1        | 2        | rbac.rego:52    |
| 42.338µs | 1        | 1        | data.rbac.allow |
+----------+----------+----------+-----------------+
```
As seen from the above table, result are first arranged based on _number of evaluations_,
then _number of re-evaluations_ and finally the default criteria is used.
In this case, total_time_ns => file => line.
The `--profile-sort` options accepts repeated or comma-separated values for the criteria.
The order of the criteria on the command line determine their priority.

Another way to get the same output as above would be the following:
```bash
opa eval --data rbac.rego --profile-limit 5 --profile-sort num_eval --profile-sort num_redo --format=pretty 'data.rbac.allow'
```

## Benchmarking Queries
OPA provides CLI options to benchmark a single query via the `opa bench` command. This will evaluate similarly to
`opa eval` but it will repeat the evaluation (in its most efficient form) a number of times and report metrics.


#### Example: Benchmark rbac allow
Using the same [policy source as shown above](#example-policy):
```bash
$ opa bench --data rbac.rego 'data.rbac.allow'
```

Will result in an output similar to:
```
+-------------------------------------------+------------+
| samples                                   |      27295 |
| ns/op                                     |      45032 |
| B/op                                      |      20977 |
| allocs/op                                 |        382 |
| histogram_timer_rego_query_eval_ns_stddev |      25568 |
| histogram_timer_rego_query_eval_ns_99.9%  |     335906 |
| histogram_timer_rego_query_eval_ns_99.99% |     336493 |
| histogram_timer_rego_query_eval_ns_mean   |      40355 |
| histogram_timer_rego_query_eval_ns_median |      35846 |
| histogram_timer_rego_query_eval_ns_99%    |     133936 |
| histogram_timer_rego_query_eval_ns_90%    |      44780 |
| histogram_timer_rego_query_eval_ns_95%    |      50815 |
| histogram_timer_rego_query_eval_ns_min    |      31284 |
| histogram_timer_rego_query_eval_ns_max    |     336493 |
| histogram_timer_rego_query_eval_ns_75%    |      38254 |
| histogram_timer_rego_query_eval_ns_count  |      27295 |
+-------------------------------------------+------------+
```

These results capture metrics of `samples` runs, where only the query evaluation is measured. All time spent preparing
to evaluate (loading, parsing, compiling, etc.) is omitted.

> Note: all `*/op` results are an average over the number of `samples` (or `N` in the JSON format)

#### Options for `opa bench`
| Option | Detail | Default |
| --- | --- | --- |
| <span class="opa-keep-it-together">`--benchmem`</span> | Report memory allocations with benchmark results. | true |
| <span class="opa-keep-it-together">`--metrics`</span> | Report additional query performance metrics. | true |
| <span class="opa-keep-it-together">`--count`</span> | Number of times to repeat the benchmark. | 1 |


### Benchmarking OPA Tests

There is also a `--bench` option for `opa test` which will perform benchmarking on OPA unit tests. This will evaluate
any loaded tests as benchmarks. There will be additional time for any test-specific actions are included so the timing
will typically be longer than what is seen with `opa bench`. The primary use-case is not for absolute time, but to
track relative time as policies change.

#### Options for `opa test --bench`
| Option | Detail | Default |
| --- | --- | --- |
| <span class="opa-keep-it-together">`--benchmem`</span> | Report memory allocations with benchmark results. | true |
| <span class="opa-keep-it-together">`--count`</span> | Number of times to repeat the benchmark. | 1 |


#### Example Tests
Adding a unit test file for the [policy source as shown above](#example-policy):

```rego
package rbac


test_user_has_role_dev {
    user_has_role["dev"] with input as {"subject": "alice"}
}

test_user_has_role_negative {
    not user_has_role["super-admin"] with input as {"subject": "alice"}
}
```

Which when run normally will output something like:
```
$ opa test -v ./rbac.rego ./rbac_test.rego
data.rbac.test_user_has_role_dev: PASS (605.076µs)
data.rbac.test_user_has_role_negative: PASS (318.047µs)
--------------------------------------------------------------------------------
PASS: 2/2
```

#### Example: Benchmark rbac unit tests

```bash
opa test -v --bench ./rbac.rego ./rbac_test.rego
```
Results in output:
```
data.rbac.test_user_has_role_dev	   44749	     27677 ns/op	     23146 timer_rego_query_eval_ns/op	   12303 B/op	     229 allocs/op
data.rbac.test_user_has_role_negative	   44526	     26348 ns/op	     22033 timer_rego_query_eval_ns/op	   12470 B/op	     235 allocs/op
--------------------------------------------------------------------------------
PASS: 2/2
```

#### Example: Benchmark rbac unit tests and compare with `benchstat`
The benchmark output formats default to `pretty`, but support a `gobench` format which complies with the
[Golang Benchmark Data Format](https://go.googlesource.com/proposal/+/master/design/14313-benchmark-format.md).
This allows for usage of tools like [benchstat](https://godoc.org/golang.org/x/perf/cmd/benchstat) to gain additional
insight into the benchmark results and to diff between benchmark results.

Example:
```bash
opa test -v --bench --count 10 --format gobench ./rbac.rego ./rbac_test.rego | tee ./old.txt
```
Will result in an `old.txt` and output similar to:
```
BenchmarkDataRbacTestUserHasRoleDev	   45152	     26323 ns/op	     22026 timer_rego_query_eval_ns/op	   12302 B/op	     229 allocs/op
BenchmarkDataRbacTestUserHasRoleNegative	   45483	     26253 ns/op	     21986 timer_rego_query_eval_ns/op	   12470 B/op	     235 allocs/op
--------------------------------------------------------------------------------
PASS: 2/2
.
.
```
Repeated 10 times (as specified by the `--count` flag).

This format can then be loaded by `benchstat`:

```bash
benchstat ./old.txt
```
Output:
```
name                             time/op
DataRbacTestUserHasRoleDev                       29.8µs ±18%
DataRbacTestUserHasRoleNegative                  32.0µs ±35%

name                             timer_rego_query_eval_ns/op
DataRbacTestUserHasRoleDev                        25.0k ±18%
DataRbacTestUserHasRoleNegative                   26.7k ±35%

name                             alloc/op
DataRbacTestUserHasRoleDev                       12.3kB ± 0%
DataRbacTestUserHasRoleNegative                  12.5kB ± 0%

name                             allocs/op
DataRbacTestUserHasRoleDev                          229 ± 0%
DataRbacTestUserHasRoleNegative                     235 ± 0%
```

If later on a change was introduced that altered the performance we can run again:

```bash
opa test -v --bench --count 10 --format gobench ./rbac.rego ./rbac_test.rego | tee ./new.txt
```
```
BenchmarkDataRbacTestUserHasRoleDev	   27415	     43671 ns/op	     39301 timer_rego_query_eval_ns/op	   17201 B/op	     379 allocs/op
BenchmarkDataRbacTestUserHasRoleNegative	   27583	     44743 ns/op	     40152 timer_rego_query_eval_ns/op	   17369 B/op	     385 allocs/op
--------------------------------------------------------------------------------
PASS: 2/2
.
.
```
(Repeated 10 times)

Then we can compare the results via:

```bash
benchstat ./old.txt ./new.txt
```
```
name                             old time/op                      new time/op                      delta
DataRbacTestUserHasRoleDev                           29.8µs ±18%                      47.4µs ±15%  +59.06%  (p=0.000 n=9+10)
DataRbacTestUserHasRoleNegative                      32.0µs ±35%                      47.1µs ±14%  +47.48%  (p=0.000 n=10+9)

name                             old timer_rego_query_eval_ns/op  new timer_rego_query_eval_ns/op  delta
DataRbacTestUserHasRoleDev                            25.0k ±18%                       42.6k ±15%  +70.51%  (p=0.000 n=9+10)
DataRbacTestUserHasRoleNegative                       26.7k ±35%                       42.3k ±14%  +58.15%  (p=0.000 n=10+9)

name                             old alloc/op                     new alloc/op                     delta
DataRbacTestUserHasRoleDev                           12.3kB ± 0%                      17.2kB ± 0%  +39.81%  (p=0.000 n=10+10)
DataRbacTestUserHasRoleNegative                      12.5kB ± 0%                      17.4kB ± 0%  +39.28%  (p=0.000 n=10+10)

name                             old allocs/op                    new allocs/op                    delta
DataRbacTestUserHasRoleDev                              229 ± 0%                         379 ± 0%  +65.50%  (p=0.000 n=10+10)
DataRbacTestUserHasRoleNegative                         235 ± 0%                         385 ± 0%  +63.83%  (p=0.000 n=10+10)
```

This gives clear feedback that the evaluations have slowed down considerably by looking at the `delta`

> Note that for [benchstat](https://godoc.org/golang.org/x/perf/cmd/benchstat) you will want to run with `--count` to
> repeat the benchmarks a number of times (5-10 is usually enough). The tool requires several data points else the `p`
> value will not show meaningful changes and the `delta` will be `~`.

### Key Takeaways

For high-performance use cases:

* Write your policies to minimize iteration and search.
  * Use objects instead of arrays when you have a unique identifier for the elements of the array.
  * Consider [partial-evaluation](https://blog.openpolicyagent.org/partial-evaluation-162750eaf422) to compile non-linear policies to linear policies.
* Write your policies with indexed statements so that [rule-indexing](https://blog.openpolicyagent.org/optimizing-opa-rule-indexing-59f03f17caf3) is effective.
* Use the profiler to help identify portions of the policy that would benefit the most from improved performance.
* Use the benchmark tools to help get real world timing data and detect policy performance changes.

