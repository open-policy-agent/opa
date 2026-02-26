# in-wildcard-key

**Summary**: Unnecessary wildcard key

**Category**: Idiomatic

**Avoid**

```rego
package policy

allow if {
    # since only the value is used, we don't need to iterate the keys
    some _, user in input.users

    # do something with each user
}
```

**Prefer**

```rego
package policy

allow if {
    some user in input.users

    # do something with each user
}
```

## Rationale

The `some .. in` iteration form can either iterate only values:

```rego
some value in object
```

Or keys and values:

```rego
some key, value in object
```

Using a wildcard variable for the key in the key-value form is thus unnecessary, and:

```rego
some _, value in object
```

Can simply be replaced by:

```rego
some value in object
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    in-wildcard-key:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/in-wildcard-key/in_wildcard_key.rego)
