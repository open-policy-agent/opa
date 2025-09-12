# equals-pattern-matching

**Summary**: Prefer pattern matching in function arguments

**Category**: Idiomatic

**Avoid**
```rego
package policy

readable_number(x) := "one" if x == 1
readable_number(x) := "two" if x == 2
```

**Prefer**
```rego
package policy

readable_number(1) := "one"
readable_number(2) := "two"
```

## Rationale

Pattern matching on equality in function arguments is one of Rego's most well-kept secrets. As secret as it might be,
it's a great way to simplify custom functions performing equality checks on their arguments in the rule body, by
moving the equality check to match on the function call itself. This means that a function like the one below:

```rego
package policy

normalize_role(role) := "admin" if {
    role == "administrator"
}

normalize_role(role) := "admin" if {
    role == "root"
}
```

May have the equality check moved to the function argument, and the function only evaluated in case the argument matches
the equality "pattern":

```rego
package policy

normalize_role("administrator") := "admin"

normalize_role("root") := "admin"
```

Rules that evaluate to `true` may even have the assignment removed altogether, i.e.:

```rego
package policy

is_admin(role) if role == "admin"

is_admin(role) if role == "administrator"

is_admin(role) if role == "root"
```

Can be simplified to just:

```rego
package policy

is_admin("admin")

is_admin("administrator")

is_admin("root")
```

## Limitations

This rule is currently limited to simple rules where the equality check is the **only** condition in the rule body. This
will be improved in future releases.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    equals-pattern-matching:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Styra Blog: [How to express OR in Rego](https://www.styra.com/blog/how-to-express-or-in-rego/)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/equals-pattern-matching/equals_pattern_matching.rego)
