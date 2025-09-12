# custom-has-key-construct

**Summary**: Custom function may be replaced by `in` and `object.keys`

**Category**: Idiomatic

**Avoid**
```rego
package policy

mfa if has_key(input.claims, "mfa")

has_key(map, key) if {
    _ = map[key]
}
```

**Prefer**
```rego
package policy

mfa if "mfa" in object.keys(input.claims)
```

## Rationale

Checking if a key exists in an object (regardless of the attribute's value) used to be done using custom functions. With
the introduction of the [object.keys](https://www.openpolicyagent.org/docs/policy-reference/#builtin-object-objectkeys)
(OPA [v0.47.0](https://github.com/open-policy-agent/opa/releases/tag/v0.47.0)) function, this is no longer necessary,
and using the built-in function together with `in` should be preferred.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    custom-has-key-construct:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/custom-has-key-construct/custom_has_key_construct.rego)
