---
title: OPA 1.0
kind: misc
weight: 4
---

The following changes are proposed to be included in the OPA v1.0 release. Most of these changes are backwards incompatible.

# Make `future.keywords` import on by default

The `in`, `every`, `if` and `contains` keywords currently require opt-in to prevent them from breaking existing policies. 
A special import `future.keywords` is required to use all these future keywords. 
In OPA v1.0 all the future keywords will be part of the language by default and the `future.keywords` import will become a no-op. 
To avoid breaking existing policies and give users a chance to update them such that these future keywords don’t clash with, for example, existing variable names, a new flag called `--v0-compatible` will be added to OPA commands such as `opa run` to maintain backward compatibility. 
OPA’s Go SDK and Go API will be equipped with similar functionality as well. 
In addition to this, the bundle manifest will also be updated to include a bit that indicates whether pre-OPA v1.0 behavior is desired. 
OPA tooling such as the `opa build` command will be equipped with a new flag that will allow users to control the relevant bit in the manifest. 
This should be useful for cases when pre-OPA v1.0 functionality is desired but the users themselves have no control over the OPA deployment and hence cannot set the CLI flag.

## Why is this included

The `future.keywords` import was first introduced almost two years ago and since then multiple keywords have been added as part of this special import. 
These keywords help to increase the readability of policies and provide syntactic sugar for commonly used operations such as iteration, membership checks, defining multi-value rules and so on. 
There is growing adoption of these keywords and their usage is prevalent in the OPA documentation, Rego Playground etc. 
Now seems like a good opportunity to include them as part of the Rego language by default.

## How do we get there

1. Before the `future.keywords` import is on by default in OPA v1.0, a new `rego.v1` import will be introduced. 
   If a Rego module imports `rego.v1`, it would mean `future.keywords` import is implied and not allowed. 
   This means the `in`, `every`, `if` and `contains` keywords will be enabled by default.
2. A new mode will be added to the `opa fmt` command which will replace the `future.keywords` import with `rego.v1` in existing policies.

# Enforce use of `if` and `contains` keywords in rule head declarations

Currently, there is semantic ambiguity between rules like `a.b {true}` and `a.b.c {true}`. Although syntactically similar, the former generates a set at path `data.a`, while the latter generates an object at path `data.a.b`. 
This inconsistency makes it difficult for new users to understand how Rego works. 
To make things simpler in the future, OPA v1.0 will require the usage of `if` and `contains` keywords when declaring rules. This would mean:

* All rules are single-value by default. When the value is omitted from the head, it defaults to true.
* To make rules multi-value (i.e. partial set rules), use the `contains` keyword to convert the value into a set.

The `contains` keyword is required to disambiguate rules that generate a single value from rules that generate multiple values. 
The `if` keyword ensures that the semantics of rules does not change between today and v1.0. The table below illustrates why `if` is required.

