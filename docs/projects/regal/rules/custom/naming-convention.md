# naming-convention

**Summary**: Naming convention violation

**Category**: Custom

## Description

This custom rule allows teams and organizations to define their own naming conventions for their Rego projects, without
having to write custom linter policies. Naming conventions are simply defined in the Regal configuration file using
regex patterns.

Regal can enforce naming conventions for:

- Package names
- Rule names
- Function names
- Variable names

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  custom:
    naming-convention:
      # note that all rules in the "custom" category are disabled by default
      # (i.e. level "ignore") as some configuration needs to be provided by
      # the user (i.e. you!) in order for them to be useful.
      #
      # one of "error", "warning", "ignore"
      level: error
      conventions:
          # allow only "private" rules and functions, i.e. those starting with
          # underscore, or rules named "deny" or "allow"
        - pattern: '^_[a-z]+$|^deny$|^allow$'
          # one of "package", "rule", "function", "variable"
          targets:
            - rule
            - function
        # any number of naming rules may be added
        # package names must start with "acmecorp" or "system"
        - pattern: '^acmecorp|^system'
          targets:
            - package
```

**Note:** In order to avoid characters accidentally getting escaped, always use single quotes to encode your regex
patterns. Additionally, you'll most often want to include anchors for the start and end of the string (`^` and `$`) in
your patterns, or else your pattern might accidentally match only parts of the name rather than the whole name.

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/custom/naming-convention/naming_convention.rego)
