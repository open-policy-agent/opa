# prefer-snake-case

**Summary**: Prefer snake_case for names

**Category**: Style

**Avoid**
```rego
package policy

# camelCase rule name
userIsAdmin if "admin" in input.user.roles
```

**Prefer**
```rego
package policy

# snake_case rule name
user_is_admin if "admin" in input.user.roles
```

## Rationale

The built-in functions use `snake_case` for naming — follow that convention for your own packages, rules, functions,
and variables, unless you have a really good reason not to.

## Exceptions

In many cases, you might not control the format of the `input` data — if the domain of a policy (e.g. Envoy)
mandates a different style, making an exception might seem reasonable. Adapting policy format after `input` is however
prone to inconsistencies, as you'll likely end up mixing different styles in the same policy (due to imports of common
code, etc).

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    prefer-snake-case:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Rego Style Guide: [Prefer snake_case for rule names and variables](https://github.com/StyraInc/rego-style-guide#prefer-snake_case-for-rule-names-and-variables)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/prefer-snake-case/prefer_snake_case.rego)