| rule              | output today             | output in 1.0            |
|-------------------|--------------------------|--------------------------|
| p { true }        | {"p": true}              | compile error            |
| p.a { true }      | {"p": {"a"}}             | compile error            |
| p.a.b { true }    | {"p": {"a": {"b": true}} | compile error            |
| p if { true }     | {"p": true}              | {"p": true}              |
| p.a if { true }   | {"p":{"a": true}}        | {"p":{"a": true}}        |
| p.a.b if { true } | {"p": {"a": {"b": true}} | {"p": {"a": {"b": true}} |
| p contains “a”    | {"p": {"a"}}             | {"p": {"a"}}             |

If we only introduced `contains` (and used it to denote sets) and did not require `if`, then the meaning of a rule like `p.a { true }` would change between today and v1 without generating an error. 
We believe that generating errors in this case is preferable to changing the semantics.

The `if` keyword is only required for rules with a declared body. Constants, rules that only consist of a value assignment, do not require `if`.

| rule       | output today           | output in 1.0          |
|------------|------------------------|------------------------|
| p := 1     | {“p”: 1}               | {“p”: 1}               |
| p.a := 1   | {“p”: {“a”: 1}}        | {“p”: {“a”: 1}}        |
| p.a.b := 1 | {“p”: {“a”: {“b”: 1}}} | {“p”: {“a”: {“b”: 1}}} |

Because the `if` keyword can only be used in front of a rule body, rules with no body and no value assignment, i.e. a solitary reference, will not be allowed in the v1.0 Rego syntax:

| rule  | output today              | output in 1.0 |
|-------|---------------------------|---------------|
| p     | compile error             | compile error |
| p.a   | {“p”: {“a”}}              | compile error |
| p.a.b | {“p”: {“a”: {“b”: true}}} | compile error |

Following is an example of how to define a rule that generates a set:

```rego
package play

import rego.v1

a contains b if { b := 1}
```

When the above rule is evaluated the output is (sets are serialized into arrays in JSON):

```json
{
  "a": [1]
}
```

Following is an example of how to define a rule that generates a object:

```rego
package play

import rego.v1

a[b] if { b := 1}
```

When the above rule is evaluated the output is:

```json
{
  "a": {
    "1": true
  }
}
```

By default, OPA v1.0 will require the usage of `if` and `contains` as described above but to maintain backward compatibility for policies written pre-OPA v1.0, a new flag called `--v0-compatible` will be added to OPA commands such as opa run, to avoid breaking existing policies and give users time to update them. 
Similar functionality will be added to OPA’s Go SDK and Go API to assist users that embed OPA as a library in their Go services and also to OPA’s build command.

## Why is this included

The requirement of `if` and `contains` keywords will remove the ambiguity between single-value and multi-value rule declaration. 
This will make Rego code easier to author and read thereby making it simpler for users to author their policies. 
This feature will especially benefit new Rego authors and hopefully drive creation of more open-source policy libraries.

## How do we get there

1. The new `rego.v1` import will be introduced pre OPA v1.0 which will enforce the usage of the `if` and `contains` keywords where applicable.
2. A new mode will be added to the `opa fmt` command which would rewrite existing policies to be compliant with the usage of the `if` and `contains` keywords.

# Prohibit duplicate imports

The Rego compiler supports [strict mode](../policy-language/#strict-mode) which enforces additional constraints and safety checks during compilation. 
One of these checks is to prohibit duplicate imports where one import shadows another. OPA v1.0 will enforce this check by default. 
If pre-OPA v1.0 behavior is desired where this check is only enforced when `strict mode` is enabled, a new flag `--v0-compatible` will be added to the OPA CLI to achieve that. 
Similar functionality will be added to OPA’s Go SDK, Go API and build command.

## Why is this included 

An import shadowing another is most likely an authoring error and probably unintentional. OPA checking this by default will help to avoid policy evaluations resulting in error-prone decisions.

## How do we get there

1. The new `rego.v1` import described previously will enforce this requirement in modules when imported.
2. Also, `opa fmt` will be updated with new functionality that will generate an error when duplicate imports are detected.

# `input` and `data` keywords are reserved

The Rego compiler supports [strict mode](../policy-language/#strict-mode), which enforces additional constraints and safety checks during compilation. 
One of these checks is to ensure that `input` and `data` are reserved keywords and may not be used as names for rules and variable assignments.
OPA v1.0 will enforce this check by default. If pre-OPA v1.0 behavior is desired where this check is only enforced when `strict mode` is enabled, a new flag `--v0-compatible` will be added to the OPA CLI to achieve that. 
Similar functionality will be added to OPA’s Go SDK, Go API and build command.

## Why is this included 

The `input` document holds the user-provided input, while the data pushed into OPA and rule evaluation results are nested under the `data` document. 
Hence, if a rule or variable shadows `input` or `data` you have the unintended consequence of erasing information under these, resulting in incorrect policy decisions. In OPA v1.0 such a scenario will be avoided by default.

## How do we get there

1. The new `rego.v1` import will enforce this requirement in modules when imported.
2. Also, `opa fmt` will be updated with new functionality that will by default generate an error when `input` and `data` are used as names for rules and variable assignments. 
   A new flag `--refactor-local-variables` will be added to `opa fmt` to refactor local variable assignments.

# Prohibit use of deprecated builtins

The Rego compiler supports [strict mode](../policy-language/#strict-mode), which enforces additional constraints and safety checks during compilation. 
One of these checks is to prohibit use of deprecated built-in functions. OPA v1.0 will enforce this check by default. 
If pre-OPA v1.0 behavior is desired where this check is only enforced when `strict` mode is enabled, a new flag `--v0-compatible` will be added to the OPA CLI to achieve that. 
Similar functionality will be added to OPA’s Go SDK, Go API and build command.

## Why is this included 

The following built-in functions are deprecated: `any`, `all`, `re_match`, `net.cidr_overlap`, `set_diff`, `cast_array`, `cast_set`, `cast_string`, `cast_boolean`, `cast_null`, `cast_object`. 
In some cases, new built-in functions have been added that provide functionality at least similar to a deprecated built-in. In OPA v1.0, these built-in functions will be removed.

## How do we get there

1. The new `rego.v1` import will enforce this requirement in modules when imported.
2. Also, `opa fmt` will be updated with new functionality that will rewrite policies to use an existing built-in function when possible and/or provide suggestions on how to author policies that avoid usage of deprecated built-ins. 
   An FAQ section will be published that explains why each built-in function is deprecated and why it should be removed or replaced in the policy.

# Bind OPA server to `localhost` interface by default

By default, OPA binds to the `0.0.0.0` interface, which allows the OPA server to be exposed to services running outside of the same machine. 
Though not inherently insecure in a trusted environment, it's good practice to bind OPA to the `localhost` interface by default if OPA is not intended to be exposed to remote services.

## Why is this included 

This is a good security practice that should generally be followed for any non-public OPA deployment to prevent unintended OPA server access.

## How do we get there 

A new CLI flag called `--v1-compatible` will be added to the `opa run` command which will help users opt-in to future OPA features such as this one, that will eventually be enabled by default.
