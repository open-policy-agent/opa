---
title: Policy Testing
kind: documentation
weight: 4
restrictedtoc: true
has_side_notes: true
---

OPA gives you a high-level declarative language
([Rego](../policy-language)) to author fine-grained policies that
codify important requirements in your system.

To help you verify the correctness of your policies, OPA also gives you a
framework that you can use to write _tests_ for your policies. By writing
tests for your policies you can speed up the development process of new rules
and reduce the amount of time it takes to modify rules as requirements evolve.

{{< info >}}
The examples in this section try to represent the best practices. As such, they
make use of keywords that are meant to become standard keywords at some point in
time, but have been introduced gradually.
[See the docs on _future keywords_](../policy-language/#future-keywords) for more information.
{{< /info >}}

## Getting Started

Let's use an example to get started. The file below implements a simple
policy that allows new users to be created and users to access their own
profile.

**example.rego**:

```live:example:module:read_only,openable
package authz

allow if {
	input.path == ["users"]
	input.method == "POST"
}

allow if {
	input.path == ["users", input.user_id]
	input.method == "GET"
}
```

To test this policy, we will create a separate Rego file that contains test cases.

**example_test.rego**:

```live:example/test:module:read_only
package authz_test

import data.authz

test_post_allowed if {
	authz.allow with input as {"path": ["users"], "method": "POST"}
}

test_get_anonymous_denied if {
	not authz.allow with input as {"path": ["users"], "method": "GET"}
}

test_get_user_allowed if {
	authz.allow with input as {"path": ["users", "bob"], "method": "GET", "user_id": "bob"}
}

test_get_another_user_denied if {
	not authz.allow with input as {"path": ["users", "bob"], "method": "GET", "user_id": "alice"}
}
```

Both of these files are saved in the same directory.

```console
$ ls
example.rego      example_test.rego
```

To exercise the policy, run the `opa test` command in the directory containing the files.

```console
$ opa test . -v
data.authz_test.test_post_allowed: PASS (1.417µs)
data.authz_test.test_get_anonymous_denied: PASS (426ns)
data.authz_test.test_get_user_allowed: PASS (367ns)
data.authz_test.test_get_another_user_denied: PASS (320ns)
--------------------------------------------------------------------------------
PASS: 4/4
```

The `opa test` output indicates that all of the tests passed.

Try exercising the tests a bit more by removing the first rule in **example.rego**.

```console
$ opa test . -v
FAILURES
--------------------------------------------------------------------------------
data.authz_test.test_post_allowed: FAIL (277.306µs)

  query:1                 Enter data.authz_test.test_post_allowed = _
  example_test.rego:3     | Enter data.authz_test.test_post_allowed
  example_test.rego:4     | | Fail data.authz_test.allow with input as {"method": "POST", "path": ["users"]}
  query:1                 | Fail data.authz_test.test_post_allowed = _

SUMMARY
--------------------------------------------------------------------------------
data.authz_test.test_post_allowed: FAIL (277.306µs)
data.authz_test.test_get_anonymous_denied: PASS (124.287µs)
data.authz_test.test_get_user_allowed: PASS (242.2µs)
data.authz_test.test_get_another_user_denied: PASS (131.964µs)
--------------------------------------------------------------------------------
PASS: 3/4
FAIL: 1/4
```

## Enriched Test Report With Variable Values

Sometimes, e.g. when testing rules with complex output, it can be useful to know more about the circumstances that caused a certain expression to fail a test.
The `--var-values` flag can be used to enrich the test report with the exact expression that caused a test rule to fail, including the values of any variables or references used in the expression.

Consider the following utility module:

```live:example_vars:module:read_only,openable
package authz

allowed_actions(user) := [action |
	user in data.actions[action]
]
```

with accompanying tests:

```live:example_vars/test:module:read_only
package authz_test

import data.authz

test_allowed_actions_all_can_read if {
	users := ["alice", "bob", "jane"]
	r := ["alice", "bob"]
	w := ["jane"]
	p := {"read": r, "write": w}

	every user in users {
		"read" in authz.allowed_actions(user) with data.actions as p
	}
}
```

Exercising the tests with the `--var-values` flag:

```console
opa test . --var-values
FAILURES
--------------------------------------------------------------------------------
data.authz_test.test_allowed_actions_all_can_read: FAIL (904µs)

  util_test.rego:13:
    "read" in authz.allowed_actions(user) with data.actions as p
              |                     |                          |
              |                     |                          {"read": ["alice", "bob"], "write": ["jane"]}
              |                     "jane"
              ["write"]

SUMMARY
--------------------------------------------------------------------------------
util_test.rego:
data.authz_test.test_allowed_actions_all_can_read: FAIL (904µs)
--------------------------------------------------------------------------------
FAIL: 1/1
```

The test failed because it expected users with __write__ permission to implicitly also have the __read__ permission, an expectation the function under test didn't meet.
By including the failing expression and its local variable assignments in the test report, we make troubleshooting easier for the developer, as it's immediately apparent what assertion and combination of parameters caused the test to fail.

## Test Format

Tests are expressed as standard Rego rules with a convention that the rule
name is prefixed with `test_`. It's a good practice for tests to be placed in a package suffixed with `_test`, but not a requirement.

```live:example_format:module:read_only
package mypackage_test

import data.mypackage

test_some_descriptive_name if {
	# test logic
}
```

## Test Discovery

The `opa test` subcommand runs all of the tests (i.e., rules prefixed with
`test_`) found in Rego files passed on the command line. If directories are
passed as command line arguments, `opa test` will load their file contents
recursively.

## Specifying Tests to Run

The `opa test` subcommand supports a `--run`/`-r` regex option to further
specify which of the discovered tests should be evaluated. The option supports
[re2 syntax](https://github.com/google/re2/wiki/Syntax)

## Test Results

If the test rule is undefined or generates a non-`true` value the test result
is reported as `FAIL`. If the test encounters a runtime error (e.g., a divide
by zero condition) the test result is marked as an `ERROR`. Tests prefixed with
`todo_` will be reported as `SKIPPED`. Otherwise, the test result is marked as
`PASS`.

**pass_fail_error_test.rego**:

```live:example_results:module:read_only
package example_test

import data.example

# This test will pass.
test_ok if true

# This test will fail.
test_failure if 1 == 2

# This test will error.
test_error if 1 / 0

# This test will be skipped.
todo_test_missing_implementation if {
	example.allow with data.roles as ["not", "implemented"]
}
```

By default, `opa test` reports the number of tests executed and displays all
of the tests that failed or errored.

```console
$ opa test pass_fail_error_test.rego
data.example_test.test_failure: FAIL (253ns)
data.example_test.test_error: ERROR (289ns)
  pass_fail_error_test.rego:15: eval_builtin_error: div: divide by zero
--------------------------------------------------------------------------------
PASS: 1/3
FAIL: 1/3
ERROR: 1/3
```

By default, OPA prints the test results in a human-readable format. If you
need to consume the test results programmatically, use the JSON output format.

```bash
opa test --format=json pass_fail_error_test.rego
```

```json
[
  {
    "location": {
      "file": "pass_fail_error_test.rego",
      "row": 4,
      "col": 1
    },
    "package": "data.example_test",
    "name": "test_ok",
    "duration": 618515
  },
  {
    "location": {
      "file": "pass_fail_error_test.rego",
      "row": 9,
      "col": 1
    },
    "package": "data.example_test",
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
    "package": "data.example_test",
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

## Parameterized Tests and Data-driven Testing

A test rule can define multiple test cases for evaluation. 
Test cases are declared by adding their name(s) to the rule as variables in its head's reference, and are evaluated through regular enumeration.

**example_test.rego**:

```live:example_test_cases:module:read_only
package example_test

test_concat[note] if {
	some note, tc in {
		"empty + empty": {
			"a": [],
			"b": [],
			"exp": [],
		},
		"empty + filled": {
			"a": [],
			"b": [1, 2],
			"exp": [1, 2],
		},
		"filled + filled": {
			"a": [1, 2],
			"b": [3, 4],
			"exp": [1, 2, 3], # Faulty expectation, this test case will fail
		},
	}

	act := array.concat(tc.a, tc.b)
	act == tc.exp
}
```

```console
$ opa test example_test.rego
example_test.rego:
data.example_test.test_concat: FAIL (263.375µs)
  empty + empty: PASS
  empty + filled: PASS
  filled + filled: FAIL
--------------------------------------------------------------------------------
FAIL: 1/1
```

Just as in regular evaluation, test-case data doesn't need to be declared as inline Rego, but can be loaded from json and yaml data files:

**file_example_test.rego**:

```live:example_file_test_cases:module:read_only
package example_test

import data.test_cases

test_concat[note] if {
	some note, tc in test_cases

	act := array.concat(tc.a, tc.b)
	act == tc.exp
}
```

**file_example_test.yaml**:

```yaml
test_cases:
   empty + empty:
      a: []
      b: []
      exp: []
   empty + filled:
      a: []
      b: [1, 2]
      exp: [1, 2]
   filled + filled:
      a: [1, 2]
      b: [3, 4]
      exp: [1, 2, 3] # Faulty expectation, this test case will fail
```

```console
$ opa test file_example_test.rego file_example_test.yaml
file_example_test.rego:
data.example_test.test_concat: FAIL (280µs)
  empty + empty: PASS
  empty + filled: PASS
  filled + filled: FAIL
--------------------------------------------------------------------------------
FAIL: 1/1
```

Test cases can be nested by declaring multiple test case name variables in the head reference.
This is useful when e.g. the same set of test cases can be used for asserting the same behaviour across slightly different circumstances:

**nested_example_test.rego**:

```live:example_nested_test_cases:module:read_only
package example_test

test_sign_token[note][alg] if {
	some note, tc in {
		"claims": {
			"claims": {"foo": "bar"},
		},
		"no claims": {
			"claims": {},
		},
	}

	some alg in [
		"HS256",
		"HS333", # unknown signing algorithm, this test case will fail
		"HS512",
	]

	secret := "foobar"
	key := base64.encode(secret)

	token := io.jwt.encode_sign({
		"typ": "JWT",
		"alg": alg
	}, tc.claims, {
		"kty": "oct",
		"k": key
	})

	[valid, _, payload] := io.jwt.decode_verify(token, {"secret": secret})
	valid
	payload = tc.claims
}
```

```console
$ opa test nested_example_test.rego
nested_example_test.rego:
data.example_test.test_sign_token: FAIL (1.214541ms)
  claims: FAIL
    HS256: PASS
    HS333: FAIL
    HS512: PASS
  no claims: FAIL
    HS256: PASS
    HS333: FAIL
    HS512: PASS
--------------------------------------------------------------------------------
FAIL: 1/1
```

## Data and Function Mocking

OPA's `with` keyword can be used to replace the data document or called functions with mocks.
Both base and virtual documents can be replaced.

When replacing functions, built-in or otherwise, the following constraints are in place:

1. Replacing `internal.*` functions, or `rego.metadata.*`, or `eq`; or relations (`walk`) is not allowed.
2. Replacement and replaced function need to have the same arity.
3. Replaced functions can call the functions they're replacing, and those calls
   will call out to the original function, and not cause recursion.

Below is a simple policy that depends on the data document.

**authz.rego**:

```live:with_keyword:module:read_only,openable
package authz

allow if {
	some x in data.policies
	x.name == "test_policy"
	matches_role(input.role)
}

matches_role(my_role) if input.user in data.roles[my_role]
```

Below is the Rego file to test the above policy.

**authz_test.rego**:

```live:with_keyword/tests:module:read_only
package authz_test

import data.authz

policies := [{"name": "test_policy"}]
roles := {"admin": ["alice"]}

test_allow_with_data if {
	authz.allow with input as {"user": "alice", "role": "admin"}
		with data.policies as policies
		with data.roles as roles
}
```

To exercise the policy, run the `opa test` command.

```console
$ opa test -v authz.rego authz_test.rego
data.authz_test.test_allow_with_data: PASS (697ns)
--------------------------------------------------------------------------------
PASS: 1/1
```

Below is an example to replace a **rule without arguments**.

**authz.rego**:

```live:with_keyword_rules:module:read_only
package authz

allow1 if allow2

allow2 if 2 == 1
```

**authz_test.rego**:

```live:with_keyword_rules/tests:module:read_only
package authz_test

import data.authz

test_replace_rule if {
	authz.allow1 with authz.allow2 as true
}
```

```console
$  opa test -v authz.rego authz_test.rego
data.authz_test.test_replace_rule: PASS (328ns)
--------------------------------------------------------------------------------
PASS: 1/1
```

Here is an example to replace a rule's **built-in function** with a user-defined function.

**authz.rego**:

```live:with_keyword_builtins:module:read_only
package authz

import data.jwks.cert

allow if {
	[true, _, _] = io.jwt.decode_verify(input.headers["x-token"], {"cert": cert, "iss": "corp.issuer.com"})
}
```

**authz_test.rego**:

```live:with_keyword_builtins/tests:module:read_only
package authz_test

import data.authz

mock_decode_verify("my-jwt", _) := [true, {}, {}]
mock_decode_verify(x, _)        := [false, {}, {}] if x != "my-jwt"

test_allow if {
	authz.allow with input.headers["x-token"] as "my-jwt"
		with data.jwks.cert as "mock-cert"
		with io.jwt.decode_verify as mock_decode_verify
}
```

```console
$  opa test -v authz.rego authz_test.rego
data.authz_test.test_allow: PASS (458.752µs)
--------------------------------------------------------------------------------
PASS: 1/1
```

In simple cases, a function can also be replaced with a value, as in

```live:with_keyword_builtins/tests/value:module:read_only
test_allow_value if {
	authz.allow 
		with input.headers["x-token"] as "my-jwt"
		with data.jwks.cert as "mock-cert"
		with io.jwt.decode_verify as [true, {}, {}]
}
```

Every invocation of the function will then return the replacement value, regardless
of the function's arguments.

Note that it's also possible to replace one built-in function by another; or a non-built-in
function by a built-in function.

**authz.rego**:

```live:with_keyword_funcs:module:read_only
package authz

replace_rule if {
	replace(input.label)
}

replace(label) if {
	label == "test_label"
}
```

**authz_test.rego**:

```live:with_keyword_funcs/tests:module:read_only
package authz_test

import data.authz

test_replace_rule if {
	authz.replace_rule with input.label as "does-not-matter" with replace as true
}
```

```console
$ opa test -v authz.rego authz_test.rego
data.authz_test.test_replace_rule: PASS (648.314µs)
--------------------------------------------------------------------------------
PASS: 1/1
```


## Coverage

In addition to reporting pass, fail, and error results for tests, `opa test`
can also report _coverage_ for the policies under test.

The coverage report includes all of the lines evaluated and not evaluated in
the Rego files provided on the command line. When a line is not covered it
indicates one of two things:

* If the line refers to the head of a rule, the body of the rule was never true.
* If the line refers to an expression in a rule, the expression was never evaluated.

It is also possible that [rule indexing](../policy-performance/#use-indexed-statements)
has determined some path unnecessary for evaluation, thereby affecting the lines
reported as covered.

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
      ],
      "covered_lines": 6,
      "not_covered_lines": 1,
      "coverage": 85.7
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
      ],
      "covered_lines": 6,
      "coverage": 100
    },
    "covered_lines": 12,
    "not_covered_lines": 1,
    "coverage": 92.3
  }
}
```

## Ecosystem Projects

{{< ecosystem_feature_embed key="policy-testing" topic="Policy Testing" >}}
