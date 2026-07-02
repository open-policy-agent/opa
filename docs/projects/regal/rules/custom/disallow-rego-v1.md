# disallow-rego-v1

**Summary**: Use of disallowed `import rego v1`

**Category**: Custom

## Rationale

Since OPA v1.0, the `rego.v1` import is effectively a no-op. As such, this rule serves as a way for teams to
keep this import from popping up in code when it is no longer needed. Also, because this import is still
needed to evaluate policy for older OPA versions, this rule serves as a way to ensure older versions of
OPA (any prior to v1.0) are not being used.

## Note

This rule is intended to be enabled for projects that have been configured to target versions of OPA from 1.0
onwards, but Regal does not explicitly check which version of OPA is being targeted for this rule. If working
with older versions of OPA and Rego, you probably don't want to enable this rule.

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/custom/disallow-rego-v1/disallow_rego_v1.rego)
- OPA Docs: [Imports](https://www.openpolicyagent.org/docs/policy-language/#imports)
