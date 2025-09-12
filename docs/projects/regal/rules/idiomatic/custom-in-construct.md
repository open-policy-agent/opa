# custom-in-construct

**Summary**: Custom function may be replaced by `in` keyword

**Category**: Idiomatic

**Avoid**
```rego
package policy

allow if has_value(input.user.roles, "admin")

# This custom function was commonly seen before the introduction
# of the `in` keyword. Avoid it now.
has_value(arr, item) if {
    item == arr[_]
}
```

**Prefer**
```rego
package policy

allow if "admin" in input.user.roles
```

## Rationale

The `in` keyword was introduced in OPA [v0.34.0](https://github.com/open-policy-agent/opa/releases/tag/v0.34.0).
Prior to that, it was a common practice to create a custom helper function that would iterate over values of an array in
order to check if it contained a provided value. Since the introduction of the `in` keyword, this is no longer
necessary. The `in` keyword additionally supports sets and maps as the collection type, so using it consistently is
recommended.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    custom-in-construct:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/custom-in-construct/custom_in_construct.rego)
