# comprehension-term-assignment

**Summary**: Assignment can be moved to comprehension term

**Category**: Style

**Avoid**
```rego
package policy

names := [name |
    some user in input.users
    name := user.name # redundant assignment
]
```

**Prefer**
```rego
package policy

names := [user.name |
    some user in input.users
]

# which in this case can be made a one-liner
names := [user.name | some user in input.users]
```

## Rationale

Adding an intermediate assignment in a comprehension body to a variable used as the comprehension term (i.e. the value
to the left side of `|` in a comprehension) is redundant, as the value can be used directly as the comprehension term.
Making code as compact as possible should never be a goal in itself, but the same is true for making code needlessly
verbose. And in cases like `names := [user.name | some user in input.users]`, adding an intermediate assignment does
nothing to improve readability.

## Exceptions

This rule will only flag simple assignments where the value could be moved directly into the comprehension term. More
complex assignments involving dynamic references or function calls, will not be considered as violations.

Example:

```rego
first_names := [first_name |
    some user in input.users
    first_name := capitalize(user.name.split(" ")[0])
]
```

While it's possible to move the value of the `first_name` assignment directly to the comprehension term, it's arguably
less readable, as it makes it harder to see that the form is a comprehension to begin with.

```rego
# Not recommended
first_names := [capitalize(user.name.split(" ")[0]) |
    some user in input.users
]
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    comprehension-term-assignment:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Comprehensions](https://www.openpolicyagent.org/docs/policy-language/#comprehensions)
- Regal Docs: [pointless-reassignment](https://www.openpolicyagent.org/projects/regal/rules/style/pointless-reassignment)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/comprehension-term-assignment/comprehension_term_assignment.rego)
