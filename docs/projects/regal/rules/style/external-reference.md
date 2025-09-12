# external-reference

**Summary**: External reference in function

**Category**: Style

**Avoid**
```rego
package policy

# Depends on both `input` and `data`
is_preferred_login_method(method) if {
    preferred_login_methods := {login_method |
        some login_method in data.authentication.all_login_methods
        login_method in input.user.login_methods
    }
    method in preferred_login_methods
}
```

**Prefer**
```rego
package policy

# Depends only on function arguments
is_preferred_login_method(method, user, all_login_methods) if {
    preferred_login_methods := {login_method |
        some login_method in all_login_methods
        login_method in user.login_methods
    }
    method in preferred_login_methods
}
```

## Rationale

What separates functions from rules is that they accept arguments. While a function also may reference anything from
`input`, `data` or other rules declared in a policy, these references create dependencies that aren't obvious simply by
checking the function signature, and it makes it harder to reuse that function in other contexts. Additionally,
functions that only depend on their arguments are easier to test standalone.

## Exceptions

Rego does not provide first-class functions — functions can't be passed as arguments to other functions. Therefore, this
rule allows functions to freely reference (i.e. call) _other functions_, whether built-in functions, or custom functions
defined in the same package or elsewhere, and these do not count as "external references" simply because there is not
other way to import them into the function body.

```rego
package policy

first_name(full_name) := capitalized {
    first_name := split(full_name, " ")[0]

    # while data.utils.capitalize is an external reference, it's not flagged
    # as such, since there is no way to import it via function arguments
    capitalized := data.utils.capitalize(first_name)
}
```

### Changed default behavior since Regal v0.33.0

While we still consider it a best practice to pass any dependencies of a function in its arguments, the previous
(non-configurable) default of not allowing **any** external references was often considered too distracting. This led
to many disabling this rule entirely, or used inline ignore directives where this would be reported. Even in Regal's own
policies, there were quite a few locations where the latter option was used.

From v0.33.0 and onwards, this rule's default has been relaxed to allow 2 external references in any given function
definition, and the new `max-allowed` configuration option allows changing this value to whatever feels like a
reasonable default. If you previously disabled this rule in your projects, consider enabling it again configured to
match your preference.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    external-reference:
      # one of "error", "warning", "ignore"
      level: error
      # the number of external references to allow for any given function
      #
      # introduced in v0.33.0 and defaults to 2. set to 0 to revert to
      # original behavior to not allow any external references
      max-allowed: 2
```

## Related Resources

- Rego Style Guide: [Prefer using arguments over input, data or rule references](https://github.com/StyraInc/rego-style-guide#prefer-using-arguments-over-input-data-or-rule-references)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/external-reference/external_reference.rego)
