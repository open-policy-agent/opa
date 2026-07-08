---
sidebar_label: multiple default rules {name} found
image: /img/opa-errors.png
---

# `rego_type_error`: multiple default rules `{name}` found

The `default` keyword is used to define a base value for a rule that will be used if the other
rules do not match. `default` is a special case, and only one such case can be defined per rule.
This error is raised when multiple `default` rules are found for a single rule.

| Stage         | Category          | Message                               |
| ------------- | ----------------- | ------------------------------------- |
| `compilation` | `rego_type_error` | `multiple default rules {name} found` |

## Examples

A trivial example of a policy that contains this error is this one, where two `default`s are defined
for the `allow` rule:

```rego
package policy

default allow := false

allow if {
    "admin" in input.roles
}

default allow := false
```

Real world examples are often harder to spot as there might be more Rego code between the two
definitions. It's also possible that the `default` rules are spread across multiple files. For
example:

```rego
# main.rego
package policy

default allow := false
```

```rego
# admin.rego
package policy

default allow := false

allow if {
    "admin" in input.roles
}
```

Running OPA loading these two files produces the error:

```shell
$ opa run -s *.rego
... admin.rego:3: rego_type_error: multiple default rules data.policy.allow found
```

## How To Fix It

In almost all cases, if you have an error like this: `multiple default rules data.policy.allow found`,
searching for `default allow` in your Rego files will show up the various duplicates.

Remember that they could be spread across multiple files, and you want to make sure to check only
within the package in the error message, `policy` in the example above.
