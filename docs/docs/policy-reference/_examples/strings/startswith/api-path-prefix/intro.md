<!-- markdownlint-disable MD041 -->

When a policy only cares about the start of a string — for example an HTTP
path or a registry prefix — `startswith` is clearer (and usually safer)
than a loose `contains` check.

This example allows only paths under `/api/v1/`.
