---
sidebar_position: 3
---

<head>
  <title>Fixing Violations | Regal</title>
</head>

# Fixing Violations

For each violation Regal is able to detect, there is a documentation page explaining the issue in detail and how to fix
it. For example, here's the one for the
[`prefer-some-in-iteration`](https://www.openpolicyagent.org/projects/regal/rules/style/prefer-some-in-iteration) rule.

Some rules are **automatically** fixable, meaning that Regal can fix the violation for you. Note that while most fixes
will make minor changes to the code, some fixes make more significant modifications. As an example, the
[directory-package-mismatch](https://www.openpolicyagent.org/projects/regal/rules/idiomatic/directory-package-mismatch) fix will
automatically move a file to its idiomatic location based on the package path declared in the file.

:::tip
Before you make any automated fixes, make sure to commit or stash any other changes you may have done in your project!
That way you can easily revert the changes if anything goes wrong, and this allows you to review the diff before
committing the fixes.
:::

Currently, the following rules are automatically fixable:

- [opa-fmt](https://www.openpolicyagent.org/projects/regal/rules/style/opa-fmt)
- [non-raw-regex-pattern](https://www.openpolicyagent.org/projects/regal/rules/idiomatic/non-raw-regex-pattern)
- [use-assignment-operator](https://www.openpolicyagent.org/projects/regal/rules/style/use-assignment-operator)
- [no-whitespace-comment](https://www.openpolicyagent.org/projects/regal/rules/style/no-whitespace-comment)
- [directory-package-mismatch](https://www.openpolicyagent.org/projects/regal/rules/idiomatic/directory-package-mismatch)
- [use-rego-v1](https://www.openpolicyagent.org/projects/regal/rules/imports/use-rego-v1) (v0 Rego only)

So, how do you go on about automatically fixing reported violations?

## The `regal fix` Command

The first method is to use `regal fix` from the command line. This command can be seen as the remediating counterpart
to `regal lint`, and most configuration options are the same. For example, a linter rule configured to be ignored by
the linter will also be ignored by the fixer. Typing `regal fix --help` will show you the available options, as well
as all the supported "fixers" for your version of Regal. Lets's see what automatically fixing violations looks like!

**Example**: fixing all files in the `bundle` directory:

```shell
> regal fix bundle
3 fixes applied:
In project root: /Users/john/projects/authz/bundle

lib/roles.rego:
- use-rego-v1

policy.rego -> main/policy.rego:
- directory-package-mismatch
- no-whitespace-comment
```

In the example above, Regal made fixes corresponding to the linter rules `use-rego-v1`, `directory-package-mismatch`,
and `no-whitespace-comment` in `lib/roles.rego` and `policy.rego`. While the number of fixes applied was reported as 3,
the number of _violations_ fixed could of course have been higher, as e.g. the `no-whitespace-comment` rule might have
been violated in multiple places in `policy.rego`. Note also how one of the fixes (`directory-package-mismatch`)
involved **moving** `policy.rego` to `main/policy.rego`, as that rule requires the file to be in a directory structure
matching its package path (`package main`).

### Project Roots

All paths are relative to its closest **project root**, as reported in the second line of the output. Most policy
projects will likely only have one "root", which is the workspace directory itself. More complex projects may however
host multiple roots inside the workspace, and defining these roots — either by configuration, or by `.manifest` files —
will in some cases (like the previously mentioned `directory-package-mismatch` fix) help Regal better apply the correct
fixes. See the documentation on [project roots](https://www.openpolicyagent.org/projects/regal#project-roots) for more information.

### Dry Run

Using the `--dry-run` flag is a great way to see what changes will be made without actually applying them. Following our
example from above, adding the `--dry-run` flag to `regal fix bundle` would have told us beforehand what changes we
should expect to see. Make it a habit to dry-run your fixes before applying them, and make sure you've committed any
other changes before running the fixer!

## Fixing Violations in Editors

In addition to the `regal fix` command, users integrating Regal with their editors can fix violations directly as
they are reported in the file being edited. This is done by means of Code Actions, which commonly displays a lightbulb
icon next to where a violation occurs. Clicking on the lightbulb will show a list of available actions, which in Regal's
case maps directly to the available fix for the violation reported (if any).

Example of code action in VS Code, where available fixes can be listed either by clicking the lightbulb icon to the
right, or by clicking "Quick Fix..." in the tooltip window:

<img
src={require('./assets/lsp/code_action_show.png').default}
alt="Screenshot of code action displayed in VS Code"/>

Example of suggested Code Action for the
[use-assignment-operator](https://www.openpolicyagent.org/projects/regal/rules/style/use-assignment-operator) rule. Click to fix!

<img
src={require('./assets/lsp/code_action_fix.png').default}
alt="Screenshot of code action fix suggestion displayed in VS Code"/>

### Limitations

Compared to `regal fix`, automatically fixing violations in editors has some limitations:

- Normally works on one file at a time, not entire directories
- No ability to dry-run a fix, but on the other hand, the editor's **Undo** feature will let you easily revert any
  changes made.

:::tip
If you're curious about using Regal to fix problems directly in your editor, see the docs on editor support
[here](https://www.openpolicyagent.org/projects/regal/editor-support) to learn more!
:::
