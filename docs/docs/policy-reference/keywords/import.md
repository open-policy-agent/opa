---
sidebar_label: import
title: 'Rego Keyword Examples: import'
---

In Rego, the `import` keyword is used to include references in the current file
from other places, namely other Rego packages. However, the `import` keyword is
also used to change the Rego syntax available in the current file. Let's cover
this case first.

## Importing packages

Most importantly, the `import` keyword is used to make the rules defined in one
package, available in another.

Imagine we have a package, `package1`, that defines a rule `name` like this:

```rego
package package1

import rego.v1

name := "World"
```

<RunSnippet id="package1.rego"/>

Now, if we'd like to use the `name` rule in another package, `package2`, we
could do something like this:

```rego
package package2

import rego.v1

// highlight-next-line
output := sprintf("Hello, %v", [data.package1.name])
```

<RunSnippet
  files="#package1.rego"
  command="data.package2"/>

While this will work, it's better to use an import at the top of the file to
save repetition and declare the dependency upfront for readers of the policy.
We can achieve the same result like this:

```rego
package package2

import rego.v1

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

import rego.v1

// highlight-next-line
import data.package1 as p1

output := sprintf("Hello, %v", [p1.name])
```

<RunSnippet
  files="#package1.rego"
  command="data.package2"/>

## Importing `rego.v1`

In [OPA 1.0](https://www.openpolicyagent.org/docs/v0-upgrade) a number of
previously optional keywords will be required. These settings for the Rego
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
