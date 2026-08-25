---
sidebar_label: import
title: 'Rego Keyword Examples: import'
---

In Rego, the `import` keyword is used to include references in the current file
from other places, namely other Rego packages. However, the `import` keyword is
also used to change the Rego syntax available in the current file. This case is covered first.

## Importing packages

Most importantly, the `import` keyword is used to make the rules defined in one
package, available in another.

Consider a package, `package1`, that defines a rule `name` like this:

```rego
package package1

name := "World"
```

<RunSnippet id="package1.rego"/>

To use the `name` rule in another package, `package2`, write something like this:

```rego
package package2

// highlight-next-line
output := sprintf("Hello, %v", [data.package1.name])
```

<RunSnippet
  files="#package1.rego"
  command="data.package2"/>

While this will work, it's better to use an import at the top of the file to
save repetition and declare the dependency upfront for readers of the policy.
The same result can be achieved like this:

```rego
package package2

// highlight-next-line
import data.package1

output := sprintf("Hello, %v", [package1.name])
```

<RunSnippet
  files="#package1.rego"
  command="data.package2"/>

Sometimes, using the package name for an import many times throughout a file can
be too verbose. In such cases, it can be helpful to use an alias like this:

```rego
package package2

// highlight-next-line
import data.package1 as p1

output := sprintf("Hello, %v", [p1.name])
```

<RunSnippet
  files="#package1.rego"
  command="data.package2"/>

## Importing Future Keywords

The `in`, `every`, `if`, `contains`, and `not` (semantic update) keywords
have been introduced to the Rego language over time, and in order to prevent
them from breaking policies that existed before their introduction, an opt-in mechanism
has been necessary. The `future.keywords.*` imports facilitate this
opt-in mechanism. With the release of OPA v1.x, the `in`, `every`, `if`, and `contains`
keywords have become a standard part of the Rego language, and no longer require an import.
The `not` keyword has always been a standard part of the Rego language, but has since its introduction
received a semantic update that requires author opt-in through importing `future.keywords.not`.

### Importing `future.keywords.not`

[import future.keywords.not](./not) enables the `not` body syntax
(`not { ... }`) and implicit body wrapping for single-expression negation.
This import is independent of the [rego.v1 import](#importing-regov1).

:::important
The `future.keywords.not` import fixes a long-standing semantic issue with negation in Rego.
Read more about it in the [Improved Negation Semantics](./not#improved-negation-semantics) section of the `not` keyword overview.
:::

### Importing `future.keywords.and`

[import future.keywords.and](./logical#and) enables the `and` logical operand
(`x and y`).
This import is independent of the [rego.v1 import](#importing-regov1).

### Importing `future.keywords.or`

[import future.keywords.or](./logical#or) enables the `or` logical operand
(`x or y`).
This import is independent of the [rego.v1 import](#importing-regov1).

## Importing `rego.v1`

In [OPA 1.0](https://www.openpolicyagent.org/docs/v0-upgrade) a number of
previously optional keywords are required. These settings for the Rego
language is available in pre-1.0 versions using the `import` keyword. The two
files that follow are equivalent.

```rego title="Pre 1.0"
package example

// highlight-next-line
import rego.v1

allow if count(deny) == 0

deny contains "not admin" if input.user.role != "admin"
```

```rego title="Post 1.0"
package example

allow if count(deny) == 0

deny contains "not admin" if input.user.role != "admin"
```

## Further Reading

- Read about [imports](/docs/policy-language/#imports) in the documentation.
- Make sure you're using `import` correctly with Regal's [import rules](/projects/regal/rules/imports).
