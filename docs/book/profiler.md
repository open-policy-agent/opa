# Profiler

This page describes how to use the code profiler for Rego. The Profiler
captures the following information about Rego expressions:

  * Expression Time
  * Number of Evaluations
  * Number of Re-Evaluations (Redo)
  * Expression Location in source code

OPA's `eval` command provides the following profiler options

| Command line Option        | Detail                                                        | Default |
|:--------------------------:|:-------------------------------------------------------------:|:-----:|
| `profile`                  | Returns profiling results sorted by the default or given criteria              | NA |
| `profile-sort`             | Criteria to sort the expression profiling results. This options implies `--profile`. |  total_time_ns => num_eval => num_redo => file => line |
| `profile-limit`            | Desired number of profiling results sorted on the given criteria. This options implies `--profile`. |  10 |

## Sort criteria for the profile results
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

{%ace edit=false, lang='python'%}
package rbac

# Example input request.
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
	user_has_role[role_name]
	role_has_permission[role_name]
}

user_has_role[role_name] {
	binding := bindings[_]
	binding.user = input.subject
	role_name := binding.roles[_]
}

role_has_permission[role_name] {
	role := roles[_]
	role_name := role.name
	perm := role.permissions[_]
	perm.resource = input.resource
	perm.action = input.action
}
{%endace%}

#### Example: Display `ALL` profile results with `default` ordering criteria
```bash
opa eval --data foo --profile --format=pretty 'data.rbac.allow'
```

**Sample Output**
```ruby
false

+----------+----------+----------+------------------+
|   TIME   | NUM EVAL | NUM REDO |     LOCATION     |
+----------+----------+----------+------------------+
| 70.945µs | 1        | 1        | data.rbac.allow  |
| 32.02µs  | 1        | 1        | foo/rbac.rego:41 |
| 24.175µs | 1        | 1        | foo/rbac.rego:11 |
| 18.263µs | 2        | 1        | foo/rbac.rego:47 |
| 15.763µs | 1        | 1        | foo/rbac.rego:38 |
| 11.645µs | 1        | 2        | foo/rbac.rego:46 |
| 11.023µs | 1        | 1        | foo/rbac.rego:55 |
| 10.267µs | 1        | 0        | foo/rbac.rego:56 |
| 9.84µs   | 1        | 1        | foo/rbac.rego:48 |
| 9.441µs  | 1        | 2        | foo/rbac.rego:52 |
+----------+----------+----------+------------------+

+------------------------------+-----------+
|             NAME             |   VALUE   |
+------------------------------+-----------+
| timer_rego_query_compile_ns  | 63763     |
| timer_rego_query_eval_ns     | 255809    |
| timer_rego_query_parse_ns    | 138497172 |
| timer_rego_module_compile_ns | 34227625  |
+------------------------------+-----------+
```
As seen from the above table, all results are dislayed. The profile results are
sorted on the default sort criteria.

#### Example: Display top `5` profile results
```bash
opa eval --data foo --profile-limit 5 --format=pretty 'data.rbac.allow'
```

**Sample Output**
```ruby
false

+----------+----------+----------+------------------+
|   TIME   | NUM EVAL | NUM REDO |     LOCATION     |
+----------+----------+----------+------------------+
| 56.298µs | 1        | 1        | foo/rbac.rego:41 |
| 52.851µs | 1        | 1        | data.rbac.allow  |
| 41.498µs | 2        | 1        | foo/rbac.rego:47 |
| 38.146µs | 1        | 1        | foo/rbac.rego:4  |
| 24.714µs | 1        | 1        | foo/rbac.rego:38 |
+----------+----------+----------+------------------+

+------------------------------+-----------+
|             NAME             |   VALUE   |
+------------------------------+-----------+
| timer_rego_query_parse_ns    | 139512691 |
| timer_rego_module_compile_ns | 31450043  |
| timer_rego_query_compile_ns  | 71703     |
| timer_rego_query_eval_ns     | 357086    |
+------------------------------+-----------+
```
The profile results are sorted on the default sort criteria.
Also `--profile` option is implied and does not need to be provided.

#### Example: Display top `5` profile results based on the `number of times an expression is evaluated`
```bash
opa  eval --data foo --profile-limit 5 --profile-sort num_eval --format=pretty 'data.rbac.allow'
```

**Sample Profile Output**
```ruby
+----------+----------+----------+------------------+
|   TIME   | NUM EVAL | NUM REDO |     LOCATION     |
+----------+----------+----------+------------------+
| 20.036µs | 2        | 1        | foo/rbac.rego:47 |
| 9.295µs  | 2        | 1        | foo/rbac.rego:53 |
| 37.609µs | 1        | 1        | foo/rbac.rego:41 |
| 33.43µs  | 1        | 1        | data.rbac.allow  |
| 24.19µs  | 1        | 1        | foo/rbac.rego:11 |
+----------+----------+----------+------------------+
```
As seen from the above table, the results are arranged first in decreasing
order of number of evaluations and if two expressions have been evaluated
the same number of times, the default criteria is used since no other sort criteria is provided.
In this case, total_time_ns => num_redo => file => line.
Also `--profile` option is implied and does not need to be provided.

#### Example: Display top `5` profile results based on the `number of times an expression is evaluated` and `number of times an expression is re-evaluated`
```bash
opa eval --data foo --profile-limit 5 --profile-sort num_eval,num_redo --format=pretty 'data.rbac.allow'
```

**Sample Profile Output**
```ruby
+----------+----------+----------+------------------+
|   TIME   | NUM EVAL | NUM REDO |     LOCATION     |
+----------+----------+----------+------------------+
| 19.158µs | 2        | 1        | foo/rbac.rego:47 |
| 7.86µs   | 2        | 1        | foo/rbac.rego:53 |
| 11.526µs | 1        | 2        | foo/rbac.rego:46 |
| 9.435µs  | 1        | 2        | foo/rbac.rego:52 |
| 37.436µs | 1        | 1        | foo/rbac.rego:41 |
+----------+----------+----------+------------------+
```
As seen from the above table, result are first arranged based on _number of evaluations_,
then _number of re-evaluations_ and finally the default criteria is used.
In this case, total_time_ns => file => line.
The `--profile-sort` options accepts repeated or comma-separated values for the criteria.
The order of the criteria on the command line determine their priority.

Another way to get the same output as above would be the following:
```bash
opa eval --data foo --profile-limit 5 --profile-sort num_eval --profile-sort num_redo --format=pretty 'data.rbac.allow'
```
