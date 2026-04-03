<!-- markdownlint-disable MD041 -->

The `units.parse` function is useful for normalizing resource limits that use different unit suffixes.
In this example, we validate that container memory limits don't exceed a specified maximum,
even when the limits are expressed in different formats (Mi, Gi, etc.).
