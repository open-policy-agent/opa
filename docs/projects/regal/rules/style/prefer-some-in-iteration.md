# prefer-some-in-iteration

**Summary**: Prefer `some .. in` for iteration

**Category**: Style

**Avoid**
```rego
package policy

engineering_roles := {"engineer", "dba", "developer"}

engineers contains employee if {
    employee := data.employees[_]
    employee.role in engineering_roles
}
```

**Prefer**
```rego
package policy

engineering_roles := {"engineer", "dba", "developer"}

engineers contains employee if {
    some employee in data.employees
    employee.role in engineering_roles
}
```

## Rationale

Using the `some .. in` construct for iteration removes ambiguity around iteration vs. membership checks, and is
generally more pleasant to read. Consider the following example:

```rego
some_condition if {
    other_rule[user]
    # ...
}
```

Are we iterating users over a partial "other_rule" here, or checking if the set contains a user defined elsewhere?
Or is `other_rule` a map-generating rule, and we're checking for the existence of a key? We won't know without looking
elsewhere in the code. Using `some .. in` removes this ambiguity, and makes the intent clear without having to jump
around in the policy.

Improved readability is not the only benefit of using `some .. in`. The `some` keyword ensures that the bindings
following the keyword are bound to the local scope, and modifications outside of e.g. a rule body won't affect how the
variables are evaluated. Consider the following simplified example to iterate over the keys of a map:

```rego
package policy

key_traversal if {
    map[key]
    # do something with key
}


key_traversal if {
    some key in object.keys(map)
    # do something with key
}
```

The two rules above are equivalent in that they both bind the variable `key` to the keys of `map`. The first
example would however change behavior entirely if a rule named `key` was introduced in the package, as the expression
would then mean "does map have key `key`?". While this isn't common, using `some .. in` means one less thing to worry
about.

## Exceptions

Deeply nested iteration is often easier to read using the more compact form.

```rego
package policy

# These rules are equivalent, but the more compact form is arguably easier to read

any_user_is_admin if {
    some user in input.users
    some attribute in user.attributes
    some role in attribute.roles
    role == "admin"
}

any_user_is_admin if {
    input.users[_].attributes[_].roles[_] == "admin"
}

# Using "if", we may even omit the brackets for single line rules
any_user_is_admin if input.users[_].attributes[_].roles[_] == "admin"
```

The `ignore-nesting-level` configuration option allows setting the threshold for nesting. Any level of nesting
**equal or greater than** the threshold won't be considered a violation. The default setting of `2` allows all _nested_
iteration, but not e.g. `my_array[x]`.

**Note:** not all nesting is _iteration_! The following example is considered to have a nesting level of `1`, as only
one of the variables (including wildcards: `_`) is an output variable bound in iteration:

```rego
package policy

example_users contains user if {
    domain := "example.com"
    user := input.sites[domain].users[_]
}
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    prefer-some-in-iteration:
      # one of "error", "warning", "ignore"
      level: error
      # except iteration if nested at or above the level i.e. setting of
      # '2' will allow `input[_].users[_]` but not `input[_]`
      ignore-nesting-level: 2
      # except iteration over items with sub-attributes, like
      # `name := input.users[_].name`
      # default is true
      ignore-if-sub-attribute: true
```

## Related Resources

- Rego Style Guide: [Prefer some .. in for iteration](https://github.com/StyraInc/rego-style-guide#prefer-some--in-for-iteration)
- Regal Docs: [Use `some` to declare output variables](https://openpolicyagent.org/projects/regal/rules/idiomatic/use-some-for-output-vars)
- OPA Docs: [Membership and Iteration: `in`](https://www.openpolicyagent.org/docs/policy-language/#membership-and-iteration-in)
- OPA Docs: [Some Keyword](https://www.openpolicyagent.org/docs/policy-language/#some-keyword)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/prefer-some-in-iteration/prefer_some_in_iteration.rego)
