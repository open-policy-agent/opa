# missing-metadata

**Summary**: Package or rule missing metadata

**Category**: Custom

**Avoid**
```rego
package acmecorp.authz

authorized_users contains user if {
    # logic to determine authorized users
}
```

**Prefer**
```rego
# METADATA
# description: The `acmecorp.authz` module provides authorization logic for the AcmeCorp application.
package acmecorp.authz

# METADATA
# description: Provides a set of all authorized users given the conditions in `input`.
# scope: document
authorized_users contains user if {
    # logic to determine authorized users
}
```

## Rationale

Using metadata annotations is a great way to document your policies, for both yourself and others. While using metadata
annotations _everywhere_ might be overkill for many projects, it should absolutely be considered for libraries, or
policies that target a larger audience.

## Exceptions

Rules and functions with an underscore prefix in their name are commonly used to denote that they are intended
to be used internally (i.e. within the same file) only, and while metadata occasionally help document these,
they are not part of the "public API". The `missing-metadata` thus excludes these from the metadata requirement.

It is also possible to configure your own exceptions for both package and rule paths. See the configuration options
below.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  custom:
    missing-metadata:
      # note that all rules in the "custom" category are disabled by default
      # (i.e. level "ignore"), so make sure to set the level to "error" if you
      # want this enabled!
      #
      # one of "error", "warning", "ignore"
      level: error
      # package path pattern(s) to exclude from the requirement
      # defaults to no exclusions
      except-package-path-pattern: ^internal\.*
      # rule path pattern(s) to exclude from the requirement
      # defaults to no exclusions
      except-rule-path-pattern: \.report$
      # you might also want to exclude files based on their name,
      # like e.g. tests:
      ignore:
        files:
          - "*_test.rego"
```

## Related Resources

- OPA Docs: [Metadata](https://www.openpolicyagent.org/docs/policy-language/#metadata)
- OPA Docs: [Annotations](https://www.openpolicyagent.org/docs/policy-language/#annotations)
- Rego Style Guide: [Use Metadata Annotations](https://github.com/StyraInc/rego-style-guide)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/custom/missing-metadata/missing_metadata.rego)
