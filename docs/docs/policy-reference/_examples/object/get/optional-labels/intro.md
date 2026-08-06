<!-- markdownlint-disable MD041 -->

Kubernetes objects and API payloads often omit optional fields. `object.get`
reads a key (or a nested path) and returns a default when it is missing, so
the rest of the policy does not need extra existence checks. Using a path
array also covers the case where an intermediate field like `labels` is
undefined.

Here, workloads without an `env` label are treated as `dev`.
