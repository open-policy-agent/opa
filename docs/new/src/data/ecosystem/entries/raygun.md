---
title: raygun
subtitle: Black-box Automated Testing for Rego
labels:
  category: tooling
  layer: shell
inventors:
- paclabsnet
code:
- https://github.com/paclabsnet/raygun
tutorials:
- https://github.com/paclabsnet/raygun/blob/main/README.md
blogs:
- https://paclabs.substack.com/p/raygun-automated-testing-for-rego
docs_features:
  policy-testing:
    note: |
      [Raygun](https://github.com/paclabsnet/raygun) 
      makes it easier to test Rego code in a "real-world" facsimile. 
---

A command-line tool for "black-box" automated testing of Rego. Raygun uses 
OPA as a client, instead of as the test driver. You create YAML
test suites that specify the location of the bundle, the policy path, 
the input JSON and the expected response, and Raygun starts up an OPA 
process with that bundle, POSTs the input JSON to OPA, checks substrings
of the response against your expectations and reports on the results. 
Easy to integrate into existing build chains. Keeps the tests separate from
the policy source code.

