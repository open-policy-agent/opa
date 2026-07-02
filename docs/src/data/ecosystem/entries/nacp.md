---
title: Nomad Admission Control Proxy
subtitle: NACP
labels:
  type: poweredbyopa
  layer: configuration
inventors:
- mxab
code:
- https://github.com/mxab/nacp
videos:
- https://github.com/mxab/nacp/tree/main/example
tutorials:
- https://github.com/mxab/nacp/tree/main/example
---

NACP is a proxy in front of the Nomad API that allows for mutation and
validation of job data. It intercepts Nomad API calls that include job data
(plan, register, validate) and performs the necessary operations, with
validation available through embedded Rego rules or webhooks.
