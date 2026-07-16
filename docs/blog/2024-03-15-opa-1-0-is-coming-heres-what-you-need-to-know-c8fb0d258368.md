---
title: "OPA 1.0 is coming. Here's what you need to know."
authors: ["anderseknert"]
date: 2024-03-15
slug: opa-1-0-is-coming-heres-what-you-need-to-know-c8fb0d258368
---

![Open Policy Agent Logo with "1.0" written below](/img/blog/opa-1-0-is-coming-heres-what-you-need-to-know-c8fb0d258368/1.png)
_Towards Open Policy Agent 1.0_

December 28th marked the 8th anniversary of the first commit in the Open Policy Agent project. 5000+ commits from more than 400 contributors later, we're starting to prepare for OPA 1.0.

Following the rules of semantic versioning, one would be excused to think of 1.0 as the first "stable" version. That's not really the case for this project. Since the first public releases of OPA, great care has been taken to ensure new changes don't break existing policy. Thousands of organizations have come to rely on OPA for policy enforcement across the whole stack — often for critical production use cases.

While many **features** have been added to OPA and the Rego language since 2015, few have ever been removed. All this means that almost any policy written eight, five or three years ago still evaluates using the very latest version OPA, just as it did when it was written! But to keep adding features without ever being able to remove things that might not have worked out as we imagined — or simply didn't age well — also comes at a price. An increased cost of maintenance for sure, but more importantly, old ideas, language constructs and built-in functions all add to the cost of learning Rego, compared to having policy look and act consistently using only modern equivalent features.

With OPA 1.0, we're aiming to fix this.

## TL;DR

If you have a busy day ahead and want to get right to something actionable — here's what you can do. From OPA [v0.59.0](https://github.com/open-policy-agent/opa/releases/tag/v0.59.0) and onwards, you can start to prepare for the changes in the upcoming 1.0 release following these steps:

- Use import rego.v1 in each of your Rego files. This replaces all future.keywords imports from previous versions, and is all you need to import until OPA 1.0. Use opa fmt --rego-v1 to format your policy with automatic additions of OPA v1.0 constructs, like if, contains and more. This will also replace any future.keywords imports with the rego.v1 import.
- Use opa check --rego-v1 to ensure your policy is compatible with "Rego 1.0" mode.

We'll get back to practical concerns by the end of the blog, but before that, let's see what changes are planned for the first major OPA version.

### Changes to Rego coming in OPA 1.0

**Note: Below follows a non-comprehensive list subject to change.** While these features are planned — and most of them even implemented already! — updates may be made before the final 1.0 release. I'll do my best to keep this blog post up to date, as we will with the OPA 1.0 documentation.

#### The future is now — no more import future.keywords

Several keywords (in, every, _if_ and contains*)* have been added to OPA since the start. Introducing new keywords means there's always a risk that existing policy might break in case identifiers, like rule names or variables, have been named in a way that clashes with the new keywords. In order to prevent this, access to these keywords have required an import of future.keywords. We now live in that future.

![That was a problem for future me, and now I am future me.](/img/blog/opa-1-0-is-coming-heres-what-you-need-to-know-c8fb0d258368/2.jpeg)

OPA v1.0 makes import future.keywords a no-op, as all keywords are now made available everywhere. In the time before OPA 1.0 is released, the new rego.v1 should be used in place of future.keywords imports.

#### The if keyword made mandatory

The if keyword helps explain the "inverted if … then" nature of Rego rules, and makes rules easier to read. Additionally, any rule body with only a single expression, like:

```rego
allow {
    "admin" in user.roles
}
```

May with the help of if be expressed as a one-liner, with the curly brackets removed:

```rego
allow if "admin" in user.roles
```

OPA 1.0 makes if a natural — and mandatory — part of every rule's anatomy.

**Tip:** Use the opa fmt --rego-v1 utility in OPA v0.59.0+ to automatically rewrite all of your rules with if added to the rule head.

#### The contains keyword made mandatory

