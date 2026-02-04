---
sidebar_label: "unexpected assign token"
image: /img/opa-errors.png
---

# rego_parse_error: unexpected assign token

In some cases when an assign token (`:=`) precedes some invalid Rego code, the
parser will raise this error.

:::info
This used to be one of the common, difficult-to-understand errors in OPA prior to version v0.66.0.
If you encounter it, please check your OPA version.
:::

| Stage     | Category           | Message                              |
| --------- | ------------------ | ------------------------------------ |
| `parsing` | `rego_parse_error` | `unexpected unexpected assign token` |

## Examples

A simple example of a policy that contains this error follows, note the unmatched `"` after `a :=`:

```rego
package policy

a := "
```

The code above will raise the following error:

```txt
1 error occurred: policy.rego:3: rego_parse_error: unexpected assign token: expected rule value term (e.g., a := <VALUE> { ... })
  a := "
    ^
```

The parser error is pointing to the `:=` token, which in this case is not the issue.

## How To Fix It

While the fix for this error depends on other elements on the line of Rego,
the error message will usually point to you to the line in question. Have a look for
invalid code later on that line as it is likely the cause of the error.

:::info
In OPA versions prior to v0.66.0, you can use a trick to get more helpful error messages.
By changing the `:=` to `=`, the parser will raise a more helpful error message:

```rego
package policy

a = "
```

This new code will raise the following, more helpful, error:

```txt
3 errors occurred:
policy.rego:3: rego_parse_error: non-terminated string
  a = "
      ^
```

**Note** however that generally this is not a recommended approach, see more detail
[here in the Regal docs](/projects/regal/rules/style/use-assignment-operator).
:::
