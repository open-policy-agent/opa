# use-some-for-output-vars

**Summary**: Use `some` to declare output variables

**Category**: Idiomatic

**Avoid**
```rego
package policy

allow if {
    userinfo := data.users[id]
    # ...
}
```

**Prefer**
```rego
package policy

allow if {
    some id
    userinfo := data.users[id]
    # ...
}

# alternatively, and arguably more idiomatic:
allow if {
    some id, userinfo in data.users
    # ...
}
```

## Rationale

An interesting, and likely unfamiliar aspect of Rego for developers coming from other languages, is the concept of
[unification](https://en.wikipedia.org/wiki/Unification_(computer_science)). Unification happens not just explicitly via
the unification operator (`=`), but is an integral part of Rego. In the context of this rule, unification means that a
variable can either be an _input_ or an _output_. What does that mean?

From the example above, consider that `data.users` is a map of user IDs to user objects:

```json
{
  "jane": {"email": "jane@acmecorp.com", "firstname": "Jane", "lastname": "Doe"},
  "joe": {"email": "joe@example.com", "firstname": "Joe", "lastname": "Bloggs"},
  "john": {"email": "john@opa.org", "firstname": "John", "lastname": "Smith"}
}
```

```rego
usernames contains name if {
    data.users[name]
}
```

What is the meaning of `name` in the body of the `usernames` rule? In most programming languages, evaluation would
fail unless `name` was defined elsewhere in the code. That is because `name` would be expected to be an **input** in the
expression — the result of using, say "joe", as the input in `users["joe"]` would predictably be the value associated
with that key. In Rego, however, `name` may also be an **output** — meaning that if the variable is not defined
elsewhere, OPA will attempt to _unify_ it with any value that satisfies the expression. In this case, that means that
`name` will be bound to each of the keys in the `users` object in turn, and the rule will succeed for each of them.

This is a powerful feature of Rego, but it can also be a source of confusion. If we were to define `name` somewhere
else in the policy, perhaps by mistake:

```rego
name := "joe"

# hundreds of lines of Rego later..

usernames contains name if {
    data.users[name]
}
```

Our `usernames` rule would no longer iterate over all the users, as the condition would be satisfied by simply mapping
the key "joe" to its value. By using `some` to locally declare `name`, we can avoid this problem:

```rego
name := "joe"

# hundreds of lines of Rego later..

usernames contains name if {
    some name
    data.users[name]
}
```

Even though `name` is defined in the global scope, the `some` keyword will ensure it's now considered as an output
variable in the local scope of the `usernames` rule.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    use-some-for-output-vars:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Rego Style Guide: [Don't use undeclared variables](https://www.openpolicyagent.org/docs/style-guide#dont-use-undeclared-variables)
- OPA Docs: [The `some` keyword](https://www.openpolicyagent.org/docs/policy-language/#some-keyword)
- Wikipedia: [Unification](https://en.wikipedia.org/wiki/Unification_(computer_science))
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/use-some-for-output-vars/use_some_for_output_vars.rego)
