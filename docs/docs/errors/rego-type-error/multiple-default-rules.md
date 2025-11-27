---
sidebar_label: multiple default rules
image: /img/opa-errors.png
---

# rego_type_error: multiple default rules `{name}` found

This error is emitted by the type checker during the compilation stage when multiple `default` definitions are found
for one rule. The error message will show the rule e.g. `data.example.allow` that has multiple definitions, but the line
number for the error will only show the first location.

| Stage         | Category          | Message                               |
| ------------- | ----------------- | ------------------------------------- |
| `compilation` | `rego_type_error` | `multiple default rules <rule> found` |

## Examples

In the example below, the default value for the `allow` rule is defined twice:

```rego
package example

import rego.v1

default allow := false

allow if input.admin

default allow := false
```

This will result in the following error:

```txt
1 error occurred: policy.rego:5: rego_type_error: multiple default rules data.example.allow found
```

In this example, it's relatively easy to spot the issue, but in larger policies, it can be more challenging to find the
duplicate definitions. It's also possible that the default is defined in a different Rego file. For example:

```rego
# example1.rego
package example

default allow := false
```

```rego
# example2.rego
package example

default allow := true
```

We would see an error like this when using the two files:

```shell
$ opa eval data.example.allow -d example1.rego -d example2.rego
1 error occurred: example1.rego:3: rego_type_error: multiple default rules data.example.allow found
```

:::info
OPA will load files in lexicographical order, so temporarily renaming the file in the error message can be used to
search for the other definitions.
:::

## How To Fix It

Fixing this error is usually simple: just remove one of the repeated `default` definitions if they are setting the same
default value. If the intention is to set different default values, this is trickier, but perhaps you'd be better served
by two different rules.
