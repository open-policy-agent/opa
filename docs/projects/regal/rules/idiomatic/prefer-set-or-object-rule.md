# prefer-set-or-object-rule

**Summary**: Prefer set or object rule over comprehension

**Category**: Idiomatic

**Avoid**

```rego
package policy

# top level set comprehension
developers := {developer |
    some user in input.users
    "developer" in user.roles
    developer := user.name
}

# top level object comprehension
user_roles_mapping := {user: roles |
    some user in input.users
    roles := user.roles
}
```

**Prefer**

```rego
package policy

# set generating rule
developers contains developer if {
    some user in input.users
    "developer" in user.roles
    developer := user.name
}

# object generating rule
user_roles_mapping[user] := roles if {
    some user in input.users
    roles := user.roles
}
```

## Rationale

Comprehensions are [awesome](https://www.styra.com/blog/five-things-you-didnt-know-about-opa/), and should be part of
any policy author's toolbox. Using comprehensions inside of rule bodies allow for a wide variety of elegant solutions to
otherwise hard problems. However, when used as the value directly (and unconditionally) assigned to a rule, it is almost
always better to use a rule that generates a set or object in the rule body rather than having a comprehension do so in
the rule head. Why is that?

### Readability

Rules that generate objects, and sets even more so, read more natural than comprehensions, and are generally more
descriptive. While both constructs are easy to spot for a seasoned Rego author, anything that helps improve readability
is a win.

### Extensibility

While readability is important, the real benefit of using a rule to generate a set or object is that it allows the rule
to be _extended_. This is particularly true for set generating rules, and it's not by accident they often are referred
to as **multi-value rules**. A rule assigned the value of a set comprehension can't have its value changed later, or
more items added to the set. A set generating rule however, can easily be extended to contain more items, conditionally
or unconditionally.

```rego
package policy

# Getting developers from input
developers contains developer if {
    some user in input.users
    "developer" in user.roles
    developer := user.name
}

# *Also* getting developers from data
developers contains developer if {
    some user in data.users
    "developer" in user.roles
    developer := user.name
}

# Unconditionally adding a developer to the set
developers contains "Hackerman"
```

In the example above, all three rules contribute to the `developers` set. If we wanted to, we could even create another
policy file using the same package, and have more rules added there that would contribute to the set. This creates some
great opportunities for extensibility, and collaboration across developers and teams working on policy together.

Objects differ somewhat from sets in that while several rules can be used to generate an object, there cannot be more
than one rule contributing to a single key-value pair.

```rego
package policy

novels[title] := content if {
    some document in input.documents
    document.type == "novel"
    title := document.title
    content := document.content
}

# This works as long as "The Hobbit" is not already in the novels object
novels["The Hobbit"] := "In a hole in the ground there lived a hobbit."

# Map and set generating objects can also be combined, in which case the
# value is extensible even for the same key! In the example above, more
# rules could help contribute titles to an author, perhaps using different
# data sources.
titles_by_author[document.author] contains document.title if {
    some document in input.documents
}
```

## Exceptions

Note that this rule does **not** apply to array comprehensions, as there is no equivalent tp use a rule to generate an
array.

This rule will also ignore simple comprehensions used solely for the purpose of converting an array to a set, i.e:

```rego
package policy

# Convert set to array. This is fine.
my_set := {item | some item in arr}
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    prefer-set-or-object-rule:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Generating Sets](https://www.openpolicyagent.org/docs/policy-language/#generating-sets)
- OPA Docs: [Generating Objects](https://www.openpolicyagent.org/docs/policy-language/#generating-objects)
- OPA Docs: [Comprehensions](https://www.openpolicyagent.org/docs/policy-language/#comprehensions)
- Styra Blog: [Five Things You Didn't Know About OPA](https://www.styra.com/blog/five-things-you-didnt-know-about-opa/)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/prefer-set-or-object-rule/prefer_set_or_object_rule.rego)
