---
sidebar_label: conflicting rules {name} found
image: /img/opa-errors.png
---

# `rego_type_error`: conflicting rules `{name}` found

This error happens when a rule is incrementally defined in a way that contradicts itself. This could for example happen
when a rule is declared to be both single-value _and_ multi-value, or when a function is declared twice with a
different number of arguments (i.e. different arity).

| Stage         | Category          | Message                                                                              |
| ------------- | ----------------- | ------------------------------------------------------------------------------------ |
| `compilation` | `rego_type_error` | `conflicting rules {name} found` (where name is the reference to a rule or function) |

## Examples

In the following example, the `deny` rule is first declared to be a boolean single-value rule, and then later declared
to be a multi-value rule building a set of strings. Since a rule can't reasonably both evaluate to a boolean and a set,
this will cause a conflict:

```rego
package policy

import future.keywords.contains
import future.keywords.if
import future.keywords.in

deny if {
    input.request.method == "PUT"
    not "admin" in input.user.roles
}

deny contains reason if {
    input.request.method == "PUT"
    not "admin" in input.user.roles

    reason := "only admins can modify resources"
}
```

```txt
1 error occurred: policy.rego:7: rego_type_error: conflicting rules data.policy.deny found
```

Another example would be a function that is declared twice with different, conflicting, arity:

```rego
package policy

import future.keywords.if
import future.keywords.in

find(arr, item) := item if item in arr

find(arr, item, not_found) := not_found if not item in arr
```

While overloading functions on arity might work in some languages, it is not supported in Rego.

```txt
1 error occurred: policy.rego:6: rego_type_error: conflicting rules data.policy.find found
```

## How To Fix It

Use a composite type, like objects, to return multiple values — like a boolean and a set of strings — from a rule.
In the example below, we've added a `decision` rule, which compiles its value from both the boolean `deny` rule and
the `reasons` set-generating rule:

```rego
package policy

import future.keywords.contains
import future.keywords.if
import future.keywords.in

decision["deny"] := deny
decision["reasons"] := reasons if deny

default deny := false

deny if {
    input.request.method == "PUT"
    not "admin" in input.user.roles
}

reasons contains reason if {
    input.request.method == "PUT"
    not "admin" in input.user.roles

    reason := "only admins can modify resources"
}
```

For the case of functions, simply ensure that the same arity is used for all declarations of the function. Use the
wildcard operator (`_`) for arguments that are unused in any given declaration:

```rego
package policy

import future.keywords.if
import future.keywords.in

find(arr, item, _) := item if item in arr

find(arr, item, not_found) := not_found if not item in arr
```
