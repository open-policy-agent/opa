---
title: OPAL
subtitle: Open Policy Administration Layer
labels:
  category: updates
  layer: application
inventors:
- permitio
software:
- opal
code:
- https://github.com/permitio/opal
tutorials:
- https://github.com/permitio/opal/tree/master/documentation/docs/tutorials
videos:
- https://www.youtube.com/watch?v=K1Zm2FPfrh8
docs_features:
  rest-api-integration:
    note: |
      OPAL uses the OPA REST API to update the policy and data pushed down
      from the OPAL server.
      See [how this works](https://docs.opal.ac/overview/architecture).
  external-data:
    note: |
      The OPAL Client uses the OPA REST API to update the state pushed down
      from the OPAL server.
      See [how this works](https://docs.opal.ac/overview/architecture).
  external-data-realtime-push:
    note: |
      OPAL is able to deliver real-time data updates to OPA instances.
      See
      [how this works](https://docs.opal.ac/getting-started/quickstart/opal-playground/publishing-data-update)
      in the OPAL docs.
---
OPAL is an administration layer for Open Policy Agent (OPA), detecting changes in realtime to both policy and policy data and pushing live updates to your agents.
OPAL brings open-policy up to the speed needed by live applications. As your application state changes (whether it's via your APIs, DBs, git, S3 or 3rd-party SaaS services), OPAL will make sure your services are always in sync with the authorization data and policy they need (and only those they need).

