# narrow-argument

**Summary**: Function argument can be narrowed

**Category**: Custom

**Avoid**
```rego
package policy

valid_user(user) if endswith(user.email, "acmecorp.com")
valid_user(user) if endswith(user.email, "acmecorp.org")
```

**Prefer**
```rego
package policy

valid_email(email) if endswith(email, "acmecorp.com")
valid_email(email) if endswith(email, "acmecorp.org")
```

## Rationale

**Note!** This is a highly opinionated rule with some caveats that you should be aware of before you use it.

Accepting the most minimal types/values as functions arguments avoids unnecessary "dependencies", makes them more
likely to be reusable, and tends to make code more readable as it's often easier to predict from a call site what
a specifically named function does compared to a more generic version. It's fairly common to realize a function with
narrowed arguments would benefit from being renamed. That is however left as an exercise to you!

This rule scans all use of arguments inside function heads and bodies, and will suggest narrowing down any argument
passed to the minimal value which the function depends on. Incrementally defined functions are scanned in their entirety
to make sure the narrowing is valid for all definitions. Example:

```rego
package policy

country_code(user) := 61 if user.country == "Australia"
country_code(user) := 81 if user.country == "Japan"
```

In the above example, the functions only depends on `user.country`, and this rule (when enabled) will thus recommend
narrowing the argument passed:

```rego
package policy

country_code(country) := 61 if country == "Australia"
country_code(country) := 81 if country == "Japan"
```

Instead of passing around potentially large `user` objects, our function now only needs to consider a `country` string,
which perhaps may prove useful for more than just users. Another benefit of this approach is that it's often possible
to simplify functions even further by moving the equality comparison directly into the function's arguments — a simple
form of [pattern matching](https://www.openpolicyagent.org/projects/regal/rules/idiomatic/equals-pattern-matching):

```rego
package policy

country_code("Australia") := 61
country_code("Japan") := 81
```

### Reference prefix narrowing

So far we have looked only at functions using an identical reference to one of its arguments. That's not always the
case, but it doesn't mean the value passed can't be narrowed! Consider the following example:

```rego
package policy

internal_user(context) if endswith(context.user.email, "@acmecorp.com")
internal_user(context) if "staff" in context.user.roles
```

In the example above, the `narrow-argument` rule would point out that while two different references to the `context`
argument are used, they both have the `context.user` prefix in common, and the value passed could thus be narrowed to
that:

```rego
package policy

internal_user(user) if endswith(user.email, "@acmecorp.com")
internal_user(user) if "staff" in user.roles
```

## Caveats

Narrowing the types passed as function arguments may come with unintended and/or undesired side-effects. More
specifically, the way OPA evaluates functions means that arguments are evaluated before the function is called. "Big"
objects, like a `user` tend to be less likely to be undefined than e.g. a `user.fax` attribute. Aborting evaluation only
because a user is without a fax machine is probably not what we want! But could be an unfortunate consequence of our
change unless we are careful (or better, have extensive test coverage). Consider the following example:

```rego
package policy

is_unreachable(user) if {
    not has_phone(user)
    not has_fax(user)
}

has_phone(user) if
    is_string(user.phone)
    user.phone != ""
}

has_fax(user) if {
    is_string(user.fax)
    user.fax != ""
}
```

While it's tempting to try and narrow the arguments passed to `has_phone` and `has_fax` only to what they need:

```rego
package policy

is_unreachable(user) if {
    not has_phone(user.phone)
    not has_fax(user.fax)
}

has_phone(phone) if
    is_string(phone)
    phone != ""
}

has_fax(fax) if {
    is_string(fax)
    fax != ""
}
```

We have now changed the behavior of `is_unreachable`, and a user without phone or fax will no longer be considered
unreachable. Why? Again, because OPA evaluates the function arguments before they are passed to the function, **and**
before the result is negated by `not`, an expression like:

```rego
not has_phone(user.phone)
```

Will be rewritten by OPA to something like this:

```rego
arg1 := user.phone
not has_phone(arg1)
```

If the `user.phone` attribute doesn't exist, evaluation will never reach the next line where the function is called!

Before narrowing arguments, always consider the impact of undefined values, negation and how functions are evaluated.
And make sure to not rewrite any function that isn't extensively covered by unit tests! With that said, there are often
ways to deal with undefined attributes even when passing narrower argument types. In the example above, we could for
example rewrite `is_unreachable` to `is_reachable`, and then use `not` to negate *that* to answer if the user is
impossible to reach.

Find what works best for you, and use the `exclude-args` configuration option (see below) to exclude arg names that you
commonly don't want to narrow, or [ignore directives](https://www.openpolicyagent.org/projects/regal#inline-ignore-directives) for single
locations.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  custom:
    narrow-argument:
      # note that all rules in the "custom" category are disabled by default
      # (i.e. level "ignore")
      #
      # one of "error", "warning", "ignore"
      level: error
      # exclude args by name
      # example below excludes any argument named 'config' or 'user'
      exclude-args:
        - config
        - user
```
