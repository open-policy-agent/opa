# dubious-print-sprintf

**Summary**: Dubious use of `print` and `sprintf`

**Category**: Testing

**Avoid**
```rego
package policy

allow if {
    # if any of input.name or input.domain are undefined, this will just print <undefined>
    print(sprintf("name is: %s domain is: %s", [input.name, input.domain]))

    input.name == "admin"
}
```

**Prefer**
```rego
package policy

allow if {
    # if any of input.name or input.domain are undefined, this will still print the whole
    # sentence, with the value undefined printed as such, e.g.
    # name is: admin domain is: <undefined>
    print("name is:", input.name, "domain is:", input.domain)

    input.name == "admin"
}
```

## Rationale

Since `print` allows any number of arguments, there's rarely any benefit to using `sprintf` for formatting the output of
a `print` call. But more importantly, the `print` function is unique in that it will allow any arguments passed to be
*undefined* without terminating, but will print such values as `<undefined>`. Using `sprintf` will however nullify this
benefit, and just print `<undefined>` without the context.

Note that using `print` is generally discouraged outside of development, and other rules exists to check for its use.
However, in the context of development and testing, one may choose to allow `print`, in e.g. `_test.rego` files, while
still wanting to avoid the use of `sprintf` in such cases.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  testing:
    dubious-print-sprintf:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Regal Docs: [Call to `print` or `trace` function](https://www.openpolicyagent.org/projects/regal/rules/testing/print-or-trace-call)
- OPA Docs: [Policy Testing](https://www.openpolicyagent.org/docs/policy-testing/)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/testing/dubious-print-sprintf/dubious_print_sprintf.rego)