The contains keyword helps express multi-value (or as they're often called, partial) rules — i.e. rules that build a _set_ of values. It also helps avoid ambiguities around certain classes of rules, like the fairly recently introduced "nested" rule type:

```rego
users.names contains name if {
    # ...
    name := sprintf("%s, %s", [first_name, last_name])
}
```

The contains keyword was already mandatory for nested rules. OPA v1.0 makes the use of this consistent across all multi-value rules by making contains a requirement.

**Tip:** Use the opa fmt --rego-v1 utility in OPA v0.59.0+ to automatically rewrite all of your multi-value rules with contains added to the rule head.

#### Strict mode made (mostly) the default

Most of the rules that have existed in OPA [strict mode](https://www.openpolicyagent.org/docs/latest/policy-language/#strict-mode) will be made the default in OPA 1.0. This will help users catch mistakes early, and have them fixed right away. If you have been running opa check --strict as part of your policy build pipeline, you're already in the clear here.

The rules from strict mode that will be made default in OPA 1.0 are:

#### No duplicate imports

Duplicate imports should realistically not be a problem in any repo, and the fix is simply to have them removed.

```rego
package policy

import data.authz
import data.authz # this is now an error
```

#### No deprecated built-in functions

Deprecated built-in functions will be removed in OPA 1.0, and most of them are trivial to replace using a single line of Rego, or a different built-in function.

```rego
package policy

# simply change to use `true in {input.foo, input.bar}`
# using `in` additionally has the benefit that it can be used
# to check for any type of value, and not just boolean "true"
a := any([input.foo, input.bar])

# change to use `every` keyword, e.g.
# every x in [input.foo, input.bar] {
#     x == true
# }
# just like `in` may be used for much more, `every` can be
# used to evaluate complex expressions
e := all([input.foo, input.bar])

# simply use the minus (`-`) operator instead, e.g.
# s3 := s1 - s2
s3 := set_diff(s1, s2)

# simply change to use regex.match instead
r := re_match(..)

# simply change to use net.cidr_intersects
n := net.cidr_overlap(..)

# cast_array, cast_set, cast_string, cast_boolean, cast_null, cast_object
# use the "is_x" equivalent built-in function in their place
s := is_string("yes")
```

#### input and data now reserved keywords

OPA 1.0 prohibits the use of input or data as identifiers:

```rego
# this is not allowed in OPA 1.0
input := "overloaded"

# and neither is this
data := {}
```

Overloading input has mostly been common in tests. Do note that with input as `{..}` remains valid. If you're using assignment to input however, (`input := {..}`), you'll just need to change the name to something like inp instead.

### Other changes coming in OPA 1.0

See the [1.0 tag](https://github.com/open-policy-agent/opa/issues?q=is%3Aissue+label%3A1.0+) in the OPA backlog for a list of all issues related to OPA 1.0. Do note though that not all issues marked 1.0 might be picked for inclusion, and new issues may pop up before the release!

Other notable changes include:

#### import rego.v1

As previously mentioned — beginning with OPA v0.59.0 a new handy import to help with the 1.0 transition is made available. By adding import rego.v1 to a Rego policy, you can tell OPA to treat the policy just as it will handle it once version 1.0 is released.

- Since rego.v1 implies all the (no longer) future keywords, the importing future.keywords is no longer needed when import rego.v1 is present, and will in fact be an error.
- Just as in OPA 1.0, the use of if and contains will be enforced
- The strict mode requirements brought in OPA 1.0 will be checked automatically

#### Bind server to localhost interface by default

In OPA 1.0, the server will bind to the localhost interface by default, and not 0.0.0.0 (all interfaces). This change is needed in order to avoid accidentally exposing OPA to the internet, which while uncommon (as OPA normally runs behind firewalls and gateways) still happens, and we should aim to provide a secure default. Should you still want to bind against 0.0.0.0, or some other interface, you can use the --addr flag of the opa run command, like opa run --server --addr 0.0.0.0:8181. The impact of this change is expected to be small, but good to keep in mind.

### Documentation

The OPA docs have been updated to cover much of what's mentioned in this blog in greater detail. It also covers more of the technical reasons some of these changes are needed. See the docs on [OPA 1.0](https://www.openpolicyagent.org/docs/latest/opa-1/) for more information.

### How to prepare

As we covered in the TL;DR section, we're providing a number of tools to help with the transition starting from OPA v0.59.0 already. These tools will likely be extended and improved in following releases, but starting to use them today will ensure as smooth transition as possible.

#### To summarize

- OPA 1.0 planned for release this year, including some backwards incompatible changes
- Starting now, you should use import rego.v1 in all of your policies (this replaces future.keywords) imports
- Use opa check --rego-v1 for testing compliance against 1.0
- Use opa fmt --rego-v1 to have your Rego code updated for 1.0 compliance
- Run the OPA server with the --v1-compatible flag for OPA 1.0 compliance

Also worth pointing out — following guides like the [Rego Style Guide](https://docs.styra.com/opa/rego-style-guide), and using tools like [Regal](https://docs.styra.com/regal), is an excellent way to ensure not just compliance with future changes to Rego, but that your current policy repo is continuously kept in the best possible condition.

If you have any questions, concerns or would like to provide feedback around the upcoming 1.0 release, or the tools made available to help you transition smoothly — don't hesitate to reach out using any of the below channels:

- The OPA community's [discussion board](https://github.com/open-policy-agent/community/)
- The [OPA Slack](https://slack.openpolicyagent.org/)
- The OPA project's [backlog](https://github.com/open-policy-agent/opa/issues) (to file an issue or feature request)
