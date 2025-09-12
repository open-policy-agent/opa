# zero-arity-function

**Summary**: Avoid functions without args

**Category**: Bugs

**Avoid**
```rego
package policy

first_user() := input.users[0]
```

**Prefer**
```rego
package policy

first_user := input.users[0]
```

## Rationale

Zero-arity functions, or functions without arguments, aren't treated as functions by Rego, but as regular rules. For
that reason, they should also be expressed as such. One potential benefit of using functions over rules is that
functions don't contribute to the
[document](https://www.openpolicyagent.org/docs/philosophy/#the-opa-document-model) when a package is evaluated,
and as such sometimes used to "hide" information from the result of evaluation. Whether this is a good practice or not,
it importantly *doesn't work* with zero-arity functions, as they are treated as rules and *do* contribute to the
document.

There is an [open issue](https://github.com/open-policy-agent/opa/issues/6315) in the OPA project to try and address
this in the future, and allow zero-arity functions to be treated as other functions. Until then, the recommendation
is to avoid them and just use rules in their place.

Note that if you're using the [opa fmt](https://openpolicyagent.org/projects/regal/rules/style/opa-fmt) command to format your code,
it will remove the parentheses from a zero-arity function definition for you.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    zero-arity-function:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [The OPA Document Model](https://www.openpolicyagent.org/docs/philosophy/#the-opa-document-model)
- OPA Issues: [Allow user-defined zero-argument functions in Rego](https://github.com/open-policy-agent/opa/issues/6315)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/zero-arity-function/zero_arity_function.rego)
