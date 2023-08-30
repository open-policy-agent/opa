---
title: Regal
subtitle: The Linter of Rego Language
labels:
  category: tooling
  layer: shell
inventors:
- styra
blogs:
- https://www.styra.com/blog/guarding-the-guardrails-introducing-regal-the-rego-linter/
code:
- https://github.com/StyraInc/regal
videos:
- https://www.youtube.com/live/Xx8npd2TQJ0?feature=share&t=2567
tutorials:
- https://docs.styra.com/regal#try-it-out
docs_features:
  learning-rego:
    note: |
      Regal can automatically check for common Rego mistakes as you code.
      Each violation is accompanied by a detailed explanation which can be a
      great learning tool for new Rego users. See the
      [Supported Rules](https://github.com/StyraInc/regal/tree/main#rules).
  go-integration:
    note: |
      Regal is built using the Go Rego API. Regal evaluates linting rules
      defined in Rego against the Rego AST of policy files. The
      [linter package](https://github.com/StyraInc/regal/blob/main/pkg/linter/linter.go)
      is a good place to see how OPA is used in the project.
  policy-testing:
    note: |
      Regal is a useful step to use when testing Rego policies to ensure
      code is correct and free of common errors. See
      [the README](https://github.com/StyraInc/regal#try-it-out)
      to get started.
---
Regal is a linter for Rego, with the goal of making your Rego magnificent!

Regal can:
* Identify common mistakes, bugs and inefficiencies in Rego policies, and suggest better approaches
* Provide advice on best practices, coding style, and tooling
* Allow users, teams and organizations to enforce custom rules on their policy code
