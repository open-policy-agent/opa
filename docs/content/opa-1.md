---
title: OPA 1.0
kind: documentation
weight: 120
---

OPA v1.0 will introduce breaking changes to the Rego language and OPA runtime. 
Here, these changes are described, along with the presentation of tools to make new and existing policies compatible with OPA 1.0.
The changes to the Rego language are backwards compatible in the sense that it is possible to author policies that are compatible with both OPA v1.0 and older versions of OPA.

{{< info >}}
The full set of breaking changes in OPA v1.0 is not yet finalized.
The tools mentioned in the below sections - such as the `rego.v1` import and the `--rego-v1` command flag - may change over time, so it's good practice to regularly employ them to ensure your policies are compatible with the future release of OPA v1.0.
{{< /info >}}

## What's changing in OPA 1.0

### The `future.keywords` imports

The `in`, `every`, `if` and `contains` keywords have been introduced over time, and currently require opt-in to prevent them from breaking policies that existed before their introduction. 
The `future.keywords` imports facilitate this opt-in mechanism.
These keywords help to increase the readability of policies and provide syntactic sugar for commonly used operations such as iteration, membership checks, defining multi-value rules, and so on.
There is growing adoption of these keywords and their usage is prevalent in the OPA documentation, Rego Playground, etc.

In OPA v1.0 the `in`, `every`, `if` and `contains` keywords will be part of the language by default and the `future.keywords` imports will become a no-op.
A policy that makes use of these keywords, but doesn't import `future.keywords` is valid in OPA v1.0 but not in older versions of OPA.

#### Making new and existing policies compatible with OPA v1.0

