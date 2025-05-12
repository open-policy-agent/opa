---
title: Bottle Application Authorization
labels:
  layer: network
  category: application
inventors:
- dolevf
software:
- bottle
code:
- https://github.com/dolevf/bottle-acl-openpolicyagent
blogs:
- https://blog.lethalbit.com/open-policy-agent-for-bottle-web-framework/
docs_features:
  rest-api-integration:
    note: |
      This sample python application calls has a middleware to call OPA
      before processing each request. See the
      [example code](https://github.com/dolevf/bottle-acl-openpolicyagent/blob/00a4336/main.py#L37).
---
This integration demonstrates using Open Policy Agent to perform API authorization for a Python application backed by Bottle.
Bottle is a fast, simple and lightweight WSGI micro web-framework for Python.

