---
sidebar_position: 8
sidebar_label: Rego Version
---

<head>
  <title>Rego Version | Regal</title>
</head>

# Configuring Rego Version

From OPA 1.0 and onwards, it is no longer necessary to include `import rego.v1` in your policies in order to use
keywords like `if` and `contains`. Since Regal works with with both 1.0+ policies and older versions of Rego, the linter
will first try to parse a policy as 1.0 and if that fails, parse using "v0" rules. This process isn't 100% foolproof,
as some policies are valid in both versions. Additionally, parsing the same file multiple times adds some overhead that
can be skipped if the version is known beforehand. To help Regal determine (and enforce) the version of your policies,
the `rego-version` attribute can be set in the `project` configuration:

```yaml
project:
  # Rego version 1.0, set to 0 for pre-1.0 policies
  rego-version: 1
```

It is also possible to set the Rego version for individual project roots (see below for more information):

```yaml
project:
  roots:
  - path: lib/legacy
    rego-version: 0
  - path: main
    rego-version: 1
```

Additionally, Regal will scan the project for any `.manifest` files, and use any `rego_version` found in the manifest
for all policies under that directory.

Note: the `rego-version` attribute in the configuration file has precedence over `rego_version` found in manifest files.
