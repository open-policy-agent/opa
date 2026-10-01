---
title: Rekor transparency log monitoring and alerting
labels:
  category: security
  layer: application
software:
- rekor
inventors:
- sigstore
code:
- https://github.com/nsmith5/rekor-sidekick
---

Rekor Sidekick monitors a Rekor signature transparency log and forwards events of interest where ever you like.
Alert policies written in Rego determine if an event is of interest.
