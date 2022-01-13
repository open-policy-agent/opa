---
title: Compiler Strict Mode
kind: misc
weight: 9
---

The Rego compiler supports `strict mode`, where additional constraints and safety checks are enforced during compilation.
Compiler rules that will be enforced by future versions of OPA, but will be a breaking change once introduced, are incubated in strict mode. 
This creates an opportunity for users to verify that their policies are compatible with the next version of OPA before upgrading. 

Compiler Strict mode is supported by the `check` command, and can be enabled through the `-S` flag.

```
-S, --strict enable compiler strict mode
```

## Strict Mode Rules

Name | Description | Enforced in OPA version
--- | --- | ---
Duplicate imports | Duplicate [imports](../policy-language/#imports), where one import shadows another, are prohibited. | 1.0