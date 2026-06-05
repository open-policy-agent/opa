<!-- markdownlint-disable MD041 -->

Tags are often grouped as key/value pairs, but a downstream check needs them
as one flat list. This example takes resource tags collected as nested
`[key, value]` pairs and turns them into a single array of values so the policy
can scan every tag in one pass.
