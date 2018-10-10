# How Do I Test Policies?

OPA gives you a high-level declarative language
([Rego](how-do-i-write-policies.md)) to author fine-grained policies that
codify important requirements in your system.

To help you verify the correctness of your policies, OPA also gives you a
framework that you can use to write _tests_ for your policies. By writing
tests for your policies you can speed up the development process of new rules
and reduce the amount of time it takes to modify rules as requirements evolve.

## Getting Started

Let's use an example to get started. The file below implements a simple
policy that allows new users to be created and users to access their own
profile.

**example.rego**:

```ruby
package authz

allow {
    input.path = ["users"]
    input.method = "POST"
}

allow {
    input.path = ["users", profile_id]
    input.method = "GET"
    profile_id = input.user_id
}
```

To test this policy, we will create a separate Rego file that contains test cases.

**example_test.rego**:

```ruby
package authz

test_post_allowed {
    allow with input as {"path": ["users"], "method": "POST"}
}

test_get_anonymous_denied {
    not allow with input as {"path": ["users"], "method": "GET"}
}

test_get_user_allowed {
    allow with input as {"path": ["users", "bob"], "method": "GET", "user_id": "bob"}
}

test_get_another_user_denied {
    not allow with input as {"path": ["users", "bob"], "method": "GET", "user_id": "alice"}
}
```

Both of these files are saved in the same directory.

```bash
$ ls
example.rego      example_test.rego
```

To exercise the policy, run the `opa test` command in the directory containing the files.

```bash
$ opa test . -v
data.authz.test_post_allowed: PASS (1.417µs)
data.authz.test_get_anonymous_denied: PASS (426ns)
data.authz.test_get_user_allowed: PASS (367ns)
data.authz.test_get_another_user_denied: PASS (320ns)
--------------------------------------------------------------------------------
PASS: 4/4
```

The `opa test` output indicates that all of the tests passed.

Try exercising the tests a bit more by removing the first rule in **example.rego**.

```bash
$ opa test . -v
data.authz.test_post_allowed: FAIL (607ns)
data.authz.test_get_anonymous_denied: PASS (288ns)
data.authz.test_get_user_allowed: PASS (346ns)
data.authz.test_get_another_user_denied: PASS (365ns)
--------------------------------------------------------------------------------
PASS: 3/4
FAIL: 1/4
```

## Test Format

Tests are expressed as standard Rego rules with a convention that the rule
name is prefixed with `test_`.

```ruby
package mypackage

test_some_descriptive_name {
    # test logic
}
```

## Test Discovery

The `opa test` subcommand runs all of the tests (i.e., rules prefixed with
`test_`) found in Rego files passed on the command line. If directories are
passed as command line arguments, `opa test` will load their file contents
recursively.

## Test Results

If the test rule is undefined or generates a non-`true` value the test result
is reported as `FAIL`. If the test encounters a runtime error (e.g., a divide
by zero condition) the test result is marked as an `ERROR`. Otherwise, the
test result is marked as `PASS`.

**pass_fail_error_test.rego**:

```ruby
package example

# This test will pass.
test_ok {
    true
}

# This test will fail.
test_failure {
    1 = 2
}

# This test will error.
test_error {
    1 / 0
}
```

By default, `opa test` reports the number of tests executed and displays all
of the tests that failed or errored.

```bash
$ opa test pass_fail_error_test.rego
data.example.test_failure: FAIL (253ns)
data.example.test_error: ERROR (289ns)
  pass_fail_error_test.rego:15: eval_internal_error: div: divide by zero
--------------------------------------------------------------------------------
PASS: 1/3
FAIL: 1/3
ERROR: 1/3
```

By default, OPA prints the test results in a human-readable format. If you
need to consume the test results programmatically, use the JSON output format.

```bash
$ opa test --format=json pass_fail_error_test.rego
```

```json
[
  {
    "location": {
      "file": "pass_fail_error_test.rego",
      "row": 4,
      "col": 1
    },
    "package": "data.example",
    "name": "test_ok",
    "duration": 618515
  },
  {
    "location": {
      "file": "pass_fail_error_test.rego",
      "row": 9,
      "col": 1
    },
    "package": "data.example",
    "name": "test_failure",
    "fail": true,
    "duration": 322177
  },
  {
    "location": {
      "file": "pass_fail_error_test.rego",
      "row": 14,
      "col": 1
    },
    "package": "data.example",
    "name": "test_error",
    "error": {
      "code": "eval_internal_error",
      "message": "div: divide by zero",
      "location": {
        "file": "pass_fail_error_test.rego",
        "row": 15,
        "col": 5
      }
    },
    "duration": 345148
  }
]
```

## Data Mocking

OPA's `with` keyword can be used to replace the data document. Both base and virtual documents can be replaced. Below is a simple policy that depends on the data document.

**authz.rego**:

