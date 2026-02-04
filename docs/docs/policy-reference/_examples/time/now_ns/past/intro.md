<!-- markdownlint-disable MD041 -->

In this example, we see compare an
[RFC3339](https://datatracker.ietf.org/doc/html/rfc3339) timestamp
with the current time to determine if the timestamp is in the past.

If you have a time in a different format, you might want to use
[`time.parse_ns`](https://www.openpolicyagent.org/docs/policy-reference/#builtin-time-timeparse_ns)
function to convert it to nanoseconds before comparing it with the current time.

Observe that in Rego, comparing time can be done using the `<` and `>` operators
just like comparing numbers and times in many other languages.
