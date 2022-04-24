---
title: Policy Performance
kind: documentation
weight: 5
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
    input.method == "GET"
    input.path = ["accounts", user]
    input.user == user
}
```

### Use objects over arrays

One common mistake people make is using arrays when they could use objects.  For example, below is an array of ID/first-name/last-names where ID is unique, and you're looking up the first-name/last-name given the ID.

```live:prefer_objects/bad:query
# DO NOT DO THIS.
# Array of objects where each object has a unique identifier
d := [{"id": "a123", "first": "alice", "last": "smith"},
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
d := {"a123": {"first": "alice", "last": "smith"},
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

default allow := false

allow {
    some user
    input.method == "GET"
    input.path = ["accounts", user]
    input.user == user
}

allow {
    input.method == "GET"
    input.path == ["accounts", "report"]
    roles[input.user][_] == "admin"
}

allow {
    input.method == "POST"
    input.path == ["accounts"]
    roles[input.user][_] == "admin"
}

roles := {
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
| `input.x == "foo"` | yes | n/a |
| `input.x.y == "bar"` | yes | n/a |
| `input.x == ["foo", i]` | yes | n/a |
| `input.x[i] == "foo"` | no | reference contains variables |
| `input.x[input.y] == "foo"` | no | reference is nested |

#### Glob statements

For `glob.match(pattern, delimiter, match)` statements to be indexed the pattern must be recognized by the indexer and the match be a non-nested reference that does not contain any variables. The indexer recognizes patterns containing the normal glob (`*`) operator but not the super glob (`**`) or character pattern matching operators.

| Expression | Indexed | Reason |
| --- | --- | --- |
| `glob.match("foo:*:bar", [":"], input.x)` | yes | n/a |
| `glob.match("foo:**:bar", [":"], input.x)` | no | pattern contains `**` |
| `glob.match("foo:*:bar", [":"], input.x[i])` | no | match contains variable(s) |


### Early Exit in Rule Evaluation

In general, OPA has to iterate all potential variable bindings to determine the outcome
of a query. However, there are conditions under which additional iterations cannot change
the result:

1. A set of complete document rules that only have one, ground value.
2. A set of function rules that only have one, ground value.

The most common case for this are a set of `allow` rules:

```live:ee:module:read_only
package earlyexit

allow {
    input.user == "alice"
}
allow {
    input.user == "bob"
}
allow {
    input.group == "admins"
}
```

since `allow { ... }` is a shorthand for `allow = true { ... }`.

Intuitively, the value can be anything that does not contain a variable:

```live:eeexamples:module:read_only
package earlyexit.examples

# p, q, r and s could be evaluated with early-exit semantics:

p {
    # ...
}

q := 123 {
    # ...
}

r := {"hello": "world"} {
    # ...
}

s(x) := 12 {
    # ...
}

# u, v, w, and y could _not_

u[x] { # not a complete document rule, but a partial set
    x := 911
}

v := x { # x is a variable, not ground
    x := true
}

w := { "foo": x } { # a compound term containing a variable
    x := "bar"
}

y(z) := r { # variable value, not ground
    r := z + 1
}
```

When "early exit" is possible for a (set of) rules, iterations inside that rule will be
**cancelled** as soon as one binding matches the rule body:

```live:eeiteration:module:read_only
package earlyexit.iteration

p {
    some p
    data.projects[p] == "project-a"
}
```

Since there's no possibility that could change the outcome of `data.earlyexit.iteration.p`
once a variable binding is found that satisfies the conditions, no further iteration will
occur.

The check if "early exit" is applicable for a query happens _after_ the indexing lookup,
so in this contrived example, an evaluation with input `{"user": "alice"}` _would_ exit
early; an evaluation with `{"user": "bob", "group": "admins"}` _would not_:

```live:eeindex:module:read_only
package earlyexit

allow {
    input.user == "alice"
}
allow = false {
    input.user == "bob"
}
allow {
    input.group == "admins"
}
```

This is because the index lookup for `{"user": "bob", "group": "admins"}` returns two complete
document rules with _different values_, `true` and `false`, whereas the indexer query for
`{"user": "alice"}` only returns rules with value `true`.

### Comprehension Indexing

Rego does not support mutation. As a result, certain operations like "group by" require
use of comprehensions to aggregate values. To avoid O(n^2) runtime complexity in
queries/rules that perform group-by, OPA may compute and memoize the entire collection
produced by comprehensions at once. This ensures that runtime complexity is O(n) where
n is the size of the collection that group-by/aggregation is being performed on.

For example, suppose the policy must check if the number of ports exposed on an interface
exceeds some threshold (e.g., any interface may expose up to 100 ports.) The policy is given
the port->interface mapping as a JSON array under `input`:

```json
{
  "exposed": [
    {
      "interface": "eth0",
      "port": 8080,
    },
    {
      "interface": "eth0",
      "port": 8081,
    },
    {
      "interface": "eth1",
      "port": 443,
    },
    {
      "interface": "lo1",
      "port": 5000,
    }
  ]
}
```

In this case, the policy must count the number of ports exposed on each interface. To do this,
the policy must first aggregate/group the ports by the interface name. Conceptually,
the policy should generate a document like this:

```json
{
  "exposed_ports_by_interface": {
    "eth0": [8080, 8081],
    "eth1": [443],
    "lo1": [5000]
  }
}
```

Since multiple ports could be exposed on a single interface, the policy must use a comprehension to
aggregate the port values by the interface names. To implement this logic in Rego, we would write:

```rego
some i
intf := input.exposed[i].interface
ports := [port | some j; input.exposed[j].interface == intf; port := input.exposed[j].port]
```

Without comprehension indexing, this query would be O(n^2) where n is the size of `input.exposed`.
However, with comprehension indexing, the query remains O(n) because OPA only computes the comprehension
_once_. In this case, the comprehension is evaluated and all possible values of `ports` are computed
at once. These values are indexed by the assignments of `intf`.

To implement the policy above we could write:

```rego
deny[msg] {
    some i
    count(exposed_ports_by_interface[i]) > 100
    msg := sprintf("interface '%v' exposes too many ports", [i])
}

exposed_ports_by_interface := {intf: ports |
    some i
    intf := input.exposed[i].interface
    ports := [port |
        some j
        input.exposed[j].interface == intf
        port := input.exposed[j].port
  ]
}
```

Indices can be built for comprehensions (nested or not) that generate collections (i.e., arrays, sets, or objects)
based on variables in an outer query. In the example above:

* `intf` is the variable in the outer query.
* `[port | some j; input.exposed[j].interface == intf; port := input.exposed[j].port]` is the comprehension.
* `ports` is the variable the collection is assigned to.

In order to be indexed, comprehensions must meet the following conditions:

1. The comprehension appears in an assignment or unification statement.
1. The expression containing the comprehension does not include a `with` statement.
1. The expression containing the comprehension is not negated.
1. The comprehension body is safe when considered independent of the outer query.
1. The comprehension body closes over at least one variable in the outer query and none of these variables appear as outputs in references or `walk()` calls or inside nested comprehensions.

The following examples show cases that are NOT indexed:

```rego
not_indexed_because_missing_assignment {
    x := input[_]
    [y | some y; x == input[y]]
}

not_indexed_because_includes_with {
    x := input[_]
    ys := [y | some y; x := input[y]] with input as {}
}

not_indexed_because_negated {
    x := input[_]
    not data.arr = [y | some y; x := input[y]]
}

not_indexed_because_safety {
    obj := input.foo.bar
    x := obj[_]
    ys := [y | some y; x == obj[y]]
}

not_indexed_because_no_closure {
    ys := [y | x := input[y]]
}

not_indexed_because_reference_operand_closure {
    x := input[y].x
    ys := [y | x == input[y].z[_]]
}

not_indexed_because_nested_closure {
    x := 1
    y := 2
    _ = [i | x == input.foo[i]
             _ = [j | y == input.bar[j]]]
}
```

> The 4th and 5th restrictions may be relaxed in the future.

### Profiling

You can also _profile_ your policies using `opa eval`. The profiler is useful if you need to understand
why policy evaluation is slow.

The `opa eval` command provides the following profiler options:

| Option | Detail | Default |
| --- | --- | --- |
| <span class="opa-keep-it-together">`--profile`</span> | Enables expression profiling and outputs profiler results. | off |
| <span class="opa-keep-it-together">`--profile-sort`</span> | Criteria to sort the expression profiling results. This options implies `--profile`. | total_time_ns => num_eval => num_redo => file => line |
| <span class="opa-keep-it-together">`--profile-limit`</span> | Desired number of profiling results sorted on the given criteria. This options implies `--profile`. | 10 |
| <span class="opa-keep-it-together">`--count`</span> | Desired number of evaluations that profiling metrics are to be captured for. With `--format=pretty`, the output will contain min, max, mean and the 90th and 99th percentile. All collected percentiles can be found in the JSON output. | 1 |

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

input := {
	"subject": "bob",
	"resource": "foo123",
	"action": "write",
}

# Example RBAC configuration.
bindings := [
	{
		"user": "alice",
		"roles": ["dev", "test"],
	},
	{
		"user": "bob",
		"roles": ["test"],
	},
]

roles := [
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

default allow := false

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

To evaluation the policy multiple times, and aggregate the profiling data over those
runs, pass `--count=NUMBER`:


```bash
opa eval --data rbac.rego --profile --format=pretty --count=10 'data.rbac.allow'
```

**Sample Output**
```ruby
false
+------------------------------+---------+----------+---------------+----------------+---------------+
|            METRIC            |   MIN   |   MAX    |     MEAN      |      90%       |      99%      |
+------------------------------+---------+----------+---------------+----------------+---------------+
| timer_rego_load_files_ns     | 349969  | 2549399  | 1.4760619e+06 | 2.5312689e+06  | 2.549399e+06  |
| timer_rego_module_compile_ns | 1087507 | 24537496 | 1.120074e+07  | 2.41699473e+07 | 2.4537496e+07 |
| timer_rego_module_parse_ns   | 275531  | 1915263  | 1.126406e+06  | 1.9016968e+06  | 1.915263e+06  |
| timer_rego_query_compile_ns  | 61663   | 64395    | 63062.5       | 64374.1        | 64395         |
| timer_rego_query_eval_ns     | 161812  | 1198092  | 637754        | 1.1846622e+06  | 1.198092e+06  |
| timer_rego_query_parse_ns    | 6078    | 6078     | 6078          | 6078           | 6078          |
+------------------------------+---------+----------+---------------+----------------+---------------+
+----------+-------------+-------------+-------------+-------------+----------+----------+-----------------+
|   MIN    |     MAX     |    MEAN     |     90%     |     99%     | NUM EVAL | NUM REDO |    LOCATION     |
+----------+-------------+-------------+-------------+-------------+----------+----------+-----------------+
| 43.875µs | 26.135469ms | 11.494512ms | 25.746215ms | 26.135469ms | 1        | 1        | data.rbac.allow |
| 21.478µs | 211.461µs   | 98.102µs    | 205.72µs    | 211.461µs   | 1        | 1        | rbac.rego:13    |
| 19.652µs | 123.537µs   | 73.161µs    | 122.75µs    | 123.537µs   | 1        | 1        | rbac.rego:40    |
| 12.303µs | 117.277µs   | 61.59µs     | 116.733µs   | 117.277µs   | 2        | 1        | rbac.rego:50    |
| 12.224µs | 93.214µs    | 51.289µs    | 92.217µs    | 93.214µs    | 1        | 1        | rbac.rego:44    |
| 5.561µs  | 84.121µs    | 43.002µs    | 83.469µs    | 84.121µs    | 1        | 1        | rbac.rego:51    |
| 5.56µs   | 71.712µs    | 36.545µs    | 71.158µs    | 71.712µs    | 1        | 0        | rbac.rego:45    |
| 4.958µs  | 66.04µs     | 33.161µs    | 65.636µs    | 66.04µs     | 1        | 2        | rbac.rego:49    |
| 4.326µs  | 65.836µs    | 30.461µs    | 65.083µs    | 65.836µs    | 1        | 1        | rbac.rego:6     |
| 3.948µs  | 43.399µs    | 24.167µs    | 43.055µs    | 43.399µs    | 1        | 2        | rbac.rego:55    |
+----------+-------------+-------------+-------------+-------------+----------+----------+-----------------+
```

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

## Resource Utilization

Policy evaluation is typically CPU-bound unless the policies have to pull additional
data on-the-fly using built-in functions like `http.send()` (in which case evaluation
likely becomes I/O-bound.) Policy evaluation is currently single-threaded. If you
are embedding OPA as a library, it is your responsibility to dispatch concurrent queries
to different Goroutines/threads. If you are running the OPA server, it will parallelize
concurrent requests and use as many cores as possible. You can limit the number of
cores that OPA can consume by starting OPA with the [`GOMAXPROCS`](https://golang.org/pkg/runtime)
environment variable.

Memory usage scales with the size of the policy (i.e., Rego) and data (e.g., JSON) that you
load into OPA. Raw JSON data loaded into OPA uses approximately 20x more memory compared to the
same data stored in a compact, serialized format (e.g., on disk). This increased
memory usage is due to the need to load the JSON data into Go data structures like maps,
slices, and strings so that it can be evaluated. For example, if you load 8MB worth of
JSON data representing 100,000 permission objects specifying subject/action/resource triplets,
OPA would consume approximately 160MB of RAM.

Memory usage also scales linearly with the number of rules loaded into OPA. For example,
loading 10,000 rules that implement an ACL-style authorization policy consumes approximately
130MB of RAM while 100,000 rules implementing the same policy (but with 10x more tuples to check)
consumes approximately 1.1GB of RAM.

## Optimization Levels

The `--optimize` (or `-O`) flag on the `opa build` command controls how bundles are optimized.

> Optimization applies partial evaluation to precompute _known_ values in the policy. The goal of
partial evaluation is to convert non-linear-time policies into linear-time policies.

By specifying the `--optimize` flag, users can control how much time and resources are spent
attempting to optimize the bundle. Generally, higher optimization levels require more time
and resources. Currently, OPA supports three optimization levels. The exact optimizations applied
in each level may change over time.

### -O=0 (default)

By default optimizations are disabled.

### -O=1 (recommended)

Policies are partially evaluated. Rules that DO NOT depend on unknowns (directly or indirectly) are
evaluated and the virtual documents they produce are inlined into call sites. Virtual documents that
are required at evaluation time are not inlined. For example, if a base or virtual document is
targetted by a `with` statement in the policy, the document will not be inlined.

Rules that depend on unknowns (directly or indirectly) are also partially evaluated however the
virtual documents they produce ARE NOT inlined into call sites. The output policy should be structurally
similar to the input policy.

The `opa build` automatically marks the `input` document as unknown. In addition to the `input` document,
if `opa build` is invoked with the `-b`/`--bundle` flag, any `data` references NOT prefixed by the
`.manifest` roots are also marked as unknown.

### -O=2 (aggressive)

Same as `-O=1` except virtual documents produced by rules that depend on unknowns may be inlined
into call sites. In addition, more aggressive inlining is applied within rules. This includes
[copy propagation](https://en.wikipedia.org/wiki/Copy_propagation) and inlining of certain negated
statements that would otherwise generate support rules.

## Key Takeaways

For high-performance use cases:

* Write your policies to minimize iteration and search.
  * Use objects instead of arrays when you have a unique identifier for the elements of the array.
  * Consider [partial-evaluation](https://blog.openpolicyagent.org/partial-evaluation-162750eaf422) to compile non-linear policies to linear policies.
* Write your policies with indexed statements so that [rule-indexing](https://blog.openpolicyagent.org/optimizing-opa-rule-indexing-59f03f17caf3) is effective.
* Use the profiler to help identify portions of the policy that would benefit the most from improved performance.
* Use the benchmark tools to help get real world timing data and detect policy performance changes.
