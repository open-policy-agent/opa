<!-- markdownlint-disable MD041 -->

`is_string` (and related `is_*` built-in functions) test a value's type and
return a boolean. In Kubernetes resources, annotation and label values must be
strings, but unquoted YAML values like `public-egress: true` parse as booleans in
JSON. Use `is_string` to validate that all annotation values are strings. For
validating full schemas, see [schema validation functions](/docs/policy-reference/builtins/json#schema-validation).
