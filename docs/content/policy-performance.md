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
| `input.x[input.y] = "foo"]` | no | reference is nested |

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


### Key Takeaways

For high-performance use cases:

* Write your policies to minimize iteration and search.
  * Use objects instead of arrays when you have a unique identifier for the elements of the array.
  * Consider [partial-evaluation](https://blog.openpolicyagent.org/partial-evaluation-162750eaf422) to compile non-linear policies to linear policies.
* Write your policies with indexed statements so that [rule-indexing](https://blog.openpolicyagent.org/optimizing-opa-rule-indexing-59f03f17caf3) is effective.
* Use the profiler to help identify portions of the policy that would benefit the most from improved performance.


