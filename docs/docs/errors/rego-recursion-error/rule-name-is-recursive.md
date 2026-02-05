---
sidebar_label: rule {name} is recursive
image: /img/opa-errors.png
---

# `rego_recursion_error`: rule `{name}` is recursive

An important property of Rego compared to general purpose programming languages is that policy evaluation should be
known to _terminate_. Some common programming constructs, like `while` loops, or recursive references, can't
reasonably provide such guarantees, and are therefore not allowed in Rego. Recursion errors in Rego happens when a
rule (or function, which is a special type of rule) makes a reference to either itself, or another rule that references
the rule that made the original reference.

| Stage         | Category               | Message                                         |
| ------------- | ---------------------- | ----------------------------------------------- |
| `compilation` | `rego_recursion_error` | `rule {name} is recursive: ref -> ref [-> ...]` |

## Examples

Recursive reference to the same rule:

```rego
package policy

rule_a := rule_a
```

```txt
1 error occurred: policy.rego:3: rego_recursion_error: rule data.policy.rule_a is recursive: data.policy.rule_a -> data.policy.rule_a
```

Recursive references between two rules:

```rego
package policy

rule_a := rule_b

rule_b := rule_a
```

```txt
2 errors occurred:
policy.rego:5: rego_recursion_error: rule data.policy.rule_b is recursive: data.policy.rule_b -> data.policy.rule_a -> data.policy.rule_b
policy.rego:3: rego_recursion_error: rule data.policy.rule_a is recursive: data.policy.rule_a -> data.policy.rule_b -> data.policy.rule_a
```

While the examples above demonstrate the principle of recursive references, the error is rarely this obvious. The most
common occurrence of recursion in real-world policy happens when the global `data`
[document](https://www.openpolicyagent.org/docs/philosophy/#the-opa-document-model) is referenced without a
specific path, or when parts of the path are dynamic, and _potentially_ recursive.

```rego
package policy

# recursive since `data` includes `data.policy` which is the current package
rule := data
```

```rego
package policy

# the compiler can't know what `input.path` might provide, so this is _potentially_
# recursive (as `input.path` could be "policy") and as such flagged by the compiler
rule := data[input.path]
```

## How To Fix It

Ensure that dynamic references to `data` aren't potentially recursive by using a leading static path that doesn't
match the current package name.

```rego
package policy

import future.keywords.contains
import future.keywords.if
import future.keywords.in

deny contains message if {
    # `data.rules` can't be recursive since the first path component ("rules") is static
    # and doesn't match the name of the current package ("policy")
    some message in data.rules[_].deny
}

# not recursive regardless of `input` as "children" never references "policy"
rule := data.children[input.path]
```
