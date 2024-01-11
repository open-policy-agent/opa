---
title: rego-test-assertions
subtitle: Helper functions for unit testing Rego
labels:
  category: library
inventors:
- anderseknert
code:
- https://github.com/anderseknert/rego-test-assertions
tutorials:
- https://github.com/anderseknert/rego-test-assertions/blob/main/README.md
docs_features:
  policy-testing:
    note: |
      The [rego-test-assertions](https://github.com/anderseknert/rego-test-assertions)
      library contains various assertion functions, which will print the
      expected result vs. the outcome to the console on failure.
  debugging-rego:
    note: |
      The [rego-test-assertions](https://github.com/anderseknert/rego-test-assertions)
      library is designed to make debugging Rego tests easier.
---

A Rego library providing helper functions for unit testing. The library
primarily contains various assertion functions, which will print the expected
result vs. the outcome to the console on failure. This allows you to quickly
grasp what went wrong in your unit tests, resulting in a faster test iteration
process!