```ruby
package authz

allow {
    x := data.policies[_]
    x.name = "test_policy"
    matches_role(input.role)
}

matches_role(my_role) {
    data.roles[my_role][_] = input.user
}
```

Below is the Rego file to test the above policy.

**authz_test.rego**:

```ruby
package authz

policies = [{"name": "test_policy"}]
roles = {"admin": ["alice"]}

test_allow_with_data {
    allow with input as {"user": "alice", "role": "admin"}  with data.policies as policies  with data.roles as roles
}
```

To exercise the policy, run the `opa test` command.

```bash
$ opa test -v authz.rego authz_test.rego
data.authz.test_allow_with_data: PASS (697ns)
--------------------------------------------------------------------------------
PASS: 1/1
```

Below is an example to replace a rule without arguments.

**authz.rego**:

```ruby
package authz

allow1 {
    allow2
}

allow2 {
    2 = 1
}
```

**authz_test.rego**:

```ruby
package authz

test_replace_rule {
    allow1 with allow2 as true
}
```

```bash
$  opa test -v authz.rego authz_test.rego
data.authz.test_replace_rule: PASS (328ns)
--------------------------------------------------------------------------------
PASS: 1/1
```

Functions with arguments cannot be replaced by the `with` keyword. For example, in the below policy the function `cannot_replace` cannot be replaced.

**authz.rego**:

```ruby
package authz

invalid_replace {
    cannot_replace(input.label)
}

cannot_replace(label) {
    label = "test_label"
}
```

**authz_test.rego**:

```ruby
package authz

test_invalid_replace {
    invalid_replace with input as {"label": "test_label"} with cannot_replace as true
}
```

```bash
$ opa test -v authz.rego authz_test.rego
1 error occurred: authz_test.rego:4: rego_compile_error: with keyword cannot replace rules with arguments
```


## Coverage

In addition to reporting pass, fail, and error results for tests, `opa test`
can also report _coverage_ for the policies under test.

The coverage report includes all of the lines evaluated and not evaluated in
the Rego files provided on the command line. When a line is not covered it
indicates one of two things:

* If the line refers to the head of a rule, the body of the rule was never true.
* If the line refers to an expression in a rule, the expression was never evaluated.

If we run the coverage report on the original **example.rego** file without
`test_get_user_allowed` from **example_test**.rego the report will indicate
that line 8 is not covered.

```bash
opa test --coverage --format=json example.rego example_test.rego
```

```json
{
  "files": {
    "example.rego": {
      "covered": [
        {
          "start": {
            "row": 3
          },
          "end": {
            "row": 5
          }
        },
        {
          "start": {
            "row": 9
          },
          "end": {
            "row": 11
          }
        }
      ],
      "not_covered": [
        {
          "start": {
            "row": 8
          },
          "end": {
            "row": 8
          }
        }
      ]
    },
    "example_test.rego": {
      "covered": [
        {
          "start": {
            "row": 3
          },
          "end": {
            "row": 4
          }
        },
        {
          "start": {
            "row": 7
          },
          "end": {
            "row": 8
          }
        },
        {
          "start": {
            "row": 11
          },
          "end": {
            "row": 12
          }
        }
      ]
    }
  }
}
```

## Profiling

In addition to testing and coverage reporting, you can also _profile_ your
policies using `opa eval`. The profiler is useful if you need to understand
why policy evaluation is slow.

The `opa eval` command provides the following profiler options:

| Option | Detail | Default |
| --- | --- | --- |
| <span class="opa-keep-it-together">`--profile`</span> | Enables expression profiling and outputs profiler results. | off |
| <span class="opa-keep-it-together">`--profile-sort`</span> | Criteria to sort the expression profiling results. This options implies `--profile`. | total_time_ns => num_eval => num_redo => file => line |
| <span class="opa-keep-it-together">`--profile-limit`</span> | Desired number of profiling results sorted on the given criteria. This options implies `--profile`. | 10 |

### Sort criteria for the profile results

  * `total_time_ns` - Results are displayed is decreasing order of _expression evaluation time_
  * `num_eval`  - Results are displayed is decreasing order of _number of times an expression is evaluated_
  * `num_redo`  - Results are displayed is decreasing order of _number of times an expression is re-evaluated(redo)_
  * `file`  - Results are sorted in reverse alphabetical order based on the _rego source filename_
  * `line`  - Results are displayed is decreasing order of _expression line number_ in the source file

When the sort criteria is not provided `total_time_ns` has the **highest** priority
while `line` has the **lowest**.

### Example Policy

The different profiling examples shown later on this page use the below
sample policy.

{%ace lang='python'%}
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

### Example: Display `ALL` profile results with `default` ordering criteria

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

#### Example: Display top `5` profile results

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

#### Example: Display top `5` profile results based on the `number of times an expression is evaluated`

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

#### Example: Display top `5` profile results based on the `number of times an expression is evaluated` and `number of times an expression is re-evaluated`

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