1. A new [rego.v1](../policy-language/#the-regov1-import) import has been introduced that, when used, makes OPA apply all restrictions that will eventually be enforced by default in OPA v1.0. 
   If a Rego module imports `rego.v1`, it means applicable `future.keywords` imports are implied. It is illegal to import both `rego.v1` and `future.keywords` in the same module.
2. The `--rego-v1` flag on the `opa fmt` command will rewrite existing modules to use the `rego.v1` import instead of `future.keywords` imports.
3. The `--rego-v1` flag on the `opa check` command will check that either the `rego.v1` import or applicable `future.keywords` imports are present if any of the `in`, `every`, `if` and `contains` keywords are used in a module.

#### Backwards compatibility in OPA v1.0

To avoid breaking existing policies and give users a chance to update them such that these future keywords don’t clash with, for example, existing variable names, a new flag called `--v0-compatible` will be added to OPA commands such as `opa run` to maintain backward compatibility. 
OPA’s Go SDK and Go API will be equipped with similar functionality, as well. 
In addition to this, the bundle manifest will also be updated to include a bit that indicates whether pre-OPA v1.0 behavior is desired.
This should be useful for cases when pre-OPA v1.0 functionality is desired but the users themselves have no control over the OPA deployment and hence cannot set the CLI flag.
OPA tooling such as the `opa build` command will be equipped with a new flag that will allow users to control the relevant bit in the manifest. 

### Enforce use of `if` and `contains` keywords in rule head declarations

Currently, there is semantic ambiguity between rules like `a.b {true}` and `a.b.c {true}`. Although syntactically similar, the former generates a set with the entry `b` at path `data.a`, while the latter generates an object with the attribute `"c": true` at path `data.a.b`. 
This inconsistency makes it difficult for new users to understand how Rego works. 
The `if` keyword is more than just syntactic sugar. When used in a rule head, that rule doesn't contribute to a partial set unless the `contains` keyword is also used. E.g. `a.b if {true}` will generate an object with the attribute `"b": true` at path `data.a`.
To make things simpler in the future, OPA v1.0 will require the usage of `if` and `contains` keywords when declaring rules. This would mean:

* All rules are single-value by default. When the value is omitted from the head, it defaults to `true`.
* To make rules multi-value (i.e. partial set rules), use the `contains` keyword to convert the value into a set.

The `contains` keyword is required to disambiguate rules that generate a single value from rules that generate multiple values. 
The `if` keyword ensures that the semantics of rules does not change between today and v1.0. The table below illustrates why `if` is required.

| rule              | output today             | output in v1.0           |
|-------------------|--------------------------|--------------------------|
| p { true }        | {"p": true}              | compile error            |
| p.a { true }      | {"p": {"a"}}             | compile error            |
| p.a.b { true }    | {"p": {"a": {"b": true}} | compile error            |
| p if { true }     | {"p": true}              | {"p": true}              |
| p.a if { true }   | {"p":{"a": true}}        | {"p":{"a": true}}        |
| p.a.b if { true } | {"p": {"a": {"b": true}} | {"p": {"a": {"b": true}} |
| p contains “a”    | {"p": {"a"}}             | {"p": {"a"}}             |

If the Rego language was changed so that all rules were single-value by default, unless the `contains` keyword was used to make them multi-value, then the outcome of a rule like `p.a { true }` would change between today and v1.0 without generating an error.
Generating errors in this case is preferable to changing the semantics of existing rules. Whereby, use of the `if` keyword will be a requirement in OPA v1.0, as this is also backwards compatible with older versions of OPA.

In OPA v1.0, the `if` keyword is only required for rules with a declared body. Constants, rules that only consist of a value assignment, do not require `if`. 
The following forms therefore remain valid in OPA v1.0:

| rule       | output today           | output in v1.0         |
|------------|------------------------|------------------------|
| p := 1     | {“p”: 1}               | {“p”: 1}               |
| p.a := 1   | {“p”: {“a”: 1}}        | {“p”: {“a”: 1}}        |
| p.a.b := 1 | {“p”: {“a”: {“b”: 1}}} | {“p”: {“a”: {“b”: 1}}} |

Because the `if` keyword can only be used in front of a rule body, rules with no body and no value assignment, i.e. a solitary reference, will not be allowed in the v1.0 Rego syntax:

| rule  | output today              | output in v1.0 |
|-------|---------------------------|----------------|
| p     | compile error             | compile error  |
| p.a   | {“p”: {“a”}}              | compile error  |
| p.a.b | {“p”: {“a”: {“b”: true}}} | compile error  |

The below table gives examples of currently valid Rego syntax that will be invalid in OPA v1.0, along with the equivalent valid syntax in OPA v1.0: 

| invalid in v1.0 | v1.0 equivalent            | Note                    |
|-----------------|----------------------------|-------------------------|
| p { true }      | p if { true }              | Single-value rule       |
| p.a             | p contains "a"             | Multi-value insertion   |
| p.a { true }    | p contains "a" if { true } | Multi-value rule        |
| p.a.b           | p.a.b := true              | Single-value assignment |
| p.a.b { true }  | p.a.b if { true }          | Single-value rule       |

Following is an example of how to define a rule that generates a set:

```rego
package play

import rego.v1 # Implies future.keywords.if and future.keywords.contains

a contains b if { b := 1 }
```

When the above rule is evaluated the output is (sets are serialized into arrays in JSON):

```json
{
  "a": [1]
}
```

Following is an example of how to define a rule that generates an object:

```rego
package play

import rego.v1 # Implies future.keywords.if and future.keywords.contains

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

The requirement of `if` and `contains` keywords will remove the ambiguity between single-value and multi-value rule declaration.
This will make Rego code easier to author and read; thereby making it simpler for users to author their policies.

#### Making new and existing policies compatible with OPA v1.0

1. A new [rego.v1](../policy-language/#the-regov1-import) import has been introduced that, when used, makes OPA apply all restrictions that will eventually be enforced by default in OPA v1.0.
   If a Rego module imports `rego.v1`, it means the `if` and `contains` keywords are required when declaring rules. Constants, rules that only consist of a value assignment, are exempted.
2. The `--rego-v1` flag on the `opa fmt` command will rewrite existing modules to use the `if` and `contains` keywords where applicable.
3. The `--rego-v1` flag on the `opa check` command will check that the `if` and `contains` keywords are used where applicable in a module.

#### Backwards compatibility in OPA v1.0

By default, OPA v1.0 will require the usage of `if` and `contains` as described above, but to maintain backward compatibility for policies written pre-OPA v1.0, a new flag called `--v0-compatible` will be added to OPA commands such as `opa run`, to avoid breaking existing policies and give users time to update them. 
Similar functionality will be added to OPA’s Go SDK and Go API to assist users that embed OPA as a library in their Go services and also to OPA’s build command.

### Prohibit duplicate imports

The Rego compiler supports [strict mode](../policy-language/#strict-mode) which enforces additional constraints and safety checks during compilation. 
One of these checks is to prohibit duplicate imports where one import shadows another. OPA v1.0 will enforce this check by default.

An import shadowing another is most likely an authoring error and probably unintentional. OPA checking this by default will help to avoid policy evaluations resulting in error-prone decisions.

#### Making new and existing policies compatible with OPA v1.0

1. A new [rego.v1](../policy-language/#the-regov1-import) import has been introduced that, when used, makes OPA apply all restrictions that will eventually be enforced by default in OPA v1.0.
   If a Rego module imports `rego.v1`, duplicate imports are prohibited.
2. The `--rego-v1` flag on the `opa fmt` command will reject modules with duplicate imports.
3. The `--rego-v1` flag on the `opa check` command will check that duplicate imports are not present in a module.

#### Backwards compatibility in OPA v1.0

If pre-OPA v1.0 behavior is desired where this check is only enforced when `strict mode` is enabled, a new `--v0-compatible` flag will be added to the OPA CLI to achieve that.
Similar functionality will be added to OPA’s Go SDK, Go API and build command.

### `input` and `data` keywords are reserved

The Rego compiler supports [strict mode](../policy-language/#strict-mode), which enforces additional constraints and safety checks during compilation. 
One of these checks is to ensure that `input` and `data` are reserved keywords and may not be used as names for rules and variable assignments.

The `input` document holds the user-provided input, while the data pushed into OPA and rule evaluation results are nested under the `data` document.
Hence, if a rule or variable shadows `input` or `data` you have the unintended consequence of erasing information under these, resulting in incorrect policy decisions. In OPA v1.0 such a scenario will be avoided by default.

Note, using the [with](../policy-language/#with-keyword) keyword to insert values into - or to fully replace - the `input` or `data` documents, as in `my_func(x) with input as {...}` does not constitute shadowing and is therefore allowed in OPA v1.0.

#### Making new and existing policies compatible with OPA v1.0

1. A new [rego.v1](../policy-language/#the-regov1-import) import has been introduced that, when used, makes OPA apply all restrictions that will eventually be enforced by default in OPA v1.0.
   If a Rego module imports `rego.v1`, it means `input` and `data` are reserved keywords and may not be used as names for rules and variable assignments.
2. The `--rego-v1` flag on the `opa fmt` command will reject modules where `input` and `data` are used as names for rules and local variable assignments.
   In a future release, a `--refactor-local-variables` flag will be added to `opa fmt` to refactor local variable assignments.
3. The `--rego-v1` flag on the `opa check` command will check that `input` and `data` are not used as names for rules and local variable assignments in a module.

#### Backwards compatibility in OPA v1.0

OPA v1.0 will enforce this check by default. If pre-OPA v1.0 behavior is desired where this check is only enforced when `strict mode` is enabled, a new flag `--v0-compatible` will be added to the OPA CLI to achieve that.
Similar functionality will be added to OPA’s Go SDK, Go API and build command.

### Prohibit use of deprecated builtins

The Rego compiler supports [strict mode](../policy-language/#strict-mode), which enforces additional constraints and safety checks during compilation. 
One of these checks is to prohibit use of deprecated built-in functions. In OPA v1.0, these built-ins will be removed.

The following built-in functions are deprecated: `any`, `all`, `re_match`, `net.cidr_overlap`, `set_diff`, `cast_array`, `cast_set`, `cast_string`, `cast_boolean`, `cast_null`, `cast_object`.
In some cases, new built-in functions have been added that provide functionality at least similar to a deprecated built-in.

#### Making new and existing policies compatible with OPA v1.0

1. A new [rego.v1](../policy-language/#the-regov1-import) import has been introduced that, when used, makes OPA apply all restrictions that will eventually be enforced by default in OPA v1.0.
   If a Rego module imports `rego.v1`, it means deprecated built-in functions are prohibited.
2. The `--rego-v1` flag on the `opa fmt` command will reject modules with calls to deprecated built-in functions.
   In a future release, `opa fmt --rego-v1` will also rewrite modules to use an alternative, existing built-in function when possible and/or provide suggestions on how to author policies that avoid usage of deprecated built-ins.
   An FAQ section will be published that explains why each built-in function is deprecated and why it should be removed or replaced in the policy.
3. The `--rego-v1` flag on the `opa check` command will check that deprecated built-in functions are not used in a module.

#### Backwards compatibility in OPA v1.0

If pre-OPA v1.0 behavior is desired, where this check is only enforced when `strict` mode is enabled, a new flag `--v0-compatible` will be added to the OPA CLI to achieve that. 
Similar functionality will be added to OPA’s Go SDK, Go API and build command.

### Binding the OPA server to the `localhost` interface by default

By default, OPA binds to the `0.0.0.0` interface, which allows the OPA server to be exposed to services running outside of the local machine. 
Though not inherently insecure in a trusted environment, it's good practice to bind OPA to the `localhost` interface by default if OPA is not intended to be exposed to remote services.

In OPA v1.0, the OPA server will bind to the `localhost` interface by default.

The `--v1-compatible` flag on the `opa run` command makes OPA employ configuration defaults that will eventually be used in OPA v1.0. 
Binding to the `localhost` interface is one such default.

## Running OPA in 1.0 compatibility mode

OPA can be run in 1.0 compatibility mode by using the `--v1-compatible` flag. When this mode is enabled, the current release of OPA will behave as OPA v1.0 will eventually behave by default when it comes to the changes described [here](#whats-changing-in-opa-10).

The `--v1-compatible` flag is currently supported on the following commands:

* `bench`: requires modules to be compatible with OPA v1.0 syntax.
* `build`: requires modules to be compatible with OPA v1.0 syntax.
* `deps`: requires modules to be compatible with OPA v1.0 syntax.
* `check`*: requires modules to be compatible with OPA v1.0 syntax.
* `eval`: requires modules to be compatible with OPA v1.0 syntax.
* `exec`: requires modules to be compatible with OPA v1.0 syntax.
* `fmt`*: formats modules to be compatible with OPA v1.0 syntax, but not the current 0.x syntax.
* `inspect`: requires modules to be compatible with OPA v1.0 syntax.
* `parse`: requires modules to be compatible with OPA v1.0 syntax.
* `run`: requires modules (including discovery bundle) to be compatible with OPA v1.0 syntax. Binds server listeners to the `localhost` interface by default.
* `test`: requires modules to be compatible with OPA v1.0 syntax.

Note (*): the `check` and `fmt` commands also support the `--rego-v1` flag, which will check/format Rego modules as if compatible with the Rego syntax of _both_ the current 0.x OPA version and OPA v1.0.
If both flags are used at the same time, `--rego-v1` takes precedence over `--v1-compatible`.

{{< info >}}
Support for more commands will be added over time, leading up to the release of OPA 1.0.
{{< /info >}}
