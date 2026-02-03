---
sidebar_label: Style Guide
---

# Rego Style Guide

<!-- The source of truth for this file is at https://github.com/StyraInc/rego-style-guide/blob/main/style-guide.md -->

The purpose of this style guide is to provide a collection of recommendations and best practices for authoring
[Rego](https://www.openpolicyagent.org/docs/policy-language).
From the maintainers of [Open Policy Agent](https://www.openpolicyagent.org) (OPA),
and some of the most experienced members of the community,
we hope to share lessons learnt from authoring and reviewing hundreds of thousands of lines of Rego over the years.

With new features, language constructs, and other improvements continuously finding their way into OPA, we aim to keep
this style guide a reflection of what we consider current best practices. Make sure to check back every once in a while,
and see the changelog for updates since your last visit.

## Regal

Inspired by this style guide, [Regal](/projects/regal) is a new linter for Rego that allows you
to enforce many of the recommendations in this guide, as well as identifying issues, bugs and potential problems in your
Rego policies. If you enjoy this style guide, make sure to check it out!

## General Advice

### Optimize for readability, not performance

Rego is a declarative language, which in the best case means you express **what** you want rather than **how** it
should be retrieved. When authoring policy, do not try to be "smart" about assumed performance characteristics or
optimizations. That's what OPA should worry about!

Optimize for **readability** and **obviousness**. Optimize for performance _only_ if you've identified performance
issues in your policy, and even if you do — making your policy more compact or "clever" almost never helps at addressing
the problem at hand.

#### Related Resources

- [Policy Performance](https://www.openpolicyagent.org/docs/policy-performance)

### Use `opa fmt`

The `opa fmt` tool ensures consistent formatting across teams and projects. While certainly not
[perfect](https://github.com/open-policy-agent/opa/issues/4508) (yet!), unified formatting is a big win, and saves a
lot of time in code reviews arguing over details around style.

A good idea could be to run `opa fmt --write` on save, which can be configured in most editors. If you want to enforce
`opa fmt` formatting as part of your build pipeline, use `opa fmt --fail`.

In order to not flood this guide with data, formatting conventions covered by `opa fmt` will not be included here.

**Tip**: `opa fmt` uses tabs for indentation. By default, GitHub uses 8 spaces to display tabs, which is arguably a bit
much. You can change this preference for your account in
[github.com/settings/appearance](https://github.com/settings/appearance),
or provide an [.editorconfig](https://editorconfig.org/) file in your policy repository, which will be used by GitHub
(and other tools) to properly display your Rego files:

```ini
[*.rego]
end_of_line = lf
insert_final_newline = true
charset = utf-8
indent_style = tab
indent_size = 4
```

Sadly, there doesn't seem to be a way to enforce this for code blocks displayed in markdown (`.md`) files.

:::tip
You can lint for this recommendation using the [`opa-fmt`](/projects/regal/rules/style/opa-fmt)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

### Use strict mode

[Strict mode](https://www.openpolicyagent.org/docs/policy-language#strict-mode) provides extra checks for common mistakes like
redundant imports, or unused variables. Include an `opa check --strict path/to/polices` step as part of your build
pipeline.

### Use metadata annotations

Favor [metadata annotations](https://www.openpolicyagent.org/docs/policy-language#metadata) over regular comments.

Metadata annotations allow external tools and editors to parse their contents, potentially leveraging them for
something useful, like in-line explanations, generated docs, etc.

Annotations are also a good way to de-duplicate information such as documentation links, contact emails
and error codes where explanations are returned as part of the result.

**Avoid**
```rego
# Example package with documentation
package example

import future.keywords.contains
import future.keywords.if

# E123: Deny non admin users.
# Only admin users are allowed to access these resources, see https://docs.example.com/policy/rule/E123
deny contains {
	"code": 401,
	"message": "Unauthorized due to policy rule (E123, https://docs.example.com/policy/rule/E123)",
} if {
	input.admin == false
}
```

**Prefer**
```rego
# METADATA
# title: Example
# description: Example package with documentation
package example

import future.keywords.contains
import future.keywords.if

# METADATA
# title: Deny non admin users
# description: Only admin users are allowed to access these resources
# related_resources:
# - https://docs.example.com/policy/rule/E123
# custom:
#   code: 401
#   error_id: E123
deny contains {
	"code": metadata.custom.code,
	"message": sprintf("Unauthorized due to policy rule (%s, %s)", [
		metadata.custom.error_id,
		concat(",", [ref | ref := metadata.related_resources[_].ref]),
	]),
} if {
	input.admin == false

	metadata := rego.metadata.rule()
}
```

**Notes / Exceptions**

Use regular comments inside of rule bodies, or for packages and rules you consider "internal".

#### Related Resources

- [Annotations](https://www.openpolicyagent.org/docs/policy-language#metadata)

### Get to know the built-in functions

With more than 150 [built-in functions](https://www.openpolicyagent.org/docs/latest/policy-reference/#built-in-functions)
tailor-made for policy evaluation, there's a good chance that some of them can help you accomplish your goal.

### Consider using JSON schemas for type checking

As you author Rego policy, providing JSON schemas for your `input` (and possibly `data`) enables strict
[type checking](https://www.openpolicyagent.org/docs/policy-language#schema), letting you avoid simple — but common — mistakes,
like typos, or referencing nested attributes in the wrong location. This extra level of verification improves both the
developer experience as well as the quality of your policies.

## Style

### Prefer snake_case for rule names and variables

The built-in functions use `snake_case` for naming — follow that convention for your own rules, functions, and variables.

**Avoid**
```rego
userIsAdmin if "admin" in input.user.roles
```

**Prefer**
```rego
user_is_admin if "admin" in input.user.roles
```

**Notes / Exceptions**

In many cases, you might not control the format of the `input` data — if the domain of a policy (e.g. Envoy)
mandates a different style, making an exception might seem reasonable. Adapting policy format after `input` is however
prone to inconsistencies, as you'll likely end up mixing different styles in the same policy (due to imports of common
code, etc).

:::tip
You can lint for this recommendation using the [`prefer-snake-case`](/projects/regal/rules/style/prefer-snake-case)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

### Optionally, use leading underscore for rules intended for internal use

While OPA doesn't have "private" rules or functions, a pretty common convention that we've seen in the community is to
use a leading underscore for rules and functions that are intended to be internal to the package that they are in:

```rego
developers contains user if {
    some user in input.users
    _is_developer(user)
}

_is_developer(user) if {
    # some conditions
}

_is_developer(user) if {
    # some other conditions
}
```

While an `is_developer` function may seem like a good candidate for reuse, it could easily be the case that this should
be considered to what **this** package considers a developer, and not necessarily a universal truth. Using a leading
underscore to denote this is a good way to communicate this intent, but there are also other ways to do this, like
agreed upon naming conventions, or using custom metadata annotation attributes.

One benefit of sticking to the leading underscore convention is that tools like [Regal](/projects/regal),
and the language server for Rego that it provides, may use this information to provide better suggestions, like not
adding references to these rules and functions from other packages.

### Keep line length `<=` 120 characters

Long lines are tedious to read. Keep line length at 120 characters or below.

**Avoid**
```rego
frontend_admin_users := [username | some user in input.users; "frontend" in user.domains; "admin" in user.roles; username := user.username]
```

**Prefer**
```rego
frontend_admin_users := [username |
    some user in input.users
    "frontend" in user.domains
    "admin" in user.roles
    username := user.username
]
```

:::tip
You can lint for this recommendation using the [`line-length`](/projects/regal/rules/style/line-length)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

## Rules

### Use helper rules and functions

Helper rules makes policies more readable, and for repeated conditions more performant as well. If your rule contains
more than a few simple expressions, consider splitting it into multiple rules with good names.

**Avoid**
```rego
allow if {
    "developer" in input.user.roles
    input.request.method in {"GET", "HEAD"}
    startswith(input.request.path, "/docs")
}

allow if {
    "developer" in input.user.roles
    input.request.method in {"GET", "HEAD"}
    startswith(input.request.path, "/api")
}
```

**Prefer**
```rego
allow if {
    is_developer
    read_request
    startswith(input.request.path, "/docs")
}

allow if {
    is_developer
    read_request
    startswith(input.request.path, "/api")
}

read_request if input.request.method in {"GET", "HEAD"}

is_developer if "developer" in input.user.roles
```

Additionally, helper rules and functions may be kept in (and imported from) separate modules, allowing you to build a
logical — and reusable! — structure for your policy files.

### Use negation to handle undefined

When encountering undefined references inside of rules, evaluation of the rule halts and the rule _itself_ evaluates to
undefined, unless of course, a `default` value has been provided. While saying `allow is undefined` or `allow is false`
if encountering undefined in a rule is likely desirable, this doesn't hold true when working with "inverted" rules -
i.e. rules like `deny` (as opposed to `allow`). Saying `deny is undefined` or `deny is false` if undefined is
encountered, essentially means that any occurrence of undefined (such as when attributes are missing in the input
document) would lead to the `deny` rule not getting enforced. This is particularly common writing partial rules (i.e.
rules that build [sets](https://www.openpolicyagent.org/docs/policy-language#generating-sets) or
[objects](https://www.openpolicyagent.org/docs/policy-language#generating-objects)).

Consider for example this simple rule:

**Avoid**
```rego
authorized := count(deny) == 0

deny contains "User is anonymous" if input.user_id == "anonymous"
```

At first glance, it might seem obvious that evaluating the rule should add a violation to the set of messages if the
`user_id` provided in `input` is equal to "anonymous". But what happens if there is no `user_id` provided _at all_?
Evaluation will stop when encountering undefined, and the comparison will never be invoked, leading to **nothing**
being added to the `deny` set — the rule allows someone without a `user_id`. We could of course add another
rule, checking only for its presence:

```rego
deny contains "User ID missing from input" if not input.user_id
```

This is nice in that we'll get an even more granular message returned to the caller, but quickly becomes tedious when
working with a large set of input data. To deal with this, a helper rule using _negation_ may be used.

**Prefer**
```rego
authorized := count(deny) == 0

deny contains "User is anonymous" if not authenticated_user

authenticated_user if input.user_id != "anonymous"
```

In the above case, the `authenticated_user` rule will fail **both** in the the undefined case, and if defined
but equal to "anonymous". Since we negate the result of the helper rule in the `deny` rule, we'll have both
cases covered.

#### Related Resources

- [OPA AWS CloudFormation Hook Tutorial](https://www.openpolicyagent.org/docs/aws-cloudformation-hooks)

### Consider partial helper rules over comprehensions in rule bodies

While comprehensions inside of rule bodies allows for compact rules, these are often harder to debug, and can't easily
be reused by other rules. Partial rules may be referenced by any other rule, and more importantly, by you!
Having many smaller, composable rules, is often key to quickly identifying where things fail, as each rule may be
queried individually.

**Avoid**
```rego
allow if {
    input.request.method in {"GET", "HEAD"}
    input.request.path[0] == "credit_reports"
    input.user.name in {username |
        # These should not count as MFA
        insecure_methods := {"email"}

        some user in data.users
        mfa_methods := {method | some method in user.authentication.methods} - insecure_methods

        count(mfa_methods) > 1
        username := user.name
    }
}
```

**Prefer**
```rego
allow if {
    input.request.method in {"GET", "HEAD"}
    input.request.path[0] == "credit_reports"
    input.user.name in mfa_authenticated_users
}

mfa_authenticated_users contains username if {
    # These should not count as MFA
    insecure_methods := {"email"}

    some user in data.users
    mfa_methods := {method | some method in user.authentication.methods} - insecure_methods

    count(mfa_methods) > 1
    username := user.name
}
```

**Notes / Exceptions**

Does not apply if ordering is of importance, or duplicate values should be allowed. For those cases, use array
comprehensions.

### Avoid prefixing rules and functions with `get_` or `list_`

Since Rego evaluation is generally free of side effects, any rule or function is essentially a "getter". Adding a
`get_` prefix to a rule or function (like `get_resources`) thus adds little of value compared to just naming it
`resources`. Additionally, the type and return value of the rule should serve to tell whether a rule might return a
single value (i.e. a complete rule) or a collection (a partial rule).

**Avoid**
```rego
get_first_name(user) := split(user.name, " ")[0]

# Partial rule, so a set of users is to be expected
list_developers contains user if {
    some user in data.application.users
    user.type == "developer"
}
```

**Prefer**
```rego
# "get" is implied
first_name(user) := split(user.name, " ")[0]

# Partial rule, so a set of users is to be expected
developers contains user if {
    some user in data.application.users
    user.type == "developer"
}
```

**Notes / Exceptions**

Using `is_`, or `has_` for boolean helper functions, like `is_admin(user)` may be easier to comprehend than
`admin(user)`.

:::tip
You can lint for this recommendation using the [`avoid-get-and-list-prefix`](/projects/regal/rules/style/avoid-get-and-list-prefix)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

### Prefer unconditional assignment in rule head over rule body

Rules that return values unconditionally should place the assignment directly in the rule head, as doing so
in the rule body adds unnecessary noise.

**Avoid**
```rego
full_name := name {
    name := concat(", ", [input.first_name, input.last_name])
}

divide_by_ten(x) := y {
    y := x / 10
}
```

**Prefer**
```rego
full_name := concat(", ", [input.first_name, input.last_name])

divide_by_ten(x) := x / 10
```

:::tip
You can lint for this recommendation using the [`unconditional-assignment`](/projects/regal/rules/style/unconditional-assignment)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

## Variables and Data Types

### Use `in` to check for membership

Using `in` for membership checks clearly communicates intent, and is less prone to errors. This is especially true when
checking if something is _not_ part of a collection.

**Avoid**
```rego
# "Old" way of checking for membership - iteration + comparison
allow {
    "admin" == input.user.roles[_]
}
```

**Prefer**
```rego
allow if "admin" in input.user.roles
```

**Avoid**
```rego
deny contains "Only admin allowed" if not user_is_admin

user_is_admin if {
    "admin" == input.user.roles[_]
}
```

**Prefer**
```rego
deny contains "Only admin allowed" if not "admin" in input.user.roles
```

:::tip
You can lint for this recommendation using the [`use-in-operator`](/projects/regal/rules/idiomatic/use-in-operator)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

### Prefer `some .. in` for iteration

Using the `some` .. `in` construct for iteration removes ambiguity around iteration vs. membership checks, and is
generally more pleasant to read.

**Avoid**
```rego
my_rule if {
    # Are we iterating users over a partial "other_rule" here,
    # or checking if the set contains a user defined elsewhere?
    other_rule[user]
}
```

While this could be alleviated by declaring `some user` before the iteration, we can't take that consideration for
granted when reading code from someone else.

**Avoid**
```rego
# Iterating over array
internal_hosts contains hostname if {
    host := data.network.hosts[_]
    host.internal == true
    hostname := host.name
}

# Iterating over object
public_endpoints contains endpoint if {
    some endpoint
    attributes := endpoints[endpoint]
    attributes.public
}
```

**Prefer**
```rego
internal_hosts contains hostname if {
    some host in data.network.hosts
    host.internal == true
    hostname := host.name
}

# Iterating over object
public_endpoints contains endpoint if {
    some endpoint, attributes in endpoints
    attributes.public
}
```

**Notes / Exceptions**

Using the "old" style of iteration may still be preferable when iterating over deeply nested structures.

```rego
# Building a list of all hostnames from a deeply nested structure

all_hostnames := [hostname | hostname := data.regions[_].networks[_].servers[_].hostname]

# ⬆️ is likely preferable over ⬇️

all_hostnames := [hostname |
    some region in data.regions
    some network in region
    some server in network
    hostname := server.hostname
]
```

:::tip
You can lint for this recommendation using the [`prefer-some-in-iteration`](/projects/regal/rules/style/prefer-some-in-iteration)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

### Use `every` to express FOR ALL

The `every` keyword makes it trivial to describe "for all" type expressions, which previously required the use of
helper rules, or comparing counts of items in the original collection against a filtered one produced by a
comprehension.

**Avoid**
```rego
# Negate result of _any_ match
allow if not any_old_registry

any_old_registry if {
    some container in input.request.object.spec.containers
    startswith(container.image, "old.docker.registry/")
}
```

**Prefer**
```rego
allow if {
    every container in input.request.object.spec.containers {
        not startswith(container.image, "old.docker.registry/")
    }
}
```

**Avoid**
```rego
words := ["always", "arbitrary", "air", "brand", "asphalt"]

all_starts_with_a if {
    starts_with_a := [word |
        some word in words
        startswith(word, "a")
    ]
    count(starts_with_a) == count(words)
}
```

**Prefer**
```rego
words := ["always", "arbitrary", "air", "brand", "asphalt"]

all_starts_with_a if {
    every word in words {
        startswith(word, "a")
    }
}
```

**Notes / Exceptions**

Older versions of OPA used the `all` built-in function to check that all elements of an array had the value `true`.
This function has been deprecated for a long time, and will eventually be removed.

### Don't use unification operator for assignment or comparison

The [unification](https://www.openpolicyagent.org/docs/policy-language#unification-) operator (`=`) allows you
to combine assignment and comparison. While this is useful in a few specific cases (see "Notes / Exceptions" below),
using the assignment operator (`:=`) for assignment, and the comparison operator (`==`) for comparison, is almost always
preferable. Separating assignment from comparison clearly demonstrates intent, and removes the ambiguity around scope
associated with unification.

**Avoid**
```rego
# Top level assignment using unification operator
roles = input.user.roles

allow if {
    # Unification operator - used for assignment to `username` variable or for
    # comparing to a `username` variable or rule defined elsewhere? Who knows.
    username = input.user.name

    # ...
}

allow if {
    # Unification operator used for comparison
    input.request.method = "GET"
}

allow if {
    some user
    input.request.path = ["users", user]
    input.request.user == user
}
```

**Prefer**
```rego
# Top level assignment using assignment operator
roles := input.user.roles

allow if {
    # Assignment operator used for assignment - no ambiguity around
    # intent, or variable scope
    username := input.user.name

    # ... do something with username
}

allow if {
    # Comparison operator used for comparison
    input.request.method == "GET"
}

allow if {
    input.request.path == ["users", input.request.user]
}
```

**Notes / Exceptions**

Unification was used extensively in older versions of OPA, and following that, in the policy examples provided in
the OPA documentation, blogs, and elsewhere. With the assignment and comparison operators now available for use in
any context, there are generally few reasons to use the unification operator in modern Rego.

One notable exception is when matching for example, the path of a request (as presented in array form),
where you'll want to do both comparison and assignment to variables from the path components:

```rego
# Using unification - compact but clear
router {
    some user_id, podcast_id
    ["users", user_id, "podcasts", podcast_id] = input.request.path

    # .. do something with user_id, podcast_id
}

# Using comparison + assignment - arguably messier
router {
    input.request_path[0] == "users"
    input.request_path[2] == "podcasts"

    user_id := input.request_path[1]
    podcast_id := input.request_path[3]

    # .. do something with user_id, podcast_id
}
```

#### Related Resources

- [Strict-mode to phase-out the "single =" operator](https://github.com/open-policy-agent/opa/issues/4688)
- [OPA fmt 2.0](https://github.com/open-policy-agent/opa/issues/4508)

:::tip
You can lint for this recommendation using the [`use-assignment-operator`](/projects/regal/rules/style/use-assignment-operator)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

### Don't use undeclared variables

Using undeclared variables (i.e. not declared using `some` or `:=`) makes it harder to understand what's going on
in a rule, and introduces ambiguities around scope.

**Avoid**
```rego
messages contains message if {
    message := input.topics[topic].body
}
```

**Prefer**
```rego
messages contains message if {
    some topic
    message := input.topics[topic].body
}

# Alternatively
messages contains message if {
    some topic in input.topics
    message := topic.body
}

# or

messages contains message if {
    message := input.topics[_].body
}
```

:::tip
You can lint for this recommendation using the [`use-some-for-output-vars`](/projects/regal/rules/idiomatic/use-some-for-output-vars)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

### Prefer sets over arrays (where applicable)

For any _unordered_ sequence of _unique_ values, prefer to use
[sets](https://www.openpolicyagent.org/docs/latest/policy-reference/#sets) over
[arrays](https://www.openpolicyagent.org/docs/latest/policy-reference/#arrays).

This is almost always the case for common policy data like **roles** and **permissions**.
For any applicable sequence of values, sets have the following benefits over arrays:

- Clearly communicate uniqueness and non-ordered characteristics
- Performance: set lookups are O(1) while array lookups are O(n)
- Powerful [set operations](https://www.openpolicyagent.org/docs/latest/policy-reference/#sets-2) available

**Avoid**
```rego
required_roles := ["accountant", "reports-writer"]
provided_roles := [role | some role in input.user.roles]

allow if {
    every required_role in required_roles {
        required_role in provided_roles
    }
}
```

**Prefer**
```rego
required_roles := {"accountant", "reports-writer"}
provided_roles := {role | some role in input.user.roles}

allow if {
    every required_role in required_roles {
        required_role in provided_roles
    }
}
```

**Prefer**
```rego
# Alternatively, use set intersection
allow if {
    required_roles & provided_roles == required_roles
}
```

#### Related Resources

- [Five things you didn't know about OPA](https://www.styra.com/blog/five-things-you-didnt-know-about-opa/).

## Functions

### Prefer using arguments over `input`, `data` or rule references

What separates functions from rules is that they accept _arguments_. While a function too may reference anything from
`input`, `data` or other rules declared in a policy, these references create dependencies that aren't obvious simply by
checking the function signature, and it makes it harder to reuse that function in other contexts. Additionally,
functions that only depend on their arguments are easier to test standalone.

**Avoid**
```rego
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
# Depends only on function arguments
is_preferred_login_method(method, user, all_login_methods) if {
    preferred_login_methods := {login_method |
        some login_method in all_login_methods
        login_method in user.login_methods
    }
    method in preferred_login_methods
}
```

### Avoid using the last argument for the return value

Older Rego policies sometimes contain an unusual way to declare where the return value of a function call should be
stored — the last argument of the function. True to it's
[Datalog](https://www.openpolicyagent.org/docs/policy-language#what-is-rego) roots, return values may be stored
either using assignment (i.e. `:=`) or by appending a variable name to the argument list of a function. These two
expressions are thus equivalent:

**Avoid**
```rego
first_a := i if {
    indexof("answer", "a", i)
}
```

**Prefer**
```rego
first_a := i if {
    i := indexof("answer", "a")
}
```

While the first form is valid, it is almost guaranteed to confuse developers coming from the most common programming
languages. Again, optimize for readability!

:::tip
You can lint for this recommendation using the [`function-arg-return`](/projects/regal/rules/style/function-arg-return)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

## Regex

### Use raw strings for regex patterns

[Raw strings](https://www.openpolicyagent.org/docs/policy-language/#strings) are interpreted literally, allowing
you to avoid having to escape special characters like `\` in your regex patterns.

**Avoid**
```rego
all_digits if {
    regex.match("[\\d]+", "12345")
}
```

**Prefer**
```rego
all_digits if {
    regex.match(`[\d]+`, "12345")
}
```

:::tip
You can lint for this recommendation using the [`non-raw-regex-pattern`](/projects/regal/rules/idiomatic/non-raw-regex-pattern)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

## Packages

### Package name should match file location

When naming packages, the package name should reflect the file location. This
makes the package implementation easier to find when looking up from elsewhere
in a project as well.

When choosing to follow this recommendation, there are two options:

- **Matching the directory and filename**
  - Pros: Reduced nesting for simple policies.
  - Cons: Large packages can become unwieldy in long files.
- **Matching the directory only**
  - Pros: Large packages can be broken into many files.
  - Cons: Exception needed to co-locate test files (i.e. `package foo_test`
    should still be in `foo/`).

Either is acceptable, just remember to use the same convention throughout
your project.

#### Matching the directory and filename

**Avoid**
```rego
# foo/bar.rego
package bar.foo

# ...
```

**Prefer**
```rego
# foo/bar.rego
package foo.bar

# ...
```

#### Matching the directory only

**Avoid**
```rego
# foo/bar.rego
package baz

# ...
```

**Prefer**
```rego
# foo/bar.rego
package foo

# ...
```

## Imports

### Prefer importing packages over rules and functions

Importing packages rather than specific rules and functions allows you to reference them by the package name, making it
obvious where the rule or function was declared. Additionally, well-named packages help provide context to assertions.

**Avoid**
```rego
import data.user.is_admin

allow if is_admin
```

**Prefer**
```rego
import data.user

allow if user.is_admin
```

:::tip
You can lint for this recommendation using the [`prefer-package-imports`](/projects/regal/rules/imports/prefer-package-imports)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

### Avoid importing `input`

While importing attributes from the global `input` variable might eliminate some levels of nesting, it makes the origin
of the attribute(s) less apparent. Clearly differentiating `input` and `data` from values, functions, and rules
defined inside of the same package helps in making things _obvious_, and few things beat obviousness!

**Avoid**
```rego
import input.request.context.user

# ... many lines of code later

fin_dept if {
    # where does "user" come from?
    contains(user.department, "finance")
}
```

**Prefer**
```rego
fin_dept if {
    contains(input.request.context.user.department, "finance")
}
```

**Prefer**
```rego
fin_dept if {
    # Alternatively, assign an intermediate variable close to where it's referenced
    user := input.request.context.user
    contains(user.department, "finance")
}
```

**Notes / Exceptions**

In some contexts, the source of data is obvious even when imported and/or renamed. A common practice is
to rename `input` in Terraform policies for example, either via `import` or a new top-level variable.

```rego
import input as tfplan

violations contains message if {
    # still obvious where "tfplan" comes from, perhaps even more so — this is generally acceptable
    some change in tfplan.resource_changes
    # ...
}
```

:::tip
You can lint for this recommendation using the [`avoid-importing-input`](/projects/regal/rules/imports/avoid-importing-input)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

## Older Advice

Advice placed here was valid at the time, but has since been replaced by new recommendations. Kept here for the sake
of completeness, and to provide context for older policies.

### Use explicit imports for future keywords

**With the introduction of the `import rego.v1` construct in OPA v0.59.0, this is no longer needed**

In order to evolve the Rego language without breaking existing policies, many new features require importing
["future" keywords](https://www.openpolicyagent.org/docs/policy-language#future-keywords), like `contains`,
`every`, `if` and `in`. While it might seem convenient to use the "catch-all" form of `import future.keywords` to
import all of the future keywords, this construct risks breaking your policies when new keywords are introduced, and
their names happen to collide with names you've used for variables or rules.

**Avoid**
```rego
import future.keywords

severe_violations contains violation if {
    some violation in input.violations
    violation.severity > 5
}
```

**Prefer**
```rego
import future.keywords.contains
import future.keywords.if
import future.keywords.in

severe_violations contains violation if {
    some violation in input.violations
    violation.severity > 5
}
```

**Tip**: Importing the `every` keyword implicitly imports `in` as well, as it is required by the `every` construct.
Leaving out the import of `in` when `every` is imported is considered okay.

:::tip
You can lint for this recommendation using the [`implicit-future-keywords`](/projects/regal/rules/imports/implicit-future-keywords)
Regal rule. Get started with [Regal, the Rego linter](/projects/regal).
:::

---

## Contributing

This document is meant to reflect the style preferences and best practices as compiled by the OPA community. As such,
we welcome contributions from any of its members. Since most of the topics in a guide like this are likely subject to
discussion, please open an issue, and allow some time for people to comment, before opening a PR.

If you'd like to add or remove items for your own company, team or project, forking this repo is highly encouraged!
