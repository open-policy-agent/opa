# superfluous-object-get

**Summary**: Superfluous `object.get` call

**Category**: Idiomatic

**Avoid**

```rego
package policy

allow if {
    object.get(input, ["path", "to", "value"], "default") == "expected"
}
```

**Prefer**

```rego
package policy

allow if {
    input.path.to.value == "expected"
}
```

## Rationale

The `object.get` function is sometimes used to guard against undefined references halting evaluation. Immediately
comparing the result of the call to a constant value that isn't the same as the default is however superfluous, as
the expression will evaluate the same without using `object.get`. In such cases the call can simply be removed.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    superfluous-object-get:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/superfluous-object-get/superfluous_object_get.rego)
