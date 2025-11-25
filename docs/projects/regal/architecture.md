---
sidebar_position: 4
sidebar_label: Architecture
---

<head>
  <title>Architecture | Regal</title>
</head>

# Architecture

Or "How does Regal work?"

As you might have [read](https://www.styra.com/blog/guarding-the-guardrails-introducing-regal-the-rego-linter/), Regal
[uses Rego for linting Rego](https://www.styra.com/blog/linting-rego-with-rego/) — or rather, Rego policies turned into
a JSON representation of their abstract syntax tree (AST).

## High-level Overview

When running Regal against a directory, like `regal lint my-policies/`, Regal does the following:

- For each source file provided for linting, Regal parses the Rego into its AST representation. This AST representation
  is then turned into JSON and provided as the **input** variable to the linter rules.
- Each linter rule (and there are almost 40 of them at the time of writing this) uses the **input**, which contains
  information such as the package name, what imports are used, and all the rules and the expressions they contain, to
  determine whether the Rego policy linted contains any violations against the rule. An example could be a rule that
  [forbids shadowing](https://www.openpolicyagent.org/projects/regal/rules/bugs/rule-shadows-builtin)
  (i.e. using the same name as) built-in functions and operators.
- Since rule bodies aren’t necessarily flat, but may contain nested bodies of constructs such as
  [comprehensions](https://www.openpolicyagent.org/docs/policy-language/#comprehensions) or
  [every](https://www.openpolicyagent.org/docs/policy-language/#every-keyword) blocks, many linter rules need to
  traverse all expressions in order to find what they are looking for. This is normally done with the help of the
  built-in [walk](https://www.openpolicyagent.org/docs/policy-reference/#graph) function.
- Traversing huge AST structures — and some policies contain millions of AST nodes! — takes time. This isn’t noticeable
  when linting a single file, but for some of the largest policy repositories out there, with several thousands of
  policy files and tests, the cost may be prohibitive. To alleviate this, Regal is implemented to process files
  concurrently to minimize the impact of IO bound tasks, and to make use of multiple cores when available.
- The result of linting each file is eventually collected and compiled into a linter report, which is presented to the
  user in one of the available [output formats](https://www.openpolicyagent.org/projects/regal#output-formats).

## Rego Rules Evaluation

The main entrypoint for Rego rule evaluation is unsurprisingly found in
[main.rego](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/main/main.rego), in which we query the `report`
rule from the [Go](https://github.com/open-policy-agent/regal/blob/main/pkg/linter/linter.go) application.

The `report` rule in turn uses
[dynamic policy composition](https://www.styra.com/blog/dynamic-policy-composition-for-opa/) to query all rules named
`report` under `data.regal.rules[category][title]` for built-in rules, and `data.custom.regal.rules[category][title]`
for custom rules. The violations reported from each rule is added to the `report` set and sent back to the application,
which will compile a final report and present it to the user.
