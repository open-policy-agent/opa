# deprecated-builtin

**Summary**: Constant condition

**Category**: Bugs

## Notice: Rule disabled with OPA 1.0

Since Regal v0.30.0, this rule is only enabled for projects that have either been explicitly configured to target
versions of OPA before 1.0, or if no configuration is provided — where Regal is able to determine that an older version
of OPA/Rego is being targeted. Consult the documentation on Regal's
[configuration](https://openpolicyagent.org/projects/regal#configuration) for information on how to best work with older versions of
OPA and Rego.

Since OPA v1.0, this rule is automatically disabled, as there currently are no deprecated built-in functions
in that version, and trying to use a previously deprecated function will result in a parser error. Note however that
this may change if later OPA versions deprecate current built-in functions. If/when that happens, this rule will be
re-enabled.

**Avoid**
```rego
package policy

import future.keywords.if

# call to deprecated `any` built-in function
allow if any([input.user.is_admin, input.user.is_root])
```

**Prefer**
```rego
package policy

import future.keywords.if

allow if input.user.is_admin
allow if input.user.is_root
```

## Rationale

Calling deprecated built-in functions should always be avoided, and replacing them is usually trivial.

## Replacing Deprecated Built-in Functions

### `any`

Use the `in` keyword (OPA v0.34.0+) to replace the `any` function:

**Instead of**
```rego
a := any([input.foo, input.bar])
```

**Do this**
```rego
import future.keywords.in # or `import rego.v1` (OPA v0.59.0+)

a := true in [input.foo, input.bar]
```

Using `in` additionally has the benefit that it can be used to check for any type of value, and not just boolean
`true`!

### `all`

Use the `every` keyword (OPA v0.34.0+) to replace the `all` function:

**Instead of**
```rego
a {
    all([input.foo, input.bar])
}
```

**Do this**
```rego
import future.keywords.every # or `import rego.v1` (OPA v0.59.0+)

a {
    every x in [input.foo, input.bar] {
        x == true
    }
}
```

Just like `in` may be used for much more than `any`, `every` can be used to evaluate complex expressions!

### `set_diff`

Use the minus (`-`) operator instead, of `set_diff`:

**Instead of**
```rego
a := set_diff(s1, s2)
```

**Do this**
```rego
a := s1 - s2
```

### `re_match` and `net.cidr_overlap`

These built-in function were renamed `regex.match` and `net.cidr_intersects` respectively, so simply use the new names
instead.

### `cast_array`, `cast_set`, `cast_string`, `cast_boolean`, `cast_null`, `cast_object`

Use the `is_X` equivalent built-in function in their place:

**Instead of**
```rego
a {
    cast_string(input.name)
}
```

**Do this**
```rego
a {
    is_string(input.name)
}
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    deprecated-builtin:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Strict Mode](https://www.openpolicyagent.org/docs/policy-language/#strict-mode)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/deprecated-builtin/deprecated_builtin.rego)
