---
sidebar_label: OPA 1.0
sidebar_position: 14
---

<head>
  <title>OPA 1.0 | Regal</title>
</head>

# OPA 1.0 and Regal

While we always recommend using the latest version of OPA, we're well aware that there may be situations where — for
one reason or another — that might not be possible. As we want everyone to benefit from Regal, we do our very best to
ensure it works seamlessly with OPA versions both before and after 1.0, and even projects that use a mix of both! While
this should mostly work out of the box and without additional configuration, it's good to be aware of how Regal parses
and lints policies of different versions of Rego, and how you can tell Regal to target only a specific version.

**Note:** This document does not cover the specifics of OPA 1.0, but rather how Regal works with it. If you want to
learn more about what OPA 1.0 is and how to upgrade, see the [related resources](#related-resources) at the bottom of
this page.

## Telling Regal which Rego version to target

While Regal pretty accurately guesses the Rego version of the policies it's linting — and will adapt how it parses and
asseses Rego files accordingly — telling Regal which version to target is always going to produce the most reliable
results — and much faster too! Guessing which Rego version to target often involves multiple passes of parsing, and
as some files are both valid Rego v0 and v1, there will always be some ambiguity. In order to avoid this, our
recommendation is to always provide Regal with the Rego version(s) targeted. This can be done in a couple of ways, and
the precedence of these methods is as listed below:

1. Setting the `rego-version` configuration option under `project.roots` attribute
2. Setting the `rego-version` configuration option under `project` attribute
3. Setting the `rego_version` in a `.manifest` file in any directory (will apply to that directory and any below it)

Note that it is perfectly possible to use different `rego-version`s for different roots of a project:

```yaml
project:
  rego-version: 1
  roots:
  # lib/legacy overriding project version to set version 0
  - path: lib/legacy
    rego-version: 0
  # main directory will inherit version 1 from project
  - path: main
```

See the documentation covering Regal's [configuration](https://www.openpolicyagent.org/projects/regal#configuration) for more information
on [configuring Rego version](https://www.openpolicyagent.org/projects/regal#configuring-rego-version) for your project.

Finally, Regal will automatically parse and lint any file with a `_v0.rego` suffix as Rego v0. This is intended only
for testing and development, where you sometimes may want to try something out using and older Rego version without
configuration. Note that this has lower precedence than Rego versions set by other means, and should not be considered
as anything but a convenience for testing.

## Rules disabled with OPA 1.0

Some linter rules don't really make sense to enforce post OPA 1.0, as they are now either enforced by OPA itself or
otherwise no longer relevant. The following rules are now disabled by default, unless Regal is configured to target
Rego versions before 1.0, or in the case where no configuration is provided, Regal determines that the project being
linted is not yet using OPA 1.0:

- [deprecated-builtin](https://www.openpolicyagent.org/projects/regal/rules/bugs/deprecated-builtin)
- [import-shadows-import](https://www.openpolicyagent.org/projects/regal/rules/imports/import-shadows-import)
- [rule-named-if](https://www.openpolicyagent.org/projects/regal/rules/bugs/rule-named-if)
- [use-contains](https://www.openpolicyagent.org/projects/regal/rules/idiomatic/use-contains)
- [use-if](https://www.openpolicyagent.org/projects/regal/rules/idiomatic/use-if)
- [use-rego-v1](https://www.openpolicyagent.org/projects/regal/rules/imports/use-rego-v1)

Except for the `deprecated-builtin` rule — which is disabled simply because there currently are no deprecated built-ins
in OPA 1.0 — these rules are now enforced automatically by OPA, and so there's no reason for Regal to duplicate that
effort.

## Related Resources

- OPA Docs: [Upgrading to v1.0](https://www.openpolicyagent.org/docs/v0-upgrade/)
- OPA Docs: [v0 Backwards Compatibility](https://www.openpolicyagent.org/docs/v0-compatibility/)
- Styra Blog: [Renovating Rego](https://www.styra.com/blog/renovating-rego/)
- OPA Blog: [OPA 1.0 Is Coming, Here's What You Need to Know](https://blog.openpolicyagent.org/opa-1-0-is-coming-heres-what-you-need-to-know-c8fb0d258368)
- OPA Blog: [Announcing OPA 1.0: A New Standard for Policy as Code](https://blog.openpolicyagent.org/announcing-opa-1-0-a-new-standard-for-policy-as-code-a6d8427ee828)
