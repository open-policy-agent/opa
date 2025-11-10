---
sidebar_position: 7
---


# Project Roots

While many projects consider the project's root directory (in editors often referred to as **workspace**) their
"main" directory for policies, some projects may contain code from other languages, policy "subprojects", or multiple
[bundles](https://www.openpolicyagent.org/docs/management-bundles/). While most of Regal's features work
independently of this — linting, for example, doesn't consider where in a workspace policies are located as long as
those locations aren't [ignored](./ignore-rules) — some features, like automatically
[fixing](https://www.openpolicyagent.org/projects/regal/fixing) violations, benefit from knowing when a project contains multiple roots.

To provide an example, consider the
[directory-package-mismatch](https://www.openpolicyagent.org/projects/regal/rules/idiomatic/directory-package-mismatch) rule, which states
that a file declaring a `package` path like `policy.permissions.users` should also be located in a directory structure
that mirrors that package, i.e. `policy/permissions/users`. When a violation against this rule is reported, the
`regal fix` command, or its equivalent [Code Action](https://www.openpolicyagent.org/projects/regal#regal-language-server) in editors,
may when invoked remediate the issue by moving the file to the correct location.
But where should the `policy/permissions/users` directory _itself_ reside?

Normally, the answer to that question would be the **project**, or **workspace** root. But if the file was found
in a subdirectory containing a **bundle**, the directory naturally belongs under that _bundle's root_ instead. The
`roots` configuration option under the top-level `project` object allows you to tell Regal where these roots are,
and have features like the `directory-package-mismatch` fixer work as you'd expect.

```yaml
project:
  roots:
  - bundle1
  - bundle2
```

The configuration file is not the only way Regal may determine project roots. Other ways include:

- A directory containing a `.manifest` file will automatically be registered as a root
- A directory containing a `.regal` directory will be registered as a root (this is normally the project root)

If a feature that depends on project roots fails to identify any, it will either fail or fall back on the directory
in which the command was run.
