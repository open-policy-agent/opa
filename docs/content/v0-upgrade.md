---
title: Upgrading to v1.0
kind: documentation
weight: 122
---

All users should plan to upgrade to OPA v1.0 eventually. Some users, with more
control over the Rego loaded into the OPA instances they run will be able to do
so more quickly. Other users with less control or running third party Rego may
wish to upgrade to OPA v1.0 and use the [v0 compatibility](./v0-compatibility) functionality to
upgrade gradually.

This documentation covers the different upgrade scenarios and the best course of
action for each. The documentation makes use of the following concepts:

- **Bundle Producer**: A system based on `opa build` that produces a bundle that
  is loaded by consumers.
- **Bundle Consumer**: An OPA instance that loads and evaluates policy
  from a bundle in the system.
- **Authoring**: The process of writing Rego policies before bundles are
  produced and consumed. In managed systems, this might be a user's only contact
  point with OPA.

In some systems, where OPA is used without a bundle, there is no producer. This
simplifies the upgrade process.

Users are encouraged to upgrade to OPA v1.0 as soon as possible. Using the v0
compatible functionality until updating Rego is preferred to delaying the
upgrade. The first part of this guide refers to upgrading OPA instances used for
producing and consuming bundles. This is the first step users should take unless
their Rego is already v1.0 compatible. See [Upgrading Rego](#upgrading-rego)
below for information on how to upgrade Rego policies to be v1.0 compatible.

## General Upgrade Approach: Upgrade Producers, then Consumers

Users will need to upgrade to OPA v1.0 in their own way, depending on their
release and change management processes, use cases and risk tolerance. The
general advice is to upgrade producers first, then consumers. This is because
the updated producers would be able to set the Rego version on bundle manifests
and as a result it wouldn't be necessary to run consumers with the `--v0-compatible` flag.
Also since it's likely there are much more consumers than producers, upgrading producers
first would lead to a smoother upgrade process.

Some users may wish to migrate to OPA v1.0 all at once, with adequate testing and validation
this is possible. Not all steps are necessary for all users so a hybrid approach
is also an option depending on your context.

The rest of this documentation is designed to meet users where they find
themselves and direct them down the smoothest path to upgrade to OPA
v1.0.

## Detailed Producer & Consumer Version Scenarios

Tabulated in this section are the different versions of OPA users might be
working with in different parts of their systems. Select the scenario that best
matches your setup to find the recommended upgrade path.

If you are in doubt, [Scenario 1](#scenario-1) is the most common starting
point and we recommended you start there.

|                   | v0.x Consumer                        | Mix Consumer              | v1.0 Consumer                        |
|-------------------|--------------------------------------|---------------------------|--------------------------------------|
| **v0.x Producer** | [Scenario 1](#scenario-1) (All v0.x) | [Scenario 4](#scenario-4) | [Scenario 7](#scenario-7)            |
| **Mix Producer**  | [Scenario 2](#scenario-2)            | [Scenario 5](#scenario-5) | [Scenario 8](#scenario-8)            |
| **v1.0 Producer** | [Scenario 3](#scenario-3)            | [Scenario 6](#scenario-6) | [Scenario 9](#scenario-9) (All v1.0) |

<!-- source https://docs.google.com/drawings/d/137EObOVhMIVk9NEWOX0u_1eQOqe0MRkPQlmwKXkYWEU/edit -->

{{< figure src="opa-v0-upgrade.png" width="65" caption="Upgrade flows to OPA v1.0" >}}

## Upgrade Scenarios

Listed below are the different upgrade scenarios and the recommended migration
plans for each case.

### Scenario 1: v0.x Producer, v0.x Consumer

All OPA runtimes - both bundle consumers and producers - are v0.x. This is the
most common starting point for users upgrading from a v0.x version of OPA.

#### How to Run

- Policies are authored to be v0.x compatible.

#### Next

Start upgrading producers to v1.0 ([Scenario 2](#scenario-2)) until all producers
are v1.0 ([Scenario 3](#scenario-3)).

### Scenario 2: Mix Producer, v0.x Consumer

Some bundle producers are v1.0, while some remain on v0.x. All bundle consumers
are v0.x. This might be the case if you have bundles from different tenants or
users using different OPA versions and you cannot control the versions they use.

#### Pre-requisites

Users cannot proceed with upgrade until they have either a single version of bundle
producers or have a means to control the use of `--v0-compatible` on newer
producers.

#### How to Run

- Policies are authored to be v0.x compatible.
- v0.x producers are run as-is.
- v1.0 producer is run with `--v0-compatible`, or modules have `rego.v1` import.
- v0.x consumers are run as-is.

#### Next

Continue migrating producers to v1.0 until all producers have been upgraded
([Scenario 3](#scenario-3)).

### Scenario 3: v1.0 Producer, v0.x Consumer

OPA bundles are produced by OPA v1.0 instances, consumers are still on v0.x. This
scenario is common as users upgrade to OPA v1.0 by upgrading their producers
first.

#### Pre-requisites

Control of producers to set `--v0-compatible` or use `rego.v1` imports is
required.

#### How to Run

- Policies are authored to be v0.x compatible.
- v0.x consumers are run as-is.
- v1.0 producer is run with `--v0-compatible`, or modules have `rego.v1` import.
- Since policies will always be consumed by a v0.x OPA, all policies _must_ be v0.x compliant.

#### Next

Now that all producers are v1.0, and consumers are still not all v1.0, it's time
to get all the consumers to v1.0, ([Scenario 6](#scenario-6)).

### Scenario 4: v0.x Producer, Mix Consumer

Producers are v0.x, consumers are a mix of v0.x and v1.0. This scenario might occur
when users have partially upgraded OPA instances to v1.0, but have not yet
upgraded their consumers. This is not a recommended step if it can be avoided as
it's recommended to upgrade producers first.

#### Pre-requisites

OPA v1.0 consumers must be able to run with `--v0-compatible` to accept v0.x
bundles. This upgrade path cannot continue until this is possible.

#### How to Run

- Policies are authored to be v0.x compatible.
- v0.x producers and v0.x consumers are run as is.

### Next

Upgrade producers to v1.0 and continue the upgrade from that point. Generally, it's recommended to
upgrade producers first, however depending on your existing OPAs v1.0 consumers deployments, you may prefer to
upgrade all your producers to v1.0 rather than to downgrade consumers.

### Scenario 5: Mix Producer, Mix Consumer

Mixed versions of OPA are being used for both bundle production and consumption.

#### Pre-requisites

As users have a mix of bundle producers, they must have control over the runtime
options for the producers to set `--v0-compatible`. Users must also have control
over their v1.0 consumers to set the `--v0-compatible` flag. Both these conditions
must be met for the upgrade to proceed.

#### How to Run

- Policies are authored to be v0.x compatible.
- v1.0 consumers are run with `--v0-compatible`.
- v1.0 producers are run with `--v0-compatible`.

### Next

Please gradually upgrade producers to v1.0 until all producers are v1.0 ([Scenario 6](#scenario-6)).

### Scenario 6: v1.0 Producer, Mix Consumer

All consumers can be run without flags, as the bundle will contain
attributes to inform v1.0 OPAs to accept v0.x modules.

#### How to Run

- Policies are authored to be v0.x compatible.
- v0.x consumers are run as-is. Bundles will contain v0.x policies
- v1.0 consumers are run as-is. Bundles will contain `rego_version` attribute, so v0.x modules are accepted.

#### Pre-requisites

If users cannot set their OPA v1.0 producers to use `--v0-compatible` to be
compatible with their v0.x consumers, then this upgrade path is blocked.

#### Next

Running exclusively v1.0 producers and consumers, ([Scenario 9](#scenario-9)), is
the next and final step.

### Scenario 7: v0.x Producer, v1.0 Consumer

All consumers are v1.0, but producers are v0.x. This scenario might occur when
OPAs used for evaluation are upgraded before the policy bundling system.

#### Pre-requisites

If v1.0 consumers cannot be run with `--v0-compatible`, when loading v0.x consumer
generated bundle, the bundles cannot include `rego_version` attribute. This means
the upgrade path is blocked until either the consumers can create bundles with a
Rego version or the `--v0-compatible` flag is available for producers.

#### How to Run

- Policies are authored to be v0.x compatible.
- v1.0 consumers are run with `--v0-compatible`

#### Next

Upgrade producers to v1.0 ([Scenario 8](#scenario-8)) until all producers are v1.0
([Scenario 9](#scenario-9)).

### Scenario 8: Mix Producer, v1.0 Consumer

All consumers are v1.0, but producers are a mix of v0.x and v1.0.

#### How to Run

- Policies are authored to be v0.x compatible.
- v0.x producers are run as is.
- v1.0 consumers are run with `--v0-compatible`
- v1.0 producers are run with `--v0-compatible`

#### Pre-requisites

If using v0.x bundles, it must be possible to use `--v0-compatible` on the bundle
producers in order for them to work in the v1.0 consumers.

v1.0 consumers will accept v1.0 producer bundles, as these will have the Rego version specified in the manifest;
they won't however accept bundles from v0.x producers unless they have `--v0-compatible` set.

#### Next

Upgrade producers to v1.0 ([Scenario 9](#scenario-9)), completing the upgrade.

### Scenario 9: v1.0 Producer, v1.0 Consumer

Once you have all consumers and producers running at v1.0 then you have
completed the upgrade to OPA v1.0. If you are using `--v0-compatible`
functionality, the next task is to upgrade the Rego loaded into OPAs to Rego v1.

Regardless of whether you are now upgrading your Rego, we encourage users to
use `opa check`, `opa check --strict` and to lint their Rego projects if you
have not already done so to identify issues.

## Changes to Rego in OPA v1.0

Once you have upgraded OPA instance to v1.0, or if you are upgrading all at
once, you will need to upgrade your Rego policies to Rego v1.0. This section
outlines the changes in Rego v1.0.

### The `future.keywords` imports

The `in`, `every`, `if` and `contains` keywords have been introduced over time,
and Rego v0.x required an opt-in to prevent them from breaking policies that
existed before their introduction. The `future.keywords` imports facilitate this
opt-in mechanism. These keywords help to increase the readability of policies
and provide syntactic sugar for commonly used operations such as iteration,
membership checks, defining multi-value rules, and so on. There is growing
adoption of these keywords and their usage is prevalent in the OPA
documentation, Rego Playground, etc.

In OPA v1.0 the `in`, `every`, `if` and `contains` keywords are part of the
language by default and the `future.keywords` imports will become a no-op. A
policy that makes use of these keywords, but doesn't import `future.keywords` is
valid in OPA v1.0 but not in older versions of OPA.

### Enforce use of `if` and `contains` keywords in rule head declarations

In Rego v0.x, there is semantic ambiguity between rules like `a.b {true}` and
`a.b.c {true}`. Although syntactically similar, the former generates a set with
the entry `b` at path `data.a`, while the latter generates an object with the
attribute `"c": true` at path `data.a.b`. This inconsistency makes it difficult
for new users to understand how Rego works. The `if` keyword is more than just
syntactic sugar. When used in a rule head, that rule doesn't contribute to a
partial set unless the `contains` keyword is also used. E.g. `a.b if {true}`
will generate an object with the attribute `"b": true` at path `data.a`. To make
things simpler, OPA v1.0 requires the usage of `if` and `contains` keywords when
declaring rules. This would mean:

- All rules are single-value by default. When the value is omitted from the
  head, it defaults to `true`.
- To make rules multi-value (i.e. partial set rules), use the `contains` keyword
  to convert the value into a set.

The `contains` keyword is required to disambiguate rules that generate a single
value from rules that generate multiple values. The `if` keyword ensures that
the semantics of rules do not change between v0.x and v1.0 Rego. The table below
illustrates why `if` is required.

| rule              | output in v0.x             | output in v1.0           |
| ----------------- | ---------------------- | ------------------------ |
| p { true }        | {"p": true}            | compile error            |
| p.a { true }      | {"p": {"a"}}           | compile error            |
| p.a.b { true }    | {"p": {"a": {"b": true}} | compile error            |
| p if { true }     | {"p": true}            | {"p": true}              |
| p.a if { true }   | {"p":{"a": true}}      | {"p":{"a": true}}        |
| p.a.b if { true } | {"p": {"a": {"b": true}} | {"p": {"a": {"b": true}} |
| p contains “a”    | {"p": {"a"}}           | {"p": {"a"}}             |

If the Rego language was changed so that all rules were single-value by default,
unless the `contains` keyword was used to make them multi-value, then the
outcome of a rule like `p.a { true }` would change between v0.x and v1.0
without generating an error. Generating errors in this case is preferable to
changing the semantics of existing rules. Therefore, use of the `if` keyword is
a requirement in OPA v1.0.

In OPA v1.0, the `if` keyword is only required for rules with a declared body.
Constants, rules that only consist of a value assignment, do not require `if`.
The following forms therefore remain valid in OPA v1.0:

| rule       | output in v0.x           | output in v1.0         |
| ---------- | -------------------- | ---------------------- |
| p := 1     | {“p”: 1}             | {“p”: 1}               |
| p.a := 1   | {“p”: {“a”: 1}}      | {“p”: {“a”: 1}}        |
| p.a.b := 1 | {“p”: {“a”: {“b”: 1}}} | {“p”: {“a”: {“b”: 1}}} |

Since the `if` keyword can only be used in front of a rule body, rules with no
body and no value assignment, i.e. a solitary reference, are not allowed in
the v1.0 Rego syntax:

| rule  | output in v0.x              | output in v1.0 |
| ----- | ----------------------- | -------------- |
| p     | compile error           | compile error  |
| p.a   | {“p”: {“a”}}            | compile error  |
| p.a.b | {“p”: {“a”: {“b”: true}}} | compile error  |

The below table gives examples of v0.x valid Rego syntax which are
invalid in OPA v1.0, along with the equivalent valid syntax in OPA v1.0:

| invalid in v1.0 | v1.0 equivalent            | Note                    |
| --------------- | -------------------------- | ----------------------- |
| p { true }      | p if { true }              | Single-value rule       |
| p.a             | p contains "a"             | Multi-value insertion   |
| p.a { true }    | p contains "a" if { true } | Multi-value rule        |
| p.a.b           | p.a.b := true              | Single-value assignment |
| p.a.b { true }  | p.a.b if { true }          | Single-value rule       |

Following is an example of how to define a rule that generates a set:

```rego
package play

a contains b if { b := 1 }
```

When the above rule is evaluated the output is (sets are serialized into arrays
in JSON):

```json
{
  "a": [1]
}
```

Following is an example of how to define a rule that generates an object:

```rego
package play

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

The requirement of `if` and `contains` keywords remove the ambiguity
between single-value and multi-value rule declaration. This makes Rego code
easier to author and read; thereby making it simpler for users to author their
policies.

### Prohibit duplicate imports

As part of `strict` mode in OPA 0.x, the Rego compiler prohibits duplicate imports where one import shadows another.
OPA v1.0 enforces this check by default.

An import shadowing another is most likely an authoring error and probably
unintentional. OPA checking this by default will help to avoid policy
evaluations resulting in error-prone decisions.

### `input` and `data` keywords are reserved

The Rego compiler ensures that `input` and `data` are reserved keywords and may
not be used as names for rules and variable assignments. This is part of`strict` mode in OPA 0.x

The `input` document holds the user-provided input, while the data pushed into
OPA and rule evaluation results are nested under the `data` document. Hence, if
a rule or variable shadows `input` or `data` you have the unintended consequence
of erasing information under these inside the local scope, resulting in incorrect policy decisions. In
OPA v1.0 such scenarios are avoided by default.

Note, using the [with](../policy-language/#with-keyword) keyword to insert
values into - or to fully replace - the `input` or `data` documents, as in
`my_func(x) with input as {...}` does not constitute shadowing and is therefore
allowed in OPA v1.0.

### Prohibit use of deprecated builtins

As part of `strict` mode in OPA 0.x, the Rego compiler prohibits use of deprecated built-in functions. In OPA v1.0,
these built-ins have been removed.

The following built-in functions are deprecated: `any`, `all`, `re_match`,
`net.cidr_overlap`, `set_diff`, `cast_array`, `cast_set`, `cast_string`,
`cast_boolean`, `cast_null`, `cast_object`. In some cases, new built-in
functions have been added that provide functionality at least similar to a
deprecated built-in.

### Rego-versioned bundles

A bundle built with OPA `v0.64.0` or later, contain a `rego_version` attribute
in their [manifest](../management-bundles/#bundle-file-format), which the OPA
consuming that bundle will use when processing the contained modules. A bundle's
internal rego-version takes precedence over the presence of the
`--v1-compatible` flag; therefore, prerequisite knowledge about what Rego syntax
any consumed bundle contains is not needed. The `--v1-compatible` flag (and
`--v0-compatible` in v1.0) on the `opa build` command allows the user to control
the `rego-version` of the built bundle.

See [Upgrading to v1.0](./v0-upgrade) for more information on how to use
versioned bundles as part of an upgrade to OPA v1.0.

## Compilation Constraints and Checks

Below constraints and safety checks are enforced by default in v1.0 during compilation. These checks along with the ones in [v1.0 strict mode](../policy-language/#strict-mode)
were part of the compiler `strict` mode in OPA 0.x.

Name | Description                                                                                                                                                                            
--- |----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
Duplicate imports | Duplicate [imports](../policy-language/#imports), where one import shadows another, are prohibited.                                                                                                                                                            
`input` and `data` reserved keywords | `input` and `data` are reserved keywords, and may not be used as names for rules and variable assignment.                                                                                                                                                     
Use of deprecated built-ins | Use of deprecated functions is prohibited, and these will be removed in OPA 1.0. Deprecated built-in functions: `any`, `all`, `re_match`,  `net.cidr_overlap`, `set_diff`, `cast_array`, `cast_set`, `cast_string`, `cast_boolean`, `cast_null`, `cast_object`

## Upgrading Rego

Users with v0.x Rego projects are encouraged to follow the below process to
upgrade their Rego code to conform to best practices, and to be compatible with
OPA v1.0. These steps are largely based on the process outlined in this
[detailed blog post](https://www.styra.com/blog/renovating-rego/).

Before starting the upgrade, users are recommended to ensure they have a local
OPA binary of version 1.0 or later.

1. `opa check --v0-v1`, this will catch any parse or compilation errors.
2. `opa check --v0-v1 --strict`, this will raise a number of other issues found in code
   that might make it incompatible with OPA v1.0 such as the use of deprecated
   built-ins or duplicate imports.
3. Automatically reformat your code for OPA v1.0 with `opa fmt --write --v0-v1`.
4. `regal lint`, the [Regal linter](/integrations/regal/) has many more rules to
   test for issues in Rego code that can lead to errors, poor performance or
   unexpected behaviour.

If you run into any issues while upgrading a Rego project, please drop a message
in the #help channel on the [OPA Slack](https://slack.openpolicyagent.org/).

## Upgrading for Go Integrations

Both users of the
[v0 SDK](https://pkg.go.dev/github.com/open-policy-agent/opa/sdk)
and
[v0 Rego](https://pkg.go.dev/github.com/open-policy-agent/opa/rego) packages are
encoraged to upgrade to the new v1 packages instead. These can be found here:

- [SDK v1](https://pkg.go.dev/github.com/open-policy-agent/opa/v1/sdk)
- [Rego v1](https://pkg.go.dev/github.com/open-policy-agent/opa/v1/rego)

In order to upgrade to a v1 package, you need to make the following change:

Before:

```
import (
    "github.com/open-policy-agent/opa/rego"
)
```

After:

```
import (
    "github.com/open-policy-agent/opa/v1/rego"
)
```

This will be needed for all OPA packages your application depends on, not just
`rego` and `sdk`, other commonly used packages are: `ast`, `bundle`, `compile`,
`types` & `topdown`.

As of OPA 1.0, all v0 packages have been deprecated. While they will remain for
the lifetime of OPA 1.0, you are encouraged to upgrade as soon as possible.

If you need to use v0 functionality, you can still use v1 packages. Please see
the [Backwards Compatibility](./v0-compatibility/) documentation for more
details.
