---
sidebar_label: var {name} is unsafe
image: /img/opa-errors.png
---

# `rego_unsafe_var_error`: var `{name}` is unsafe

This is one of the most common errors reported by OPA. When a variable is "unsafe" it means that OPA wasn't able
to determine where to find it. This is commonly caused by misspelling the name of the variable, or perhaps by
referencing a rule or function that doesn't (yet) exist. Note that this check happens at _compile time_. This means that
references like `input.username` will not be considered unsafe even when there is no `username` attribute in the `input`
— as the value of `input` isn't known at compile time.

| Stage         | Category                | Message                                                             |
| ------------- | ----------------------- | ------------------------------------------------------------------- |
| `compilation` | `rego_unsafe_var_error` | `var {name} is unsafe` (where `{name}` is the name of the variable) |

## Examples

Can you spot the unsafe variable in the following policy?

```rego
package policy

allow if {
    user_is_developer
    input.request.path == "/api"
}

allow if user_is_admin

user_is_developer if "developer" in input.user.roles
```

Fortunately the compiler reports it automatically:

```shell
1 error occurred: policy.rego:8: rego_unsafe_var_error: var user_is_admin is unsafe
```

The `user_is_admin` rule was not defined before use, which is why it's
considered unsafe.

Sometimes, the location of the errors isn't as clear-cut, even when reading the errors.
Consider the following example. Which variable is unsafe?

```rego
package policy

allow if {
    first_user := users[0]
    title := first_user.title
    title_upper := upper(title)

    title_upper == "CEO"
}
```

You'd be excused to say `users`, and you'd even be right. But it doesn't stop there:

```shell
4 errors occurred:
foo.rego:4: rego_unsafe_var_error: var first_user is unsafe
foo.rego:4: rego_unsafe_var_error: var users is unsafe
foo.rego:5: rego_unsafe_var_error: var title is unsafe
foo.rego:6: rego_unsafe_var_error: var title_upper is unsafe
```

Since `users` is unsafe, every variable that depend on `users` is _also_ considered unsafe. This means that _all_ the
variables in the `allow` rule will be considered unsafe, requiring some investigation to figure out
what the actual root cause was. Whether this should be needed or not is
[up for debate](https://github.com/open-policy-agent/opa/issues/6393), and this limitation may be removed in future
OPA versions.

## How To Fix It

Once the unsafe variable is found (the compiler should help here, as shown
above), determine _why_ it is considered unsafe. There are two
main reasons that a variable is considered unsafe:

- You have a typo in the policy pointing the compiler to the wrong place.
- There is a reference to a rule which has not been imported and so is unsafe.

Typos are generally easier to spot, but not that sometimes the typo can be in
the definition, not the location of the variable itself, so be sure to check
there too.

If the variable name and definition are correct, the issue is likely that the
definition of the rule has not been loaded into OPA. One thing to keep in mind
is that all OPA commands that accept a file can also be provided a
directory, which will be loaded recursively. This is often the best way to ensure all the files you may depend on are
loaded and available during compilation.

## More Information

Recall that the compiler won't consider a reference like `input.usrname` (note the typo!)
unsafe, even though the intent was `input.username`. Schema annotations provide a way to tell the
OPA compiler what the `input` object should look like, and have it include that in this type of check.
The desired structure (i.e. the _schema_) of both `input` and `data` may be provided to the compiler via OPA's
[JSON schema capability](https://www.openpolicyagent.org/docs/policy-language/#schema), thus extending the
compiler and the type checker with this information. It'll take some work to set up, but it's well worth it!
