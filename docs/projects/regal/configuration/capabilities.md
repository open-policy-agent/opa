---
sidebar_position: 5
---


# Capabilities

By default, Regal will lint your policies using the
[capabilities](https://www.openpolicyagent.org/docs/deployments/#capabilities) of the latest version of OPA
known to Regal (i.e. the latest version of OPA at the time Regal was released). Sometimes you might want to tell Regal
that some rules aren't applicable to your project (yet!). As an example, if you're running OPA v0.46.0, you likely won't
be helped by the [custom-has-key](https://openpolicyagent.org/projects/regal/rules/idiomatic/custom-has-key-construct) rule, as it
suggests using the `object.keys` built-in function introduced in OPA v0.47.0. The opposite could also be true —
sometimes new versions of OPA will invalidate rules that applied to older versions. An example of this is the upcoming
introduction of `import rego.v1`, which will make
[implicit-future-keywords](https://openpolicyagent.org/projects/regal/rules/imports/implicit-future-keywords) obsolete, as importing
`rego.v1` automatically imports all "future" functions.

Capabilities help you tell Regal which features to take into account, and rules with dependencies to capabilities
not available or not applicable in the given version will be skipped.

If you'd like to target a specific version of OPA, you can include a `capabilities` section in your configuration,
providing either a specific `version` of an `engine` (currently only `opa` supported):

```yaml
capabilities:
  from:
    engine: opa
    version: v0.58.0
```

You can also choose to import capabilities from a file:

```yaml
capabilities:
  from:
    file: build/capabilities.json
```

You can use `plus` and `minus` to add or remove built-in functions from the given set of capabilities:

```yaml
capabilities:
  from:
    engine: opa
    version: v0.58.0
  minus:
    builtins:
    # exclude rules that depend on the http.send built-in function
    - name: http.send
  plus:
    builtins:
    # make Regal aware of a custom "ldap.query" function
    - name: ldap.query
      type: function
      decl:
        args:
        - type: string
      result:
        type: object
```

## Loading Capabilities from URLs

Starting with Regal version v0.26.0, Regal can load capabilities from URLs with the `http`, or `https` schemes using
the `capabilities.from.url` config key. For example, to load capabilities from `https://example.org/capabilities.json`,
this configuration could be used:

```yaml
capabilities:
  from:
    url: https://example.org/capabilities.json
```

## Supported Engines

Regal includes capabilities files for the following engines:

| Engine | Website                                                         | Description          |
| ------ | --------------------------------------------------------------- | -------------------- |
| `opa`  | [OPA website](https://www.openpolicyagent.org/)                 | Open Policy Agent    |
| `eopa` | [Enterprise OPA website](https://www.styra.com/enterprise-opa/) | Styra Enterprise OPA |
